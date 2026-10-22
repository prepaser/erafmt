package format

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func (p *formatter) syntax(i int, head []byte, prefix int) int {
	if len(head) == 0 || i < 0 || i >= len(p.Lines) {
		return -1
	}
	if len(p.Opaque) > i && p.Opaque[i] {
		return -1
	}
	if bytes.Contains(head, []byte("[[")) {
		p.syntaxOpaque(i)
		return -1
	}
	if erabasic.SyntaxHasProtectedMetadataTail(head) {
		defer p.syntaxOpaque(i)
	}
	line := p.Lines[i]
	base := line.Start + prefix
	if p.SyntaxSession == nil || p.SyntaxBase != base {
		p.SyntaxSession = p.newExpressionSession(head)
		p.SyntaxBase = base
	}
	defer func() {
		p.SyntaxSession = nil
	}()
	if head[0] == '@' {
		return p.syntaxHeader(i, head, base)
	}
	if head[0] == '#' {
		return p.syntaxDeclaration(i, head, base)
	}
	command, wordEnd := p.commandOf(head)
	p.SyntaxSession.SkipFormTypes = command == "DEBUGPRINTFORM" || command == "DEBUGPRINTFORML"
	if p.syntaxReturnMode(i, head, base, command, wordEnd) {
		return -1
	}
	if command == "" {
		if start, op, ok := p.assignmentBoundary(head); ok && p.assignmentLHS(head[:start]) {
			return p.syntaxAssignment(i, head, base, op)
		}
		if p.syntaxIncrement(i, head, base) {
			return -1
		}
		p.removeLineRule(i, InstructionIndent)
		p.removeLineRule(i, SIFIndent)
		p.syntaxOpaque(i)
		return -1
	}
	if erabasic.CommandUnavailable(command, p.Dialect) && !erabasic.ReservedInstruction(command) {
		if op, ok := p.unavailableMethodAssignment(head, command); ok {
			return p.syntaxAssignment(i, head, base, op)
		}
		if p.syntaxIncrement(i, head, base) {
			return -1
		}
	}
	if end, handled := p.syntaxExtra(i, head, base, command, wordEnd); handled {
		return end
	}
	if end, handled := p.syntaxGraphics(i, head, base, command, wordEnd); handled {
		return end
	}
	if end, handled := p.syntaxRegistered(i, head, base, command, wordEnd); handled {
		return end
	}
	if erabasic.CommandUnavailable(command, p.Dialect) {
		if op, ok := p.unavailableCommandAssignment(head, command); ok {
			return p.syntaxAssignment(i, head, base, op)
		}
		if p.commandCoverageEnabled() {
			p.unsupportedCommand(i, base, command, "unavailable in selected dialect")
		} else {
			p.syntaxOpaque(i)
		}
		return -1
	}
	if erabasic.RawCommand(command) {
		p.syntaxOpaque(i)
		return -1
	}
	if end, handled := p.syntaxMethod(i, head, base, command, wordEnd); handled {
		return end
	}
	if !erabasic.SyntaxCommand(command) {
		if dataBodyCommand(p, command) {
			p.syntaxOpaque(i)
			return -1
		}
		if erabasic.ReservedInstruction(command) {
			if p.commandCoverageEnabled() {
				p.unsupportedCommand(i, base, command, erabasic.CommandDialectReason(command, p.Dialect))
				return -1
			}
			p.syntaxOpaque(i)
			return -1
		}
		if start, op, ok := p.assignmentBoundary(head); ok && p.assignmentLHS(head[:start]) {
			return p.syntaxAssignment(i, head, base, op)
		}
		if p.syntaxIncrement(i, head, base) {
			return -1
		}
		if p.commandCoverageEnabled() {
			p.unsupportedCommand(i, base, command, erabasic.CommandDialectReason(command, p.Dialect))
			return -1
		}
		p.syntaxOpaque(i)
		return -1
	}
	if erabasic.PrintVCommand(command) || erabasic.PrintFormCommand(command) {
		if wordEnd >= len(head) {
			if command == "RETURNFORM" && p.expressionRulesEnabled() {
				p.syntaxProblem(i, base+wordEnd, "missing FORM argument")
			}
			return -1
		}
		if command == "RETURNFORM" {
			tail := head[wordEnd:]
			if len(tail) == erabasic.InputSpaceSize(tail, 0, p.AllowFullwidthSpace) || len(tail) == 1 && tail[0] == ';' {
				if p.expressionRulesEnabled() {
					p.syntaxProblem(i, base+wordEnd, "missing FORM argument")
				}
				return -1
			}
		}
		bodyStart := wordEnd + erabasic.InputSpaceSize(head, wordEnd, p.AllowFullwidthSpace)
		if bodyStart == wordEnd {
			bodyStart++
		}
		end := len(head)
		if erabasic.PrintVCommand(command) && p.inputSpaceIn(head, wordEnd) && commandSpacingEnabled(command) {
			arg := erabasic.TrimInputLeft(head, bodyStart, end, p.AllowFullwidthSpace)
			if arg < end && head[arg] != ';' {
				p.addEdit(i, base+wordEnd, base+arg, []byte{' '}, CommandSpacing)
			}
		}
		var ok bool
		if erabasic.PrintVCommand(command) {
			ok = p.syntaxPrintV(i, base+bodyStart, base+end)
		} else if command == "RETURNFORM" {
			ok = p.syntaxReturnForm(i, base+bodyStart, base+end)
		} else {
			ok = p.syntaxForm(i, base+bodyStart, base+end, erabasic.FormLine)
		}
		if !ok {
			p.syntaxOpaque(i)
		}
		return -1
	}
	var body []byte
	var bodyStart, end, comment int
	var hasArg bool
	formCall := erabasic.CallCommand(command) && (strings.Contains(command, "FORM") || command == "FUNC")
	if erabasic.CallCommand(command) {
		var ok bool
		bodyStart, end, ok = erabasic.RegisteredRawBodyWithSpace(head, wordEnd, p.AllowFullwidthSpace)
		if !ok {
			p.syntaxOpaque(i)
			return -1
		}
		hasArg, comment = wordEnd < len(head), -1
		if !formCall {
			nameEnd, delimiter, valid := erabasic.CallPrefix(head, bodyStart, end)
			if !valid {
				if p.expressionParseEnabled() {
					p.syntaxProblem(i, base+nameEnd, "missing character after call escape")
				}
				p.syntaxOpaque(i)
				return -1
			}
			if delimiter < 0 {
				end = nameEnd
			} else if tailEnd, found := p.syntaxCommentEnd(head, delimiter, end); found {
				end, comment = tailEnd, tailEnd
			} else {
				end = tailEnd
			}
		}
		body = head[bodyStart:end]
	} else {
		body, bodyStart, end, comment, hasArg = p.instructionBody(head, wordEnd)
	}
	if command == "FOR" && p.expressionParseEnabled() && (!hasArg || wordEnd < len(head) && head[wordEnd] == ';' || len(erabasic.TrimInput(body, p.AllowFullwidthSpace)) == 0) {
		p.syntaxProblem(i, base+wordEnd, "FOR requires arguments")
		p.syntaxOpaque(i)
		return -1
	}
	if wordEnd < len(head) && head[wordEnd] == ';' {
		p.syntaxOpaque(i)
		return -1
	}
	if comment >= 0 {
		end = comment
	}
	if erabasic.RequiredExpressionCommand(command) && p.expressionParseEnabled() && (!hasArg || bodyStart >= end || len(erabasic.TrimInput(head[bodyStart:end], p.AllowFullwidthSpace)) == 0) {
		p.syntaxProblem(i, base+wordEnd, "missing command expression")
		p.syntaxOpaque(i)
		return -1
	}
	if !hasArg {
		p.setCodeEnd(i, base+wordEnd)
		return base + wordEnd
	}
	argumentSpace := p.AllowFullwidthSpace && !erabasic.CallCommand(command)
	if bodyStart >= len(head) || len(erabasic.TrimInput(body, argumentSpace)) == 0 {
		p.setCodeEnd(i, base+wordEnd)
		return base + wordEnd
	}
	if body[0] == ';' && !erabasic.MandatoryInlineMarker(body, 0) && !erabasic.PrintVCommand(command) && !erabasic.PrintFormCommand(command) {
		p.setCodeEnd(i, base+bodyStart)
		return base + bodyStart
	}
	if p.syntaxForSurplus(i, command, base+bodyStart, base+end) || p.syntaxScalarSurplus(i, command, base+bodyStart, base+end) {
		return -1
	}
	mark := len(p.edits)
	if p.inputSpaceIn(head, wordEnd) && commandSpacingEnabled(command) {
		arg := erabasic.TrimInputLeft(head, bodyStart, end, argumentSpace)
		if arg < end && head[arg] != ';' {
			p.addEdit(i, base+wordEnd, base+arg, []byte{' '}, CommandSpacing)
		}
	}
	var ok bool
	if offset, message, valid := p.genericCommandRoleValid(command, base+bodyStart, base+end); !valid {
		p.truncateEdits(mark)
		p.syntaxProblem(i, offset, message)
		p.syntaxOpaque(i)
		return -1
	}
	if command == "RETURN" && p.returnHasCommaAssignment(base+bodyStart, base+end) {
		p.truncateEdits(mark)
		if tokens, err := p.lexSyntaxRange(base+bodyStart, base+end); err == nil && !erabasic.ExpressionNeedsDeferral(tokens, p.Dialect) {
			p.syntaxProblem(i, base+bodyStart, "RETURN contains an assignment in a comma field")
		}
		p.syntaxOpaque(i)
		return -1
	}
	switch {
	case erabasic.ExpressionCommand(command), erabasic.SimpleIntegerCommand(command):
		if command == "SKIPLOG" {
			ok = p.syntaxExpression(i, base+bodyStart, base+end, false)
		} else {
			ok = p.syntaxOrdinaryExpression(i, base+bodyStart, base+end, command == "CASE")
		}
	case erabasic.SimpleStringCommand(command):
		if strings.Contains(command, "FORMS") {
			ok = p.syntaxPrintFormsLiteral(i, base+bodyStart, base+end)
		} else {
			ok = p.syntaxOrdinaryExpression(i, base+bodyStart, base+end, false)
		}
	case erabasic.CallCommand(command):
		ok = p.syntaxCall(i, command, base+bodyStart, base+end)
	case erabasic.ForCommand(command):
		ok = p.syntaxOrdinaryExpression(i, base+bodyStart, base+end, false)
	case erabasic.StructuredListCommand(command):
		ok = p.syntaxExpression(i, base+bodyStart, base+end, false)
	default:
		p.syntaxOpaque(i)
		return -1
	}
	if !ok {
		p.syntaxOpaque(i)
		return -1
	}
	if formCall {
		return -1
	}
	if erabasic.CallCommand(command) {
		return p.setCallCodeEnd(i, base+bodyStart, base+end)
	}
	codeEnd := erabasic.TrimInputRight(p.Src, base+bodyStart, base+end, p.AllowFullwidthSpace)
	p.setCodeEnd(i, codeEnd)
	return codeEnd
}

