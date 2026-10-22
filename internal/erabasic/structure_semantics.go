package erabasic

import "bytes"

func (c *StructuralContext) ProvenSelectorType(head []byte, wordEnd, base int) KnownValueType {
	start, end, ok := c.structureExpressionRange(head, wordEnd, base)
	if !ok {
		return KnownValueUnknown
	}
	parts := c.split(start, end, ',')
	if len(parts) == 0 {
		return KnownValueUnknown
	}
	first, last := ExtraFieldBoundsWithSpace(c.Source, parts[0], c.AllowFullwidthSpace)
	if first == last {
		return KnownValueUnknown
	}
	return c.provenExpressionType(first, last)
}

func (c *StructuralContext) CaseTypeMismatch(before State, head []byte, wordEnd, base int) bool {
	if len(before.Frames) == 0 {
		return false
	}
	want := before.Frames[len(before.Frames)-1].SelectorType
	if want == KnownValueUnknown {
		return false
	}
	start, end, ok := c.structureExpressionRange(head, wordEnd, base)
	if !ok {
		return false
	}
	types, complete := c.provenCaseTypes(start, end)
	if !complete {
		return false
	}
	for _, got := range types {
		if got != want {
			return true
		}
	}
	return false
}

func (c *StructuralContext) structureExpressionRange(head []byte, wordEnd, base int) (int, int, bool) {
	_, start, end, comment, hasArg := c.instructionBody(head, wordEnd)
	if !hasArg {
		return 0, 0, false
	}
	if comment >= 0 {
		end = comment
	}
	start += base
	end += base
	if TrimInputLeft(c.Source, start, end, c.AllowFullwidthSpace) >= TrimInputRight(c.Source, start, end, c.AllowFullwidthSpace) {
		return 0, 0, false
	}
	return start, end, true
}

func (c *StructuralContext) provenExpressionType(start, end int) KnownValueType {
	if !c.ExpressionEnabled {
		return KnownValueUnknown
	}
	tokens, err := c.lex(start, end)
	if err != nil || len(tokens) == 0 || CommandMacroPossibleForDialect(tokens, c.Dialect) {
		return KnownValueUnknown
	}
	for _, token := range tokens {
		if token.Kind == ExprMarker {
			return KnownValueUnknown
		}
	}
	return KnownExpressionTypeForDialect(tokens, c.Dialect)
}

func (c *StructuralContext) provenCaseTypes(start, end int) ([]KnownValueType, bool) {
	if !c.ExpressionEnabled {
		return nil, false
	}
	parts := c.split(start, end, ',')
	types := make([]KnownValueType, 0, len(parts))
	for n, part := range parts {
		left, right := ExtraFieldBoundsWithSpace(c.Source, part, c.AllowFullwidthSpace)
		if left == right {
			if n == len(parts)-1 && len(types) > 0 {
				continue
			}
			return nil, false
		}
		tokens, err := c.lex(left, right)
		if err != nil || len(tokens) == 0 {
			return nil, false
		}
		for _, token := range tokens {
			if token.Kind == ExprMarker {
				return nil, false
			}
		}
		classifyOperators(tokens, true)
		if caseCommandMacroPossible(tokens, c.Dialect) {
			return nil, false
		}
		actual := provenCaseFieldType(tokens, c.Dialect)
		if actual == KnownValueUnknown {
			return nil, false
		}
		types = append(types, actual)
	}
	return types, len(types) > 0
}

func caseCommandMacroPossible(tokens []ExprToken, dialect Dialect) bool {
	for n, token := range tokens {
		if token.Kind == ExprIdentifier && !token.CaseKeyword && (!setterKnownNameForDialect(token.Text, dialect) || n+1 < len(tokens) && string(tokens[n+1].Text) == "(") {
			return true
		}
	}
	return false
}

func provenCaseFieldType(tokens []ExprToken, dialect Dialect) KnownValueType {
	classifyOperators(tokens, true)
	is, to := -1, -1
	for i, token := range tokens {
		if !token.CaseKeyword {
			continue
		}
		if bytes.EqualFold(token.Text, []byte("IS")) {
			if is >= 0 {
				return KnownValueUnknown
			}
			is = i
		} else if bytes.EqualFold(token.Text, []byte("TO")) {
			if to >= 0 {
				return KnownValueUnknown
			}
			to = i
		}
	}
	if is < 0 && to < 0 {
		return KnownExpressionTypeForDialect(tokens, dialect)
	}
	if is >= 0 && is != 0 {
		return KnownValueUnknown
	}
	if is >= 0 && to >= 0 {
		if to <= is+1 {
			return KnownValueUnknown
		}
		left := caseBoundType(tokens[is+1:to], dialect)
		right := caseBoundType(tokens[to+1:], dialect)
		if left != KnownValueUnknown && left == right {
			return left
		}
		return KnownValueUnknown
	}
	if is >= 0 {
		return caseBoundType(tokens[is+1:], dialect)
	}
	if to == 0 || to == len(tokens)-1 {
		return KnownValueUnknown
	}
	left := caseBoundType(tokens[:to], dialect)
	right := caseBoundType(tokens[to+1:], dialect)
	if left != KnownValueUnknown && left == right {
		return left
	}
	return KnownValueUnknown
}

func caseBoundType(tokens []ExprToken, dialect Dialect) KnownValueType {
	actual := KnownExpressionTypeForDialect(tokens, dialect)
	if actual != KnownValueUnknown || len(tokens) == 0 || tokens[0].Kind != ExprOperator {
		return actual
	}
	return KnownExpressionTypeForDialect(tokens[1:], dialect)
}
