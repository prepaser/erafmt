package cli

import (
	"math"
	"testing"
)

const (
	corpusConstantMaxTokens = 4096
	corpusConstantMaxDepth  = 8192
	corpusConstantMaxParens = 512
)

type corpusConstantToken struct {
	kind  byte
	op    string
	value int64
}

func corpusFormConstant(src []byte) (int64, bool) {
	tokens, ok := corpusConstantLex(src)
	if !ok || len(tokens) == 0 || len(tokens) > corpusConstantMaxTokens {
		return 0, false
	}
	p := corpusConstantParser{tokens: tokens}
	value, ok := p.expression(1)
	if !ok || p.pos != len(tokens) {
		return 0, false
	}
	return value, true
}

func corpusConstantLex(src []byte) ([]corpusConstantToken, bool) {
	tokens := make([]corpusConstantToken, 0, 16)
	for i := 0; i < len(src); {
		switch {
		case corpusConstantSpace(src[i]):
			i++
		case src[i] >= '0' && src[i] <= '9':
			end, value, ok := corpusConstantNumber(src, i)
			if !ok {
				return nil, false
			}
			tokens = append(tokens, corpusConstantToken{kind: 'n', value: value})
			i = end
		case src[i] == '(' || src[i] == ')':
			tokens = append(tokens, corpusConstantToken{kind: src[i], op: string(src[i])})
			i++
		default:
			op, size := corpusConstantOperator(src[i:])
			if size == 0 {
				return nil, false
			}
			tokens = append(tokens, corpusConstantToken{kind: 'o', op: op})
			i += size
		}
		if len(tokens) > corpusConstantMaxTokens {
			return nil, false
		}
	}
	return tokens, true
}

func corpusConstantSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n', '\v', '\f':
		return true
	default:
		return false
	}
}

func corpusConstantOperator(src []byte) (string, int) {
	if len(src) >= 2 {
		switch string(src[:2]) {
		case "<<", ">>", "<=", ">=", "==", "!=", "&&", "||", "^^", "!&", "!|":
			return string(src[:2]), 2
		}
	}
	if len(src) != 0 {
		switch src[0] {
		case '+', '-', '*', '/', '%', '&', '|', '^', '<', '>', '?', '#', '!', '~':
			return string(src[:1]), 1
		}
	}
	return "", 0
}

func corpusConstantNumber(src []byte, start int) (int, int64, bool) {
	base, pos := 10, start
	if pos+1 < len(src) && src[pos] == '0' {
		switch src[pos+1] {
		case 'x', 'X':
			base, pos = 16, pos+2
		case 'b', 'B':
			base, pos = 2, pos+2
		}
	}
	if base != 10 && pos < len(src) && src[pos] == '+' {
		pos++
	}
	digitStart := pos
	for pos < len(src) && corpusConstantDigit(src[pos], base) {
		pos++
	}
	if pos == digitStart {
		return pos, 0, false
	}
	mantissaEnd := pos
	exponentBase := 0
	if pos < len(src) {
		switch {
		case (src[pos] == 'e' || src[pos] == 'E') && base == 10:
			exponentBase = 10
		case (src[pos] == 'e' || src[pos] == 'E') && base == 2:
			exponentBase = 10
		case src[pos] == 'p' || src[pos] == 'P':
			exponentBase = 2
		}
	}
	if exponentBase == 0 {
		value, ok := corpusConstantDigits(src[digitStart:mantissaEnd], base)
		return pos, value, ok
	}
	exponentStart := pos + 1
	pos++
	if pos < len(src) && (src[pos] == '+' || src[pos] == '-') {
		if base != 10 && src[pos] == '-' {
			return pos, 0, false
		}
		pos++
	}
	exponentDigits := pos
	for pos < len(src) && corpusConstantDigit(src[pos], base) {
		pos++
	}
	if pos == exponentDigits {
		return pos, 0, false
	}
	value, ok := corpusConstantDigits(src[digitStart:mantissaEnd], base)
	if !ok {
		return pos, 0, false
	}
	exponent, ok := corpusConstantExponent(src[exponentStart:pos], base)
	if !ok {
		return pos, 0, false
	}
	if int32(exponent) == 0 {
		return pos, value, true
	}
	number := float64(value) * math.Pow(float64(exponentBase), float64(int32(exponent)))
	if math.IsNaN(number) || math.IsInf(number, 0) || number < float64(math.MinInt64) || number > float64(math.MaxInt64) {
		return pos, 0, false
	}
	if number == math.Ldexp(1, 63) || number == float64(math.MinInt64) {
		return pos, math.MinInt64, true
	}
	return pos, int64(number), true
}