func (p *formatter) syntaxReturnMode(i int, head []byte, base int, command string, wordEnd int) bool {
	analysis := p.statementAnalysis()
	result := analysis.ReturnMode(head, base, command, wordEnd)
	if result.Issue == nil {
		return false
	}
	p.syntaxProblem(i, result.Issue.Offset, result.Issue.Message)
	p.syntaxOpaque(i)
	return true
}

func (p *formatter) syntaxForSurplus(i int, command string, start, end int) bool {
	analysis := p.statementAnalysis().ForSurplus(command, start, end)
	if !analysis.Handled {
		return false
	}
	if analysis.Macro {
		p.syntaxOpaque(i)
		return true
	}
	mark := len(p.edits)
	if offset, message, valid := p.genericCommandRoleValid(command, start, end); !valid {
		p.syntaxProblem(i, offset, message)
	} else if p.syntaxOrdinaryExpression(i, start, analysis.Parts[len(analysis.Parts)-1].End, false) {
		p.ignoredCommandArguments(i, start, command)
	}
	p.truncateEdits(mark)
	p.syntaxOpaque(i)
	return true
}

func (p *formatter) syntaxScalarSurplus(i int, command string, start, end int) bool {
	analysis := p.statementAnalysis().ScalarSurplus(command, start, end)
	if !analysis.Handled {
		return false
	}
	if analysis.Macro {
		p.syntaxOpaque(i)
		return true
	}
	mark := len(p.edits)
	first := analysis.Parts[0]
	if offset, message, valid := p.genericCommandRoleValid(command, first.Start, first.End); !valid {
		p.syntaxProblem(i, offset, message)
	} else if p.syntaxOrdinaryExpression(i, first.Start, first.End, false) {
		p.ignoredCommandArguments(i, start, command)
	}
	p.truncateEdits(mark)
	p.syntaxOpaque(i)
	return true
}

