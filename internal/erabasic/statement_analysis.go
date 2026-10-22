package erabasic

import (
	"bytes"
	"strings"
)

type StatementAnalysis struct {
	Source              []byte
	Dialect             Dialect
	AllowFullwidthSpace bool
	ExpressionEnabled   bool
	Base                int
	Optional            bool
	Conditional         bool
	Preserved           bool
	State               *State
	Semantic            *SemanticContext
	Structural          *StructuralContext
}

type statementIssue struct {
	Offset  int
	Message string
}

func CallUsesForm(command string) bool {
	return bytes.Contains([]byte(command), []byte("FORM")) || command == "FUNC"
}

func ReturnCommand(command string) bool {
	return command == "RETURN" || command == "RETURNFORM" || command == "RETURNF"
}

func ScalarSurplusCommand(command string) bool {
	return SimpleIntegerCommand(command) && command != "RETURN" || SimpleStringCommand(command) && command != "RETURNF"
}

func (a *StatementAnalysis) expressionSession(start, end int) (*ExpressionSession, int) {
	if a.Semantic != nil {
		return a.Semantic.ExpressionSessionFor(start, end)
	}
	s := NewExpressionSessionWithSpace(a.Source[start:end], a.AllowFullwidthSpace)
	s.BuiltinTypes = true
	s.Dialect = a.Dialect
	return s, 0
}

func (a *StatementAnalysis) sourceSession(src []byte) *ExpressionSession {
	if a.Semantic != nil {
		return a.Semantic.newSourceSession(src)
	}
	session := NewExpressionSessionWithSpace(src, a.AllowFullwidthSpace)
	session.BuiltinTypes = true
	session.Dialect = a.Dialect
	return session
}

func (a *StatementAnalysis) lexRange(start, end int) ([]ExprToken, error) {
	session, offset := a.expressionSession(start, end)
	return session.LexRange(ExpressionSourceOriginal, offset, offset+end-start, start)
}

func (a *StatementAnalysis) split(start, end int, separator byte) []SyntaxSpan {
	if a.Semantic != nil {
		return a.Semantic.SplitTopLevel(a.Source, start, end, separator)
	}
	session := NewExpressionSessionWithSpace(a.Source[start:end], a.AllowFullwidthSpace)
	session.BuiltinTypes = true
	session.Dialect = a.Dialect
	spans := SplitTopLevel(a.Source[start:end], 0, end-start, separator, session)
	for i := range spans {
		spans[i].Start += start
		spans[i].End += start
	}
	return spans
}

type ordinaryExpressionResult struct {
	Valid bool
	Err   error
}

func (a *StatementAnalysis) OrdinaryExpressionProof(start, end int, caseMode bool) ordinaryExpressionResult {
	if !a.ExpressionEnabled || start > end || end > len(a.Source) {
		return ordinaryExpressionResult{Valid: true}
	}
	session, offset := a.expressionSession(start, end)
	bodyEnd, unsupported, err := expressionBodyEndWithSession(session, ExpressionSourceOriginal, a.Source[start:end], offset)
	if err != nil || unsupported {
		return ordinaryExpressionResult{Valid: true}
	}
	tokens, err := session.LexRange(ExpressionSourceOriginal, offset, offset+bodyEnd, 0)
	if err != nil {
		return ordinaryExpressionResult{Valid: true}
	}
	if syntaxExpressionEndsWithHash(tokens) {
		if !ExpressionNeedsDeferral(tokens, a.Dialect) {
			return ordinaryExpressionResult{Err: expressionShapeError(tokens[len(tokens)-1], "expression has an invalid operator")}
		}
		return ordinaryExpressionResult{}
	}
	return ordinaryExpressionResult{Valid: true, Err: ProvenExpressionShape(tokens, a.Dialect, caseMode)}
}

type returnModeAnalysis struct{ Issue *statementIssue }

