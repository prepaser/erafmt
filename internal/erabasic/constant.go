package erabasic

type constantParser struct {
	Tokens []ExprToken
	Pos    int
}

func constantExpressionType(tokens []ExprToken) (KnownValueType, bool, bool) {
	depth := 0
	for _, token := range tokens {
		switch token.Kind {
		case exprNumber, exprString:
		case ExprOperator:
			if string(token.Text) == "=" {
				return KnownValueUnknown, false, false
			}
		case exprSymbol:
			switch string(token.Text) {
			case "(":
				depth++
				if depth > 256 {
					return KnownValueUnknown, false, false
				}
			case ")":
				depth--
			}
		default:
			return KnownValueUnknown, false, false
		}
	}
	p := constantParser{Tokens: tokens}
	value, valid := p.expression()
	return value, valid && p.Pos == len(tokens), true
}

func (p *constantParser) expression() (KnownValueType, bool) {
	var values []KnownValueType
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
		value := KnownValueUnknown
		if op == "?" || op == "#" {
			if len(ops) == 0 || ops[len(ops)-1] != "?" || len(values) == 0 || values[len(values)-1] != KnownValueNumeric || left != right {
				return false
			}
			ops = ops[:len(ops)-1]
			values = values[:len(values)-1]
			value = left
		} else {
			switch {
			case left == KnownValueNumeric && right == KnownValueNumeric:
				value = KnownValueNumeric
			case left == KnownValueString && right == KnownValueString:
				switch op {
				case "+":
					value = KnownValueString
				case "==", "!=", "<", ">", "<=", ">=":
					value = KnownValueNumeric
				}
			case left != right && op == "*":
				value = KnownValueString
			}
		}
		if value == KnownValueUnknown {
			return false
		}
		values = append(values, value)
		return true
	}
	for p.Pos < len(p.Tokens) && string(p.Tokens[p.Pos].Text) != ")" {
		token := p.Tokens[p.Pos]
		p.Pos++
		op := string(token.Text)
		if token.Kind == exprNumber || token.Kind == exprString || op == "(" {
			if !want {
				return KnownValueUnknown, false
			}
			value := KnownValueNumeric
			if token.Kind == exprString {
				value = KnownValueString
			} else if op == "(" {
				var valid bool
				value, valid = p.expression()
				if !valid || p.Pos >= len(p.Tokens) || string(p.Tokens[p.Pos].Text) != ")" {
					return KnownValueUnknown, false
				}
				p.Pos++
			}
			if unary && value != KnownValueNumeric {
				return KnownValueUnknown, false
			}
			values = append(values, value)
			want, unary = false, false
			continue
		}
		if want {
			if unary || op != "+" && op != "-" && op != "!" && op != "~" {
				return KnownValueUnknown, false
			}
			unary = true
			continue
		}
		priority := constantPriority(op)
		if priority == 0 {
			return KnownValueUnknown, false
		}
		if op == "?" {
			ternary++
		} else if op == "#" {
			ternary--
			if ternary < 0 {
				return KnownValueUnknown, false
			}
		}
		for len(ops) > 0 && constantPriority(ops[len(ops)-1]) >= priority {
			if !reduce() {
				return KnownValueUnknown, false
			}
		}
		ops = append(ops, op)
		want = true
	}
	if want || ternary != 0 {
		return KnownValueUnknown, false
	}
	for len(ops) > 0 {
		if !reduce() {
			return KnownValueUnknown, false
		}
	}
	if len(values) != 1 {
		return KnownValueUnknown, false
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

func ConstantExpressionsValid(tokens []ExprToken) error {
	return provenExpressionsValid(tokens, false, CommonDialect)
}

func provenExpressionsValid(tokens []ExprToken, builtinTypes bool, dialect Dialect) error {
	for _, token := range tokens {
		if token.Kind == ExprMarker {
			return nil
		}
	}
	depth, first := 0, 0
	for index := 0; index <= len(tokens); index++ {
		if index < len(tokens) {
			switch string(tokens[index].Text) {
			case "(", "[":
				depth++
			case ")", "]":
				depth--
			}
			if string(tokens[index].Text) != "," || depth != 0 {
				continue
			}
		}
		if first < index {
			field := tokens[first:index]
			_, valid, known := constantExpressionType(field)
			if known && !valid {
				return &ExprError{Offset: tokens[first].Off, Msg: "invalid constant expression"}
			}
			if !known && builtinTypes {
				if _, valid, proven := BuiltinExpressionType(field, dialect); proven && !valid {
					return &ExprError{Offset: tokens[first].Off, Msg: "invalid builtin variable expression"}
				}
			}
		}
		first = index + 1
	}
	return nil
}

func BuiltinExpressionType(tokens []ExprToken, dialect Dialect) (KnownValueType, bool, bool) {
	if len(tokens) > 4096 || CommandMacroPossibleForDialect(tokens, dialect) {
		return KnownValueUnknown, false, false
	}
	if value, valid, proven := builtinIncrementType(tokens, dialect); proven {
		return value, valid, true
	}
	tokens, collapsed, invalid := collapseBuiltinParentheses(tokens, dialect)
	if invalid {
		return KnownValueUnknown, false, true
	}
	if value, valid, proven := builtinIncrementType(tokens, dialect); proven {
		return value, valid, true
	}
	typed := make([]ExprToken, 0, len(tokens))
	operator, indexed := false, collapsed
	nesting := 0
	for n := 0; n < len(tokens); n++ {
		token := tokens[n]
		switch token.Kind {
		case ExprIdentifier:
			if n+1 < len(tokens) && string(tokens[n+1].Text) == ":" {
				if end, value, known, invalid := directBuiltinIndexAtom(tokens, n, dialect); known && nesting <= 256 {
					if invalid {
						return KnownValueUnknown, false, true
					}
					typed = append(typed, ExprToken{Kind: valueTokenKind(value), Off: token.Off})
					indexed = true
					n = end - 1
					continue
				}
				end := n + 2
				for {
					fieldStart := end
					for end < len(tokens) && string(tokens[end].Text) != ":" && tokens[end].Kind != ExprOperator {
						end++
					}
					if fieldStart == end {
						return KnownValueUnknown, false, true
					}
					if end >= len(tokens) || string(tokens[end].Text) != ":" {
						break
					}
					end++
				}
				value, valid, proven := indexedBuiltinType(tokens[n:end], dialect)
				if !proven {
					return KnownValueUnknown, false, false
				}
				if !valid {
					return KnownValueUnknown, false, true
				}
				typed = append(typed, ExprToken{Kind: valueTokenKind(value), Off: token.Off})
				indexed = true
				n = end - 1
				continue
			}
			info, known := BuiltinVariableInfoByName(string(token.Text), dialect)
			if !known {
				return KnownValueUnknown, false, false
			}
			token.Kind = valueTokenKind(info.Value)
			typed = append(typed, token)
		case exprSymbol:
			if string(token.Text) != "(" && string(token.Text) != ")" {
				return KnownValueUnknown, false, false
			}
			if string(token.Text) == "(" {
				nesting++
			} else {
				nesting--
			}
			typed = append(typed, token)
		case ExprOperator:
			op := string(token.Text)
			if constantPriority(op) == 0 && op != "!" && op != "~" {
				return KnownValueUnknown, false, false
			}
			typed = append(typed, token)
			operator = true
		case exprNumber, exprString:
			typed = append(typed, token)
		default:
			return KnownValueUnknown, false, false
		}
	}
	if !operator && !indexed {
		return KnownValueUnknown, false, false
	}
	return constantExpressionType(typed)
}

func builtinIncrementType(tokens []ExprToken, dialect Dialect) (KnownValueType, bool, bool) {
	tokens = stripExpressionParens(tokens)
	if len(tokens) < 2 {
		return KnownValueUnknown, false, false
	}
	var operand []ExprToken
	if isIncrementOperator(tokens[len(tokens)-1]) {
		operand = tokens[:len(tokens)-1]
	} else if isIncrementOperator(tokens[0]) {
		operand = tokens[1:]
	} else {
		return KnownValueUnknown, false, false
	}
	operand = stripExpressionParens(operand)
	if len(operand) == 1 && operand[0].Kind == ExprIdentifier {
		if info, known := BuiltinVariableInfoByName(string(operand[0].Text), dialect); known {
			if info.ReadOnly {
				return info.Value, false, true
			}
			return info.Value, info.Value == KnownValueNumeric, true
		}
	}
	if value, valid, proven := indexedBuiltinType(operand, dialect); proven {
		if BuiltinVariableReadOnly(operand, dialect) {
			return value, false, true
		}
		return value, valid && value == KnownValueNumeric, true
	}
	if info, known := builtinVariableInfo(operand); known {
		return info.Value, info.Value == KnownValueNumeric, true
	}
	if value := directBuiltinValueType(operand, dialect); value == KnownValueString {
		return value, false, true
	}
	return KnownValueUnknown, false, false
}

func collapseBuiltinParentheses(tokens []ExprToken, dialect Dialect) ([]ExprToken, bool, bool) {
	return collapseBuiltinParenthesesDepth(tokens, dialect, 0)
}

func collapseBuiltinParenthesesDepth(tokens []ExprToken, dialect Dialect, depth int) ([]ExprToken, bool, bool) {
	if depth > 256 || len(tokens) < 3 {
		return tokens, false, false
	}
	var out []ExprToken
	copied := 0
	replace := func(start, end int, replacement []ExprToken) {
		if out == nil {
			out = make([]ExprToken, 0, len(tokens))
		}
		out = append(out, tokens[copied:start]...)
		out = append(out, replacement...)
		copied = end
	}
	for i := 0; i < len(tokens); i++ {
		if string(tokens[i].Text) != "(" {
			continue
		}
		close := expressionMatchingParen(tokens, i)
		if close < 0 || close == i+1 {
			continue
		}
		if i > 0 && string(tokens[i-1].Text) == ":" {
			i = close
			continue
		}
		inner, changed, invalid := collapseBuiltinParenthesesDepth(tokens[i+1:close], dialect, depth+1)
		if invalid {
			return nil, false, true
		}
		var atom ExprToken
		proof := false
		if len(inner) == 1 {
			if inner[0].Kind == ExprIdentifier {
				if info, known := BuiltinVariableInfoByName(string(inner[0].Text), dialect); known {
					atom = ExprToken{Kind: valueTokenKind(info.Value), Off: tokens[i].Off}
					proof = true
				}
			} else if inner[0].Kind == exprNumber || inner[0].Kind == exprString {
				atom = inner[0]
				proof = true
			}
		} else if len(inner) >= 3 && inner[0].Kind == ExprIdentifier && string(inner[1].Text) == ":" {
			value, valid, proven := indexedBuiltinType(inner, dialect)
			if proven && !valid {
				return nil, false, true
			}
			if proven {
				atom = ExprToken{Kind: valueTokenKind(value), Off: tokens[i].Off}
				proof = true
			}
		}
		if proof {
			replace(i, close+1, []ExprToken{atom})
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

func expressionMatchingParen(tokens []ExprToken, start int) int {
	depth := 0
	for i := start; i < len(tokens); i++ {
		switch string(tokens[i].Text) {
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

func valueTokenKind(value KnownValueType) exprTokenKind {
	if value == KnownValueString {
		return exprString
	}
	return exprNumber
}

func directBuiltinIndexAtom(tokens []ExprToken, start int, dialect Dialect) (int, KnownValueType, bool, bool) {
	end := start + 1
	if end >= len(tokens) || string(tokens[end].Text) != ":" {
		return end, KnownValueUnknown, false, false
	}
	var indexes []KnownValueType
	invalidIndex := false
	for end < len(tokens) && string(tokens[end].Text) == ":" {
		end++
		if end >= len(tokens) {
			return end, KnownValueUnknown, false, false
		}
		fieldStart := end
		if tokens[end].Kind == exprSymbol && string(tokens[end].Text) == "(" {
			close := expressionMatchingParen(tokens, end)
			if close < 0 {
				return end, KnownValueUnknown, false, false
			}
			end = close + 1
		} else {
			switch tokens[end].Kind {
			case ExprIdentifier, exprNumber, exprString:
				end++
			default:
				return end, KnownValueUnknown, false, false
			}
		}
		field := tokens[fieldStart:end]
		value, valid, proven := constantExpressionType(field)
		if !proven {
			value, valid, proven = BuiltinExpressionType(field, dialect)
		}
		if proven && !valid {
			invalidIndex = true
		}
		if !proven {
			if len(field) > 1 && ProvenExpressionShape(field, dialect, false) != nil {
				invalidIndex = true
			}
			value = KnownExpressionTypeForDialect(field, dialect)
		}
		indexes = append(indexes, value)
	}
	value := directBuiltinValueType(tokens[start:end], dialect)
	if value == KnownValueUnknown {
		return end, value, false, false
	}
	name := string(tokens[start].Text)
	info, _ := BuiltinVariableInfoByName(name, dialect)
	if invalidIndex || !indexedBuiltinArityAllowed(info, len(indexes)) {
		return end, value, true, true
	}
	for i, kind := range indexes {
		if kind == KnownValueString && !builtinStringIndexAllowed(name, info, i, len(indexes), dialect) {
			return end, value, true, true
		}
	}
	return end, value, true, false
}
