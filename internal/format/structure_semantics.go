package format

import "bytes"

func (p *formatter) provenSelectorType(head []byte, wordEnd, base int) knownValueType {
	start, end, ok := p.structureExpressionRange(head, wordEnd, base)
	if !ok {
		return knownValueUnknown
	}
	parts := p.splitTopLevel(p.src, start, end, ',')
	if len(parts) == 0 {
		return knownValueUnknown
	}
	first, last := extraFieldBoundsWithSpace(p.src, parts[0], p.allowFullwidthSpace)
	if first == last {
		return knownValueUnknown
	}
	return p.provenExpressionType(first, last)
}

func (p *formatter) caseTypeMismatch(before state, head []byte, wordEnd, base int) bool {
	if len(before.frames) == 0 {
		return false
	}
	want := before.frames[len(before.frames)-1].selectorType
	if want == knownValueUnknown {
		return false
	}
	start, end, ok := p.structureExpressionRange(head, wordEnd, base)
	if !ok {
		return false
	}
	types, complete := p.provenCaseTypes(start, end)
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

func (p *formatter) structureExpressionRange(head []byte, wordEnd, base int) (int, int, bool) {
	_, start, end, comment, hasArg := p.instructionBody(head, wordEnd)
	if !hasArg {
		return 0, 0, false
	}
	if comment >= 0 {
		end = comment
	}
	start += base
	end += base
	if trimInputLeft(p.src, start, end, p.allowFullwidthSpace) >= trimInputRight(p.src, start, end, p.allowFullwidthSpace) {
		return 0, 0, false
	}
	return start, end, true
}

func (p *formatter) provenExpressionType(start, end int) knownValueType {
	if !p.expressionParseEnabled() {
		return knownValueUnknown
	}
	tokens, err := p.lexSyntaxRange(start, end)
	if err != nil || len(tokens) == 0 || commandMacroPossibleForDialect(tokens, p.dialect) {
		return knownValueUnknown
	}
	for _, token := range tokens {
		if token.kind == exprMarker {
			return knownValueUnknown
		}
	}
	return knownExpressionTypeForDialect(tokens, p.dialect)
}

func (p *formatter) provenCaseTypes(start, end int) ([]knownValueType, bool) {
	if !p.expressionParseEnabled() {
		return nil, false
	}
	parts := p.splitTopLevel(p.src, start, end, ',')
	types := make([]knownValueType, 0, len(parts))
	for n, part := range parts {
		left, right := extraFieldBoundsWithSpace(p.src, part, p.allowFullwidthSpace)
		if left == right {
			if n == len(parts)-1 && len(types) > 0 {
				continue
			}
			return nil, false
		}
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil || len(tokens) == 0 {
			return nil, false
		}
		for _, token := range tokens {
			if token.kind == exprMarker {
				return nil, false
			}
		}
		classifyOperators(tokens, true)
		if caseCommandMacroPossible(tokens, p.dialect) {
			return nil, false
		}
		actual := provenCaseFieldType(tokens, p.dialect)
		if actual == knownValueUnknown {
			return nil, false
		}
		types = append(types, actual)
	}
	return types, len(types) > 0
}

func caseCommandMacroPossible(tokens []exprToken, dialect Dialect) bool {
	for n, token := range tokens {
		if token.kind == exprIdentifier && !token.caseKeyword && (!setterKnownNameForDialect(token.text, dialect) || n+1 < len(tokens) && string(tokens[n+1].text) == "(") {
			return true
		}
	}
	return false
}

func provenCaseFieldType(tokens []exprToken, dialect Dialect) knownValueType {
	classifyOperators(tokens, true)
	is, to := -1, -1
	for i, token := range tokens {
		if !token.caseKeyword {
			continue
		}
		if bytes.EqualFold(token.text, []byte("IS")) {
			if is >= 0 {
				return knownValueUnknown
			}
			is = i
		} else if bytes.EqualFold(token.text, []byte("TO")) {
			if to >= 0 {
				return knownValueUnknown
			}
			to = i
		}
	}
	if is < 0 && to < 0 {
		return knownExpressionTypeForDialect(tokens, dialect)
	}
	if is >= 0 && is != 0 {
		return knownValueUnknown
	}
	if is >= 0 && to >= 0 {
		if to <= is+1 {
			return knownValueUnknown
		}
		left := caseBoundType(tokens[is+1:to], dialect)
		right := caseBoundType(tokens[to+1:], dialect)
		if left != knownValueUnknown && left == right {
			return left
		}
		return knownValueUnknown
	}
	if is >= 0 {
		return caseBoundType(tokens[is+1:], dialect)
	}
	if to == 0 || to == len(tokens)-1 {
		return knownValueUnknown
	}
	left := caseBoundType(tokens[:to], dialect)
	right := caseBoundType(tokens[to+1:], dialect)
	if left != knownValueUnknown && left == right {
		return left
	}
	return knownValueUnknown
}

func caseBoundType(tokens []exprToken, dialect Dialect) knownValueType {
	actual := knownExpressionTypeForDialect(tokens, dialect)
	if actual != knownValueUnknown || len(tokens) == 0 || tokens[0].kind != exprOperator {
		return actual
	}
	return knownExpressionTypeForDialect(tokens[1:], dialect)
}
