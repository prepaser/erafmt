package erabasic

import "math"

func numberLiteralFits(mantissa, exponent []byte, base, exponentBase int) bool {
	_, ok := numberLiteralValue(mantissa, exponent, base, exponentBase)
	return ok
}

func DirectIntegerLiteral(tokens []ExprToken) (int64, KnownValueType) {
	tokens = bitLiteralTokens(tokens)
	negative := false
	if len(tokens) > 0 && tokens[0].Kind == ExprOperator {
		switch string(tokens[0].Text) {
		case "+", "-":
			negative = string(tokens[0].Text) == "-"
			tokens = bitLiteralTokens(tokens[1:])
		}
	}
	if len(tokens) != 1 {
		return 0, KnownValueUnknown
	}
	switch tokens[0].Kind {
	case exprString, ExprForm:
		return 0, KnownValueString
	case exprNumber:
		value, ok := numericLiteralValue(tokens[0].Text)
		if !ok {
			return 0, KnownValueUnknown
		}
		if negative && value != math.MinInt64 {
			value = -value
		}
		return value, KnownValueNumeric
	default:
		return 0, KnownValueUnknown
	}
}

func numericLiteralValue(src []byte) (int64, bool) {
	base, pos := 10, 0
	if len(src) > 2 && src[0] == '0' {
		switch src[1] {
		case 'x', 'X':
			base, pos = 16, 2
		case 'b', 'B':
			base, pos = 2, 2
		}
	}
	if base != 10 && pos < len(src) && src[pos] == '+' {
		pos++
	}
	digitStart := pos
	for pos < len(src) && digitForBase(src[pos], base) {
		pos++
	}
	if pos == digitStart {
		return 0, false
	}
	exponentBase := baseExponent(src[pos:], base)
	if exponentBase == 0 {
		value, ok := numberLiteralValue(src[digitStart:pos], nil, base, 0)
		return value, ok && pos == len(src)
	}
	exponentStart := pos + 1
	return numberLiteralValue(src[digitStart:pos], src[exponentStart:], base, exponentBase)
}

// The runtime rounds its Int64 bounds to float64 before comparing.
// float64(math.MaxInt64) is 2^63, which the runtime converts to MinInt64.
func numberLiteralValue(mantissa, exponent []byte, base, exponentBase int) (int64, bool) {
	value, ok := bitInteger(mantissa, base)
	if !ok {
		return 0, false
	}
	if exponentBase == 0 {
		return value, true
	}
	power, ok := bitInteger(exponent, base)
	if !ok {
		return 0, false
	}
	if int32(power) == 0 {
		return value, true
	}
	number := float64(value) * math.Pow(float64(exponentBase), float64(int32(power)))
	if math.IsNaN(number) || math.IsInf(number, 0) || number < float64(math.MinInt64) || number > float64(math.MaxInt64) {
		return 0, false
	}
	if number == math.Ldexp(1, 63) || number == float64(math.MinInt64) {
		return math.MinInt64, true
	}
	return int64(number), true
}
