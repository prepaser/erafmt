package erabasic

import "math"

type integerParser struct {
	Tokens []ExprToken
	Pos    int
}

func constantIntegerValue(tokens []ExprToken) (int64, bool) {
	if len(tokens) > 4096 {
		return 0, false
	}
	valueType, valid, known := constantExpressionType(tokens)
	if !known || !valid || valueType != KnownValueNumeric {
		return 0, false
	}
	for _, token := range tokens {
		if token.Kind == exprString {
			return 0, false
		}
	}
	p := integerParser{Tokens: tokens}
	value, ok := p.expression(1)
	return value, ok && p.Pos == len(tokens)
}

func (p *integerParser) expression(priority int) (int64, bool) {
	left, ok := p.operand()
	if !ok {
		return 0, false
	}
	for p.Pos < len(p.Tokens) {
		op := string(p.Tokens[p.Pos].Text)
		prec := constantPriority(op)
		if op == "#" || prec < priority {
			break
		}
		p.Pos++
		if op == "?" {
			yes, valid := p.expression(1)
			if !valid || p.Pos >= len(p.Tokens) || string(p.Tokens[p.Pos].Text) != "#" {
				return 0, false
			}
			p.Pos++
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
	if p.Pos >= len(p.Tokens) {
		return 0, false
	}
	token := p.Tokens[p.Pos]
	p.Pos++
	if token.Kind == exprNumber {
		return numericLiteralValue(token.Text)
	}
	op := string(token.Text)
	if op == "(" {
		value, ok := p.expression(1)
		if !ok || p.Pos >= len(p.Tokens) || string(p.Tokens[p.Pos].Text) != ")" {
			return 0, false
		}
		p.Pos++
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
