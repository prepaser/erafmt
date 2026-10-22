package erabasic

import (
	"math"
	"strconv"
)

type bitArgumentValidation struct {
	Offset   int
	Message  string
	ExprErr  error
	Excluded bool
	Valid    bool
}

func (p *SemanticContext) ValidateBitArguments(head []byte, base, start, end, codeEnd int) bitArgumentValidation {
	result := bitArgumentValidation{Valid: true}
	if !RegisteredHasArgumentWithSpace(head, start, end, p.AllowFullwidthSpace) {
		result.Valid = false
		result.Offset = base + len(head)
		result.Message = "bit command requires arguments"
		return result
	}
	session := p.ScanSession(head, 0, len(head))
	if ExpressionExcludedWithSession(session, head) {
		result.Excluded = true
		return result
	}
	parts := p.SplitTopLevel(p.Src, base+start, codeEnd, ',')
	markerPresent := false
	for _, field := range parts {
		left := TrimInputLeft(p.Src, field.Start, field.End, p.AllowFullwidthSpace)
		right := TrimInputRight(p.Src, left, field.End, p.AllowFullwidthSpace)
		if left < right {
			tokens, err := p.LexSyntaxRange(left, right)
			if err == nil {
				for _, token := range tokens {
					if token.Kind == ExprMarker {
						markerPresent = true
						break
					}
				}
			}
		}
	}
	for n, field := range parts {
		left := TrimInputLeft(p.Src, field.Start, field.End, p.AllowFullwidthSpace)
		right := TrimInputRight(p.Src, left, field.End, p.AllowFullwidthSpace)
		if left == right {
			if n == len(parts)-1 && n > 0 {
				continue
			}
			return bitArgumentValidation{Offset: left, Message: "bit command contains an empty argument"}
		}
		tokens, err := p.LexSyntaxRange(left, right)
		if err != nil {
			return bitArgumentValidation{Offset: 0, ExprErr: err}
		}
		tokens = bitLiteralTokens(tokens)
		if len(tokens) == 1 && (tokens[0].Kind == exprString || tokens[0].Kind == ExprForm) {
			return bitArgumentValidation{Offset: left, Message: "bit command requires numeric arguments"}
		}
		if n == 0 && p.setterInvalidDestination(tokens) {
			return bitArgumentValidation{Offset: left, Message: "bit command destination cannot be assigned"}
		}
		if !markerPresent && BuiltinScalarIndexesMissing(tokens, p.Dialect) {
			return bitArgumentValidation{Offset: left, Message: "scalar variable requires indexes"}
		}
		if n == 0 && !markerPresent && !CommandMacroPossibleForDialect(tokens, p.Dialect) && KnownExpressionTypeForDialect(tokens, p.Dialect) == KnownValueString {
			return bitArgumentValidation{Offset: left, Message: "bit command destination must be numeric"}
		}
		negative := false
		if len(tokens) == 2 && tokens[0].Kind == ExprOperator && (string(tokens[0].Text) == "+" || string(tokens[0].Text) == "-") {
			negative = string(tokens[0].Text) == "-"
			tokens = tokens[1:]
		}
		if len(tokens) != 1 || tokens[0].Kind != exprNumber {
			continue
		}
		if n == 0 {
			return bitArgumentValidation{Offset: left, Message: "bit command destination cannot be a numeric literal"}
		}
		if outside, known := bitLiteralOutsideRange(tokens[0].Text, negative); known && outside {
			return bitArgumentValidation{Offset: left, Message: "bit index literal is outside 0..63"}
		}
	}
	return result
}

func bitLiteralTokens(tokens []ExprToken) []ExprToken {
	for len(tokens) >= 3 && string(tokens[0].Text) == "(" && string(tokens[len(tokens)-1].Text) == ")" {
		depth := 0
		enclosed := true
		for i, token := range tokens {
			if string(token.Text) == "(" {
				depth++
			} else if string(token.Text) == ")" {
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
