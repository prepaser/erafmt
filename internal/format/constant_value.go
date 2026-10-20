package format

import "math"

type integerParser struct {
	tokens []exprToken
	pos    int
}

func constantIntegerValue(tokens []exprToken) (int64, bool) {
	if len(tokens) > 4096 {
		return 0, false
	}
	valueType, valid, known := constantExpressionType(tokens)
	if !known || !valid || valueType != knownValueNumeric {
		return 0, false
	}
	for _, token := range tokens {
		if token.kind == exprString {
			return 0, false
		}
	}
	p := integerParser{tokens: tokens}
	value, ok := p.expression(1)
	return value, ok && p.pos == len(tokens)
}

func (p *integerParser) expression(priority int) (int64, bool) {
	left, ok := p.operand()
	if !ok {
		return 0, false
	}
	for p.pos < len(p.tokens) {
		op := string(p.tokens[p.pos].text)
		prec := constantPriority(op)
		if op == "#" || prec < priority {
			break
		}
		p.pos++
		if op == "?" {
			yes, valid := p.expression(1)
			if !valid || p.pos >= len(p.tokens) || string(p.tokens[p.pos].text) != "#" {
				return 0, false
			}
			p.pos++
			no, valid := p.expression(prec + 1)
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
		right, valid := p.expression(prec + 1)
		if !valid {
			return 0, false
		}
		left, ok = integerBinary(op, left, right)
		if !ok {
			return 0, false
		}
	}
	return left, true
}

func (p *integerParser) operand() (int64, bool) {
	if p.pos >= len(p.tokens) {
		return 0, false
	}
	token := p.tokens[p.pos]
	p.pos++
	if token.kind == exprNumber {
		return numericLiteralValue(token.text)
	}
	op := string(token.text)
	if op == "(" {
		value, ok := p.expression(1)
		if !ok || p.pos >= len(p.tokens) || string(p.tokens[p.pos].text) != ")" {
			return 0, false
		}
		p.pos++
		return value, true
	}
	if op != "+" && op != "-" && op != "!" && op != "~" {
		return 0, false
	}
	value, ok := p.operand()
	if !ok {
		return 0, false
	}
	switch op {
	case "-":
		value = -value
	case "!":
		value = integerBool(value == 0)
	case "~":
		value = ^value
	}
	return value, true
}

func integerBinary(op string, left, right int64) (int64, bool) {
	switch op {
	case "+":
		return left + right, true
	case "-":
		return left - right, true
	case "*":
		return left * right, true
	case "/", "%":
		if right == 0 || left == math.MinInt64 && right == -1 {
			return 0, false
		}
		if op == "/" {
			return left / right, true
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
		return integerBool(left == right), true
	case "!=":
		return integerBool(left != right), true
	case "<":
		return integerBool(left < right), true
	case ">":
		return integerBool(left > right), true
	case "<=":
		return integerBool(left <= right), true
	case ">=":
		return integerBool(left >= right), true
	case "&&":
		return integerBool(left != 0 && right != 0), true
	case "||":
		return integerBool(left != 0 || right != 0), true
	case "^^":
		return integerBool((left != 0) != (right != 0)), true
	case "!&":
		return integerBool(left == 0 || right == 0), true
	case "!|":
		return integerBool(left == 0 && right == 0), true
	}
	return 0, false
}

func integerBool(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