func corpusConstantDigit(b byte, base int) bool {
	if b >= '0' && b <= '9' {
		return int(b-'0') < base
	}
	return base == 16 && (b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F')
}

func corpusConstantDigits(src []byte, base int) (int64, bool) {
	var value uint64
	for _, b := range src {
		digit := uint64(corpusConstantDigitValue(b))
		if value > (math.MaxUint64-digit)/uint64(base) {
			return 0, false
		}
		value = value*uint64(base) + digit
	}
	if base == 10 && value > uint64(math.MaxInt64) {
		return 0, false
	}
	return int64(value), true
}

func corpusConstantDigitValue(b byte) byte {
	if b >= '0' && b <= '9' {
		return b - '0'
	}
	if b >= 'a' && b <= 'f' {
		return b - 'a' + 10
	}
	return b - 'A' + 10
}

func corpusConstantExponent(src []byte, base int) (int64, bool) {
	if len(src) == 0 {
		return 0, false
	}
	negative := src[0] == '-'
	if negative || src[0] == '+' {
		src = src[1:]
	}
	var value int64
	if base == 10 && negative {
		magnitude, ok := corpusConstantMagnitude(src, base, uint64(math.MaxInt64)+1)
		if !ok {
			return 0, false
		}
		if magnitude == uint64(math.MaxInt64)+1 {
			value = math.MinInt64
		} else {
			value = -int64(magnitude)
		}
	} else {
		var ok bool
		value, ok = corpusConstantDigits(src, base)
		if !ok {
			return 0, false
		}
	}
	if negative && base != 10 {
		return -value, true
	}
	return value, true
}

func corpusConstantMagnitude(src []byte, base int, limit uint64) (uint64, bool) {
	var value uint64
	for _, b := range src {
		digit := uint64(corpusConstantDigitValue(b))
		if value > (limit-digit)/uint64(base) {
			return 0, false
		}
		value = value*uint64(base) + digit
	}
	return value, true
}

type corpusConstantParser struct {
	tokens []corpusConstantToken
	pos    int
	depth  int
	parens int
}

func (p *corpusConstantParser) expression(minPriority int) (int64, bool) {
	p.depth++
	if p.depth > corpusConstantMaxDepth {
		p.depth--
		return 0, false
	}
	defer func() { p.depth-- }()
	left, ok := p.prefix()
	if !ok {
		return 0, false
	}
	for p.pos < len(p.tokens) {
		token := p.tokens[p.pos]
		if token.kind != 'o' {
			break
		}
		if token.op == "#" {
			break
		}
		priority := corpusConstantPriority(token.op)
		if priority < minPriority {
			break
		}
		p.pos++
		if token.op == "?" {
			yes, valid := p.expression(1)
			if !valid || p.pos >= len(p.tokens) || p.tokens[p.pos].kind != 'o' || p.tokens[p.pos].op != "#" {
				return 0, false
			}
			p.pos++
			no, valid := p.expression(priority + 1)
			if !valid {
				return 0, false
			}
			if left != 0 {
				left = yes
			} else {
				left = no
			}
			continue
		}
		right, valid := p.expression(priority + 1)
		if !valid {
			return 0, false
		}
		left, valid = corpusConstantBinary(token.op, left, right)
		if !valid {
			return 0, false
		}
	}
	return left, true
}

func (p *corpusConstantParser) prefix() (int64, bool) {
	p.depth++
	if p.depth > corpusConstantMaxDepth {
		p.depth--
		return 0, false
	}
	defer func() { p.depth-- }()
	if p.pos >= len(p.tokens) {
		return 0, false
	}
	token := p.tokens[p.pos]
	p.pos++
	if token.kind == 'n' {
		return token.value, true
	}
	if token.kind == '(' {
		p.parens++
		if p.parens > corpusConstantMaxParens {
			p.parens--
			return 0, false
		}
		value, ok := p.expression(1)
		if p.pos >= len(p.tokens) || p.tokens[p.pos].kind != ')' {
			p.parens--
			return 0, false
		}
		p.pos++
		p.parens--
		return value, ok
	}
	if token.kind != 'o' || token.op != "+" && token.op != "-" && token.op != "!" && token.op != "~" {
		return 0, false
	}
	if p.pos >= len(p.tokens) || p.tokens[p.pos].kind == 'o' && (p.tokens[p.pos].op == "+" || p.tokens[p.pos].op == "-" || p.tokens[p.pos].op == "!" || p.tokens[p.pos].op == "~") {
		return 0, false
	}
	value, ok := p.prefix()
	if !ok {
		return 0, false
	}
	switch token.op {
	case "-":
		value = -value
	case "!":
		if value == 0 {
			value = 1
		} else {
			value = 0
		}
	case "~":
		value = ^value
	}
	return value, true
}

func corpusConstantPriority(op string) int {
	switch op {
	case "*", "/", "%":
		return 144
	case "+", "-":
		return 128
	case "<<", ">>":
		return 112
	case "<", ">", "<=", ">=":
		return 101
	case "==", "!=":
		return 96
	case "&", "|", "^":
		return 80
	case "&&", "||", "^^", "!&", "!|":
		return 64
	case "?":
		return 5
	default:
		return 0
	}
}

func corpusConstantBinary(op string, left, right int64) (int64, bool) {
	switch op {
	case "+":
		return left + right, true
	case "-":
		return left - right, true
	case "*":
		return left * right, true
	case "/":
		if right == 0 || left == math.MinInt64 && right == -1 {
			return 0, false
		}
		return left / right, true
	case "%":
		if right == 0 || left == math.MinInt64 && right == -1 {
			return 0, false
		}
		return left % right, true
	case "<<":
		return left << (uint64(right) & 63), true
	case ">>":
		return left >> (uint64(right) & 63), true
	case "&":
		return left & right, true
	case "|":
		return left | right, true
	case "^":
		return left ^ right, true
	case "==":
		return corpusConstantBool(left == right), true
	case "!=":
		return corpusConstantBool(left != right), true
	case "<":
		return corpusConstantBool(left < right), true
	case ">":
		return corpusConstantBool(left > right), true
	case "<=":
		return corpusConstantBool(left <= right), true
	case ">=":
		return corpusConstantBool(left >= right), true
	case "&&":
		return corpusConstantBool(left != 0 && right != 0), true
	case "||":
		return corpusConstantBool(left != 0 || right != 0), true
	case "^^":
		return corpusConstantBool((left != 0) != (right != 0)), true
	case "!&":
		return corpusConstantBool(left == 0 || right == 0), true
	case "!|":
		return corpusConstantBool(left == 0 && right == 0), true
	default:
		return 0, false
	}
}

func corpusConstantBool(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func TestCorpusFormConstant(t *testing.T) {
	tests := []struct {
		source string
		value  int64
		ok     bool
	}{
		{"1+2*3", 7, true},
		{"(1+2)*3", 9, true},
		{"1<<64", 1, true},
		{"1<<-1", math.MinInt64, true},
		{"-8>>65", -4, true},
		{"!0+~0", 0, true},
		{"1<2==1", 1, true},
		{"1&&2||0", 1, true},
		{"1^^2", 0, true},
		{"0!&1", 1, true},
		{"0!|0", 1, true},
		{"9223372036854775807+1", math.MinInt64, true},
		{"1?2+3#4", 5, true},
		{"0?2#3*4", 12, true},
		{"1?-1#0?1#1", 1, true},
		{"1?0#0?1#-1", -1, true},
		{"~1+2", 0, true},
		{"08", 8, true},
		{"-2147483648", -2147483648, true},
		{"2147483648", 2147483648, true},
		{"0x80000000", 0x80000000, true},
		{"1e3", 1000, true},
		{"9007201402224639e4294967296", 9007201402224639, true},
		{"1p3", 8, true},
		{"1/0", 0, false},
		{"0x8000000000000000/-1", 0, false},
		{"RESULT+1", 0, false},
		{"1,2", 0, false},
	}
	for _, test := range tests {
		value, ok := corpusFormConstant([]byte(test.source))
		if value != test.value || ok != test.ok {
			t.Errorf("%q: got %d, %v; want %d, %v", test.source, value, ok, test.value, test.ok)
		}
	}
}
