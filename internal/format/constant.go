package format

type constantParser struct {
	tokens []exprToken
	pos    int
}

func constantExpressionType(tokens []exprToken) (knownValueType, bool, bool) {
	depth := 0
	for _, token := range tokens {
		switch token.kind {
		case exprNumber, exprString:
		case exprOperator:
			if string(token.text) == "=" {
				return knownValueUnknown, false, false
			}
		case exprSymbol:
			switch string(token.text) {
			case "(":
				depth++
				if depth > 256 {
					return knownValueUnknown, false, false
				}
			case ")":
				depth--
			}
		default:
			return knownValueUnknown, false, false
		}
	}
	p := constantParser{tokens: tokens}
	value, valid := p.expression()
	return value, valid && p.pos == len(tokens), true
}

func (p *constantParser) expression() (knownValueType, bool) {
	var values []knownValueType
	var ops []string
	want, unary, ternary := true, false, 0
	reduce := func() bool {
		if len(values) < 2 || len(ops) == 0 {
			return false
		}
		op := ops[len(ops)-1]
		ops = ops[:len(ops)-1]
		left, right := values[len(values)-2], values[len(values)-1]
		values = values[:len(values)-2]
		value := knownValueUnknown
		if op == "?" || op == "#" {
			if len(ops) == 0 || ops[len(ops)-1] != "?" || len(values) == 0 || values[len(values)-1] != knownValueNumeric || left != right {
				return false
			}
			ops = ops[:len(ops)-1]
			values = values[:len(values)-1]
			value = left
		} else {
			switch {
			case left == knownValueNumeric && right == knownValueNumeric:
				value = knownValueNumeric
			case left == knownValueString && right == knownValueString:
				switch op {
				case "+":
					value = knownValueString
				case "==", "!=", "<", ">", "<=", ">=":
					value = knownValueNumeric
				}
			case left != right && op == "*":
				value = knownValueString
			}
		}
		if value == knownValueUnknown {
			return false
		}
		values = append(values, value)
		return true
	}
	for p.pos < len(p.tokens) && string(p.tokens[p.pos].text) != ")" {
		token := p.tokens[p.pos]
		p.pos++
		op := string(token.text)
		if token.kind == exprNumber || token.kind == exprString || op == "(" {
			if !want {
				return knownValueUnknown, false
			}
			value := knownValueNumeric
			if token.kind == exprString {
				value = knownValueString
			} else if op == "(" {
				var valid bool
				value, valid = p.expression()
				if !valid || p.pos >= len(p.tokens) || string(p.tokens[p.pos].text) != ")" {
					return knownValueUnknown, false
				}
				p.pos++
			}
			if unary && value != knownValueNumeric {
				return knownValueUnknown, false
			}
			values = append(values, value)
			want, unary = false, false
			continue
		}
		if want {
			if unary || op != "+" && op != "-" && op != "!" && op != "~" {
				return knownValueUnknown, false
			}
			unary = true
			continue
		}
		priority := constantPriority(op)
		if priority == 0 {
			return knownValueUnknown, false
		}
		if op == "?" {
			ternary++
		} else if op == "#" {
			ternary--
			if ternary < 0 {
				return knownValueUnknown, false
			}
		}
		for len(ops) > 0 && constantPriority(ops[len(ops)-1]) >= priority {
			if !reduce() {
				return knownValueUnknown, false
			}
		}
		ops = append(ops, op)
		want = true
	}
	if want || ternary != 0 {
		return knownValueUnknown, false
	}
	for len(ops) > 0 {
		if !reduce() {
			return knownValueUnknown, false
		}
	}
	if len(values) != 1 {
		return knownValueUnknown, false
	}
	return values[0], true
}

func constantPriority(op string) int {
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
	case "#":
		return 16
	case "?":
		return 5
	default:
		return 0
	}
}

func constantExpressionsValid(tokens []exprToken) error {
	return provenExpressionsValid(tokens, false, CommonDialect)
}

func provenExpressionsValid(tokens []exprToken, builtinTypes bool, dialect Dialect) error {
	for _, token := range tokens {
		if token.kind == exprMarker {
			return nil
		}
	}
	depth, first := 0, 0
	for index := 0; index <= len(tokens); index++ {
		if index < len(tokens) {
			switch string(tokens[index].text) {
			case "(", "[":
				depth++
			case ")", "]":
				depth--
			}
			if string(tokens[index].text) != "," || depth != 0 {
				continue
			}
		}
		if first < index {
			field := tokens[first:index]
			_, valid, known := constantExpressionType(field)
			if known && !valid {
				return &exprError{offset: tokens[first].off, msg: "invalid constant expression"}
			}
			if !known && builtinTypes {
				if _, valid, proven := builtinExpressionType(field, dialect); proven && !valid {
					return &exprError{offset: tokens[first].off, msg: "invalid builtin variable expression"}
				}
			}
		}
		first = index + 1
	}
	return nil
}

