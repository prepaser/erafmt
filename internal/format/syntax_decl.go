package format

import (
	"bytes"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func (p *formatter) syntaxHeader(i int, head []byte, base int) int {
	mark := len(p.edits)
	fail := func() int {
		if len(p.edits) > mark {
			p.truncateEdits(mark)
		}
		p.syntaxOpaque(i)
		return -1
	}
	if len(head) < 2 || bytes.Contains(head, []byte("[[")) {
		return fail()
	}
	end, _ := p.syntaxCommentEnd(head, 1, len(head))
	if end == len(head) {
		end = erabasic.SyntaxHeaderMarkerEnd(head)
	}
	head = head[:end]
	_, delimiter := erabasic.HeaderPrefix(head, 1)
	if delimiter < 0 {
		p.setCodeEnd(i, base+len(head))
		return base + len(head)
	}
	if p.rules[DelimiterSpacing] && (head[delimiter] == '[' || head[delimiter] == '(') {
		gapStart := delimiter
		gapStart = erabasic.TrimInputRight(head, 1, gapStart, p.AllowFullwidthSpace)
		p.addEdit(i, base+gapStart, base+delimiter, nil, DelimiterSpacing)
	}
	if head[delimiter] == '[' {
		close := p.matchingDelimiter(head, delimiter, len(head), '[', ']')
		if close < 0 || !p.syntaxExpression(i, base+delimiter+1, base+close, false) {
			return fail()
		}
		p.syntaxDeclarationDelimiter(i, base+delimiter, base+close)
		delimiter = close + 1
		gapStart := delimiter
		for delimiter < len(head) {
			size := erabasic.InputSpaceSize(head, delimiter, p.AllowFullwidthSpace)
			if size == 0 {
				break
			}
			delimiter += size
		}
		if p.rules[DelimiterSpacing] && delimiter < len(head) && head[delimiter] == '(' {
			p.addEdit(i, base+gapStart, base+delimiter, nil, DelimiterSpacing)
		}
		if delimiter >= len(head) {
			p.setCodeEnd(i, base+len(head))
			return base + len(head)
		}
	}
	if head[delimiter] == '(' {
		close := p.matchingDelimiter(head, delimiter, len(head), '(', ')')
		if close < 0 || !p.syntaxSignatureArgs(i, base+delimiter+1, base+close) {
			return fail()
		}
		p.syntaxDeclarationDelimiter(i, base+delimiter, base+close)
	} else if head[delimiter] == ',' {
		if !p.syntaxSignatureArgs(i, base+delimiter+1, base+len(head)) {
			return fail()
		}
		p.syntaxCommaGap(i, base+delimiter, base+len(head))
	} else {
		return fail()
	}
	p.setCodeEnd(i, base+len(head))
	return base + len(head)
}

func (p *formatter) syntaxSignatureArgs(i, start, end int) bool {
	mark := len(p.edits)
	fail := func() bool {
		p.truncateEdits(mark)
		return false
	}
	spans := p.semanticContext().SplitTopLevel(p.Src, start, end, ',')
	for index, span := range spans {
		if erabasic.TrimInputLeft(p.Src, span.Start, span.End, p.AllowFullwidthSpace) >= erabasic.TrimInputRight(p.Src, span.Start, span.End, p.AllowFullwidthSpace) {
			if len(spans) == 1 || index == len(spans)-1 && index > 0 {
				continue
			}
			return fail()
		}
		if eq, ok := p.semanticContext().DeclarationTopEqual(span.Start, span.End); ok {
			if erabasic.TrimInputLeft(p.Src, span.Start, eq, p.AllowFullwidthSpace) >= erabasic.TrimInputRight(p.Src, span.Start, eq, p.AllowFullwidthSpace) || erabasic.TrimInputLeft(p.Src, eq+1, span.End, p.AllowFullwidthSpace) >= erabasic.TrimInputRight(p.Src, eq+1, span.End, p.AllowFullwidthSpace) {
				return fail()
			}
			if p.rules[DeclarationDefaultSpacing] {
				p.addEdit(i, erabasic.TrimInputRight(p.Src, span.Start, eq, p.AllowFullwidthSpace), eq, []byte{' '}, DeclarationDefaultSpacing)
				p.addEdit(i, eq+1, erabasic.TrimInputLeft(p.Src, eq+1, span.End, p.AllowFullwidthSpace), []byte{' '}, DeclarationDefaultSpacing)
			}
			if !p.syntaxExpression(i, span.Start, eq, false) || !p.syntaxExpression(i, eq+1, span.End, false) {
				return fail()
			}
		} else if !p.syntaxExpression(i, span.Start, span.End, false) {
			return fail()
		}
	}
	p.syntaxDeclarationCommas(i, start, end)
	return true
}

func (p *formatter) syntaxDeclaration(i int, head []byte, base int) int {
	mark := len(p.edits)
	diagnosticMark := len(p.diagnostics)
	fail := func() int {
		if len(p.edits) > mark {
			p.truncateEdits(mark)
		}
		if len(p.diagnostics) > diagnosticMark {
			p.removeLineRule(i, DeclarationIndent)
		}
		p.syntaxOpaque(i)
		return -1
	}
	if bytes.Contains(head, []byte("[[")) {
		return fail()
	}
	end, _ := p.syntaxCommentEnd(head, 0, len(head))
	head = head[:end]
	word := p.declarationWordAnalysis(head)
	name := word.Name
	if name == "DEFINE" {
		if p.Kind == erabasic.ERH && !p.SharpStateUnknown && erabasic.FunctionStyleDefine(head) {
			p.PreserveFile = true
			p.problem(i, base+erabasic.DeclarationWordEnd(head), "unsupported_declaration", "function-style #DEFINE is unsupported", erabasic.Unsupported)
			p.syntaxOpaque(i)
			return fail()
		}
		if p.Kind == erabasic.ERH && !p.SharpStateUnknown {
			p.syntaxObjectDefine(i, head, base)
			return -1
		}
		return fail()
	}
	if name == "FUNCTION" || name == "FUNCTIONS" {
		if p.Kind == erabasic.ERB && erabasic.BareSharpFunction(head, name, p.AllowFullwidthSpace) {
			p.setCodeEnd(i, base+len(head))
			return base + len(head)
		}
		return fail()
	}
	if name == "SINGLE" || name == "LATER" || name == "PRI" || name == "ONLY" {
		p.setCodeEnd(i, base+len(head))
		return base + len(head)
	}
	wordEnd := word.WordEnd
	if wordEnd < 0 {
		return fail()
	}
	bodyStart := word.BodyStart
	if bodyStart >= len(head) {
		if name == "DIM" || name == "DIMS" {
			p.syntaxProblem(i, base+bodyStart, "declaration requires an identifier")
			return fail()
		}
		if (name == "LOCALSIZE" || name == "LOCALSSIZE") && p.expressionParseEnabled() {
			p.warning(i, base+wordEnd, "missing_local_size", name+" requires a value")
		}
		p.setCodeEnd(i, base+len(head))
		return base + len(head)
	}
	p.addDeclarationWordGaps(i, head, base, wordEnd, &bodyStart)
	if bodyStart >= len(head) && (name == "DIM" || name == "DIMS") {
		p.syntaxProblem(i, base+bodyStart, "declaration requires an identifier")
		return fail()
	}
	if name == "LOCALSIZE" || name == "LOCALSSIZE" {
		if p.eventFunction() {
			p.setCodeEnd(i, base+len(head))
			return base + len(head)
		}
		if !p.syntaxExpression(i, base+bodyStart, base+len(head), false) {
			return fail()
		}
		if p.expressionParseEnabled() && !p.SharpStateUnknown && !p.Optional && !p.conditional() {
			left := base + bodyStart
			right := base + len(head)
			if tokens, err := p.semanticContext().LexSyntaxRange(left, right); err == nil && !erabasic.CommandMacroPossible(tokens) {
				status, value := erabasic.LocalSizeConstantProof(tokens)
				if status == erabasic.LocalSizeConstantInvalid {
					p.warning(i, left, "suspicious_local_size", "local size expression is not a valid integer constant")
					return fail()
				}
				if status == erabasic.LocalSizeConstantValid && (value <= 0 || value >= 1<<31-1) {
					p.warning(i, left, "suspicious_local_size", "constant local size is ignored because it is outside the range 1..2147483646")
					return fail()
				}
			}
		}
		p.setCodeEnd(i, base+len(head))
		return base + len(head)
	}
	if name != "DIM" && name != "DIMS" {
		return fail()
	}
	modifierAnalysis := p.declarationModifierAnalysis(base+wordEnd, base+bodyStart)
	modifiers, modifierKnown := modifierAnalysis.Set, modifierAnalysis.Known
	end = base + len(head)
	eq, hasEqual := p.semanticContext().DeclarationTopEqual(base+bodyStart, end)
	leftEnd := end
	if hasEqual {
		leftEnd = eq
	}
	nameSpans := p.semanticContext().SplitTopLevel(p.Src, base+bodyStart, leftEnd, ',')
	if p.Aborted {
		return fail()
	}
	modifierIssue := modifierAnalysis.Issue
	if len(nameSpans) > 0 && declarationMacroNameSpan(p, nameSpans[0]) && modifierIssue == "" && !(p.Kind == erabasic.ERH && modifiers.Ref) {
		return fail()
	}
	if p.Kind == erabasic.ERB && !p.Optional && !p.conditional() {
		if p.privateDeclarationNameMacroPossible(nameSpans) {
			return fail()
		}
	}
	if declarationReservedName(p, base+bodyStart, leftEnd) {
		if p.Kind == erabasic.ERB {
			p.problem(i, base+bodyStart, "unsupported_syntax", "declaration name is reserved", erabasic.Unsupported)
		} else {
			p.syntaxProblem(i, base+bodyStart, "declaration name is reserved")
		}
		return fail()
	}
	if p.Kind == erabasic.ERH && modifierKnown && modifiers.Ref {
		p.PreserveFile = true
		p.syntaxProblem(i, base+wordEnd, "ERH REF declarations are unsupported")
		return fail()
	}
	if hasEqual {
		if erabasic.TrimInputLeft(p.Src, base+bodyStart, eq, p.AllowFullwidthSpace) >= erabasic.TrimInputRight(p.Src, base+bodyStart, eq, p.AllowFullwidthSpace) || erabasic.TrimInputLeft(p.Src, eq+1, end, p.AllowFullwidthSpace) >= erabasic.TrimInputRight(p.Src, eq+1, end, p.AllowFullwidthSpace) {
			if p.expressionParseEnabled() && modifierKnown {
				p.warning(i, eq+1, "suspicious_initializer", "declaration initializer is empty")
			}
			return fail()
		}
		if p.rules[DeclarationDefaultSpacing] {
			p.addEdit(i, erabasic.TrimInputRight(p.Src, base+bodyStart, eq, p.AllowFullwidthSpace), eq, []byte{' '}, DeclarationDefaultSpacing)
			p.addEdit(i, eq+1, erabasic.TrimInputLeft(p.Src, eq+1, end, p.AllowFullwidthSpace), []byte{' '}, DeclarationDefaultSpacing)
		}
		if !p.syntaxDeclarationInitializerExpression(i, eq+1, end) {
			return fail()
		}
	}
	if p.expressionParseEnabled() && modifierKnown {
		if message := modifierAnalysis.Issue; message != "" {
			p.syntaxProblem(i, base+wordEnd, message)
			return fail()
		}
		if !hasEqual && modifiers.IsConst {
			p.warning(i, base+bodyStart, "suspicious_initializer", "CONST declaration has no initializer; macro expansion may change its role")
			return fail()
		}
		if hasEqual && declarationInitializerSuspicious(p, i, name == "DIMS", modifiers, p.Src, base+bodyStart, leftEnd, eq+1, end) {
			return fail()
		}
	}
	allowRefOmittedDimensions := p.Kind == erabasic.ERB && p.Base > 0 && p.State.Declarations && !hasEqual && erabasic.DeclarationModifiersAllowRefOmissionWithSpace(p.Src, base+wordEnd, base+bodyStart, p.AllowFullwidthSpace)
	initializerStart, initializerEnd := -1, -1
	if hasEqual {
		initializerStart, initializerEnd = eq+1, end
	}
	if !p.syntaxDeclarationDimensions(i, base+wordEnd, base+bodyStart, leftEnd, allowRefOmittedDimensions, initializerStart, initializerEnd, name == "DIMS") {
		return fail()
	}
	p.setCodeEnd(i, end)
	return end
}

func (p *formatter) syntaxDeclarationInitializerExpression(i, start, end int) bool {
	if p.expressionParseEnabled() {
		if shapeErr := p.semanticContext().DeclarationInitializerShapeIssue(start, end); shapeErr != nil {
			p.expressionProblem(i, start, erabasic.ShiftExpressionError(shapeErr, -start))
			return false
		}
	}
	return p.syntaxExpression(i, start, end, false)
}

func (p *formatter) syntaxObjectDefine(i int, head []byte, base int) {
	if !p.rules[DeclarationWordSpacing] || len(head) < len("#DEFINE") || !bytes.Equal(head[:len("#DEFINE")], []byte("#DEFINE")) {
		return
	}
	line := p.Lines[i]
	analysis := p.semanticContext().ObjectDefineAnalysis(head, p.Src[line.Start:line.BodyEnd], base)
	if !analysis.Valid {
		return
	}
	p.addEdit(i, base+analysis.WordEnd, base+analysis.NameStart, []byte{' '}, DeclarationWordSpacing)
	p.addEdit(i, base+analysis.NameEnd, base+analysis.BodyStart, []byte{' '}, DeclarationWordSpacing)
}

func declarationInitializerSuspicious(p *formatter, line int, stringType bool, modifiers erabasic.DeclarationModifierSet, src []byte, fieldsStart, fieldsEnd, initStart, initEnd int) bool {
	return p.declarationInitializerSuspicious(line, stringType, modifiers, fieldsStart, fieldsEnd, initStart, initEnd)
}

func declarationInitializerDimensionsKnown(p *formatter, spans []erabasic.SyntaxSpan) bool {
	return p.semanticContext().DeclarationInitializerDimensionsKnown(spans)
}

func declarationInitializerSize(p *formatter, spans []erabasic.SyntaxSpan) (bool, int64) {
	return p.semanticContext().DeclarationInitializerSize(spans)
}

func (p *formatter) declarationInitializerFields(start, end int) []erabasic.SyntaxSpan {
	return p.semanticContext().DeclarationInitializerFields(start, end)
}

func declarationMacroNameSpan(p *formatter, span erabasic.SyntaxSpan) bool {
	return p.semanticContext().DeclarationMacroNameSpan(span)
}

func declarationReservedName(p *formatter, start, end int) bool {
	return p.semanticContext().DeclarationReservedName(start, end)
}

func (p *formatter) eventFunction() bool {
	event, _ := p.eventFunctionStatus()
	return event
}

func (p *formatter) eventFunctionStatus() (bool, bool) {
	if p.Kind != erabasic.ERB {
		return false, false
	}
	if p.FunctionName != "" {
		return p.EventFunctionKnown, p.EventFunctionExact
	}
	if p.FunctionStart < 0 || p.FunctionStart >= len(p.Lines) {
		return false, false
	}
	head, _, _, _ := p.headOf(erabasic.LBody(p.Src, p.Lines[p.FunctionStart]))
	start := erabasic.TrimInputLeft(head, 1, len(head), p.AllowFullwidthSpace)
	end, _ := erabasic.HeaderPrefix(head, start)
	end = erabasic.TrimInputRight(head, start, end, p.AllowFullwidthSpace)
	name := string(head[start:end])
	return erabasic.EventFunctionContext(name)
}

func (p *formatter) addDeclarationWordGaps(i int, head []byte, base, wordEnd int, bodyStart *int) {
	pos := *bodyStart
	if p.rules[DeclarationWordSpacing] && pos > wordEnd && pos < len(head) {
		p.addEdit(i, base+wordEnd, base+pos, []byte{' '}, DeclarationWordSpacing)
	}
	for {
		wordEnd := erabasic.AsciiWordEnd(head, pos)
		separator := wordEnd < len(head) && erabasic.InputSpaceAt(head, wordEnd, p.AllowFullwidthSpace)
		if wordEnd == pos || wordEnd < len(head) && !separator && !erabasic.IsDelimiter(rune(head[wordEnd])) || !erabasic.DeclarationModifier(string(bytes.ToUpper(head[pos:wordEnd]))) {
			break
		}
		next := wordEnd
		for next < len(head) {
			size := erabasic.InputSpaceSize(head, next, p.AllowFullwidthSpace)
			if size == 0 {
				break
			}
			next += size
		}
		if p.rules[DeclarationWordSpacing] && next > wordEnd && next < len(head) {
			p.addEdit(i, base+wordEnd, base+next, []byte{' '}, DeclarationWordSpacing)
		}
		pos = next
	}
	*bodyStart = pos
}

func (p *formatter) syntaxDeclarationDimensions(i, modifiersStart, start, end int, allowRefOmitted bool, initializerStart, initializerEnd int, stringType bool) bool {
	spans := p.semanticContext().SplitTopLevel(p.Src, start, end, ',')
	modifiers, modifiersKnown := erabasic.DeclarationModifierInfo(p.Src, modifiersStart, start, p.AllowFullwidthSpace)
	if declarationMacroNameSpan(p, spans[0]) {
		return false
	}
	left := erabasic.TrimInputLeft(p.Src, spans[0].Start, spans[0].End, p.AllowFullwidthSpace)
	right := erabasic.TrimInputRight(p.Src, left, spans[0].End, p.AllowFullwidthSpace)
	name, err := p.semanticContext().LexSyntaxRange(left, right)
	identifier := erabasic.AnalyzeDeclarationIdentifier(name)
	if err != nil || !identifier.Valid {
		p.syntaxProblem(i, start, "declaration requires an identifier before its dimensions or initializer")
		return false
	}
	for _, span := range spans[1:] {
		if erabasic.TrimInputLeft(p.Src, span.Start, span.End, p.AllowFullwidthSpace) >= erabasic.TrimInputRight(p.Src, span.Start, span.End, p.AllowFullwidthSpace) {
			continue
		}
		if !p.syntaxExpression(i, span.Start, span.End, false) {
			return false
		}
	}
	checks := p.semanticContext().DeclarationDimensionChecks(spans, modifiersStart, start, end, allowRefOmitted, p.expressionParseEnabled(), erabasic.DeclarationKind(p.Kind), modifiers, modifiersKnown, initializerStart, initializerEnd, stringType)
	for _, issue := range checks.Issues {
		if issue.Severity == erabasic.DeclarationDiagnosticWarning {
			p.warning(i, issue.Offset, issue.Code, issue.Message)
		} else {
			p.syntaxProblem(i, issue.Offset, issue.Message)
		}
	}
	if checks.Reject {
		return false
	}
	valuesProven := checks.ValuesProven
	if p.Kind == erabasic.ERH && valuesProven {
		if !p.registerGlobalDeclaration(i, left, name) {
			return false
		}
	}
	if p.Kind == erabasic.ERB && p.Base > 0 && valuesProven {
		if !p.registerPrivateDeclaration(i, left, identifier.Name) {
			return false
		}
	}
	p.syntaxDeclarationCommas(i, start, end)
	return true
}

func (p *formatter) privateDeclarationNameMacroPossible(spans []erabasic.SyntaxSpan) bool {
	return p.semanticContext().PrivateDeclarationNameMacroPossible(spans)
}

func declarationOmittedDimensionProvenOrdinary(p *formatter, spans []erabasic.SyntaxSpan, modifiersStart, start int) bool {
	return p.semanticContext().DeclarationOmittedDimensionProvenOrdinary(spans, modifiersStart, start)
}

func declarationCharadataDimensionsProven(p *formatter, spans []erabasic.SyntaxSpan) (erabasic.SyntaxSpan, bool) {
	return p.semanticContext().DeclarationCharadataDimensionsProven(spans)
}

func declarationValuesProven(p *formatter, start, end, initializerStart, initializerEnd int, stringType bool, modifiers erabasic.DeclarationModifierSet) bool {
	return p.semanticContext().DeclarationValuesProven(start, end, initializerStart, initializerEnd, stringType, modifiers)
}

func (p *formatter) literalZero(start, end int) bool {
	return p.semanticContext().DeclarationLiteralZero(start, end)
}

func (p *formatter) syntaxDeclarationCommas(i, start, end int) {
	spans := p.semanticContext().SplitTopLevel(p.Src, start, end, ',')
	for _, span := range spans[:max(0, len(spans)-1)] {
		p.syntaxCommaGap(i, span.End, end)
	}
}

func (p *formatter) syntaxDeclarationDelimiter(i, open, close int) {
	if !p.rules[DelimiterSpacing] || close <= open+1 {
		return
	}
	innerStart, innerEnd := open+1, close
	leftEnd := erabasic.TrimInputLeft(p.Src, innerStart, innerEnd, p.AllowFullwidthSpace)
	rightStart := erabasic.TrimInputRight(p.Src, innerStart, innerEnd, p.AllowFullwidthSpace)
	if leftEnd > rightStart {
		p.addEdit(i, innerStart, innerEnd, nil, DelimiterSpacing)
		return
	}
	if !erabasic.MandatoryInlineMarker(p.Src, leftEnd) {
		p.addEdit(i, innerStart, leftEnd, nil, DelimiterSpacing)
	}
	if !erabasic.MandatoryInlineMarker(p.Src[:rightStart], rightStart-3) {
		p.addEdit(i, rightStart, innerEnd, nil, DelimiterSpacing)
	}
}

func (p *formatter) headerArgumentsValid(i int, head []byte, base int) bool {
	if !p.expressionParseEnabled() {
		return true
	}
	session := p.SyntaxSession
	if session == nil || !bytes.Equal(session.Sources[erabasic.ExpressionSourceOriginal], head) {
		session = p.newExpressionSession(head)
	}
	start, end, ok := erabasic.HeaderArgumentRangeWithSession(head, session)
	if !ok {
		return true
	}
	for _, diagnostic := range erabasic.AnalyzeHeaderArgumentsWithSession(head, start, end, session, p.Dialect) {
		if diagnostic.Severity == erabasic.DeclarationDiagnosticWarning {
			p.warning(i, base+diagnostic.Offset, diagnostic.Code, diagnostic.Message)
			continue
		}
		p.syntaxProblem(i, base+diagnostic.Offset, diagnostic.Message)
		return false
	}
	return true
}
