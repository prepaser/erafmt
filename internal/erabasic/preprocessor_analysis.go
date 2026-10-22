package erabasic

import (
	"bytes"
	"strconv"
	"strings"
)

const MaxConvergenceWork = 1 << 22

type preprocessorIssueSeverity uint8

const (
	PreprocessorWarning preprocessorIssueSeverity = iota
	preprocessorUnsupported
)

type PreprocessorIssue struct {
	Line, Offset int
	Code         string
	Message      string
	Severity     preprocessorIssueSeverity
	Preserve     bool
}

type PreprocessorContext struct {
	Semantic            *SemanticContext
	Src                 []byte
	Lines               []Line
	Kind                Kind
	Dialect             Dialect
	AllowFullwidthSpace bool
	Base                int
	ResourceLine        int
	ResourceBase        int
	Optional            bool
	InContinuationProxy bool
	State               *State
	Pp                  *[]ppFrame
	Function            *FunctionContext
	Opaque              []bool
	ConvergenceWork     *int
	PpStateUnits        *int
	PreserveFile        *bool
	Resource            func(line, offset int, message string)
	StatementResource   func(line, offset int, message string)
	Issue               func(PreprocessorIssue)
	ProtectLine         func(line int)
	CommentLine         func(line int) bool
}

func (c *PreprocessorContext) currentState() State {
	if c.State == nil {
		return State{}
	}
	return *c.State
}

func (c *PreprocessorContext) currentPP() []ppFrame {
	if c.Pp == nil {
		return nil
	}
	return *c.Pp
}

func (c *PreprocessorContext) Conditional() bool {
	return preprocessorHas(c.currentPP(), "IF")
}

func (c *PreprocessorContext) Skipped() bool {
	return preprocessorHas(c.currentPP(), "SKIP")
}

func preprocessorHas(frames []ppFrame, kind string) bool {
	for _, f := range frames {
		if f.Kind == kind {
			return true
		}
	}
	return false
}

func (c *PreprocessorContext) report(issue PreprocessorIssue) {
	if c.Issue != nil {
		c.Issue(issue)
	}
}

func (c *PreprocessorContext) unsupported(line int, code, message string) {
	if c.PreserveFile != nil {
		*c.PreserveFile = true
	}
	c.report(PreprocessorIssue{Line: line, Offset: c.lineIndentEnd(line), Code: code, Message: message, Severity: preprocessorUnsupported, Preserve: true})
}

func (c *PreprocessorContext) preserve() {
	if c.PreserveFile != nil {
		*c.PreserveFile = true
	}
}