func builtinExpressionType(tokens []exprToken, dialect Dialect) (knownValueType, bool, bool) {
	if len(tokens) > 4096 || commandMacroPossibleForDialect(tokens, dialect) {
		return knownValueUnknown, false, false
	}
	if value, valid, proven := builtinIncrementType(tokens, dialect); proven {
		return value, valid, true
	}
	tokens, collapsed, invalid := collapseBuiltinParentheses(tokens, dialect)
	if invalid {
		return knownValueUnknown, false, true
	}
	if value, valid, proven := builtinIncrementType(tokens, dialect); proven {
		return value, valid, true
	}
	typed := make([]exprToken, 0, len(tokens))
	operator, indexed := false, collapsed
	nesting := 0
	for n := 0; n < len(tokens); n++ {
		token := tokens[n]
		switch token.kind {
		case exprIdentifier:
			if n+1 < len(tokens) && string(tokens[n+1].text) == ":" {
				if end, value, known, invalid := directBuiltinIndexAtom(tokens, n, dialect); known && nesting <= 256 {
					if invalid {
						return knownValueUnknown, false, true
					}
					typed = append(typed, exprToken{kind: valueTokenKind(value), off: token.off})
					indexed = true
					n = end - 1
					continue
				}
				end := n + 2
				for {
					fieldStart := end
					for end < len(tokens) && string(tokens[end].text) != ":" && tokens[end].kind != exprOperator {
						end++
					}
					if fieldStart == end {
						return knownValueUnknown, false, true
					}
					if end >= len(tokens) || string(tokens[end].text) != ":" {
						break
					}
					end++
				}
				value, valid, proven := indexedBuiltinType(tokens[n:end], dialect)
				if !proven {
					return knownValueUnknown, false, false
				}
				if !valid {
					return knownValueUnknown, false, true
				}
				typed = append(typed, exprToken{kind: valueTokenKind(value), off: token.off})
				indexed = true
				n = end - 1
				continue
			}
			info, known := builtinVariableInfoByName(string(token.text), dialect)
			if !known {
				return knownValueUnknown, false, false
			}
			token.kind = valueTokenKind(info.value)
			typed = append(typed, token)
		case exprSymbol:
			if string(token.text) != "(" && string(token.text) != ")" {
				return knownValueUnknown, false, false
			}
			if string(token.text) == "(" {
				nesting++
			} else {
				nesting--
			}
			typed = append(typed, token)
		case exprOperator:
			op := string(token.text)
			if constantPriority(op) == 0 && op != "!" && op != "~" {
				return knownValueUnknown, false, false
			}
			typed = append(typed, token)
			operator = true
		case exprNumber, exprString:
			typed = append(typed, token)
		default:
			return knownValueUnknown, false, false
		}
	}
	if !operator && !indexed {
		return knownValueUnknown, false, false
	}
	return constantExpressionType(typed)
}

func builtinIncrementType(tokens []exprToken, dialect Dialect) (knownValueType, bool, bool) {
	tokens = stripExpressionParens(tokens)
	if len(tokens) < 2 {
		return knownValueUnknown, false, false
	}
	var operand []exprToken
	if isIncrementOperator(tokens[len(tokens)-1]) {
		operand = tokens[:len(tokens)-1]
	} else if isIncrementOperator(tokens[0]) {
		operand = tokens[1:]
	} else {
		return knownValueUnknown, false, false
	}
	operand = stripExpressionParens(operand)
	if len(operand) == 1 && operand[0].kind == exprIdentifier {
		if info, known := builtinVariableInfoByName(string(operand[0].text), dialect); known {
			if info.readOnly {
				return info.value, false, true
			}
			return info.value, info.value == knownValueNumeric, true
		}
	}
	if value, valid, proven := indexedBuiltinType(operand, dialect); proven {
		if builtinVariableReadOnly(operand, dialect) {
			return value, false, true
		}
		return value, valid && value == knownValueNumeric, true
	}
	if info, known := builtinVariableInfo(operand); known {
		return info.value, info.value == knownValueNumeric, true
	}
	if value := directBuiltinValueType(operand, dialect); value == knownValueString {
		return value, false, true
	}
	return knownValueUnknown, false, false
}