func (a *StatementAnalysis) ReturnMode(head []byte, base int, command string, wordEnd int) returnModeAnalysis {
	if !ReturnCommand(command) || !a.ExpressionEnabled || a.Optional || a.Conditional || a.Base == 0 || string(head[:wordEnd]) != command || CommandFieldHasMandatoryMarker(head) || CommandFieldHasDeferredMarker(head) {
		return returnModeAnalysis{}
	}
	value := KnownValueUnknown
	if command == "RETURNF" {
		_, start, end, _, hasArg := InstructionBodyWithSpace(head, wordEnd, a.AllowFullwidthSpace)
		if hasArg {
			parts := a.split(base+start, base+end, ',')
			if len(parts) == 1 {
				tokens, err := a.lexRange(base+start, base+end)
				if err == nil && a.Structural != nil {
					value = a.Structural.TrustedReturnExpressionType(tokens)
				}
			}
		}
	}
	if a.Structural == nil {
		return returnModeAnalysis{}
	}
	message := a.Structural.ReturnCommandMessage(command, value)
	if message == "" {
		return returnModeAnalysis{}
	}
	return returnModeAnalysis{Issue: &statementIssue{Offset: base, Message: message}}
}

type surplusAnalysis struct {
	Handled bool
	Macro   bool
	Parts   []SyntaxSpan
	all     []SyntaxSpan
}

func (a *StatementAnalysis) ForSurplus(command string, start, end int) surplusAnalysis {
	if command != "FOR" || !a.ExpressionEnabled || CommandFieldHasDeferredMarker(a.Source[start:end]) || CommandFieldHasMandatoryMarker(a.Source[start:end]) {
		return surplusAnalysis{}
	}
	all := a.split(start, end, ',')
	parts, surplus := CommandPrefixParts(a.Source, all, 4, 3, a.AllowFullwidthSpace, true)
	if !surplus {
		return surplusAnalysis{}
	}
	for _, field := range all {
		tokens, err := a.lexRange(field.Start, field.End)
		if err == nil && CommandMacroPossibleForDialect(tokens, a.Dialect) {
			return surplusAnalysis{Handled: true, Macro: true, Parts: parts, all: all}
		}
	}
	return surplusAnalysis{Handled: true, Parts: parts, all: all}
}

func (a *StatementAnalysis) ScalarSurplus(command string, start, end int) surplusAnalysis {
	if !a.ExpressionEnabled || !ScalarSurplusCommand(command) || CommandFieldHasDeferredMarker(a.Source[start:end]) || CommandFieldHasMandatoryMarker(a.Source[start:end]) {
		return surplusAnalysis{}
	}
	parts, surplus := CommandPrefixParts(a.Source, a.split(start, end, ','), 1, 1, a.AllowFullwidthSpace, true)
	if !surplus {
		return surplusAnalysis{}
	}
	first := parts[0]
	tokens, err := a.lexRange(first.Start, first.End)
	if err == nil && CommandMacroPossibleForDialect(tokens, a.Dialect) {
		return surplusAnalysis{Handled: true, Macro: true, Parts: parts}
	}
	return surplusAnalysis{Handled: true, Parts: parts}
}

type incrementAnalysis struct {
	Handled   bool
	ReadOnly  bool
	Start     int
	End       int
	LhsStart  int
	lhsLength int
}

func (a *StatementAnalysis) Increment(head []byte, base int) incrementAnalysis {
	session := a.sourceSession(head)
	end, _ := SyntaxCommentEnd(head, 0, len(head), session)
	end = TrimInputRight(head, 0, end, a.AllowFullwidthSpace)
	if end < 2 {
		return incrementAnalysis{}
	}
	lhs := head[:end-2]
	op := head[end-2 : end]
	if bytes.Equal(head[:2], []byte("++")) || bytes.Equal(head[:2], []byte("--")) {
		op, lhs = head[:2], head[2:end]
	}
	if !(bytes.Equal(op, []byte("++")) || bytes.Equal(op, []byte("--"))) || !a.AssignmentLHS(lhs) {
		return incrementAnalysis{}
	}
	lhsStart := base
	if bytes.Equal(head[:2], op) {
		lhsStart += 2
	}
	result := incrementAnalysis{Handled: true, Start: base, End: base + end, LhsStart: lhsStart, lhsLength: len(lhs)}
	if a.ExpressionEnabled {
		if tokens, err := a.lexRange(lhsStart, lhsStart+len(lhs)); err == nil {
			result.ReadOnly = BuiltinVariableReadOnly(tokens, a.Dialect) || !SetterMacroPossibleForDialect(tokens, a.Dialect) && knownVariableType(tokens) == KnownValueString
		}
	}
	return result
}