func (c *PreprocessorContext) Directive(i int, head []byte) {
	end := bytes.IndexByte(head, ']')
	if end < 0 {
		c.unsupported(i, "invalid_preprocessor", "unterminated preprocessor directive")
		return
	}
	first, second, fieldCount := PreprocessorFields(head, end, c.AllowFullwidthSpace)
	if fieldCount == 0 {
		c.unsupported(i, "invalid_preprocessor", "empty preprocessor directive")
		return
	}
	name := string(first)
	expectedFields := 1
	if name == "IF" || name == "ELSEIF" {
		expectedFields = 2
	}
	conditionValid := true
	if name == "IF" || name == "ELSEIF" {
		conditionValid = fieldCount == expectedFields && preprocessorConditionToken(second)
	}
	if fieldCount != expectedFields {
		c.preserve()
		c.report(PreprocessorIssue{Line: i, Offset: c.lineIndentEnd(i), Code: "unsupported_structure", Message: "preprocessor arguments cannot be resolved", Severity: preprocessorUnsupported, Preserve: true})
		if name != "IF" && name != "ELSEIF" {
			return
		}
	}
	if (name == "IF" || name == "ELSEIF") && !conditionValid && fieldCount == expectedFields {
		c.preserve()
		c.report(PreprocessorIssue{Line: i, Offset: c.lineIndentEnd(i), Code: "unsupported_structure", Message: "preprocessor arguments cannot be resolved", Severity: preprocessorUnsupported, Preserve: true})
	}
	tailStart := TrimInputLeft(head, end+1, len(head), c.AllowFullwidthSpace)
	tail := head[tailStart:]
	if len(tail) != 0 && (tail[0] != ';' || ExecutableMarker(tail, 0) || bytes.Contains(tail, []byte("[["))) {
		c.preserve()
		c.report(PreprocessorIssue{Line: i, Offset: c.lineIndentEnd(i), Code: "unsupported_structure", Message: "preprocessor directive has a non-comment suffix", Severity: preprocessorUnsupported, Preserve: true})
	}
	pp := c.currentPP()
	s := c.currentState()
	switch name {
	case "SKIPSTART":
		if len(pp) >= MaxPreprocessorDepth {
			c.statementResourceProblem(i, c.lineIndentEnd(i), "preprocessor nesting exceeds the formatter resource limit")
			return
		}
		if c.Skipped() {
			c.unsupported(i, "invalid_structure", "nested SKIPSTART")
			return
		}
		pp = append(pp, ppFrame{Kind: "SKIP", Line: i + 1})
		*c.Pp = pp
	case "IF", "IF_DEBUG", "IF_NDEBUG":
		if len(pp) >= MaxPreprocessorDepth {
			c.statementResourceProblem(i, c.lineIndentEnd(i), "preprocessor nesting exceeds the formatter resource limit")
			return
		}
		f := ppFrame{Kind: "IF", Line: i + 1, Malformed: name == "IF" && !conditionValid}
		if !c.retainPreprocessorState(i, &f, s) {
			return
		}
		f.Entry = s.Clone()
		pp = append(pp, f)
		*c.Pp = pp
	case "ELSE", "ELSEIF":
		if len(pp) == 0 || pp[len(pp)-1].Kind != "IF" {
			c.unsupported(i, "invalid_structure", name+" has no matching preprocessor IF")
			return
		}
		f := &pp[len(pp)-1]
		if f.HasElse {
			c.unsupported(i, "invalid_structure", name+" follows preprocessor ELSE")
			return
		}
		if !c.retainPreprocessorState(i, f, s) {
			return
		}
		f.Branches = append(f.Branches, s.Clone())
		f.HasElse = name == "ELSE"
		f.Malformed = f.Malformed || name == "ELSEIF" && !conditionValid
		s = f.Entry.Clone()
		*c.State = s
	case "SKIPEND", "ENDIF":
		expected := "IF"
		if name == "SKIPEND" {
			expected = "SKIP"
		}
		if len(pp) == 0 || pp[len(pp)-1].Kind != expected {
			c.unsupported(i, "invalid_structure", name+" has no matching preprocessor opener")
			return
		}
		f := pp[len(pp)-1]
		if f.Kind == "IF" && !c.retainPreprocessorState(i, &f, s) {
			return
		}
		if c.PpStateUnits != nil {
			*c.PpStateUnits -= f.StateUnits
		}
		pp = pp[:len(pp)-1]
		*c.Pp = pp
		if f.Kind == "SKIP" || c.Skipped() {
			return
		}
		if f.Malformed {
			*c.State = f.Entry
			return
		}
		f.Branches = append(f.Branches, s.Clone())
		if !f.HasElse {
			f.Branches = append(f.Branches, f.Entry)
		}
		for _, branch := range f.Branches {
			if !branch.equal(f.Branches[0]) {
				if c.BranchesConverge(i, f.Branches) {
					break
				}
				c.preserve()
				c.report(PreprocessorIssue{Line: i, Offset: c.lineIndentEnd(i), Code: "unsupported_structure", Message: "preprocessor branches leave different block or SIF states", Severity: preprocessorUnsupported, Preserve: true})
				*c.State = f.Entry
				return
			}
		}
		merged := mergeBranchState(f.Branches)
		if merged.Sif != 0 && !merged.SifOriginKnown {
			merged.SifOrigin = i + 1
		}
		for _, branch := range f.Branches[1:] {
			merged.Declarations = merged.Declarations && branch.Declarations
		}
		*c.State = merged
	default:
		if c.Base == 0 || len(pp) != 0 {
			c.preserve()
		}
		c.report(PreprocessorIssue{Line: i, Offset: c.lineIndentEnd(i), Code: "unsupported_structure", Message: "unknown preprocessor directive " + name, Severity: preprocessorUnsupported, Preserve: c.Base == 0 || len(pp) != 0})
	}
}