func (p *formatter) syntaxIncrement(i int, head []byte, base int) bool {
	analysis := p.statementAnalysis().Increment(head, base)
	if !analysis.Handled {
		return false
	}
	if analysis.ReadOnly {
		p.syntaxProblem(i, analysis.LhsStart, "increment requires a mutable numeric destination")
		p.syntaxOpaque(i)
		return true
	}
	if p.syntaxExpression(i, analysis.Start, analysis.End, false) {
		p.setCodeEnd(i, analysis.End)
	}
	return true
}

func (p *formatter) syntaxReturnForm(i, start, end int) bool {
	if !p.expressionRulesEnabled() {
		return true
	}
	var edits []edit
	var firstErr error
	session, offset := p.expressionSessionFor(start, end)
	defer p.addFormAdvisories(i, session, offset, offset+end-start, start)
	for pos := start; pos < end; {
		stop, err := session.ScanFormAt(erabasic.ExpressionSourceOriginal, offset+pos-start, offset+end-start, erabasic.FormComma)
		n := stop - (offset + pos - start)
		if err != nil {
			p.expressionProblem(i, pos, erabasic.ShiftExpressionError(err, -(offset+pos-start)))
			return false
		}
		item, err := formEditsWithSession(session, erabasic.ExpressionSourceOriginal, p.Src[pos:pos+n], offset+pos-start, p.rules, erabasic.FormComma, true)
		if err != nil && firstErr == nil {
			firstErr = erabasic.ShiftExpressionError(err, pos-start)
		}
		for _, e := range item {
			e.start += pos - start
			e.end += pos - start
			edits = append(edits, e)
		}
		pos += n
		if pos < end && p.Src[pos] == 0 {
			break
		}
		if pos < end {
			pos++
		}
	}
	p.addSyntaxEdits(i, start, edits)
	if firstErr != nil {
		p.expressionProblem(i, start, firstErr)
	}
	return true
}

