package format

import (
	"bytes"
	"strconv"
	"strings"
)

const maxConvergenceWork = 1 << 22

func (p *formatter) optionalSIFPair(first int, head []byte, prefix int) bool {
	if first < 0 || first+1 >= len(p.lines) || p.state.sif != 0 {
		return false
	}

	firstHead, firstPrefix, ok := debugMarkerLineWithSpace(p.src, p.lines[first], p.allowFullwidthSpace)
	if !ok || firstPrefix != prefix || !bytes.Equal(firstHead, head) {
		return false
	}
	if !bytes.HasPrefix(firstHead, []byte("SIF")) || len(firstHead) == 3 || !inputSpaceAt(firstHead, 3, p.allowFullwidthSpace) {
		return false
	}
	firstCommand, firstWordEnd := p.commandOf(firstHead)
	if firstCommand != "SIF" || firstWordEnd != 3 {
		return false
	}
	conditionStart := firstWordEnd + inputSpaceSize(firstHead, firstWordEnd, p.allowFullwidthSpace)
	condition := firstHead[conditionStart:]
	condition = trimInput(condition, p.allowFullwidthSpace)
	if len(condition) == 0 || condition[0] == ';' || bytes.Contains(condition, []byte("[[")) {
		return false
	}

	targetHead, targetPrefix, ok := debugMarkerLineWithSpace(p.src, p.lines[first+1], p.allowFullwidthSpace)
	if !ok || bytes.Contains(targetHead, []byte("[[")) {
		return false
	}
	target := p.headRoleAt(first+1, p.lines[first+1].start+targetPrefix, targetHead)
	if target == "" {
		if start, _, assignment := p.assignmentBoundaryAt(first+1, p.lines[first+1].start+targetPrefix, targetHead); assignment {
			if _, variable := p.variableLHS(targetHead[:start]); variable {
				return false
			}
		}
		var targetWordEnd int
		target, targetWordEnd = p.commandOf(targetHead)
		if target == "" || string(targetHead[:targetWordEnd]) != target || target == "SIF" || partial(target) || commandUnavailable(target, p.dialect) {
			return false
		}
		known, available := commandAvailability(target, p.dialect)
		if (!known && !pairInstruction(target)) || (known && !available) {
			return false
		}
	}

	state := p.state.clone()
	if _, message := state.instruction("SIF", first+1); message != "" {
		return false
	}
	if _, message := state.instruction(target, first+2); message != "" || !state.equal(p.state) {
		return false
	}

	p.opaque[first] = true
	p.opaque[first+1] = true
	p.state.declarations = false
	return true
}

func (p *formatter) branchesConverge(i int, branches []state) bool {
	if i+1 < len(p.lines) && !p.consumeConvergenceWork(branches, i+1) {
		return false
	}
	if p.branchesCloseTogether(i, branches) {
		return true
	}
	if i+1 >= len(p.lines) {
		return false
	}

	states := make([]state, 0, len(branches))
	seen := make(map[string]struct{}, len(branches))
	for _, branch := range branches {
		key := convergenceStateKey(branch)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		states = append(states, branch.clone())
	}
	advanced := false
	for lineNo := i + 1; lineNo < len(p.lines); lineNo++ {
		if p.ordinaryBlank(lineNo) {
			if !p.consumeConvergenceWork(states, lineNo) {
				return false
			}
			advanced = true
			continue
		}
		if p.isOrdinaryComment(lineNo) {
			if !p.consumeConvergenceWork(states, lineNo) {
				return false
			}
			if !sameBranchDepth(states) {
				return false
			}
			advanced = true
			continue
		}

		line := p.lines[lineNo]
		head, prefix, _, marked := p.headOf(p.src[line.start:line.bodyEnd])
		if marked || len(head) == 0 || bytes.Contains(head, []byte("[[")) {
			return false
		}
		if !p.consumeConvergenceWork(states, lineNo) {
			return false
		}
		command := p.headRoleAt(lineNo, line.start+prefix, head)
		if command == "" {
			var wordEnd int
			command, wordEnd = p.commandOf(head)
			if command == "" || wordEnd == 0 || string(head[:wordEnd]) != command {
				return false
			}
			if start, _, ok := p.assignmentBoundaryAt(lineNo, line.start+prefix, head); ok {
				if _, variable := p.variableLHS(head[:start]); variable {
					return false
				}
			}
		}
		if command == "ENDIF" || command == "ENDSELECT" {
			if advanced && !ifBranchMetadataOnly(branches) {
				return false
			}
			if advanced && !sameBranchDepth(states) {
				return false
			}
			if !p.consumeConvergenceWork(states, lineNo) {
				return false
			}
			return closedBranchesMatch(command, lineNo+1, states)
		}
		if partial(command) || commandUnavailable(command, p.dialect) || !knownNonstructural(command) {
			return false
		}
		if !sameBranchDepth(states) {
			return false
		}
		for n := range states {
			if states[n].sif != 0 {
				return false
			}
			if _, message := states[n].instruction(command, lineNo+1); message != "" {
				return false
			}
		}
		advanced = true
	}
	return false
}

