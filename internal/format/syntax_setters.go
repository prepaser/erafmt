package format

import "github.com/prepaser/erafmt/internal/erabasic"

func (p *formatter) syntaxSetters(i int, head []byte, base, wordEnd int, command string) int {
	start, end, comment, ok := p.registeredBody(head, wordEnd)
	if !ok {
		p.syntaxOpaque(i)
		return -1
	}
	codeEnd := erabasic.RegisteredCodeEnd(base, end, comment)
	mark := len(p.edits)
	fail := func(offset int, message string) int {
		p.truncateEdits(mark)
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, offset, message)
		}
		p.syntaxOpaque(i)
		return -1
	}
	if start >= end {
		return fail(base+wordEnd, "setter command requires arguments")
	}
	if !p.expressionParseEnabled() {
		p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
		codeEnd = p.trimInputRight(base+start, codeEnd)
		p.setCodeEnd(i, codeEnd)
		return codeEnd
	}
	analysis := p.semanticContext()
	bodyStart := base + start
	body := p.Src[bodyStart:codeEnd]
	if erabasic.ExpressionExcludedWithSession(analysis.ScanSession(body, 0, len(body)), body) {
		p.syntaxOpaque(i)
		return -1
	}
	if erabasic.CommandFieldHasDeferredMarker(p.Src[bodyStart:codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	parts := analysis.SplitTopLevel(p.Src, bodyStart, codeEnd, ',')
	checked := erabasic.SetterCheckedFields(command)
	markerPresent := erabasic.CommandFieldHasMandatoryMarker(p.Src[bodyStart:codeEnd])
	if command == "POWER" && !markerPresent && len(parts) < 3 {
		if tokens, err := analysis.LexSyntaxRange(bodyStart, codeEnd); err == nil && !analysis.SetterMacroPossibleForDialect(tokens) {
			return fail(codeEnd, "POWER requires three arguments")
		}
	}
	macroPrefix := false
	destinationType := erabasic.KnownValueUnknown
	for n, field := range parts {
		if n >= checked {
			continue
		}
		left := p.trimInputLeft(field.Start, field.End)
		right := p.trimInputRight(left, field.End)
		validation := analysis.ValidateSetterField(command, n, left, right, len(parts), markerPresent, erabasic.SetterValidationState{MacroPrefix: macroPrefix, DestinationType: destinationType})
		if validation.ExprErr != nil {
			p.truncateEdits(mark)
			p.expressionProblem(i, 0, validation.ExprErr)
			p.syntaxOpaque(i)
			return -1
		}
		if validation.Opaque {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
		if !validation.Valid {
			return fail(validation.Offset, validation.Message)
		}
		macroPrefix = validation.MacroPrefix
		destinationType = validation.DestinationType
		valid := false
		if macroPrefix {
			valid = p.syntaxExpression(i, left, right, false)
		} else {
			valid = p.syntaxOrdinaryExpression(i, left, right, false)
		}
		if !valid {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
	}
	effective := parts
	if len(effective) > 0 {
		last := effective[len(effective)-1]
		left := p.trimInputLeft(last.Start, last.End)
		if left >= p.trimInputRight(left, last.End) {
			effective = effective[:len(effective)-1]
		}
	}
	if len(effective) > checked && !markerPresent {
		if analysis.SetterSurplusUncertain(effective, checked, macroPrefix) {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
		p.ignoredCommandArguments(i, codeEnd, command)
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	p.syntaxRegisteredCommaGaps(i, parts, codeEnd)
	codeEnd = p.trimInputRight(bodyStart, codeEnd)
	p.setCodeEnd(i, codeEnd)
	return codeEnd
}

func (p *formatter) destinationRoleHasMarker(parts []erabasic.SyntaxSpan) bool {
	for _, field := range parts {
		left := p.trimInputLeft(field.Start, field.End)
		right := p.trimInputRight(left, field.End)
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil {
			continue
		}
		for _, token := range tokens {
			if token.Kind == erabasic.ExprMarker {
				return true
			}
		}
	}
	return false
}