func (p *formatter) syntaxOrdinaryExpression(i, start, end int, caseMode bool) bool {
	result := p.statementAnalysis().OrdinaryExpressionProof(start, end, caseMode)
	if result.Err != nil {
		p.expressionProblem(i, start, result.Err)
		return false
	}
	if !result.Valid {
		return false
	}
	return p.syntaxExpression(i, start, end, caseMode)
}

func (p *formatter) syntaxExpression(i, start, end int, caseMode bool) bool {
	if start > end || end > len(p.Src) {
		return false
	}
	if !p.expressionParseEnabled() {
		return true
	}
	session, offset := p.expressionSessionFor(start, end)
	defer p.addFormAdvisories(i, session, offset, offset+end-start, start)
	excluded, err := erabasic.ExpressionExclusionWithSession(session, p.Src[start:end])
	if err != nil {
		p.expressionProblem(i, start, err)
		return false
	}
	if excluded {
		p.syntaxOpaque(i)
		return false
	}
	if tokens, err := session.LexRange(erabasic.ExpressionSourceOriginal, offset, offset+end-start, 0); err == nil {
		if err := erabasic.ProvenKnownExpressionBoundaries(tokens, p.Dialect); err != nil {
			p.expressionProblem(i, start, err)
			return false
		}
	}
	edits, err := expressionEditsWithSession(session, erabasic.ExpressionSourceOriginal, p.Src[start:end], offset, caseMode, p.rules, true)
	if err != nil {
		p.expressionProblem(i, start, err)
		return false
	}
	p.addSyntaxEdits(i, start, edits)
	return true
}

func (p *formatter) syntaxForm(i, start, end int, mode erabasic.FormMode) bool {
	if start > end || end > len(p.Src) {
		return false
	}
	if !p.expressionRulesEnabled() {
		return true
	}
	session, offset := p.expressionSessionFor(start, end)
	defer p.addFormAdvisories(i, session, offset, offset+end-start, start)
	edits, err := formEditsWithSession(session, erabasic.ExpressionSourceOriginal, p.Src[start:end], offset, p.rules, mode, true)
	p.addSyntaxEdits(i, start, edits)
	if err != nil {
		p.expressionProblem(i, start, err)
		return len(edits) > 0
	}
	return true
}