func ifBranchMetadataOnly(states []state) bool {
	if len(states) == 0 {
		return false
	}
	base := states[0]
	for _, current := range states[1:] {
		if (current.sif != 0) != (base.sif != 0) || len(current.frames) != len(base.frames) {
			return false
		}
		for i, frame := range current.frames {
			want := base.frames[i]
			if frame.kind != want.kind || frame.branch != want.branch {
				return false
			}
			if frame.lastBranch != want.lastBranch && frame.kind != "IF" {
				return false
			}
		}
	}
	return true
}

func sameBranchDepth(states []state) bool {
	if len(states) < 2 {
		return true
	}
	want := states[0].depth()
	for _, current := range states[1:] {
		if current.depth() != want {
			return false
		}
	}
	return true
}

func convergenceStateKey(s state) string {
	var b strings.Builder
	if s.sif != 0 {
		b.WriteByte('1')
	} else {
		b.WriteByte('0')
	}
	b.WriteString(strconv.Itoa(len(s.frames)))
	b.WriteByte('|')
	for _, f := range s.frames {
		b.WriteString(strconv.Itoa(len(f.kind)))
		b.WriteByte(':')
		b.WriteString(f.kind)
		if f.branch {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
		if f.lastBranch {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	return b.String()
}

func (p *formatter) consumeConvergenceWork(states []state, line int) bool {
	added := len(states)
	for _, current := range states {
		added += len(current.frames)
	}
	if !appendWithinLimit(p.convergenceWork, added, maxConvergenceWork) {
		p.resourceProblem(line, p.lines[line].indentEnd, "preprocessor convergence work exceeds the formatter resource limit")
		return false
	}
	p.convergenceWork += added
	return true
}

func knownNonstructural(command string) bool {
	if syntaxCommand(command) || rawCommand(command) || reservedInstructions[command] || reservedBuiltinMethod(command) {
		return true
	}
	known, _ := commandAvailability(command, CommonDialect)
	return known
}

func (p *formatter) headRole(head []byte) string {
	if len(head) == 0 || bytes.Contains(head, []byte("[[")) {
		return ""
	}
	if command := p.rawHeadRole(head); command != "" {
		return command
	}
	if command := p.printVHeadCommand(head); command != "" {
		return command
	}
	if start, _, ok := p.assignmentBoundary(head); ok {
		if name, variable := p.variableLHS(head[:start]); variable && (!reservedInstructions[name] && !methodDialect(builtinMethodMasks[name], p.dialect) || p.unavailableAssignment(head, name)) {
			return "SET"
		}
		return ""
	}
	end, _ := p.syntaxCommentEnd(head, 0, len(head))
	end = trimInputRight(head, 0, end, p.allowFullwidthSpace)
	if end < 2 {
		return ""
	}
	start := 0
	if bytes.Equal(head[:2], []byte("++")) || bytes.Equal(head[:2], []byte("--")) {
		start = 2
	} else if bytes.Equal(head[end-2:end], []byte("++")) || bytes.Equal(head[end-2:end], []byte("--")) {
		start = 0
		end -= 2
	} else {
		return ""
	}
	if p.assignmentLHS(head[start:end]) {
		return "SET"
	}
	return ""
}

func (p *formatter) printVHeadCommand(head []byte) string {
	command, wordEnd := p.commandOf(head)
	if command == "" || string(head[:wordEnd]) != command {
		return ""
	}
	if printVCommand(command) {
		return command
	}
	spec, ok := registeredCommandSpecs[command]
	if ok && spec.mode == registeredPrintV && registeredDialect(spec.dialects, p.dialect) {
		return command
	}
	return ""
}

func (p *formatter) headRoleAt(line, base int, head []byte) string {
	parseLine, parseBase, parseSegments := p.parseLine, p.parseBase, p.parseSegments
	p.parseLine, p.parseBase, p.parseSegments = line, base, nil
	role := p.headRole(head)
	p.parseLine, p.parseBase, p.parseSegments = parseLine, parseBase, parseSegments
	return role
}

func (p *formatter) assignmentBoundaryAt(line, base int, head []byte) (int, assignmentOp, bool) {
	parseLine, parseBase, parseSegments := p.parseLine, p.parseBase, p.parseSegments
	p.parseLine, p.parseBase, p.parseSegments = line, base, nil
	start, op, ok := p.assignmentBoundary(head)
	p.parseLine, p.parseBase, p.parseSegments = parseLine, parseBase, parseSegments
	return start, op, ok
}

func (p *formatter) rawHeadRole(head []byte) string {
	command, wordEnd := p.commandOf(head)
	if command == "" || string(head[:wordEnd]) != command || !p.rawPayloadCommand(command) || commandUnavailable(command, p.dialect) {
		return ""
	}
	return command
}

func (p *formatter) rawPayloadCommand(command string) bool {
	if command == "DEBUGPRINTFORM" || command == "DEBUGPRINTFORML" {
		return true
	}
	if rawCommand(command) && command != "TIMES" {
		return true
	}
	spec, ok := registeredCommandSpecs[command]
	return ok && spec.mode == registeredRaw
}

func (p *formatter) unavailableAssignment(head []byte, name string) bool {
	command, wordEnd := p.commandOf(head)
	if command != name || wordEnd == 0 || !commandUnavailable(command, p.dialect) {
		return false
	}
	if reservedBuiltinMethod(name) {
		_, ok := p.unavailableMethodAssignment(head, command)
		return ok
	}
	_, ok := p.unavailableCommandAssignment(head, command)
	return ok
}

func (p *formatter) unavailableMethodAssignment(head []byte, command string) (assignmentOp, bool) {
	mask, ok := builtinMethodMasks[command]
	if !ok || methodDialect(mask, p.dialect) || reservedInstructions[command] {
		return assignmentOp{}, false
	}
	start, op, ok := p.assignmentBoundary(head)
	if !ok {
		return assignmentOp{}, false
	}
	name, variable := p.variableLHS(head[:start])
	if !variable || name != command {
		return assignmentOp{}, false
	}
	return op, true
}

func pairInstruction(command string) bool {
	if syntaxCommand(command) || rawCommand(command) {
		return true
	}
	switch command {
	case "PRINT_IMG", "PRINT_RECT", "PRINT_SPACE", "SPLIT", "SWAP":
		return true
	default:
		return false
	}
}

func debugMarkerLineWithSpace(src []byte, l line, allowFullwidthSpace bool) ([]byte, int, bool) {
	body := src[l.start:l.bodyEnd]
	head, prefix, _, marked := headOfWithSpace(body, allowFullwidthSpace)
	if !marked || len(head) == 0 || !bytes.Equal(trimInput(body[:prefix], allowFullwidthSpace), []byte(";#;")) {
		return nil, 0, false
	}
	return head, prefix, true
}

func closedBranchesMatch(command string, number int, branches []state) bool {
	var want state
	var depth int
	for n, branch := range branches {
		if branch.sif != 0 {
			return false
		}
		closed := branch.clone()
		gotDepth, message := closed.instruction(command, number)
		if message != "" {
			return false
		}
		if n == 0 {
			want, depth = closed, gotDepth
		} else if gotDepth != depth || !closed.equal(want) {
			return false
		}
	}
	return true
}
