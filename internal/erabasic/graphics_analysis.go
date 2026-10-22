package erabasic

func (p *SemanticContext) GraphicsHasDebugMarker(parts []SyntaxSpan) bool {
	for _, field := range parts {
		left := TrimInputLeft(p.Src, field.Start, field.End, p.AllowFullwidthSpace)
		right := TrimInputRight(p.Src, left, field.End, p.AllowFullwidthSpace)
		if left < right && p.GraphicsDebugMarker(left, right) {
			return true
		}
	}
	return false
}

func (p *SemanticContext) GraphicsFieldType(field SyntaxSpan) GraphicsFieldKind {
	left := TrimInputLeft(p.Src, field.Start, field.End, p.AllowFullwidthSpace)
	right := TrimInputRight(p.Src, left, field.End, p.AllowFullwidthSpace)
	if left >= right {
		return graphicsInvalid
	}
	exprEnd, _, hasUnit := p.GraphicsUnitSpan(left, right)
	if !hasUnit {
		_, _, candidate := p.GraphicsUnitCandidate(left, right)
		if candidate {
			return graphicsInvalid
		}
		exprEnd = right
	}
	tokens, opaque, marked := p.GraphicsFieldTokens(left, exprEnd)
	if opaque {
		return GraphicsNumericUnknown
	}
	if len(tokens) == 0 || ConstantExpressionsValid(tokens) != nil {
		return graphicsInvalid
	}
	if value := graphicsKnownType(tokens); value == KnownValueString {
		return GraphicsString
	}
	if value := graphicsKnownType(tokens); value == KnownValueNumeric {
		return GraphicsNumericCertain
	}
	if !marked && p.GraphicsStringSpan(left, exprEnd) {
		return GraphicsString
	}
	if !p.GraphicsExpressionComplete(left, exprEnd) {
		return graphicsInvalid
	}
	for _, token := range tokens {
		if token.Kind == ExprIdentifier {
			return GraphicsNumericUnknown
		}
	}
	return GraphicsNumericUnknown
}

type graphicsFieldCheck struct {
	Valid     bool
	Opaque    bool
	Marked    bool
	Left      int
	Right     int
	ExprEnd   int
	UnitStart int
	HasUnit   bool
	Tokens    []ExprToken
}

func (p *SemanticContext) GraphicsStringFieldCheck(field SyntaxSpan, knownOnly bool) graphicsFieldCheck {
	check := graphicsFieldCheck{Left: TrimInputLeft(p.Src, field.Start, field.End, p.AllowFullwidthSpace), Right: TrimInputRight(p.Src, field.Start, field.End, p.AllowFullwidthSpace)}
	if check.Left >= check.Right {
		return check
	}
	check.Tokens, check.Opaque, check.Marked = p.GraphicsFieldTokens(check.Left, check.Right)
	if check.Opaque {
		check.Valid = true
		return check
	}
	if len(check.Tokens) == 0 || !knownOnly && graphicsKnownType(check.Tokens) != KnownValueString {
		return check
	}
	if p.expressionParseEnabled() && !check.Marked {
		if err := ProvenExpressionShape(check.Tokens, p.Dialect, false); err != nil {
			return check
		}
	}
	if knownOnly && p.expressionParseEnabled() && ConstantExpressionsValid(check.Tokens) != nil {
		return check
	}
	if knownOnly && p.Dialect != EMEE && p.expressionParseEnabled() && graphicsKnownType(check.Tokens) == KnownValueNumeric {
		return check
	}
	if !knownOnly && !check.Marked && !p.GraphicsStringSpan(check.Left, check.Right) && p.Dialect != EMEE {
		return check
	}
	check.Valid = true
	return check
}

func (p *SemanticContext) GraphicsNumericFieldCheck(field SyntaxSpan, allowUnit, validateConstant bool) graphicsFieldCheck {
	check := graphicsFieldCheck{Left: TrimInputLeft(p.Src, field.Start, field.End, p.AllowFullwidthSpace), Right: TrimInputRight(p.Src, field.Start, field.End, p.AllowFullwidthSpace)}
	if check.Left >= check.Right {
		return check
	}
	check.ExprEnd, check.UnitStart, check.HasUnit = p.GraphicsUnitSpan(check.Left, check.Right)
	if check.HasUnit && !allowUnit {
		return check
	}
	_, _, candidate := p.GraphicsUnitCandidate(check.Left, check.Right)
	if !check.HasUnit && candidate {
		return check
	}
	if check.ExprEnd <= check.Left {
		return check
	}
	check.Tokens, check.Opaque, check.Marked = p.GraphicsFieldTokens(check.Left, check.ExprEnd)
	if check.Opaque {
		check.Valid = true
		return check
	}
	if len(check.Tokens) == 0 {
		return check
	}
	if p.expressionParseEnabled() && !check.Marked {
		if err := ProvenExpressionShape(check.Tokens, p.Dialect, false); err != nil {
			return check
		}
	}
	if provenClosedOperandAdjacency(check.Tokens, p.Dialect) != nil || provenClosedOperandOperators(check.Tokens, p.Dialect) != nil || provenQualifiedBuiltinReference(check.Tokens, p.Dialect) != nil || adjacentExpressionOperands(check.Tokens, p.Dialect) != nil {
		return check
	}
	if (p.expressionParseEnabled() || validateConstant && allowUnit) && ConstantExpressionsValid(check.Tokens) != nil {
		return check
	}
	known := graphicsKnownType(check.Tokens)
	if p.Dialect != EMEE && known != KnownValueNumeric && !graphicsNumericOperandsValid(check.Tokens) {
		if known != KnownValueUnknown || !graphicsExpressionMayBeUnknown(check.Tokens) || !p.GraphicsExpressionComplete(check.Left, check.ExprEnd) {
			return check
		}
	}
	if graphicsConstantSyntax(check.Tokens) && !p.GraphicsExpressionComplete(check.Left, check.ExprEnd) {
		return check
	}
	if p.Dialect != EMEE && p.expressionParseEnabled() && known == KnownValueString {
		return check
	}
	last := check.Tokens[len(check.Tokens)-1]
	if last.Kind == ExprOperator && string(last.Text) != "++" && string(last.Text) != "--" {
		return check
	}
	check.Valid = true
	return check
}