func (p *formatter) addFormAdvisories(i int, session *erabasic.ExpressionSession, sourceStart, sourceEnd, absoluteBase int) {
	p.addFormAdvisoriesMapped(i, session, sourceStart, sourceEnd, func(offset int) int {
		return absoluteBase + offset - sourceStart
	})
}

func (p *formatter) addFormAdvisoriesMapped(i int, session *erabasic.ExpressionSession, sourceStart, sourceEnd int, mapOffset func(int) int) {
	if p.Aborted {
		return
	}
	previousEnd, previousColumn := len(p.diagnostics), 0
	ordered := true
	for _, advisory := range session.FormAdvisoriesIn(erabasic.ExpressionSourceOriginal, sourceStart, sourceEnd) {
		if p.Aborted {
			return
		}
		offset := mapOffset(advisory.Offset)
		line := i + 1
		column := p.columnAt(i, offset)
		ordered = ordered && column >= previousColumn
		previousColumn = column
		duplicate := false
		for j := len(p.diagnostics) - 1; j >= p.statementDiagnostic; j-- {
			diagnostic := p.diagnostics[j]
			if ordered && j >= previousEnd && diagnostic.Column < column {
				j = previousEnd
				continue
			}
			if diagnostic.Line == line && diagnostic.Column == column && diagnostic.Severity == Warning && diagnostic.Code == advisory.Code {
				duplicate = true
				break
			}
		}
		if !duplicate {
			p.warning(i, offset, advisory.Code, advisory.Message)
		}
	}
}

func (p *formatter) syntaxPrintFormsLiteral(i, start, end int) bool {
	if !p.expressionRulesEnabled() {
		return true
	}
	left := erabasic.TrimInputLeft(p.Src, start, end, p.AllowFullwidthSpace)
	right := erabasic.TrimInputRight(p.Src, left, end, p.AllowFullwidthSpace)
	if left >= right || p.Src[left] != '"' || erabasic.QuotedEnd(p.Src[left:right], 0) != right-left {
		return p.syntaxOrdinaryExpression(i, start, end, false)
	}
	innerStart, innerEnd := left+1, right-1
	if bytes.IndexByte(p.Src[innerStart:innerEnd], '\\') >= 0 {
		return p.syntaxOrdinaryExpression(i, start, end, false)
	}
	session, offset := p.expressionSessionFor(innerStart, innerEnd)
	defer p.addFormAdvisories(i, session, offset, offset+innerEnd-innerStart, innerStart)
	edits, err := formEditsWithSession(session, erabasic.ExpressionSourceOriginal, p.Src[innerStart:innerEnd], offset, p.rules, erabasic.FormLine, true)
	p.addSyntaxEdits(i, innerStart, edits)
	if err != nil {
		p.expressionProblem(i, innerStart, erabasic.ShiftExpressionError(err, 0))
		return len(edits) > 0
	}
	return true
}

func (p *formatter) expressionSessionFor(start, end int) (*erabasic.ExpressionSession, int) {
	if p.SyntaxSession != nil && start >= p.SyntaxBase && end-p.SyntaxBase <= len(p.SyntaxSession.Sources[erabasic.ExpressionSourceOriginal]) {
		return p.SyntaxSession, start - p.SyntaxBase
	}
	session := p.newExpressionSession(p.Src[start:end])
	session.OnResource = func(offset int, message string) {
		p.resourceProblem(p.ParseLine, start+offset, message)
	}
	return session, 0
}

func (p *formatter) lexSyntaxRange(start, end int) ([]erabasic.ExprToken, error) {
	session, offset := p.expressionSessionFor(start, end)
	return session.LexRange(erabasic.ExpressionSourceOriginal, offset, offset+end-start, start)
}

func (p *formatter) expressionRulesEnabled() bool {
	for _, id := range []RuleID{BinarySpacing, TernarySpacing, UnarySpacing, CommaSpacing, DelimiterSpacing, IndexSpacing, CaseSpacing, TokenSpacing, FormPadding, FormConditionSpacing} {
		if p.rules[id] {
			return true
		}
	}
	return false
}

