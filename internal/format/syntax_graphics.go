package format

import "github.com/prepaser/erafmt/internal/erabasic"

func (p *formatter) syntaxGraphics(i int, head []byte, base int, command string, wordEnd int) (int, bool) {
	if command != "PRINT_IMG" && command != "PRINT_RECT" && command != "PRINT_SPACE" {
		return -1, false
	}
	start, end, comment, ok := p.registeredBody(head, wordEnd)
	if !ok {
		p.syntaxOpaque(i)
		return -1, true
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	mark := len(p.edits)
	diagMark := len(p.diagnostics)
	if start >= end || erabasic.TrimInputLeft(head, start, end, p.AllowFullwidthSpace) >= erabasic.TrimInputRight(head, start, end, p.AllowFullwidthSpace) {
		return p.graphicsInvalid(i, base+wordEnd, mark, diagMark, "missing graphics argument")
	}
	codeEnd := erabasic.RegisteredCodeEnd(base, end, comment)
	session, offset := p.expressionSessionFor(base+start, codeEnd)
	defer p.addFormAdvisories(i, session, offset, offset+codeEnd-base-start, base+start)
	parts := p.splitTopLevel(p.Src, base+start, codeEnd, ',')
	if p.graphicsHasDebugMarker(parts) {
		p.syntaxOpaque(i)
		return -1, true
	}
	if !p.graphicsFields(i, command, parts) {
		return p.graphicsInvalid(i, base+start, mark, diagMark, "unsupported graphics argument shape")
	}
	if p.graphicsLiteralRangeWarnings(i, command, parts) {
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1, true
	}
	if !p.graphicsCommaFormattingDeferred(command, parts) {
		p.syntaxRegisteredCommaGaps(i, parts, codeEnd)
	}
	codeEnd = erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace)
	p.setCodeEnd(i, codeEnd)
	return codeEnd, true
}

func (p *formatter) graphicsLiteralRangeWarnings(i int, command string, parts []erabasic.SyntaxSpan) bool {
	if p.Dialect == erabasic.EMEE || command == "PRINT_IMG" || !p.expressionParseEnabled() {
		return false
	}
	warned := false
	for _, field := range erabasic.GraphicsEffectiveFields(p.Src, parts, p.AllowFullwidthSpace) {
		left, right := erabasic.ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
		tokens, opaque, _ := p.graphicsFieldTokens(left, right)
		if opaque {
			continue
		}
		value, kind := erabasic.DirectIntegerLiteral(tokens)
		if kind == erabasic.KnownValueNumeric && (value < 0 || value > 1<<31-1) {
			p.warning(i, left, "suspicious_graphics_range", command+" literal is outside the base rendering range 0..2147483647")
			warned = true
		}
	}
	return warned
}

func (p *formatter) graphicsHasDebugMarker(parts []erabasic.SyntaxSpan) bool {
	return p.semanticContext().GraphicsHasDebugMarker(parts)
}

func (p *formatter) graphicsCommaFormattingDeferred(command string, parts []erabasic.SyntaxSpan) bool {
	if command != "PRINT_IMG" || p.Dialect != erabasic.EMEE {
		return false
	}
	context := p.semanticContext()
	for _, field := range erabasic.GraphicsEffectiveFields(p.Src, parts, p.AllowFullwidthSpace)[1:] {
		if context.GraphicsFieldType(field) == erabasic.GraphicsNumericUnknown {
			return true
		}
	}
	return false
}

func (p *formatter) graphicsInvalid(i, offset, mark, diagMark int, message string) (int, bool) {
	p.truncateEdits(mark)
	if (p.expressionParseEnabled() || p.rules[PixelUnitSpacing] && p.Dialect == erabasic.EMEE) && len(p.diagnostics) == diagMark {
		p.syntaxProblem(i, offset, message)
	}
	p.syntaxOpaque(i)
	return -1, true
}

func (p *formatter) graphicsFields(i int, command string, parts []erabasic.SyntaxSpan) bool {
	parts = erabasic.GraphicsEffectiveFields(p.Src, parts, p.AllowFullwidthSpace)
	switch command {
	case "PRINT_IMG":
		return p.graphicsImageFields(i, parts)
	case "PRINT_RECT":
		if len(parts) != 1 && len(parts) != 4 {
			return false
		}
		for _, field := range parts {
			if !p.graphicsNumericField(i, field, p.Dialect == erabasic.EMEE) {
				return false
			}
		}
		return true
	case "PRINT_SPACE":
		return len(parts) == 1 && p.graphicsNumericField(i, parts[0], p.Dialect == erabasic.EMEE)
	default:
		return false
	}
}

