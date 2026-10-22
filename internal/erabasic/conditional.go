package erabasic

import "bytes"

func (p *PreprocessorContext) OptionalSIFPair(first int, head []byte, prefix int) bool {
	if first < 0 || first+1 >= len(p.Lines) || p.State == nil || p.State.Sif != 0 {
		return false
	}
	firstHead, firstPrefix, ok := debugMarkerLineWithSpace(p.Src, p.Lines[first], p.AllowFullwidthSpace)
	if !ok || firstPrefix != prefix || !bytes.Equal(firstHead, head) {
		return false
	}
	if !bytes.HasPrefix(firstHead, []byte("SIF")) || len(firstHead) == 3 || !InputSpaceAt(firstHead, 3, p.AllowFullwidthSpace) {
		return false
	}
	firstCommand, firstWordEnd := p.commandOf(firstHead)
	if firstCommand != "SIF" || firstWordEnd != 3 {
		return false
	}
	conditionStart := firstWordEnd + InputSpaceSize(firstHead, firstWordEnd, p.AllowFullwidthSpace)
	condition := TrimInput(firstHead[conditionStart:], p.AllowFullwidthSpace)
	if len(condition) == 0 || condition[0] == ';' || bytes.Contains(condition, []byte("[[")) {
		return false
	}
	targetHead, targetPrefix, ok := debugMarkerLineWithSpace(p.Src, p.Lines[first+1], p.AllowFullwidthSpace)
	if !ok || bytes.Contains(targetHead, []byte("[[")) {
		return false
	}
	target := p.HeadRoleAt(first+1, p.Lines[first+1].Start+targetPrefix, targetHead)
	if target == "" {
		if start, _, assignment := p.AssignmentBoundaryAt(first+1, p.Lines[first+1].Start+targetPrefix, targetHead); assignment {
			if _, variable := p.variableLHS(targetHead[:start]); variable {
				return false
			}
		}
		var targetWordEnd int
		target, targetWordEnd = p.commandOf(targetHead)
		if target == "" || string(targetHead[:targetWordEnd]) != target || target == "SIF" || Partial(target) || CommandUnavailable(target, p.Dialect) {
			return false
		}
		known, available := commandAvailability(target, p.Dialect)
		if (!known && !pairInstruction(target)) || (known && !available) {
			return false
		}
	}
	state := p.State.Clone()
	if _, message := state.Instruction("SIF", first+1); message != "" {
		return false
	}
	if _, message := state.Instruction(target, first+2); message != "" || !state.equal(*p.State) {
		return false
	}
	if p.ProtectLine != nil {
		p.ProtectLine(first)
		p.ProtectLine(first + 1)
	}
	p.State.Declarations = false
	return true
}

func ifBranchMetadataOnly(states []State) bool {
	if len(states) == 0 {
		return false
	}
	base := states[0]
	for _, current := range states[1:] {
		if (current.Sif != 0) != (base.Sif != 0) || len(current.Frames) != len(base.Frames) {
			return false
		}
		for i, frame := range current.Frames {
			want := base.Frames[i]
			if frame.Kind != want.Kind || frame.Branch != want.Branch {
				return false
			}
			if frame.LastBranch != want.LastBranch && frame.Kind != "IF" {
				return false
			}
		}
	}
	return true
}

func sameBranchDepth(states []State) bool {
	if len(states) < 2 {
		return true
	}
	want := states[0].Depth()
	for _, current := range states[1:] {
		if current.Depth() != want {
			return false
		}
	}
	return true
}

func knownNonstructural(command string) bool {
	if SyntaxCommand(command) || RawCommand(command) || ReservedInstruction(command) || ReservedBuiltinMethod(command) {
		return true
	}
	known, _ := commandAvailability(command, CommonDialect)
	return known
}

func pairInstruction(command string) bool {
	if SyntaxCommand(command) || RawCommand(command) {
		return true
	}
	switch command {
	case "PRINT_IMG", "PRINT_RECT", "PRINT_SPACE", "SPLIT", "SWAP":
		return true
	default:
		return false
	}
}

func debugMarkerLineWithSpace(src []byte, l Line, allowFullwidthSpace bool) ([]byte, int, bool) {
	body := src[l.Start:l.BodyEnd]
	head, prefix, _, marked := headOfWithSpace(body, allowFullwidthSpace)
	if !marked || len(head) == 0 || !bytes.Equal(TrimInput(body[:prefix], allowFullwidthSpace), []byte(";#;")) {
		return nil, 0, false
	}
	return head, prefix, true
}

func closedBranchesMatch(command string, number int, branches []State) bool {
	var want State
	var depth int
	for n, branch := range branches {
		if branch.Sif != 0 {
			return false
		}
		closed := branch.Clone()
		gotDepth, message := closed.Instruction(command, number)
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