func commandSpacingEnabled(command string) bool {
	return erabasic.ExpressionCommand(command) || erabasic.SimpleIntegerCommand(command) || erabasic.SimpleStringCommand(command) || erabasic.PrintVCommand(command) || erabasic.CallCommand(command) || erabasic.ForCommand(command) || erabasic.StructuredListCommand(command)
}

func (p *formatter) expressionParseEnabled() bool {
	return p.expressionRulesEnabled() || p.rules[CodeTrailingWhitespace] || p.rules[InlineCommentSpacing] || p.rules[CommentTrailingWhitespace] || p.rules[CommentPrefixSpacing]
}

func (p *formatter) syntaxPrintV(i, start, end int) bool {
	if !p.expressionParseEnabled() {
		return true
	}
	projection, codeEnd, rawEnd, ok := p.printVProjection(i, start, end)
	if !ok {
		return false
	}
	analysis := erabasic.PrintVAnalysis{Dialect: p.Dialect, AllowFullwidthSpace: p.AllowFullwidthSpace, NewSession: func(src []byte) *erabasic.ExpressionSession {
		session := p.newExpressionSession(src)
		session.OnResource = func(offset int, message string) {
			p.resourceProblem(i, start+projection.OriginalOffset(offset), message)
		}
		return session
	}}
	validation := analysis.Validate(projection, codeEnd, rawEnd)
	if validation.Err != nil {
		p.expressionProblem(i, start+projection.OriginalOffset(validation.ErrorBase), erabasic.MapPrintVError(validation.Err, projection))
		return false
	}
	projection, codeEnd, rawEnd = validation.Projection, validation.CodeEnd, validation.RawEnd
	if validation.Problem != nil {
		p.syntaxProblem(i, start+projection.OriginalOffset(validation.Problem.Offset), validation.Problem.Message)
		return false
	}
	if validation.Excluded {
		p.syntaxOpaque(i)
		return false
	}
	session := validation.Session
	defer p.addFormAdvisoriesMapped(i, session, 0, len(projection.Src), func(offset int) int {
		return start + projection.OriginalOffset(offset)
	})
	if erabasic.TrimInputLeft(projection.Src, 0, codeEnd, p.AllowFullwidthSpace) >= erabasic.TrimInputRight(projection.Src, 0, codeEnd, p.AllowFullwidthSpace) {
		if !rawEnd {
			p.recordCodeEnd(i, start+projection.OriginalOffset(codeEnd))
		}
		return true
	}
	edits, err := expressionEditsWithSession(session, erabasic.ExpressionSourceOriginal, projection.Src, 0, false, p.rules, true)
	if err != nil {
		p.expressionProblem(i, start, erabasic.MapPrintVError(err, projection))
		return false
	}
	for n := range edits {
		edits[n].start = projection.OriginalOffset(edits[n].start)
		edits[n].end = projection.OriginalOffset(edits[n].end)
	}
	p.addSyntaxEdits(i, start, edits)
	if !rawEnd {
		p.recordCodeEnd(i, start+projection.OriginalOffset(codeEnd))
	}
	return true
}

func (p *formatter) syntaxCommaGap(i, comma, end int) {
	p.syntaxRegisteredCommaGap(i, comma, end, 0)
}

func (p *formatter) syntaxCallDelimiter(i, targetEnd, open, close int) {
	if !p.rules[DelimiterSpacing] {
		return
	}
	p.addEdit(i, targetEnd, open, nil, DelimiterSpacing)
	if close <= open+1 {
		return
	}
	left := erabasic.TrimInputLeft(p.Src, open+1, close, p.AllowFullwidthSpace)
	right := erabasic.TrimInputRight(p.Src, open+1, close, p.AllowFullwidthSpace)
	if left >= right {
		p.addEdit(i, open+1, close, nil, DelimiterSpacing)
		return
	}
	if !erabasic.MandatoryInlineMarker(p.Src, left) {
		p.addEdit(i, open+1, left, nil, DelimiterSpacing)
	}
	if !erabasic.MandatoryInlineMarker(p.Src[:right], right-3) {
		p.addEdit(i, right, close, nil, DelimiterSpacing)
	}
}