func (p *formatter) graphicsImageFields(i int, parts []erabasic.SyntaxSpan) bool {
	editMark := len(p.edits)
	if len(parts) == 0 {
		return false
	}
	if p.Dialect != erabasic.EMEE {
		return len(parts) == 1 && p.graphicsKnownStringField(i, parts[0])
	}
	if len(parts) > 6 {
		return false
	}
	if !p.graphicsKnownStringField(i, parts[0]) {
		return false
	}
	names, numerics := 1, 0
	numeric := false
	for _, field := range parts[1:] {
		kind := p.graphicsFieldType(field)
		switch kind {
		case erabasic.GraphicsString:
			if numeric || names == 3 {
				return false
			}
			if !p.graphicsStringField(i, field) {
				return false
			}
			names++
		case erabasic.GraphicsNumericCertain:
			if numerics == 3 {
				return false
			}
			numeric = true
			numerics++
			if !p.graphicsNumericField(i, field, true) {
				return false
			}
		case erabasic.GraphicsNumericUnknown:
			p.truncateEdits(editMark)
			return true
		default:
			return false
		}
	}
	return names <= 3
}

func (p *formatter) graphicsFieldType(field erabasic.SyntaxSpan) erabasic.GraphicsFieldKind {
	return p.semanticContext().GraphicsFieldType(field)
}

func (p *formatter) graphicsStringField(i int, field erabasic.SyntaxSpan) bool {
	check := p.semanticContext().GraphicsStringFieldCheck(field, false)
	if !check.Valid {
		return false
	}
	if check.Opaque || check.Marked {
		return true
	}
	if p.expressionParseEnabled() && !p.semanticContext().GraphicsStringSpan(check.Left, check.Right) {
		return p.syntaxExpression(i, check.Left, check.Right, false)
	}
	return true
}

func (p *formatter) graphicsKnownStringField(i int, field erabasic.SyntaxSpan) bool {
	check := p.semanticContext().GraphicsStringFieldCheck(field, true)
	if !check.Valid {
		return false
	}
	if check.Opaque {
		return true
	}
	if p.expressionRulesEnabled() && !check.Marked {
		session, offset := p.expressionSessionFor(check.Left, check.Right)
		edits, err := expressionEditsWithSession(session, erabasic.ExpressionSourceOriginal, p.Src[check.Left:check.Right], offset, false, p.rules, true)
		if err != nil {
			return false
		}
		p.addSyntaxEdits(i, check.Left, edits)
	}
	return true
}

func (p *formatter) graphicsStringSpan(left, right int) bool {
	return p.semanticContext().GraphicsStringSpan(left, right)
}

func (p *formatter) graphicsNumericField(i int, field erabasic.SyntaxSpan, allowUnit bool) bool {
	check := p.semanticContext().GraphicsNumericFieldCheck(field, allowUnit, p.rules[PixelUnitSpacing])
	if !check.Valid {
		return false
	}
	if check.Opaque {
		return true
	}
	if p.expressionRulesEnabled() && !check.Marked {
		session, offset := p.expressionSessionFor(check.Left, check.ExprEnd)
		edits, err := expressionEditsWithSession(session, erabasic.ExpressionSourceOriginal, p.Src[check.Left:check.ExprEnd], offset, false, p.rules, true)
		if err != nil {
			return false
		}
		p.addSyntaxEdits(i, check.Left, edits)
	}
	if check.HasUnit && p.rules[PixelUnitSpacing] && !erabasic.MandatoryInlineMarker(p.Src[:check.ExprEnd], check.ExprEnd-3) {
		p.addEdit(i, check.ExprEnd, check.UnitStart, []byte{' '}, PixelUnitSpacing)
	}
	return true
}

func (p *formatter) graphicsFieldTokens(left, right int) ([]erabasic.ExprToken, bool, bool) {
	return p.semanticContext().GraphicsFieldTokens(left, right)
}

func (p *formatter) graphicsDebugMarker(left, right int) bool {
	return p.semanticContext().GraphicsDebugMarker(left, right)
}

func (p *formatter) graphicsUnitSpan(left, right int) (exprEnd, unitStart int, ok bool) {
	return p.semanticContext().GraphicsUnitSpan(left, right)
}

func (p *formatter) graphicsUnitCandidate(left, right int) (exprEnd, unitStart int, ok bool) {
	return p.semanticContext().GraphicsUnitCandidate(left, right)
}

func (p *formatter) graphicsExpressionComplete(left, right int) bool {
	return p.semanticContext().GraphicsExpressionComplete(left, right)
}