func (c *PreprocessorContext) retainPreprocessorState(i int, f *ppFrame, s State) bool {
	units := 1 + len(s.Frames)
	if c.PpStateUnits != nil {
		current := *c.PpStateUnits
		if !appendWithinLimit(current, units, MaxPreprocessorStates) {
			c.statementResourceProblem(i, c.lineIndentEnd(i), "preprocessor state work exceeds the formatter resource limit")
			return false
		}
		*c.PpStateUnits += units
	}
	f.StateUnits += units
	return true
}

func (c *PreprocessorContext) statementResourceProblem(i, offset int, message string) {
	if c.StatementResource != nil {
		c.StatementResource(i, offset, message)
		return
	}
	c.resourceProblem(i, offset, message)
}

func (c *PreprocessorContext) resourceProblem(i, offset int, message string) {
	if c.Resource != nil {
		c.Resource(i, offset, message)
	}
}

func (c *PreprocessorContext) lineIndentEnd(i int) int {
	if i < 0 || i >= len(c.Lines) {
		return 0
	}
	return c.Lines[i].IndentEnd
}

func (c *PreprocessorContext) ConsumeConvergenceWork(states []State, line int) bool {
	added := len(states)
	for _, current := range states {
		added += len(current.Frames)
	}
	current := 0
	if c.ConvergenceWork != nil {
		current = *c.ConvergenceWork
	}
	if !appendWithinLimit(current, added, MaxConvergenceWork) {
		c.resourceProblem(line, c.lineIndentEnd(line), "preprocessor convergence work exceeds the formatter resource limit")
		return false
	}
	if c.ConvergenceWork != nil {
		*c.ConvergenceWork += added
	}
	return true
}

func (c *PreprocessorContext) BranchesConverge(i int, branches []State) bool {
	if i+1 < len(c.Lines) && !c.ConsumeConvergenceWork(branches, i+1) {
		return false
	}
	if c.branchesCloseTogether(i, branches) {
		return true
	}
	if i+1 >= len(c.Lines) {
		return false
	}
	states := make([]State, 0, len(branches))
	seen := make(map[string]struct{}, len(branches))
	for _, branch := range branches {
		key := convergenceStateKey(branch)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		states = append(states, branch.Clone())
	}
	advanced := false
	for lineNo := i + 1; lineNo < len(c.Lines); lineNo++ {
		if c.ordinaryBlank(lineNo) {
			if !c.ConsumeConvergenceWork(states, lineNo) {
				return false
			}
			advanced = true
			continue
		}
		if c.isOrdinaryComment(lineNo) {
			if !c.ConsumeConvergenceWork(states, lineNo) || !sameBranchDepth(states) {
				return false
			}
			advanced = true
			continue
		}
		line := c.Lines[lineNo]
		head, prefix, _, marked := c.headOf(c.Src[line.Start:line.BodyEnd])
		if marked || len(head) == 0 || bytes.Contains(head, []byte("[[")) {
			return false
		}
		if !c.ConsumeConvergenceWork(states, lineNo) {
			return false
		}
		command := c.HeadRoleAt(lineNo, line.Start+prefix, head)
		if command == "" {
			var wordEnd int
			command, wordEnd = c.commandOf(head)
			if command == "" || wordEnd == 0 || string(head[:wordEnd]) != command {
				return false
			}
			if start, _, ok := c.AssignmentBoundaryAt(lineNo, line.Start+prefix, head); ok {
				if _, variable := c.variableLHS(head[:start]); variable {
					return false
				}
			}
		}
		if command == "ENDIF" || command == "ENDSELECT" {
			if advanced && !ifBranchMetadataOnly(branches) || advanced && !sameBranchDepth(states) {
				return false
			}
			if !c.ConsumeConvergenceWork(states, lineNo) {
				return false
			}
			return closedBranchesMatch(command, lineNo+1, states)
		}
		if Partial(command) || CommandUnavailable(command, c.Dialect) || !knownNonstructural(command) || !sameBranchDepth(states) {
			return false
		}
		for n := range states {
			if states[n].Sif != 0 {
				return false
			}
			if _, message := states[n].Instruction(command, lineNo+1); message != "" {
				return false
			}
		}
		advanced = true
	}
	return false
}