func (p *formatter) syntaxAssignment(i int, head []byte, base int, op erabasic.AssignmentOp) int {
	opStart := base + op.Start
	opEnd := base + op.End
	lhsStart := base
	lhsStart = erabasic.TrimInputLeft(p.Src, lhsStart, opStart, p.AllowFullwidthSpace)
	if p.expressionParseEnabled() {
		if tokens, err := p.lexSyntaxRange(lhsStart, opStart); err == nil && erabasic.BuiltinVariableReadOnly(tokens, p.Dialect) {
			p.syntaxProblem(i, lhsStart, "assignment destination is read-only")
			p.syntaxOpaque(i)
			return -1
		}
	}
	if p.rules[AssignmentLeftSpacing] {
		left := opStart
		left = erabasic.TrimInputRight(p.Src, lhsStart, left, p.AllowFullwidthSpace)
		p.addEdit(i, left, opStart, []byte{' '}, AssignmentLeftSpacing)
	}
	if !p.syntaxExpression(i, lhsStart, opStart, false) {
		p.syntaxOpaque(i)
	}
	end := base + len(head)
	comment := false
	if !op.Plain {
		end, comment = p.syntaxCommentEnd(p.Src, opEnd, end)
	}
	rhsStart := opEnd
	rhsStart = erabasic.TrimInputLeft(p.Src, rhsStart, end, p.AllowFullwidthSpace && !op.Plain)
	if rhsStart < end && (op.Plain || p.Src[rhsStart] != ';') && p.rules[AssignmentRightSpacing] {
		p.addEdit(i, opEnd, rhsStart, []byte{' '}, AssignmentRightSpacing)
	}
	if op.Plain {
		return -1
	}
	if rhsStart < end && !p.syntaxOrdinaryExpression(i, rhsStart, end, false) {
		p.syntaxOpaque(i)
		return -1
	}
	if comment {
		end = erabasic.TrimInputRight(p.Src, rhsStart, end, p.AllowFullwidthSpace)
		p.setCodeEnd(i, end)
		return end
	}
	end = erabasic.TrimInputRight(p.Src, rhsStart, end, p.AllowFullwidthSpace)
	p.setCodeEnd(i, end)
	return end
}

func (p *formatter) addSyntaxEdits(i, base int, edits []edit) {
	sortEdits(edits)
	if len(edits) == 0 {
		return
	}
	pos := base + edits[0].start
	col := p.columnAt(i, pos)
	for _, e := range edits {
		start, end := base+e.start, base+e.end
		col += utf8.RuneCount(p.Src[pos:start])
		pos = start
		if p.rules[e.change.Rule] && !bytes.Equal(p.Src[start:end], e.text) {
			if !p.reserveReplacementBytes(start, end, len(e.text)) {
				return
			}
			if !p.appendEdit(edit{start: start, end: end, text: e.text, change: Change{Rule: e.change.Rule, Line: i + 1, Column: col}}) {
				return
			}
		}
	}
}

func (p *formatter) syntaxProblem(i, offset int, message string) {
	if p.Aborted {
		return
	}
	p.diagnostic(i, offset, erabasic.Unsupported, "unsupported_syntax", message)
}

func (p *formatter) commandCoverageEnabled() bool {
	for _, enabled := range p.rules {
		if enabled {
			return true
		}
	}
	return false
}

func (p *formatter) unsupportedCommand(i, base int, command, reason string) {
	p.removeLineRule(i, InstructionIndent)
	p.removeLineRule(i, SIFIndent)
	p.syntaxOpaque(i)
	message := "unsupported command " + command
	if reason != "" {
		message += " (" + reason + ")"
	}
	p.diagnostic(i, base, erabasic.Unsupported, "unsupported_command", message)
}

func (p *formatter) expressionProblem(i, offset int, err error) {
	if p.Aborted {
		return
	}
	message := err.Error()
	if e, ok := err.(*erabasic.ExprError); ok {
		offset += e.Offset
		message = e.Msg
		if e.Resource {
			p.resourceProblem(i, offset, message)
			return
		}
	}
	offset = min(p.Lines[i].BodyEnd, max(p.Lines[i].Start, offset))
	p.diagnostic(i, offset, erabasic.Unsupported, "unsupported_expression", message)
}

func (p *formatter) syntaxOpaque(i int) {
	if len(p.Opaque) > i {
		p.Opaque[i] = true
	}
}

func dataBodyCommand(p *formatter, command string) bool {
	if command != "DATA" {
		return false
	}
	for _, frame := range p.State.Frames {
		if frame.Kind == "DATA" || frame.Kind == "DATALIST" {
			return true
		}
	}
	return false
}