func (a *StatementAnalysis) ReturnHasCommaAssignment(start, end int) bool {
	parts := a.split(start, end, ',')
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts[1:] {
		left := TrimInputLeft(a.Source, part.Start, part.End, a.AllowFullwidthSpace)
		right := TrimInputRight(a.Source, left, part.End, a.AllowFullwidthSpace)
		if left >= right {
			continue
		}
		_, op, ok := AssignmentBoundary(a.Source[left:right])
		if ok && op.End-op.Start == 1 && a.Source[left+op.Start] == '=' {
			return true
		}
	}
	return false
}

type callAnalysis struct {
	valid          bool
	Form           bool
	NameStart      int
	NameEnd        int
	Delimiter      int
	ArgumentsStart int
	BeforeOpen     int
	SubnameOpen    int
	SubnameClose   int
	ArgumentOpen   int
	ArgumentClose  int
	Issue          *statementIssue
}

func (a *StatementAnalysis) Call(command string, start, end int) callAnalysis {
	result := callAnalysis{valid: true, Delimiter: -1, SubnameOpen: -1, SubnameClose: -1, ArgumentOpen: -1, ArgumentClose: -1}
	if start >= end {
		return result
	}
	result.Form = strings.Contains(command, "FORM") || command == "FUNC"
	nameEnd, delimiter, _ := CallPrefix(a.Source, start, end)
	if result.Form {
		session, offset := a.expressionSession(start, end)
		if n, err := session.ScanFormAt(ExpressionSourceOriginal, offset, offset+end-start, FormCall); err == nil {
			nameEnd = start + n - offset
			delimiter = nameEnd
			for delimiter < end {
				size := InputSpaceSize(a.Source, delimiter, a.AllowFullwidthSpace)
				if size == 0 {
					break
				}
				delimiter += size
			}
			if delimiter >= end {
				delimiter = -1
			}
		} else {
			return callAnalysis{Issue: &statementIssue{Offset: start, Message: err.Error()}}
		}
	}
	if command == "FUNC" {
		if issue := a.TryGotoListFuncIssue(start, end, delimiter); issue != nil {
			return callAnalysis{Issue: issue}
		}
	}
	nameStart := TrimInputLeft(a.Source, start, nameEnd, false)
	nameEnd = trimCallTargetRight(a.Source, nameStart, nameEnd)
	if nameStart == nameEnd {
		return callAnalysis{Issue: &statementIssue{Offset: start, Message: "missing call target"}}
	}
	result.NameStart, result.NameEnd, result.Delimiter = nameStart, nameEnd, delimiter
	if delimiter < 0 || delimiter >= end || a.Source[delimiter] == ';' {
		return result
	}
	result.ArgumentsStart = delimiter
	result.BeforeOpen = nameEnd
	if a.Source[delimiter] == '[' {
		close := a.matchingDelimiter(delimiter, end, '[', ']')
		if close < 0 {
			return callAnalysis{Issue: &statementIssue{Offset: delimiter, Message: "unclosed call subname list"}}
		}
		result.SubnameOpen, result.SubnameClose = delimiter, close
		delimiter = close + 1
		gapStart := delimiter
		for delimiter < end {
			size := InputSpaceSize(a.Source, delimiter, a.AllowFullwidthSpace)
			if size == 0 {
				break
			}
			delimiter += size
		}
		result.BeforeOpen = gapStart
		result.Delimiter = delimiter
		if delimiter >= end || a.Source[delimiter] == ';' {
			return result
		}
	}
	if a.Source[delimiter] == '(' {
		close := a.matchingDelimiter(delimiter, end, '(', ')')
		if close < 0 {
			return callAnalysis{Issue: &statementIssue{Offset: delimiter, Message: "unclosed call argument list"}}
		}
		result.ArgumentOpen, result.ArgumentClose = delimiter, close
		return result
	}
	return result
}

func (a *StatementAnalysis) CallTailIssue(start, close, end int) *statementIssue {
	tail := TrimInputLeft(a.Source, close+1, end, a.AllowFullwidthSpace)
	if tail >= end || a.Source[tail] == ';' || a.Source[tail] == 0 {
		return nil
	}
	if CommandFieldHasMandatoryMarker(a.Source[start:end]) || CommandFieldHasDeferredMarker(a.Source[start:end]) {
		return nil
	}
	src := a.Source[tail:end]
	if nameEnd, identifier := LabelIdentifierEndWithSession(src, 0, a.scanSession(tail, end)); identifier && !setterKnownNameForDialect(src[:nameEnd], a.Dialect) {
		return nil
	}
	tokens, err := a.lexRange(start, close+1)
	if err != nil || ExpressionNeedsDeferral(tokens, a.Dialect) {
		return nil
	}
	return &statementIssue{Offset: tail, Message: "unexpected tokens after call argument list"}
}