func (c *PreprocessorContext) branchesCloseTogether(i int, branches []State) bool {
	if i+1 >= len(c.Lines) {
		return false
	}
	l := c.Lines[i+1]
	head, _, _, marked := c.headOf(c.Src[l.Start:l.BodyEnd])
	command, wordEnd := c.commandOf(head)
	if marked || bytes.Contains(head, []byte("[[")) || string(head[:wordEnd]) != command || command != "ENDIF" && command != "ENDSELECT" {
		return false
	}
	return closedBranchesMatch(command, i+2, branches)
}

func (c *PreprocessorContext) ordinaryBlank(i int) bool {
	if i < 0 || i >= len(c.Lines) || c.lineOpaque(i) {
		return false
	}
	l := c.Lines[i]
	return len(TrimInput(c.Src[l.Start:l.BodyEnd], c.AllowFullwidthSpace)) == 0
}

func (c *PreprocessorContext) isOrdinaryComment(i int) bool {
	return c.CommentLine != nil && c.CommentLine(i)
}

func (c *PreprocessorContext) lineOpaque(i int) bool {
	return i >= 0 && i < len(c.Opaque) && c.Opaque[i]
}

func (c *PreprocessorContext) headOf(body []byte) ([]byte, int, bool, bool) {
	return HeadOfWithSpaceDialect(body, c.AllowFullwidthSpace, c.Dialect)
}

func (c *PreprocessorContext) commandOf(head []byte) (string, int) {
	return CommandOfWithSpace(head, c.AllowFullwidthSpace)
}

func (c *PreprocessorContext) HeadRole(head []byte) string {
	if len(head) == 0 || bytes.Contains(head, []byte("[[")) {
		return ""
	}
	if command := c.RawHeadRole(head); command != "" {
		return command
	}
	if command := c.PrintVHeadCommand(head); command != "" {
		return command
	}
	if start, _, ok := c.assignmentBoundary(head); ok {
		if name, variable := c.variableLHS(head[:start]); variable && (!ReservedInstruction(name) && !methodDialect(builtinMethodMasks[name], c.Dialect) || c.UnavailableAssignment(head, name)) {
			return "SET"
		}
		return ""
	}
	session := c.scanSession(head)
	end, _ := SyntaxCommentEnd(head, 0, len(head), session)
	end = TrimInputRight(head, 0, end, c.AllowFullwidthSpace)
	if end < 2 {
		return ""
	}
	start := 0
	if bytes.Equal(head[:2], []byte("++")) || bytes.Equal(head[:2], []byte("--")) {
		start = 2
	} else if bytes.Equal(head[end-2:end], []byte("++")) || bytes.Equal(head[end-2:end], []byte("--")) {
		end -= 2
	} else {
		return ""
	}
	if c.assignmentLHS(head[start:end]) {
		return "SET"
	}
	return ""
}

func (c *PreprocessorContext) PrintVHeadCommand(head []byte) string {
	command, wordEnd := c.commandOf(head)
	if command == "" || string(head[:wordEnd]) != command {
		return ""
	}
	if PrintVCommand(command) {
		return command
	}
	spec, ok := registeredCommandSpecs[command]
	if ok && spec.Mode == RegisteredPrintV && registeredDialect(spec.Dialects, c.Dialect) {
		return command
	}
	return ""
}

func (c *PreprocessorContext) RawHeadRole(head []byte) string {
	command, wordEnd := c.commandOf(head)
	if command == "" || string(head[:wordEnd]) != command || !c.RawPayloadCommand(command) || CommandUnavailable(command, c.Dialect) {
		return ""
	}
	return command
}