func (p *SemanticContext) GraphicsStringSpan(left, right int) bool {
	if left >= right {
		return false
	}
	src := p.Src[left:right]
	if src[0] == '"' {
		end := QuotedEnd(src, 0)
		return end == len(src)
	}
	mode := FormMode(0)
	switch {
	case len(src) >= 2 && src[0] == '@' && src[1] == '"':
		mode = FormQuoted
	case len(src) >= 2 && src[0] == '\\' && src[1] == '@':
		mode = FormConditional
	default:
		return false
	}
	session, offset := p.ExpressionSessionFor(left, right)
	end, err := session.ScanFormAt(ExpressionSourceOriginal, offset, offset+right-left, mode)
	return err == nil && end == offset+right-left
}

func (p *SemanticContext) GraphicsFieldTokens(left, right int) ([]ExprToken, bool, bool) {
	raw := p.Src[left:right]
	if !containsGraphicsMarker(raw) {
		tokens, err := p.LexSyntaxRange(left, right)
		if err != nil {
			return nil, false, false
		}
		return tokens, false, false
	}
	if p.GraphicsDebugMarker(left, right) {
		return nil, true, false
	}
	tokens, err := p.LexSyntaxRange(left, right)
	if err != nil {
		return nil, false, false
	}
	marked := false
	for _, token := range tokens {
		if token.Kind == ExprMarker {
			marked = true
			break
		}
	}
	if !marked {
		return tokens, false, false
	}
	return StripCommandMarkers(tokens), false, true
}

func containsGraphicsMarker(src []byte) bool {
	for i := 0; i+3 <= len(src); i++ {
		if src[i] == ';' && (src[i+1] == '!' || src[i+1] == '#') && src[i+2] == ';' {
			return true
		}
	}
	return false
}

func (p *SemanticContext) GraphicsDebugMarker(left, right int) bool {
	session := p.ScanSession(p.Src, left, right)
	for i := left; i < right; {
		next, atom := SyntaxAtomEnd(p.Src, i, right, session)
		if atom {
			if next < 0 {
				return false
			}
			i = next
			continue
		}
		if i+3 <= right && string(p.Src[i:i+3]) == ";#;" {
			return true
		}
		i++
	}
	return false
}

func (p *SemanticContext) GraphicsUnitSpan(left, right int) (exprEnd, unitStart int, ok bool) {
	session, offset := p.ExpressionSessionFor(left, right)
	return graphicsUnitSpanWithSession(p.Src, left, right, p.AllowFullwidthSpace, session, offset)
}

func (p *SemanticContext) GraphicsUnitCandidate(left, right int) (exprEnd, unitStart int, ok bool) {
	session, offset := p.ExpressionSessionFor(left, right)
	return graphicsUnitCandidateWithSession(p.Src, left, right, p.AllowFullwidthSpace, session, offset)
}

func graphicsUnitSpanWithSession(src []byte, left, right int, allowFullwidthSpace bool, session *ExpressionSession, offset int) (exprEnd, unitStart int, ok bool) {
	exprEnd, unitStart, ok = graphicsUnitCandidateWithSession(src, left, right, allowFullwidthSpace, session, offset)
	if !ok || !graphicsExpressionCompleteWithSession(src, left, exprEnd, allowFullwidthSpace, session, offset, left) {
		return right, right, false
	}
	return exprEnd, unitStart, true
}

func graphicsUnitCandidateWithSession(src []byte, left, right int, allowFullwidthSpace bool, session *ExpressionSession, offset int) (exprEnd, unitStart int, ok bool) {
	if right-left < 2 || !stringsEqualFoldASCII(src[right-2:right], "px") {
		return right, right, false
	}
	unitStart = right - 2
	if unitStart <= left {
		return right, right, false
	}
	tokens, err := session.LexRange(ExpressionSourceOriginal, offset, offset+right-left, left)
	if err != nil || len(tokens) < 2 {
		return right, right, false
	}
	unit := tokens[len(tokens)-1]
	if unit.Kind != ExprIdentifier || unit.Off != unitStart || len(unit.Text) != 2 {
		return right, right, false
	}
	exprEnd = TrimInputRight(src, left, unitStart, allowFullwidthSpace)
	return exprEnd, unitStart, exprEnd > left
}

func stringsEqualFoldASCII(a []byte, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		c := a[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != b[i] {
			return false
		}
	}
	return true
}

func (p *SemanticContext) GraphicsExpressionComplete(left, right int) bool {
	session, offset := p.ExpressionSessionFor(left, right)
	return graphicsExpressionCompleteWithSession(p.Src, left, right, p.AllowFullwidthSpace, session, offset, left)
}