func collapseBuiltinParentheses(tokens []exprToken, dialect Dialect) ([]exprToken, bool, bool) {
	return collapseBuiltinParenthesesDepth(tokens, dialect, 0)
}

func collapseBuiltinParenthesesDepth(tokens []exprToken, dialect Dialect, depth int) ([]exprToken, bool, bool) {
	if depth > 256 || len(tokens) < 3 {
		return tokens, false, false
	}
	var out []exprToken
	copied := 0
	replace := func(start, end int, replacement []exprToken) {
		if out == nil {
			out = make([]exprToken, 0, len(tokens))
		}
		out = append(out, tokens[copied:start]...)
		out = append(out, replacement...)
		copied = end
	}
	for i := 0; i < len(tokens); i++ {
		if string(tokens[i].text) != "(" {
			continue
		}
		close := expressionMatchingParen(tokens, i)
		if close < 0 || close == i+1 {
			continue
		}
		if i > 0 && string(tokens[i-1].text) == ":" {
			i = close
			continue
		}
		inner, changed, invalid := collapseBuiltinParenthesesDepth(tokens[i+1:close], dialect, depth+1)
		if invalid {
			return nil, false, true
		}
		var atom exprToken
		proof := false
		if len(inner) == 1 {
			if inner[0].kind == exprIdentifier {
				if info, known := builtinVariableInfoByName(string(inner[0].text), dialect); known {
					atom = exprToken{kind: valueTokenKind(info.value), off: tokens[i].off}
					proof = true
				}
			} else if inner[0].kind == exprNumber || inner[0].kind == exprString {
				atom = inner[0]
				proof = true
			}
		} else if len(inner) >= 3 && inner[0].kind == exprIdentifier && string(inner[1].text) == ":" {
			value, valid, proven := indexedBuiltinType(inner, dialect)
			if proven && !valid {
				return nil, false, true
			}
			if proven {
				atom = exprToken{kind: valueTokenKind(value), off: tokens[i].off}
				proof = true
			}
		}
		if proof {
			replace(i, close+1, []exprToken{atom})
		} else if changed {
			replace(i+1, close, inner)
		}
		i = close
	}
	if out == nil {
		return tokens, false, false
	}
	out = append(out, tokens[copied:]...)
	return out, true, false
}

func expressionMatchingParen(tokens []exprToken, start int) int {
	depth := 0
	for i := start; i < len(tokens); i++ {
		switch string(tokens[i].text) {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func valueTokenKind(value knownValueType) exprTokenKind {
	if value == knownValueString {
		return exprString
	}
	return exprNumber
}

func directBuiltinIndexAtom(tokens []exprToken, start int, dialect Dialect) (int, knownValueType, bool, bool) {
	end := start + 1
	if end >= len(tokens) || string(tokens[end].text) != ":" {
		return end, knownValueUnknown, false, false
	}
	var indexes []knownValueType
	invalidIndex := false
	for end < len(tokens) && string(tokens[end].text) == ":" {
		end++
		if end >= len(tokens) {
			return end, knownValueUnknown, false, false
		}
		fieldStart := end
		if tokens[end].kind == exprSymbol && string(tokens[end].text) == "(" {
			close := expressionMatchingParen(tokens, end)
			if close < 0 {
				return end, knownValueUnknown, false, false
			}
			end = close + 1
		} else {
			switch tokens[end].kind {
			case exprIdentifier, exprNumber, exprString:
				end++
			default:
				return end, knownValueUnknown, false, false
			}
		}
		field := tokens[fieldStart:end]
		value, valid, proven := constantExpressionType(field)
		if !proven {
			value, valid, proven = builtinExpressionType(field, dialect)
		}
		if proven && !valid {
			invalidIndex = true
		}
		if !proven {
			if len(field) > 1 && provenExpressionShape(field, dialect, false) != nil {
				invalidIndex = true
			}
			value = knownExpressionTypeForDialect(field, dialect)
		}
		indexes = append(indexes, value)
	}
	value := directBuiltinValueType(tokens[start:end], dialect)
	if value == knownValueUnknown {
		return end, value, false, false
	}
	name := string(tokens[start].text)
	info, _ := builtinVariableInfoByName(name, dialect)
	if invalidIndex || !indexedBuiltinArityAllowed(info, len(indexes)) {
		return end, value, true, true
	}
	for i, kind := range indexes {
		if kind == knownValueString && !builtinStringIndexAllowed(name, info, i, len(indexes), dialect) {
			return end, value, true, true
		}
	}
	return end, value, true, false
}