func (c *PreprocessorContext) RawPayloadCommand(command string) bool {
	if command == "DEBUGPRINTFORM" || command == "DEBUGPRINTFORML" {
		return true
	}
	if RawCommand(command) && command != "TIMES" {
		return true
	}
	spec, ok := registeredCommandSpecs[command]
	return ok && spec.Mode == RegisteredRaw
}

func (c *PreprocessorContext) HeadRoleAt(line, base int, head []byte) string {
	previousLine, previousBase := c.ResourceLine, c.ResourceBase
	c.ResourceLine, c.ResourceBase = line, base
	defer func() {
		c.ResourceLine, c.ResourceBase = previousLine, previousBase
	}()
	return c.HeadRole(head)
}

func (c *PreprocessorContext) AssignmentBoundaryAt(line, base int, head []byte) (int, AssignmentOp, bool) {
	previousLine, previousBase := c.ResourceLine, c.ResourceBase
	c.ResourceLine, c.ResourceBase = line, base
	defer func() {
		c.ResourceLine, c.ResourceBase = previousLine, previousBase
	}()
	return c.assignmentBoundary(head)
}

func (c *PreprocessorContext) assignmentBoundary(head []byte) (int, AssignmentOp, bool) {
	return AssignmentBoundary(head, c.scanSession(head))
}

func (c *PreprocessorContext) assignmentLHS(src []byte) bool {
	name, ok := c.variableLHS(src)
	return ok && !ReservedInstruction(name) && !methodDialect(builtinMethodMasks[name], c.Dialect)
}

func (c *PreprocessorContext) variableLHS(src []byte) (string, bool) {
	src = TrimInput(src, c.AllowFullwidthSpace)
	session := c.scanSession(src)
	session.BuiltinTypes = true
	tokens, err := session.LexRange(ExpressionSourceOriginal, 0, len(src), 0)
	return VariableLHSFromTokens(tokens, err)
}

func (c *PreprocessorContext) UnavailableAssignment(head []byte, name string) bool {
	command, wordEnd := c.commandOf(head)
	if command != name || wordEnd == 0 || !CommandUnavailable(command, c.Dialect) {
		return false
	}
	if ReservedBuiltinMethod(name) {
		_, ok := c.UnavailableMethodAssignment(head, command)
		return ok
	}
	if !EmeeOnlyInstruction(command) {
		return false
	}
	start, _, ok := c.assignmentBoundary(head)
	if !ok {
		return false
	}
	variable, isVariable := c.variableLHS(head[:start])
	if !isVariable || variable != command {
		return false
	}
	return ok
}

func (c *PreprocessorContext) UnavailableMethodAssignment(head []byte, command string) (AssignmentOp, bool) {
	mask, ok := builtinMethodMasks[command]
	if !ok || methodDialect(mask, c.Dialect) || ReservedInstruction(command) {
		return AssignmentOp{}, false
	}
	start, op, ok := c.assignmentBoundary(head)
	if !ok {
		return AssignmentOp{}, false
	}
	name, variable := c.variableLHS(head[:start])
	if !variable || name != command {
		return AssignmentOp{}, false
	}
	return op, true
}

func (c *PreprocessorContext) scanSession(src []byte) *ExpressionSession {
	if c.Semantic != nil {
		s := c.Semantic.ScanSession(src, 0, len(src))
		s.BuiltinTypes = false
		s.OnResource = func(offset int, message string) {
			c.resourceProblem(c.ResourceLine, c.ResourceBase+offset, message)
		}
		return s
	}
	s := NewExpressionSessionWithSpace(src, c.AllowFullwidthSpace)
	s.BuiltinTypes = true
	s.Dialect = c.Dialect
	return s
}

func convergenceStateKey(s State) string {
	var b strings.Builder
	if s.Sif != 0 {
		b.WriteByte('1')
	} else {
		b.WriteByte('0')
	}
	b.WriteString(strconv.Itoa(len(s.Frames)))
	b.WriteByte('|')
	for _, f := range s.Frames {
		b.WriteString(strconv.Itoa(len(f.Kind)))
		b.WriteByte(':')
		b.WriteString(f.Kind)
		if f.Branch {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
		if f.LastBranch {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	return b.String()
}
