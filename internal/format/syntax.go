package format

import (
	"bytes"
	"sort"
	"strings"
	"unicode/utf8"
)

func (p *formatter) syntax(i int, head []byte, prefix int) int {
	if len(head) == 0 || i < 0 || i >= len(p.lines) {
		return -1
	}
	if len(p.opaque) > i && p.opaque[i] {
		return -1
	}
	if bytes.Contains(head, []byte("[[")) {
		p.syntaxOpaque(i)
		return -1
	}
	if syntaxHasProtectedMetadataTail(head) {
		defer p.syntaxOpaque(i)
	}

	line := p.lines[i]
	base := line.start + prefix
	if p.syntaxSession == nil || p.syntaxBase != base {
		p.syntaxSession = p.newExpressionSession(head)
		p.syntaxBase = base
	}
	defer func() { p.syntaxSession = nil }()
	if head[0] == '@' {
		return p.syntaxHeader(i, head, base)
	}
	if head[0] == '#' {
		return p.syntaxDeclaration(i, head, base)
	}

	command, wordEnd := p.commandOf(head)
	p.syntaxSession.skipFormTypes = command == "DEBUGPRINTFORM" || command == "DEBUGPRINTFORML"
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
	if commandUnavailable(command, p.dialect) && !reservedInstructions[command] {
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
	if commandUnavailable(command, p.dialect) {
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
	if rawCommand(command) {
		p.syntaxOpaque(i)
		return -1
	}
	if end, handled := p.syntaxMethod(i, head, base, command, wordEnd); handled {
		return end
	}
	if !syntaxCommand(command) {
		if dataBodyCommand(p, command) {
			p.syntaxOpaque(i)
			return -1
		}
		if reservedInstructions[command] {
			if p.commandCoverageEnabled() {
				p.unsupportedCommand(i, base, command, commandDialectReason(command, p.dialect))
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
			p.unsupportedCommand(i, base, command, commandDialectReason(command, p.dialect))
			return -1
		}
		p.syntaxOpaque(i)
		return -1
	}
	if printVCommand(command) || printFormCommand(command) {
		if wordEnd >= len(head) {
			if command == "RETURNFORM" && p.expressionRulesEnabled() {
				p.syntaxProblem(i, base+wordEnd, "missing FORM argument")
			}
			return -1
		}
		if command == "RETURNFORM" {
			tail := head[wordEnd:]
			if len(tail) == inputSpaceSize(tail, 0, p.allowFullwidthSpace) || len(tail) == 1 && tail[0] == ';' {
				if p.expressionRulesEnabled() {
					p.syntaxProblem(i, base+wordEnd, "missing FORM argument")
				}
				return -1
			}
		}
		bodyStart := wordEnd + inputSpaceSize(head, wordEnd, p.allowFullwidthSpace)
		if bodyStart == wordEnd {
			bodyStart++
		}
		end := len(head)
		if printVCommand(command) && p.inputSpaceIn(head, wordEnd) && syntaxCommandSpacing(command) {
			arg := trimInputLeft(head, bodyStart, end, p.allowFullwidthSpace)
			if arg < end && head[arg] != ';' {
				p.addEdit(i, base+wordEnd, base+arg, []byte{' '}, CommandSpacing)
			}
		}
		var ok bool
		if printVCommand(command) {
			ok = p.syntaxPrintV(i, base+bodyStart, base+end)
		} else if command == "RETURNFORM" {
			ok = p.syntaxReturnForm(i, base+bodyStart, base+end)
		} else {
			ok = p.syntaxForm(i, base+bodyStart, base+end, formLine)
		}
		if !ok {
			p.syntaxOpaque(i)
		}
		return -1
	}

	var body []byte
	var bodyStart, end, comment int
	var hasArg bool
	formCall := callCommand(command) && (strings.Contains(command, "FORM") || command == "FUNC")
	if callCommand(command) {
		var ok bool
		bodyStart, end, ok = registeredRawBodyWithSpace(head, wordEnd, p.allowFullwidthSpace)
		if !ok {
			p.syntaxOpaque(i)
			return -1
		}
		hasArg, comment = wordEnd < len(head), -1
		if !formCall {
			nameEnd, delimiter, valid := callPrefix(head, bodyStart, end)
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
	if command == "FOR" && p.expressionParseEnabled() && (!hasArg || wordEnd < len(head) && head[wordEnd] == ';' || len(trimInput(body, p.allowFullwidthSpace)) == 0) {
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
	if requiredExpressionCommand(command) && p.expressionParseEnabled() && (!hasArg || bodyStart >= end || len(trimInput(head[bodyStart:end], p.allowFullwidthSpace)) == 0) {
		p.syntaxProblem(i, base+wordEnd, "missing command expression")
		p.syntaxOpaque(i)
		return -1
	}
	if !hasArg {
		p.setCodeEnd(i, base+wordEnd)
		return base + wordEnd
	}
	argumentSpace := p.allowFullwidthSpace && !callCommand(command)
	if bodyStart >= len(head) || len(trimInput(body, argumentSpace)) == 0 {
		p.setCodeEnd(i, base+wordEnd)
		return base + wordEnd
	}
	if body[0] == ';' && !mandatoryInlineMarker(body, 0) && !printVCommand(command) && !printFormCommand(command) {
		p.setCodeEnd(i, base+bodyStart)
		return base + bodyStart
	}
	if p.syntaxForSurplus(i, command, base+bodyStart, base+end) || p.syntaxScalarSurplus(i, command, base+bodyStart, base+end) {
		return -1
	}
	mark := len(p.edits)
	if p.inputSpaceIn(head, wordEnd) && syntaxCommandSpacing(command) {
		arg := trimInputLeft(head, bodyStart, end, argumentSpace)
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
		if tokens, err := p.lexSyntaxRange(base+bodyStart, base+end); err == nil && !expressionNeedsDeferral(tokens, p.dialect) {
			p.syntaxProblem(i, base+bodyStart, "RETURN contains an assignment in a comma field")
		}
		p.syntaxOpaque(i)
		return -1
	}
	switch {
	case expressionCommand(command), simpleIntegerCommand(command):
		if command == "SKIPLOG" {
			ok = p.syntaxExpression(i, base+bodyStart, base+end, false)
		} else {
			ok = p.syntaxOrdinaryExpression(i, base+bodyStart, base+end, command == "CASE")
		}
	case simpleStringCommand(command):
		if strings.Contains(command, "FORMS") {
			ok = p.syntaxPrintFormsLiteral(i, base+bodyStart, base+end)
		} else {
			ok = p.syntaxOrdinaryExpression(i, base+bodyStart, base+end, false)
		}
	case callCommand(command):
		ok = p.syntaxCall(i, command, base+bodyStart, base+end)
	case forCommand(command):
		ok = p.syntaxOrdinaryExpression(i, base+bodyStart, base+end, false)
	case structuredListCommand(command):
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
	if callCommand(command) {
		return p.setCallCodeEnd(i, base+bodyStart, base+end)
	}
	codeEnd := trimInputRight(p.src, base+bodyStart, base+end, p.allowFullwidthSpace)
	p.setCodeEnd(i, codeEnd)
	return codeEnd
}

func syntaxHasProtectedMetadataTail(head []byte) bool {
	if len(head) == 0 || (head[0] != '#' && head[0] != '$') {
		return false
	}
	end, found := syntaxCommentEnd(head, 0, len(head))
	return found && end+1 < len(head) && head[end+1] == ';'
}

func (p *formatter) syntaxReturnMode(i int, head []byte, base int, command string, wordEnd int) bool {
	if command != "RETURN" && command != "RETURNFORM" && command != "RETURNF" || !p.expressionParseEnabled() || p.optional || p.conditional() || p.base == 0 || string(head[:wordEnd]) != command || commandFieldHasMandatoryMarker(head) || commandFieldHasDeferredMarker(head) {
		return false
	}
	value := knownValueUnknown
	if command == "RETURNF" {
		_, start, end, _, hasArg := p.instructionBody(head, wordEnd)
		if hasArg {
			parts := p.splitTopLevel(head, start, end, ',')
			if len(parts) == 1 {
				tokens, err := p.lexSyntaxRange(base+start, base+end)
				if err == nil {
					value = p.trustedReturnExpressionType(tokens)
				}
			}
		}
	}
	message := p.returnCommandMessage(command, value)
	if message == "" {
		return false
	}
	p.syntaxProblem(i, base, message)
	p.syntaxOpaque(i)
	return true
}

func (p *formatter) syntaxForSurplus(i int, command string, start, end int) bool {
	if command != "FOR" || !p.expressionParseEnabled() || commandFieldHasDeferredMarker(p.src[start:end]) || commandFieldHasMandatoryMarker(p.src[start:end]) {
		return false
	}
	all := p.splitTopLevel(p.src, start, end, ',')
	parts, surplus := commandPrefixParts(p.src, all, 4, 3, p.allowFullwidthSpace, true)
	if !surplus {
		return false
	}
	for _, field := range all {
		tokens, err := p.lexSyntaxRange(field.start, field.end)
		if err == nil && commandMacroPossibleForDialect(tokens, p.dialect) {
			p.syntaxOpaque(i)
			return true
		}
	}
	mark := len(p.edits)
	if offset, message, valid := p.genericCommandRoleValid(command, start, end); !valid {
		p.syntaxProblem(i, offset, message)
	} else if p.syntaxOrdinaryExpression(i, start, parts[len(parts)-1].end, false) {
		p.ignoredCommandArguments(i, start, command)
	}
	p.truncateEdits(mark)
	p.syntaxOpaque(i)
	return true
}

func (p *formatter) syntaxScalarSurplus(i int, command string, start, end int) bool {
	if !p.expressionParseEnabled() || !(simpleIntegerCommand(command) && command != "RETURN" || simpleStringCommand(command) && command != "RETURNF") || commandFieldHasDeferredMarker(p.src[start:end]) || commandFieldHasMandatoryMarker(p.src[start:end]) {
		return false
	}
	parts, surplus := commandPrefixParts(p.src, p.splitTopLevel(p.src, start, end, ','), 1, 1, p.allowFullwidthSpace, true)
	if !surplus {
		return false
	}
	first := parts[0]
	tokens, err := p.lexSyntaxRange(first.start, first.end)
	if err == nil && commandMacroPossibleForDialect(tokens, p.dialect) {
		p.syntaxOpaque(i)
		return true
	}
	mark := len(p.edits)
	if offset, message, valid := p.genericCommandRoleValid(command, first.start, first.end); !valid {
		p.syntaxProblem(i, offset, message)
	} else if p.syntaxOrdinaryExpression(i, first.start, first.end, false) {
		p.ignoredCommandArguments(i, start, command)
	}
	p.truncateEdits(mark)
	p.syntaxOpaque(i)
	return true
}

func (p *formatter) syntaxIncrement(i int, head []byte, base int) bool {
	end, _ := p.syntaxCommentEnd(head, 0, len(head))
	end = trimInputRight(head, 0, end, p.allowFullwidthSpace)
	if end < 2 {
		return false
	}
	lhs := head[:end-2]
	op := head[end-2 : end]
	if bytes.Equal(head[:2], []byte("++")) || bytes.Equal(head[:2], []byte("--")) {
		op, lhs = head[:2], head[2:end]
	}
	if !(bytes.Equal(op, []byte("++")) || bytes.Equal(op, []byte("--"))) || !p.assignmentLHS(lhs) {
		return false
	}
	lhsStart := base
	if bytes.Equal(head[:2], op) {
		lhsStart += 2
	}
	if p.expressionParseEnabled() {
		if tokens, err := p.lexSyntaxRange(lhsStart, lhsStart+len(lhs)); err == nil {
			if builtinVariableReadOnly(tokens, p.dialect) || !setterMacroPossibleForDialect(tokens, p.dialect) && knownVariableType(tokens) == knownValueString {
				p.syntaxProblem(i, lhsStart, "increment requires a mutable numeric destination")
				p.syntaxOpaque(i)
				return true
			}
		}
	}
	if p.syntaxExpression(i, base, base+end, false) {
		p.setCodeEnd(i, base+end)
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
		stop, err := session.scanFormAt(expressionSourceOriginal, offset+pos-start, offset+end-start, formComma)
		n := stop - (offset + pos - start)
		if err != nil {
			p.expressionProblem(i, pos, shiftExpressionError(err, -(offset+pos-start)))
			return false
		}
		item, err := formEditsWithSession(session, expressionSourceOriginal, p.src[pos:pos+n], offset+pos-start, p.rules, formComma, true)
		if err != nil && firstErr == nil {
			firstErr = shiftExpressionError(err, pos-start)
		}
		for _, e := range item {
			e.start += pos - start
			e.end += pos - start
			edits = append(edits, e)
		}
		pos += n
		if pos < end && p.src[pos] == 0 {
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
	if p.expressionParseEnabled() && start <= end && end <= len(p.src) {
		session, offset := p.expressionSessionFor(start, end)
		bodyEnd, unsupported, err := expressionBodyEndWithSession(session, expressionSourceOriginal, p.src[start:end], offset)
		if err == nil && !unsupported {
			tokens, err := session.lexRange(expressionSourceOriginal, offset, offset+bodyEnd, 0)
			if err == nil {
				if syntaxExpressionEndsWithHash(tokens) {
					if !expressionNeedsDeferral(tokens, p.dialect) {
						p.expressionProblem(i, start, expressionShapeError(tokens[len(tokens)-1], "expression has an invalid operator"))
					}
					return false
				}
				if err := provenExpressionShape(tokens, p.dialect, caseMode); err != nil {
					p.expressionProblem(i, start, err)
					return false
				}
			}
		}
	}
	return p.syntaxExpression(i, start, end, caseMode)
}

func syntaxExpressionEndsWithHash(tokens []exprToken) bool {
	return len(tokens) != 0 && tokens[len(tokens)-1].kind == exprOperator && string(tokens[len(tokens)-1].text) == "#"
}

func (p *formatter) returnHasCommaAssignment(start, end int) bool {
	parts := p.splitTopLevel(p.src, start, end, ',')
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts[1:] {
		left := p.trimInputLeft(part.start, part.end)
		right := p.trimInputRight(left, part.end)
		if left >= right {
			continue
		}
		_, op, ok := assignmentBoundary(p.src[left:right])
		if ok && op.end-op.start == 1 && p.src[left+op.start] == '=' {
			return true
		}
	}
	return false
}

func (p *formatter) syntaxExpression(i, start, end int, caseMode bool) bool {
	if start > end || end > len(p.src) {
		return false
	}
	if !p.expressionParseEnabled() {
		return true
	}
	session, offset := p.expressionSessionFor(start, end)
	defer p.addFormAdvisories(i, session, offset, offset+end-start, start)
	excluded, err := expressionExclusionWithSession(session, p.src[start:end])
	if err != nil {
		p.expressionProblem(i, start, err)
		return false
	}
	if excluded {
		p.syntaxOpaque(i)
		return false
	}
	if tokens, err := session.lexRange(expressionSourceOriginal, offset, offset+end-start, 0); err == nil {
		if err := provenKnownExpressionBoundaries(tokens, p.dialect); err != nil {
			p.expressionProblem(i, start, err)
			return false
		}
	}
	edits, err := expressionEditsWithSession(session, expressionSourceOriginal, p.src[start:end], offset, caseMode, p.rules, true)
	if err != nil {
		p.expressionProblem(i, start, err)
		return false
	}
	p.addSyntaxEdits(i, start, edits)
	return true
}

func (p *formatter) syntaxForm(i, start, end int, mode formMode) bool {
	if start > end || end > len(p.src) {
		return false
	}
	if !p.expressionRulesEnabled() {
		return true
	}
	session, offset := p.expressionSessionFor(start, end)
	defer p.addFormAdvisories(i, session, offset, offset+end-start, start)
	edits, err := formEditsWithSession(session, expressionSourceOriginal, p.src[start:end], offset, p.rules, mode, true)
	p.addSyntaxEdits(i, start, edits)
	if err != nil {
		p.expressionProblem(i, start, err)
		return len(edits) > 0
	}
	return true
}

func (p *formatter) addFormAdvisories(i int, session *expressionSession, sourceStart, sourceEnd, absoluteBase int) {
	p.addFormAdvisoriesMapped(i, session, sourceStart, sourceEnd, func(offset int) int {
		return absoluteBase + offset - sourceStart
	})
}

func (p *formatter) addFormAdvisoriesMapped(i int, session *expressionSession, sourceStart, sourceEnd int, mapOffset func(int) int) {
	if p.aborted {
		return
	}
	previousEnd, previousColumn := len(p.diagnostics), 0
	ordered := true
	for _, advisory := range session.formAdvisoriesIn(expressionSourceOriginal, sourceStart, sourceEnd) {
		if p.aborted {
			return
		}
		offset := mapOffset(advisory.offset)
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
			if diagnostic.Line == line && diagnostic.Column == column && diagnostic.Severity == Warning && diagnostic.Code == advisory.code {
				duplicate = true
				break
			}
		}
		if !duplicate {
			p.warning(i, offset, advisory.code, advisory.message)
		}
	}
}

func (p *formatter) syntaxPrintFormsLiteral(i, start, end int) bool {
	if !p.expressionRulesEnabled() {
		return true
	}
	left := trimInputLeft(p.src, start, end, p.allowFullwidthSpace)
	right := trimInputRight(p.src, left, end, p.allowFullwidthSpace)
	if left >= right || p.src[left] != '"' || quotedEnd(p.src[left:right], 0) != right-left {
		return p.syntaxOrdinaryExpression(i, start, end, false)
	}
	innerStart, innerEnd := left+1, right-1
	if bytes.IndexByte(p.src[innerStart:innerEnd], '\\') >= 0 {
		return p.syntaxOrdinaryExpression(i, start, end, false)
	}
	session, offset := p.expressionSessionFor(innerStart, innerEnd)
	defer p.addFormAdvisories(i, session, offset, offset+innerEnd-innerStart, innerStart)
	edits, err := formEditsWithSession(session, expressionSourceOriginal, p.src[innerStart:innerEnd], offset, p.rules, formLine, true)
	p.addSyntaxEdits(i, innerStart, edits)
	if err != nil {
		p.expressionProblem(i, innerStart, shiftExpressionError(err, 0))
		return len(edits) > 0
	}
	return true
}

func (p *formatter) expressionSessionFor(start, end int) (*expressionSession, int) {
	if p.syntaxSession != nil && start >= p.syntaxBase && end-p.syntaxBase <= len(p.syntaxSession.sources[expressionSourceOriginal]) {
		return p.syntaxSession, start - p.syntaxBase
	}
	session := p.newExpressionSession(p.src[start:end])
	session.onResource = func(offset int, message string) {
		p.resourceProblem(p.parseLine, start+offset, message)
	}
	return session, 0
}

func (p *formatter) lexSyntaxRange(start, end int) ([]exprToken, error) {
	session, offset := p.expressionSessionFor(start, end)
	return session.lexRange(expressionSourceOriginal, offset, offset+end-start, start)
}

func (p *formatter) expressionRulesEnabled() bool {
	for _, id := range []RuleID{BinarySpacing, TernarySpacing, UnarySpacing, CommaSpacing, DelimiterSpacing, IndexSpacing, CaseSpacing, TokenSpacing, FormPadding, FormConditionSpacing} {
		if p.rules[id] {
			return true
		}
	}
	return false
}

func (p *formatter) expressionParseEnabled() bool {
	return p.expressionRulesEnabled() || p.rules[CodeTrailingWhitespace] || p.rules[InlineCommentSpacing] || p.rules[CommentTrailingWhitespace] || p.rules[CommentPrefixSpacing]
}

type printVProjection struct {
	src      []byte
	segments []printVProjectionSegment
}

type printVProjectionSegment struct {
	projectedStart int
	projectedEnd   int
	originalStart  int
	originalEnd    int
}

const projectionSegmentMetadataBytes = 32

func (p printVProjection) canMergeSegment(segment printVProjectionSegment) bool {
	if len(p.segments) == 0 {
		return false
	}
	last := p.segments[len(p.segments)-1]
	return last.projectedEnd == segment.projectedStart && last.originalEnd == segment.originalStart &&
		last.projectedEnd-last.projectedStart == last.originalEnd-last.originalStart &&
		segment.projectedEnd-segment.projectedStart == segment.originalEnd-segment.originalStart
}

func (p *printVProjection) appendSegment(segment printVProjectionSegment) {
	if segment.projectedStart == segment.projectedEnd {
		return
	}
	if p.canMergeSegment(segment) {
		last := &p.segments[len(p.segments)-1]
		last.projectedEnd = segment.projectedEnd
		last.originalEnd = segment.originalEnd
		return
	}
	p.segments = append(p.segments, segment)
}

func (p printVProjection) originalOffset(offset int) int {
	if len(p.segments) == 0 {
		return min(len(p.src), max(0, offset))
	}
	if offset < 0 {
		return p.segments[0].originalStart
	}
	if offset >= len(p.src) {
		return p.segments[len(p.segments)-1].originalEnd
	}
	index := sort.Search(len(p.segments), func(index int) bool {
		return p.segments[index].projectedEnd >= offset
	})
	if index >= len(p.segments) {
		return p.segments[len(p.segments)-1].originalEnd
	}
	segment := p.segments[index]
	if offset <= segment.projectedStart {
		return segment.originalStart
	}
	if offset >= segment.projectedEnd {
		return segment.originalEnd
	}
	if segment.projectedEnd-segment.projectedStart == segment.originalEnd-segment.originalStart {
		return segment.originalStart + offset - segment.projectedStart
	}
	return segment.originalStart
}

func (p *printVProjection) truncate(end int) {
	if end < 0 {
		end = 0
	}
	if end >= len(p.src) {
		return
	}
	originalEnd := p.originalOffset(end)
	p.src = p.src[:end]
	for len(p.segments) != 0 && p.segments[len(p.segments)-1].projectedStart >= end {
		p.segments = p.segments[:len(p.segments)-1]
	}
	if len(p.segments) != 0 {
		last := &p.segments[len(p.segments)-1]
		last.projectedEnd = end
		last.originalEnd = originalEnd
	}
}

func mapPrintVError(err error, projection printVProjection) error {
	e, ok := err.(*exprError)
	if !ok {
		return err
	}
	shifted := *e
	shifted.offset = projection.originalOffset(e.offset)
	return &shifted
}

func (p *formatter) printVProjection(i, start, end int) (printVProjection, int, bool, bool) {
	src := p.src[start:end]
	projection := printVProjection{src: src}
	codeEnd := len(src)
	rawEnd := false
	codeEndSet := false
	projectedBytes := 0
	rawAtoms := 0
	materialized := false
	unsupportedFullwidth := false

	tooLarge := func(offset int) bool {
		p.resourceProblem(i, start+offset, "expression work exceeds the formatter resource limit")
		return false
	}
	reserveProjectionSegment := func(segment printVProjectionSegment) bool {
		if segment.projectedStart == segment.projectedEnd || projection.canMergeSegment(segment) {
			return true
		}
		return p.reserveMetadata(i, start+segment.originalStart, projectionSegmentMetadataBytes)
	}
	ensure := func(pos int) bool {
		if materialized {
			return true
		}
		if projectedBytes > maxSyntaxBytes || pos > maxSyntaxBytes {
			return tooLarge(pos)
		}
		var prefixSegment printVProjectionSegment
		if pos != 0 {
			prefixSegment = printVProjectionSegment{projectedEnd: pos, originalEnd: pos}
			if !reserveProjectionSegment(prefixSegment) {
				return false
			}
		}
		materialized = true
		projection.src = make([]byte, 0, min(projectedBytes+16, maxSyntaxBytes))
		projection.src = append(projection.src, src[:pos]...)
		if pos != 0 {
			projection.appendSegment(prefixSegment)
		}
		return true
	}
	appendMapped := func(text []byte, from, to int) bool {
		start := len(projection.src)
		segment := printVProjectionSegment{projectedStart: start, projectedEnd: start + len(text), originalStart: from, originalEnd: to}
		if !reserveProjectionSegment(segment) {
			return false
		}
		projection.src = append(projection.src, text...)
		projection.appendSegment(segment)
		return true
	}
	appendOriginal := func(from, to int) bool {
		if to < from || projectedBytes > maxSyntaxBytes-to+from {
			return tooLarge(from)
		}
		projectedBytes += to - from
		if materialized {
			start := len(projection.src)
			segment := printVProjectionSegment{projectedStart: start, projectedEnd: start + to - from, originalStart: from, originalEnd: to}
			if !reserveProjectionSegment(segment) {
				return false
			}
			projection.src = append(projection.src, src[from:to]...)
			if to != from {
				projection.appendSegment(segment)
			}
		}
		return true
	}
	appendSpace := func(from, to int) bool {
		size := min(to-from, 2)
		if projectedBytes > maxSyntaxBytes-size {
			return tooLarge(from)
		}
		projectedBytes += size
		if materialized {
			start := len(projection.src)
			first := printVProjectionSegment{projectedStart: start, projectedEnd: start + 1, originalStart: from, originalEnd: to}
			if size == 2 {
				first = printVProjectionSegment{projectedStart: start, projectedEnd: start + 1, originalStart: from, originalEnd: from}
			}
			if !reserveProjectionSegment(first) {
				return false
			}
			projection.src = append(projection.src, ' ')
			if size == 1 {
				projection.appendSegment(first)
			} else {
				projection.appendSegment(first)
				projection.src = append(projection.src, ' ')
				second := printVProjectionSegment{projectedStart: start + 1, projectedEnd: start + 2, originalStart: from, originalEnd: to}
				if !reserveProjectionSegment(second) {
					return false
				}
				projection.appendSegment(second)
			}
		}
		return true
	}

	for pos := 0; pos < len(src); {
		if next, atom := p.syntaxAtomEnd(src, pos, len(src)); atom {
			if next < 0 {
				p.syntaxProblem(i, start+pos, "unterminated PRINTV expression atom")
				return projection, 0, false, false
			}
			if !materialized && next-pos > maxSyntaxBytes-projectedBytes {
				tooLarge(pos)
				return projection, 0, false, false
			}
			if materialized {
				if !appendOriginal(pos, next) {
					return projection, 0, false, false
				}
			} else {
				projectedBytes += next - pos
			}
			pos = next
			continue
		}
		if src[pos] == ';' {
			if mandatoryInlineMarker(src, pos) {
				if !materialized && !ensure(pos) {
					return projection, 0, false, false
				}
				if !appendOriginal(pos, pos+3) {
					return projection, 0, false, false
				}
				pos += 3
				continue
			}
			if materialized {
				codeEnd = len(projection.src)
			} else {
				codeEnd = pos
			}
			if unsupportedFullwidth {
				rawEnd = true
			}
			codeEndSet = true
			if pos+3 <= len(src) && executableMarker(src, pos) {
				if !materialized {
					projection.src = src[:pos+3]
				} else if !appendOriginal(pos, pos+3) {
					return projection, 0, false, false
				}
			} else if !materialized {
				projection.src = src[:pos]
			}
			break
		}
		if src[pos] == '\'' {
			next := pos + 1
			for next < len(src) && src[next] != ',' && src[next] != 0 {
				if src[next] == '\\' {
					if next+1 == len(src) || src[next+1] == 0 {
						p.syntaxProblem(i, start+next, "missing character after PRINTV escape")
						return projection, 0, false, false
					}
					next = escapedEnd(src, next)
				} else {
					next++
				}
			}
			if !materialized && unsupportedFullwidth {
				return projection, len(src), true, true
			}
			if !materialized && !ensure(pos) {
				return projection, 0, false, false
			}
			if projectedBytes == maxSyntaxBytes {
				return projection, 0, false, tooLarge(pos)
			}
			if rawAtoms == maxExpressionTokens {
				return projection, 0, false, tooLarge(pos)
			}
			rawAtoms++
			projectedBytes++
			if !appendMapped([]byte{'X'}, pos, next) {
				return projection, 0, false, false
			}
			rawEnd = next == len(src)
			if next < len(src) && src[next] == 0 {
				codeEnd, rawEnd = len(projection.src), true
				codeEndSet = true
				break
			}
			pos = next
			continue
		}
		if r, size := utf8.DecodeRune(src[pos:]); r == '\u3000' {
			if !p.allowFullwidthSpace {
				unsupportedFullwidth = true
				if materialized {
					if !appendOriginal(pos, pos+size) {
						return projection, 0, false, false
					}
				}
				pos += size
				continue
			}
			next := pos + size
			for next < len(src) {
				r, size = utf8.DecodeRune(src[next:])
				if r != '\u3000' && r != ' ' && r != '\t' {
					break
				}
				next += size
			}
			if !materialized && !ensure(pos) {
				return projection, 0, false, false
			}
			if !appendSpace(pos, next) {
				return projection, 0, false, false
			}
			pos = next
			continue
		}
		if src[pos] == ' ' || src[pos] == '\t' {
			next := pos + 1
			for next < len(src) && (src[next] == ' ' || src[next] == '\t') {
				next++
			}
			if next-pos > 1 {
				if !materialized && !ensure(pos) {
					return projection, 0, false, false
				}
				if !appendSpace(pos, next) {
					return projection, 0, false, false
				}
				pos = next
				continue
			}
		}
		if !materialized {
			if projectedBytes == maxSyntaxBytes {
				return projection, 0, false, tooLarge(pos)
			}
			projectedBytes++
		} else if !appendOriginal(pos, pos+1) {
			return projection, 0, false, false
		}
		pos++
	}
	if unsupportedFullwidth && !materialized {
		return projection, len(src), true, true
	}
	if !codeEndSet {
		codeEnd = len(projection.src)
	}
	return projection, codeEnd, rawEnd, true
}

func (p *formatter) syntaxPrintV(i, start, end int) bool {
	if !p.expressionParseEnabled() {
		return true
	}
	projection, codeEnd, rawEnd, ok := p.printVProjection(i, start, end)
	if !ok {
		return false
	}
	newSession := func() *expressionSession {
		session := p.newExpressionSession(projection.src)
		session.onResource = func(offset int, message string) {
			p.resourceProblem(i, start+projection.originalOffset(offset), message)
		}
		return session
	}
	active, _, err := expressionBodyEndWithSession(newSession(), expressionSourceOriginal, projection.src, 0)
	if err != nil {
		p.expressionProblem(i, start, mapPrintVError(err, projection))
		return false
	}
	if active < len(projection.src) {
		if projection.src[active] == 0 {
			projection.truncate(active)
			codeEnd, rawEnd = active, true
		}
	}
	session := newSession()
	defer p.addFormAdvisoriesMapped(i, session, 0, len(projection.src), func(offset int) int {
		return start + projection.originalOffset(offset)
	})
	if expressionExcludedWithSession(session, projection.src) {
		return false
	}
	if trimInputLeft(projection.src, 0, codeEnd, p.allowFullwidthSpace) >= trimInputRight(projection.src, 0, codeEnd, p.allowFullwidthSpace) {
		if !rawEnd {
			p.recordCodeEnd(i, start+projection.originalOffset(codeEnd))
		}
		return true
	}
	spans := splitTopLevel(projection.src, 0, codeEnd, ',', session)
	for index, span := range spans {
		if trimInputLeft(projection.src, span.start, span.end, p.allowFullwidthSpace) < trimInputRight(projection.src, span.start, span.end, p.allowFullwidthSpace) {
			continue
		}
		if index == len(spans)-1 && index > 0 {
			continue
		}
		p.syntaxProblem(i, start+projection.originalOffset(span.start), "PRINTV contains an empty expression")
		return false
	}
	macroPrefix := false
	for _, span := range spans {
		left := trimInputLeft(projection.src, span.start, span.end, p.allowFullwidthSpace)
		right := trimInputRight(projection.src, left, span.end, p.allowFullwidthSpace)
		if left >= right {
			continue
		}
		tokens, err := session.lexRange(expressionSourceOriginal, left, right, left)
		if err != nil {
			macroPrefix = true
			continue
		}
		if expressionNeedsDeferral(tokens, p.dialect) {
			macroPrefix = true
			continue
		}
		if !macroPrefix {
			if err := provenExpressionShape(tokens, p.dialect, false); err != nil {
				p.expressionProblem(i, start, mapPrintVError(err, projection))
				return false
			}
		}
		if !macroPrefix && builtinScalarIndexesMissing(tokens, p.dialect) {
			p.expressionProblem(i, start+projection.originalOffset(left), &exprError{msg: "scalar variable requires indexes"})
			return false
		}
	}
	if bodyEnd, unsupported, err := expressionBodyEndWithSession(session, expressionSourceOriginal, projection.src, 0); err == nil && !unsupported {
		if tokens, err := session.lexRange(expressionSourceOriginal, 0, bodyEnd, 0); err == nil {
			if err := provenExpressionShape(tokens, p.dialect, false); err != nil {
				p.expressionProblem(i, start, mapPrintVError(err, projection))
				return false
			}
		}
	}
	edits, err := expressionEditsWithSession(session, expressionSourceOriginal, projection.src, 0, false, p.rules, true)
	if err != nil {
		p.expressionProblem(i, start, mapPrintVError(err, projection))
		return false
	}
	for n := range edits {
		edits[n].start = projection.originalOffset(edits[n].start)
		edits[n].end = projection.originalOffset(edits[n].end)
	}
	p.addSyntaxEdits(i, start, edits)
	if !rawEnd {
		p.recordCodeEnd(i, start+projection.originalOffset(codeEnd))
	}
	return true
}

func (p *formatter) syntaxCommaGap(i, comma, end int) {
	p.syntaxRegisteredCommaGap(i, comma, end, 0)
}

func (p *formatter) syntaxCall(i int, command string, start, end int) (ok bool) {
	mark := len(p.edits)
	defer func() {
		if !ok {
			p.truncateEdits(mark)
		}
	}()
	if start >= end {
		return true
	}
	formName := strings.Contains(command, "FORM") || command == "FUNC"
	if !p.expressionParseEnabled() || formName && !p.expressionRulesEnabled() {
		return true
	}
	nameEnd, delimiter, _ := callPrefix(p.src, start, end)
	if formName {
		session, offset := p.expressionSessionFor(start, end)
		if n, err := session.scanFormAt(expressionSourceOriginal, offset, offset+end-start, formCall); err == nil {
			nameEnd = start + n - offset
			delimiter = nameEnd
			for delimiter < end {
				size := inputSpaceSize(p.src, delimiter, p.allowFullwidthSpace)
				if size == 0 {
					break
				}
				delimiter += size
			}
			if delimiter >= end {
				delimiter = -1
			}
		} else {
			p.syntaxProblem(i, start, err.Error())
			return false
		}
	}
	if command == "FUNC" && !p.tryGotoListFuncAllowed(i, start, end, delimiter) {
		return false
	}
	nameStart := start
	nameStart = trimInputLeft(p.src, nameStart, nameEnd, false)
	nameEnd = trimCallTargetRight(p.src, nameStart, nameEnd)
	if nameStart == nameEnd {
		p.syntaxProblem(i, start, "missing call target")
		return false
	}
	if formName {
		if !p.syntaxForm(i, nameStart, nameEnd, formCall) {
			return false
		}
	}
	if delimiter < 0 || delimiter >= end {
		return true
	}
	if p.src[delimiter] == ';' {
		return true
	}
	argumentsStart := delimiter
	beforeOpen := nameEnd
	if p.src[delimiter] == '[' {
		close := p.matchingDelimiter(p.src, delimiter, end, '[', ']')
		if close < 0 {
			p.syntaxProblem(i, delimiter, "unclosed call subname list")
			return false
		}
		p.syntaxCallDelimiter(i, nameEnd, delimiter, close)
		if !p.syntaxOrdinaryExpression(i, delimiter+1, close, false) {
			return false
		}
		delimiter = close + 1
		gapStart := delimiter
		for delimiter < end {
			size := inputSpaceSize(p.src, delimiter, p.allowFullwidthSpace)
			if size == 0 {
				break
			}
			delimiter += size
		}
		beforeOpen = gapStart
		if delimiter >= end || p.src[delimiter] == ';' {
			return true
		}
	}
	if p.src[delimiter] == '(' {
		close := p.matchingDelimiter(p.src, delimiter, end, '(', ')')
		if close < 0 {
			p.syntaxProblem(i, delimiter, "unclosed call argument list")
			return false
		}
		p.syntaxCallDelimiter(i, beforeOpen, delimiter, close)
		if !p.syntaxOrdinaryExpression(i, delimiter+1, close, false) {
			return false
		}
		return p.syntaxCallTail(i, argumentsStart, close, end)
	}
	if p.src[delimiter] == ',' {
		p.syntaxRegisteredCommaGap(i, delimiter, end, beforeOpen)
		return p.syntaxOrdinaryExpression(i, delimiter+1, end, false)
	}
	return true
}

func (p *formatter) syntaxCallTail(i, start, close, end int) bool {
	tail := p.trimInputLeft(close+1, end)
	if tail >= end || p.src[tail] == ';' || p.src[tail] == 0 {
		return true
	}
	if commandFieldHasMandatoryMarker(p.src[start:end]) || commandFieldHasDeferredMarker(p.src[start:end]) {
		return true
	}
	src := p.src[tail:end]
	if nameEnd, identifier := labelIdentifierEndWithSession(src, 0, p.scanSession(p.src, tail, end)); identifier && !setterKnownNameForDialect(src[:nameEnd], p.dialect) {
		return true
	}
	tokens, err := p.lexSyntaxRange(start, close+1)
	if err != nil || expressionNeedsDeferral(tokens, p.dialect) {
		return true
	}
	p.syntaxProblem(i, tail, "unexpected tokens after call argument list")
	return false
}

func (p *formatter) tryGotoListFuncAllowed(i, start, end, delimiter int) bool {
	if len(p.state.frames) == 0 || p.state.frames[len(p.state.frames)-1].kind != "FUNCLIST" || p.state.frames[len(p.state.frames)-1].listCommand != "TRYGOTOLIST" {
		return true
	}
	if p.conditional() || p.optional || p.preserved || bytes.Contains(p.src[start:end], []byte("[[")) {
		return true
	}
	for delimiter >= 0 && delimiter < end {
		left, right := delimiter+1, end
		switch p.src[delimiter] {
		case '[':
			right = p.matchingDelimiter(p.src, delimiter, end, '[', ']')
		case '(':
			right = p.matchingDelimiter(p.src, delimiter, end, '(', ')')
		case ',':
		default:
			return true
		}
		if right < left {
			return true
		}
		if p.trimInputLeft(left, right) < right {
			tokens, err := p.lexSyntaxRange(left, right)
			if err != nil || expressionNeedsDeferral(tokens, p.dialect) {
				return true
			}
			p.syntaxProblem(i, delimiter, "TRYGOTOLIST FUNC cannot have subnames or arguments")
			return false
		}
		if p.src[delimiter] != '[' {
			return true
		}
		delimiter = p.trimInputLeft(right+1, end)
	}
	return true
}

func (p *formatter) syntaxCallDelimiter(i, targetEnd, open, close int) {
	if !p.rules[DelimiterSpacing] {
		return
	}
	p.addEdit(i, targetEnd, open, nil, DelimiterSpacing)
	if close <= open+1 {
		return
	}
	left := trimInputLeft(p.src, open+1, close, p.allowFullwidthSpace)
	right := trimInputRight(p.src, open+1, close, p.allowFullwidthSpace)
	if left >= right {
		p.addEdit(i, open+1, close, nil, DelimiterSpacing)
		return
	}
	if !mandatoryInlineMarker(p.src, left) {
		p.addEdit(i, open+1, left, nil, DelimiterSpacing)
	}
	if !mandatoryInlineMarker(p.src[:right], right-3) {
		p.addEdit(i, right, close, nil, DelimiterSpacing)
	}
}

func (p *formatter) syntaxAssignment(i int, head []byte, base int, op assignmentOp) int {
	opStart := base + op.start
	opEnd := base + op.end
	lhsStart := base
	lhsStart = trimInputLeft(p.src, lhsStart, opStart, p.allowFullwidthSpace)
	if p.expressionParseEnabled() {
		if tokens, err := p.lexSyntaxRange(lhsStart, opStart); err == nil && builtinVariableReadOnly(tokens, p.dialect) {
			p.syntaxProblem(i, lhsStart, "assignment destination is read-only")
			p.syntaxOpaque(i)
			return -1
		}
	}
	if p.rules[AssignmentLeftSpacing] {
		left := opStart
		left = trimInputRight(p.src, lhsStart, left, p.allowFullwidthSpace)
		p.addEdit(i, left, opStart, []byte{' '}, AssignmentLeftSpacing)
	}
	if !p.syntaxExpression(i, lhsStart, opStart, false) {
		p.syntaxOpaque(i)
	}

	end := base + len(head)
	comment := false
	if !op.plain {
		end, comment = p.syntaxCommentEnd(p.src, opEnd, end)
	}
	rhsStart := opEnd
	rhsStart = trimInputLeft(p.src, rhsStart, end, p.allowFullwidthSpace && !op.plain)
	if rhsStart < end && (op.plain || p.src[rhsStart] != ';') && p.rules[AssignmentRightSpacing] {
		p.addEdit(i, opEnd, rhsStart, []byte{' '}, AssignmentRightSpacing)
	}
	if op.plain {
		return -1
	}
	if rhsStart < end && !p.syntaxOrdinaryExpression(i, rhsStart, end, false) {
		p.syntaxOpaque(i)
		return -1
	}
	if comment {
		end = trimInputRight(p.src, rhsStart, end, p.allowFullwidthSpace)
		p.setCodeEnd(i, end)
		return end
	}
	end = trimInputRight(p.src, rhsStart, end, p.allowFullwidthSpace)
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
		col += utf8.RuneCount(p.src[pos:start])
		pos = start
		if p.rules[e.change.Rule] && !bytes.Equal(p.src[start:end], e.text) {
			if !p.reserveReplacementBytes(start, end, len(e.text)) {
				return
			}
			if !p.appendEdit(edit{
				start: start,
				end:   end,
				text:  e.text,
				change: Change{
					Rule:   e.change.Rule,
					Line:   i + 1,
					Column: col,
				},
			}) {
				return
			}
		}
	}
}

func (p *formatter) syntaxProblem(i, offset int, message string) {
	if p.aborted {
		return
	}
	p.diagnostic(i, offset, Unsupported, "unsupported_syntax", message)
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
	p.diagnostic(i, base, Unsupported, "unsupported_command", message)
}

func (p *formatter) expressionProblem(i, offset int, err error) {
	if p.aborted {
		return
	}
	message := err.Error()
	if e, ok := err.(*exprError); ok {
		offset += e.offset
		message = e.msg
		if e.resource {
			p.resourceProblem(i, offset, message)
			return
		}
	}
	offset = min(p.lines[i].bodyEnd, max(p.lines[i].start, offset))
	p.diagnostic(i, offset, Unsupported, "unsupported_expression", message)
}

func (p *formatter) syntaxOpaque(i int) {
	if len(p.opaque) > i {
		p.opaque[i] = true
	}
}

func (p *formatter) setCallCodeEnd(i, start, end int) int {
	if p.codeEnds == nil || i < 0 || i >= len(p.lines) {
		return end
	}
	l := p.lines[i]
	if bytes.IndexByte(p.src[l.start:l.bodyEnd], 0) >= 0 {
		p.syntaxOpaque(i)
		return end
	}
	nameEnd, delimiter, _ := callPrefix(p.src, start, end)
	if delimiter < 0 || p.src[delimiter] == ';' {
		end = trimCallTargetRight(p.src, start, nameEnd)
	} else {
		end = trimInputRight(p.src, start, end, p.allowFullwidthSpace)
	}
	if end >= l.indentEnd+3 && mandatoryInlineMarker(p.src[:end], end-3) {
		delete(p.codeEnds, i)
	} else {
		p.codeEnds[i] = end
	}
	return end
}

func (p *formatter) setCodeEnd(i, end int) {
	if p.codeEnds == nil || i < 0 || i >= len(p.lines) {
		return
	}
	l := p.lines[i]
	if bytes.IndexByte(p.src[l.start:l.bodyEnd], 0) >= 0 {
		p.syntaxOpaque(i)
		return
	}
	if end < l.indentEnd || end > l.bodyEnd {
		return
	}
	end, _ = p.syntaxCommentEnd(p.src, l.indentEnd, end)
	p.recordCodeEnd(i, end)
}

func (p *formatter) recordCodeEnd(i, end int) {
	l := p.lines[i]
	end = trimInputRight(p.src, l.indentEnd, end, p.allowFullwidthSpace)
	if end >= l.indentEnd+3 && mandatoryInlineMarker(p.src[:end], end-3) {
		delete(p.codeEnds, i)
		return
	}
	p.codeEnds[i] = end
}

func syntaxCommandSpacing(command string) bool {
	return expressionCommand(command) || simpleIntegerCommand(command) || simpleStringCommand(command) || printVCommand(command) || callCommand(command) || forCommand(command) || structuredListCommand(command)
}

type assignmentOp struct {
	start int
	end   int
	plain bool
}

func assignmentBoundary(src []byte, sessions ...*expressionSession) (int, assignmentOp, bool) {
	if bytes.IndexByte(src, 0) >= 0 {
		end, _ := syntaxCommentEnd(src, 0, len(src), sessions...)
		src = src[:end]
	}
	depth := 0
	for i := 0; i < len(src); i++ {
		if next, atom := syntaxAtomEnd(src, i, len(src), sessions...); atom {
			if next < 0 {
				return 0, assignmentOp{}, false
			}
			i = next - 1
			continue
		}
		switch src[i] {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case ';':
			return 0, assignmentOp{}, false
		}
		if depth != 0 {
			continue
		}
		for _, op := range []string{"<<=", ">>=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "'=", "==", "="} {
			if !bytes.HasPrefix(src[i:], []byte(op)) {
				continue
			}
			if op == "=" && i > 0 && strings.ContainsRune("<>!", rune(src[i-1])) {
				continue
			}
			return i, assignmentOp{start: i, end: i + len(op), plain: op == "=" || op == "=="}, true
		}
	}
	return 0, assignmentOp{}, false
}

func (p *formatter) assignmentLHS(src []byte) bool {
	name, ok := p.variableLHS(src)
	return ok && !reservedInstructions[name] && !methodDialect(builtinMethodMasks[name], p.dialect)
}

func (p *formatter) variableLHS(src []byte) (string, bool) {
	src = p.trimInputIn(src)
	session := p.newExpressionSession(src)
	tokens, err := session.lexRange(expressionSourceOriginal, 0, len(src), 0)
	return variableLHSFromTokens(tokens, err)
}

func variableLHSFromTokens(tokens []exprToken, err error) (string, bool) {
	if err != nil || len(tokens) == 0 || tokens[0].kind != exprIdentifier {
		return "", false
	}
	next := 1
	if next < len(tokens) && string(tokens[next].text) == "@" {
		if next+1 >= len(tokens) || tokens[next+1].kind != exprIdentifier {
			return "", false
		}
		next += 2
	}
	return strings.ToUpper(string(tokens[0].text)), len(tokens) == next || string(tokens[next].text) == ":" || string(tokens[next].text) == "["
}

func instructionBodyWithSpace(head []byte, wordEnd int, allowFullwidthSpace bool, sessions ...*expressionSession) (body []byte, bodyStart, end, comment int, hasArg bool) {
	bodyStart = wordEnd
	if wordEnd >= len(head) {
		return nil, bodyStart, len(head), -1, false
	}
	if !inputSpaceAt(head, wordEnd, allowFullwidthSpace) && head[wordEnd] != ';' {
		return nil, bodyStart, len(head), -1, false
	}
	bodyStart += inputSpaceSize(head, wordEnd, allowFullwidthSpace)
	if bodyStart == wordEnd {
		bodyStart++
	}
	if bodyStart > len(head) {
		bodyStart = len(head)
	}
	end, found := syntaxCommentEnd(head, bodyStart, len(head), sessions...)
	if found {
		comment = end
	} else {
		comment = -1
	}
	return head[bodyStart:end], bodyStart, end, comment, true
}

func syntaxAtomEnd(src []byte, pos, end int, sessions ...*expressionSession) (int, bool) {
	if pos < 0 || pos >= end || end > len(src) {
		return -1, false
	}
	if src[pos] == '\\' && pos+1 < end && src[pos+1] == '@' {
		n, err := scanSyntaxForm(src, pos, end, formConditional, sessions)
		if err != nil {
			return -1, true
		}
		return pos + n, true
	}
	if src[pos] == '@' && pos+1 < end && src[pos+1] == '"' {
		n, err := scanSyntaxForm(src, pos, end, formQuoted, sessions)
		if err != nil {
			return -1, true
		}
		return pos + n, true
	}
	if src[pos] != '"' {
		return pos, false
	}
	n := quotedEnd(src[pos:end], 0)
	if n < 0 {
		return -1, true
	}
	return pos + n, true
}

func syntaxCommentEnd(src []byte, start, end int, sessions ...*expressionSession) (int, bool) {
	hasNUL := bytes.IndexByte(src[start:end], 0) >= 0
	for i := start; i < end; i++ {
		if next, atom := syntaxAtomEnd(src, i, end, sessions...); atom {
			if next < 0 {
				return end, false
			}
			i = next - 1
			continue
		}
		if src[i] == 0 {
			return i, false
		}
		if src[i] == ';' {
			if mandatoryInlineMarker(src[:end], i) {
				i += 2
				continue
			}
			if executableMarker(src[:end], i) {
				return end, false
			}
			return i, true
		}
		if hasNUL {
			r, size := utf8.DecodeRune(src[i:end])
			if isASCIIDigit(r) {
				next, err := scanNumber(src[:end], i)
				if err == nil {
					i = next - 1
				}
			} else if !isDelimiter(r) && r != '\u3000' {
				i = expressionIdentifierEnd(src[:end], i) - 1
			} else {
				i += size - 1
			}
		}
	}
	return end, false
}

func callPrefix(src []byte, start, end int) (int, int, bool) {
	for i := start; i < end; i++ {
		switch src[i] {
		case 0:
			return i, -1, true
		case '\\':
			if i+1 >= end || src[i+1] == 0 {
				return i, -1, false
			}
			i = escapedEnd(src[:end], i) - 1
		case '[', '(', ',', ';':
			return i, i, true
		}
	}
	return end, -1, true
}

func trimCallTargetRight(src []byte, start, end int) int {
	last := start
	for i := start; i < end; {
		if src[i] == '\\' {
			i = escapedEnd(src[:end], i)
			last = i
			continue
		}
		_, size := utf8.DecodeRune(src[i:end])
		if !halfSpace(src[i]) {
			last = i + size
		}
		i += size
	}
	return last
}

func matchingDelimiter(src []byte, start, end int, open, close byte, sessions ...*expressionSession) int {
	depth := 0
	for i := start; i < end; i++ {
		if next, atom := syntaxAtomEnd(src, i, end, sessions...); atom {
			if next < 0 {
				return -1
			}
			i = next - 1
			continue
		}
		if src[i] == open {
			depth++
		}
		if src[i] == close {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

type syntaxSpan struct{ start, end int }

func splitTopLevel(src []byte, start, end int, separator byte, sessions ...*expressionSession) []syntaxSpan {
	var out []syntaxSpan
	part, depth := start, 0
	for i := start; i < end; i++ {
		if next, atom := syntaxAtomEnd(src, i, end, sessions...); atom {
			if next < 0 {
				return []syntaxSpan{{start, end}}
			}
			i = next - 1
			continue
		}
		switch src[i] {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case separator:
			if depth == 0 {
				if len(out) >= maxExpressionTokens-1 {
					if len(sessions) != 0 && sessions[0] != nil {
						sessions[0].resource(i, "field count exceeds the formatter resource limit")
					}
					return []syntaxSpan{{start, end}}
				}
				out = append(out, syntaxSpan{part, i})
				part = i + 1
			}
		}
	}
	return append(out, syntaxSpan{part, end})
}

func topEqual(src []byte, start, end int, sessions ...*expressionSession) (int, bool) {
	depth := 0
	for i := start; i < end; i++ {
		if next, atom := syntaxAtomEnd(src, i, end, sessions...); atom {
			if next < 0 {
				return 0, false
			}
			i = next - 1
			continue
		}
		switch src[i] {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case ';':
			if depth == 0 {
				return 0, false
			}
		}
		if src[i] == '=' && depth == 0 && (i+1 >= end || src[i+1] != '=') && defaultEqualBoundary(src, start, i) {
			return i, true
		}
	}
	return 0, false
}

func defaultEqualBoundary(src []byte, start, equal int) bool {
	left := trimInputRight(src, start, equal, true)
	return left == start || !strings.ContainsRune("=<>!+-*/%&|^'", rune(src[left-1]))
}

func syntaxCommand(command string) bool {
	if expressionCommand(command) || simpleIntegerCommand(command) || simpleStringCommand(command) || printVCommand(command) || printFormCommand(command) || callCommand(command) || forCommand(command) || structuredListCommand(command) {
		return true
	}
	if noArgCommand(command) {
		return true
	}
	return false
}

func rawCommand(command string) bool {
	switch command {
	case "CUSTOMDRAWLINE", "DEBUGPRINT", "DEBUGPRINTL", "PRINTC", "PRINTCD", "PRINTCK", "PRINTLC", "PRINTLCD", "PRINTLCK", "PRINT", "PRINTL", "PRINTW", "PRINTK", "PRINTKL", "PRINTKW", "PRINTD", "PRINTDL", "PRINTDW", "PRINTSINGLE", "PRINTSINGLEK", "PRINTSINGLED", "PRINTPLAIN", "TIMES", "VARI", "VARS":
		return true
	default:
		return false
	}
}

func dataBodyCommand(p *formatter, command string) bool {
	if command != "DATA" {
		return false
	}
	for _, frame := range p.state.frames {
		if frame.kind == "DATA" || frame.kind == "DATALIST" {
			return true
		}
	}
	return false
}

func commandDialectReason(command string, dialect Dialect) string {
	if commandUnavailable(command, dialect) {
		return "unavailable in selected dialect"
	}
	return "not recognized by the formatter"
}

func noArgCommand(command string) bool {
	switch command {
	case "ELSE", "CASEELSE", "CATCH", "ENDIF", "ENDSELECT", "NEXT", "WEND", "DO", "REND", "LOOP", "ENDNOSKIP", "ENDCATCH", "ENDDATA", "ENDLIST", "ENDFUNC", "BREAK", "CONTINUE", "NOSKIP", "DATALIST", "TRYCALLLIST", "TRYJUMPLIST", "TRYGOTOLIST", "PRINT_ITEM", "PRINT_SHOPITEM", "DRAWLINE", "UPCHECK", "ADDDEFCHARA", "QUIT", "CLEARTEXTBOX", "STOPCALLTRAIN", "DELALLCHARA", "GETTIME", "QUIT_AND_RESTART", "FORCE_QUIT", "FORCE_QUIT_AND_RESTART", "OUTPUTLOG":
		return true
	default:
		return false
	}
}

func simpleIntegerCommand(command string) bool {
	switch command {
	case "RETURN", "REDRAW", "CLEARLINE", "CUPCHECK", "CALLTRAIN", "DOTRAIN", "DELDATA", "RESET_STAIN", "FORCEKANA", "SKIPDISP", "SKIPLOG", "PRINT_ABL", "PRINT_TALENT", "PRINT_MARK", "PRINT_EXP", "PRINT_PALAM":
		return true
	default:
		return false
	}
}

func simpleStringCommand(command string) bool {
	switch command {
	case "RETURNF", "PRINTS", "PRINTSL", "PRINTSW", "PRINTSK", "PRINTSKL", "PRINTSKW", "PRINTSD", "PRINTSDL", "PRINTSDW", "PRINTSINGLES", "PRINTSINGLESK", "PRINTSINGLESD", "PRINTFORMS", "PRINTFORMSL", "PRINTFORMSW", "PRINTFORMSK", "PRINTFORMSKL", "PRINTFORMSKW", "PRINTFORMSD", "PRINTFORMSDL", "PRINTFORMSDW", "PRINTSINGLEFORMS", "PRINTSINGLEFORMSK", "PRINTSINGLEFORMSD":
		return true
	default:
		return false
	}
}

func printVCommand(command string) bool {
	switch command {
	case "PRINTV", "PRINTVL", "PRINTVW", "PRINTVK", "PRINTVKL", "PRINTVKW", "PRINTVD", "PRINTVDL", "PRINTVDW", "PRINTSINGLEV", "PRINTSINGLEVK", "PRINTSINGLEVD":
		return true
	default:
		return false
	}
}

func printFormCommand(command string) bool {
	switch command {
	case "PRINTFORM", "PRINTFORML", "PRINTFORMW", "PRINTFORMK", "PRINTFORMKL", "PRINTFORMKW", "PRINTFORMD", "PRINTFORMDL", "PRINTFORMDW", "PRINTFORMC", "PRINTFORMLC", "PRINTFORMCK", "PRINTFORMLCK", "PRINTFORMCD", "PRINTFORMLCD", "PRINTSINGLEFORM", "PRINTSINGLEFORMK", "PRINTSINGLEFORMD", "DATAFORM", "RETURNFORM":
		return true
	default:
		return false
	}
}

func callCommand(command string) bool {
	switch command {
	case "CALL", "CALLF", "CALLFORM", "CALLFORMF", "JUMP", "JUMPFORM", "GOTO", "GOTOFORM", "TRYCALL", "TRYCALLFORM", "TRYJUMP", "TRYJUMPFORM", "TRYGOTO", "TRYGOTOFORM", "TRYCALLF", "TRYCALLFORMF", "TRYCCALL", "TRYCCALLFORM", "TRYCJUMP", "TRYCJUMPFORM", "TRYCGOTO", "TRYCGOTOFORM", "CALLSHARP", "FUNC":
		return true
	default:
		return false
	}
}

func forCommand(command string) bool { return command == "FOR" }

func structuredListCommand(command string) bool {
	switch command {
	case "SETBIT", "CLEARBIT", "INVERTBIT", "VARSET", "CVARSET", "POWER":
		return true
	default:
		return false
	}
}
