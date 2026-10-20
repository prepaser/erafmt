package format

import (
	"math"
	"strconv"
)

func (p *formatter) syntaxBits(i int, head []byte, base, wordEnd int) int {
	start, end, comment, ok := p.registeredBody(head, wordEnd)
	if !ok {
		p.syntaxOpaque(i)
		return -1
	}
	codeEnd := registeredCodeEnd(base, end, comment)
	fail := func(offset int, message string) int {
		p.syntaxProblem(i, offset, message)
		p.syntaxOpaque(i)
		return -1
	}
	if p.expressionParseEnabled() {
		if !registeredHasArgumentWithSpace(head, start, end, p.allowFullwidthSpace) {
			return fail(base+wordEnd, "bit command requires arguments")
		}
		if expressionExcludedWithSession(p.newExpressionSession(head), head) {
			p.syntaxOpaque(i)
			return -1
		}
		parts := p.splitTopLevel(p.src, base+start, codeEnd, ',')
		markerPresent := p.destinationRoleHasMarker(parts)
		for n, field := range parts {
			left := p.trimInputLeft(field.start, field.end)
			right := p.trimInputRight(left, field.end)
			if left == right {
				if n == len(parts)-1 && n > 0 {
					continue
				}
				return fail(left, "bit command contains an empty argument")
			}
			tokens, err := p.lexSyntaxRange(left, right)
			if err != nil {
				p.expressionProblem(i, 0, err)
				p.syntaxOpaque(i)
				return -1
			}
			tokens = bitLiteralTokens(tokens)
			if len(tokens) == 1 && (tokens[0].kind == exprString || tokens[0].kind == exprForm) {
				return fail(left, "bit command requires numeric arguments")
			}
			if n == 0 && setterInvalidDestination(tokens, p.dialect) {
				return fail(left, "bit command destination cannot be assigned")
			}
			if !markerPresent && builtinScalarIndexesMissing(tokens, p.dialect) {
				return fail(left, "scalar variable requires indexes")
			}
			if n == 0 && !markerPresent && !commandMacroPossibleForDialect(tokens, p.dialect) && knownExpressionTypeForDialect(tokens, p.dialect) == knownValueString {
				return fail(left, "bit command destination must be numeric")
			}
			negative := false
			if len(tokens) == 2 && tokens[0].kind == exprOperator && (string(tokens[0].text) == "+" || string(tokens[0].text) == "-") {
				negative = string(tokens[0].text) == "-"
				tokens = tokens[1:]
			}
			if len(tokens) != 1 || tokens[0].kind != exprNumber {
				continue
			}
			if n == 0 {
				return fail(left, "bit command destination cannot be a numeric literal")
			}
			if outside, known := bitLiteralOutsideRange(tokens[0].text, negative); known && outside {
				return fail(left, "bit index literal is outside 0..63")
			}
		}
	}
	mark := len(p.edits)
	if !p.syntaxExpression(i, base+start, codeEnd, false) {
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	p.setCodeEnd(i, p.trimInputRight(base+start, codeEnd))
	return codeEnd
}

func bitLiteralTokens(tokens []exprToken) []exprToken {
	for len(tokens) >= 3 && string(tokens[0].text) == "(" && string(tokens[len(tokens)-1].text) == ")" {
		depth := 0
		enclosed := true
		for i, token := range tokens {
			if string(token.text) == "(" {
				depth++
			} else if string(token.text) == ")" {
				depth--
				if depth == 0 && i != len(tokens)-1 {
					enclosed = false
					break
				}
			}
		}
		if !enclosed {
			break
		}
		tokens = tokens[1 : len(tokens)-1]
	}
	return tokens
}

func bitLiteralOutsideRange(src []byte, negative bool) (outside, known bool) {
	base, pos := 10, 0
	if len(src) > 2 && src[0] == '0' {
		switch src[1] {
		case 'x', 'X':
			base, pos = 16, 2
		case 'b', 'B':
			base, pos = 2, 2
		}
	}
	start := pos
	if pos < len(src) && src[pos] == '+' {
		pos++
	}
	for pos < len(src) && digitForBase(src[pos], base) {
		pos++
	}
	value, ok := bitInteger(src[start:pos], base)
	if !ok {
		return false, false
	}
	if pos == len(src) {
		if negative {
			value = -value
		}
		return value < 0 || value > 63, true
	}
	expBase := baseExponent(src[pos:], base)
	if expBase == 0 {
		return false, false
	}
	exponent, ok := bitInteger(src[pos+1:], base)
	if !ok {
		return false, false
	}
	if int32(exponent) == 0 {
		if negative {
			value = -value
		}
		return value < 0 || value > 63, true
	}
	number := float64(value) * math.Pow(float64(expBase), float64(int32(exponent)))
	if negative {
		number = -number
	}
	return math.IsNaN(number) || math.IsInf(number, 0) || number <= -1 || number >= 64, true
}

func bitInteger(src []byte, base int) (int64, bool) {
	if base == 10 {
		value, err := strconv.ParseInt(string(src), base, 64)
		return value, err == nil
	}
	if len(src) > 0 && src[0] == '+' {
		src = src[1:]
	}
	value, err := strconv.ParseUint(string(src), base, 64)
	return int64(value), err == nil
}