func (a *StatementAnalysis) TryGotoListFuncIssue(start, end, delimiter int) *statementIssue {
	if a.State == nil || len(a.State.Frames) == 0 || a.State.Frames[len(a.State.Frames)-1].Kind != "FUNCLIST" || a.State.Frames[len(a.State.Frames)-1].ListCommand != "TRYGOTOLIST" {
		return nil
	}
	if a.Conditional || a.Optional || a.Preserved || bytes.Contains(a.Source[start:end], []byte("[[")) {
		return nil
	}
	for delimiter >= 0 && delimiter < end {
		left, right := delimiter+1, end
		switch a.Source[delimiter] {
		case '[':
			right = a.matchingDelimiter(delimiter, end, '[', ']')
		case '(':
			right = a.matchingDelimiter(delimiter, end, '(', ')')
		case ',':
		default:
			return nil
		}
		if right < left {
			return nil
		}
		if TrimInputLeft(a.Source, left, right, a.AllowFullwidthSpace) < right {
			tokens, err := a.lexRange(left, right)
			if err != nil || ExpressionNeedsDeferral(tokens, a.Dialect) {
				return nil
			}
			return &statementIssue{Offset: delimiter, Message: "TRYGOTOLIST FUNC cannot have subnames or arguments"}
		}
		if a.Source[delimiter] != '[' {
			return nil
		}
		delimiter = TrimInputLeft(a.Source, right+1, end, a.AllowFullwidthSpace)
	}
	return nil
}

func (a *StatementAnalysis) scanSession(start, end int) *ExpressionSession {
	if a.Semantic != nil {
		return a.Semantic.ScanSession(a.Source, start, end)
	}
	s := NewExpressionSessionWithSpace(a.Source[start:end], a.AllowFullwidthSpace)
	s.BuiltinTypes = true
	s.Dialect = a.Dialect
	return s
}

func (a *StatementAnalysis) matchingDelimiter(start, end int, open, close byte) int {
	session := a.scanSession(start, end)
	matched := MatchingDelimiter(a.Source[start:end], 0, end-start, open, close, session)
	if matched >= 0 {
		matched += start
	}
	return matched
}

func (a *StatementAnalysis) AssignmentLHS(src []byte) bool {
	src = TrimInput(src, a.AllowFullwidthSpace)
	s := a.sourceSession(src)
	tokens, err := s.LexRange(ExpressionSourceOriginal, 0, len(src), 0)
	name, ok := VariableLHSFromTokens(tokens, err)
	return ok && !ReservedInstruction(name) && !methodDialect(builtinMethodMasks[name], a.Dialect)
}

func CodeEndValue(src []byte, l Line, end int, allowFullwidthSpace bool, sessions ...*ExpressionSession) (int, bool) {
	if end < l.IndentEnd || end > l.BodyEnd {
		return end, false
	}
	commentEnd, _ := SyntaxCommentEnd(src[l.IndentEnd:end], 0, end-l.IndentEnd, sessions...)
	end = l.IndentEnd + commentEnd
	return RecordCodeEndValue(src, l, end, allowFullwidthSpace)
}

func RecordCodeEndValue(src []byte, l Line, end int, allowFullwidthSpace bool) (int, bool) {
	end = TrimInputRight(src, l.IndentEnd, end, allowFullwidthSpace)
	if end >= l.IndentEnd+3 && MandatoryInlineMarker(src[:end], end-3) {
		return end, true
	}
	return end, false
}

func CallCodeEndValue(src []byte, l Line, start, end int, allowFullwidthSpace bool, sessions ...*ExpressionSession) (int, bool) {
	nameEnd, delimiter, _ := CallPrefix(src, start, end)
	if delimiter < 0 || src[delimiter] == ';' {
		end = trimCallTargetRight(src, start, nameEnd)
	} else {
		end = TrimInputRight(src, start, end, allowFullwidthSpace)
	}
	_, remove := RecordCodeEndValue(src, l, end, allowFullwidthSpace)
	return end, remove
}
