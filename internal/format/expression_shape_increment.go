package format

func provenIncrementOperatorState(tokens []exprToken, dialect Dialect) error {
	if !incrementOperatorPresent(tokens) {
		return nil
	}
	return walkIncrementExpression(tokens, dialect)
}

func incrementOperatorPresent(tokens []exprToken) bool {
	for _, token := range tokens {
		if isIncrementOperator(token) {
			return true
		}
	}
	return false
}

func walkIncrementExpression(tokens []exprToken, dialect Dialect) error {
	return walkIncrementExpressionDepth(tokens, dialect, 0)
}

func walkIncrementExpressionDepth(tokens []exprToken, dialect Dialect, depth int) error {
	if depth > maxBuiltinProofDepth {
		return nil
	}
	wantOperand, ternary := true, 0
	var unary string
	var operandUnary string
	operandStart := -1
	prefixIncrement, postfixIncrement := false, false
	trailingComma := false
	for i := 0; i < len(tokens); {
		token := tokens[i]
		if wantOperand {
			if token.kind == exprOperator {
				op := string(token.text)
				if isIncrementOperator(token) {
					if unary != "" || i+1 >= len(tokens) || !expressionOperandStart(tokens[i+1]) {
						return expressionShapeError(token, "expression has an invalid increment")
					}
					end := expressionOperandEndIndex(tokens, i+1, dialect)
					if end < 0 {
						return expressionShapeError(token, "expression has an invalid increment")
					}
					if invalid, proven := provenIncrementOperandSpan(tokens, i, true, i+1, end, dialect); proven && invalid {
						return expressionShapeError(token, "expression has an invalid increment")
					}
					if err := walkIncrementGroups(tokens, i+1, end, dialect, depth); err != nil {
						return err
					}
					operandStart = i + 1
					i = end
					wantOperand = false
					prefixIncrement, postfixIncrement = true, false
					operandUnary = ""
					unary = ""
					continue
				}
				if op != "+" && op != "-" && op != "!" && op != "~" {
					return expressionShapeError(token, "expression has an invalid operator")
				}
				if unary != "" {
					return expressionShapeError(token, "expression has an invalid operator")
				}
				unary = op
				i++
				continue
			}
			if token.kind == exprSymbol && string(token.text) == "," {
				trailingComma = true
				i++
				continue
			}
			if !expressionOperandStart(token) {
				return expressionShapeError(token, "expression has an invalid operator")
			}
			end := expressionOperandEndIndex(tokens, i, dialect)
			if end <= i {
				return expressionShapeError(token, "expression has an invalid operator")
			}
			if err := walkIncrementGroups(tokens, i, end, dialect, depth); err != nil {
				return err
			}
			operandStart = i
			i = end
			wantOperand = false
			prefixIncrement = false
			postfixIncrement = false
			operandUnary = unary
			unary = ""
			trailingComma = false
			continue
		}

		if token.kind == exprOperator {
			if isIncrementOperator(token) {
				if operandStart < 0 || prefixIncrement || postfixIncrement || operandUnary == "!" {
					return expressionShapeError(token, "expression has an invalid increment")
				}
				if invalid, proven := provenPostfixIncrementOperand(tokens, i, dialect); proven && invalid {
					return expressionShapeError(token, "expression has an invalid increment")
				}
				i++
				postfixIncrement = true
				continue
			}
			if op := string(token.text); op == "!" || op == "~" || op == "=" || constantPriority(op) == 0 {
				return expressionShapeError(token, "expression has an invalid operator")
			}
			switch string(token.text) {
			case "?":
				ternary++
			case "#":
				if ternary == 0 {
					return expressionShapeError(token, "expression has an invalid operator")
				}
				ternary--
			}
			wantOperand = true
			prefixIncrement, postfixIncrement = false, false
			operandUnary = ""
			unary = ""
			trailingComma = false
			i++
			continue
		}
		if token.kind == exprSymbol && (string(token.text) == ")" || string(token.text) == "]") {
			break
		}
		if token.kind == exprSymbol && string(token.text) == "," {
			wantOperand = true
			prefixIncrement, postfixIncrement = false, false
			operandUnary = ""
			unary = ""
			trailingComma = true
			i++
			continue
		}
		return expressionShapeError(token, "adjacent operands in expression")
	}
	if wantOperand && !trailingComma && len(tokens) > 0 {
		return expressionShapeError(tokens[len(tokens)-1], "expression has an invalid operator")
	}
	if ternary != 0 && len(tokens) > 0 {
		return expressionShapeError(tokens[len(tokens)-1], "expression has an invalid operator")
	}
	return nil
}

func walkIncrementGroups(tokens []exprToken, start, end int, dialect Dialect, depth int) error {
	for i := start; i < end; i++ {
		if string(tokens[i].text) != "(" {
			continue
		}
		close, ok := boundedExpressionMatchingParen(tokens, i)
		if !ok || close >= end {
			continue
		}
		if i > start && tokens[i-1].kind == exprIdentifier {
			if _, known := builtinVariableInfoByName(string(tokens[i-1].text), dialect); known && !builtinMethodCall(tokens, i-1, dialect) {
				i = close
				continue
			}
		}
		inner := tokens[i+1 : close]
		if expressionShapeIdentifiersKnown(inner, dialect) {
			if err := walkIncrementExpressionDepth(inner, dialect, depth+1); err != nil {
				return err
			}
		}
		i = close
	}
	return nil
}

func expressionOperandEndIndex(tokens []exprToken, start int, dialect Dialect) int {
	if start >= len(tokens) {
		return -1
	}
	if string(tokens[start].text) == "(" {
		close, ok := boundedExpressionMatchingParen(tokens, start)
		if !ok {
			return -1
		}
		return close + 1
	}
	if builtinMethodCall(tokens, start, dialect) {
		close, ok := boundedExpressionMatchingParen(tokens, start+1)
		if !ok {
			return -1
		}
		return close + 1
	}
	if start+1 < len(tokens) && string(tokens[start+1].text) == "(" {
		if close, ok := boundedExpressionMatchingParen(tokens, start+1); ok {
			return close + 1
		}
	}
	if end, _, _, known := knownBuiltinAtomEnd(tokens, start, dialect); known {
		return end
	}
	return start + 1
}

func expressionOperandStart(token exprToken) bool {
	return expressionWordLike(token) || token.kind == exprSymbol && (string(token.text) == "(" || string(token.text) == "[")
}

func incrementOperandStart(tokens []exprToken, postfix int) int {
	start := postfix - 1
	if start < 0 {
		return start
	}
	for {
		if string(tokens[start].text) == ")" {
			open, ok := matchingExpressionOpen(tokens, start)
			if !ok {
				return start
			}
			start = open
			continue
		}
		if start >= 2 && string(tokens[start-1].text) == ":" {
			start -= 2
			continue
		}
		if start >= 2 && string(tokens[start-1].text) == "@" && tokens[start-2].kind == exprIdentifier {
			start -= 2
			continue
		}
		return start
	}
}

func provenPostfixIncrementOperand(tokens []exprToken, index int, dialect Dialect) (bool, bool) {
	start := incrementOperandStart(tokens, index)
	if start < 0 {
		return false, false
	}
	return provenIncrementOperandSpan(tokens, index, false, start, index+1, dialect)
}

func provenIncrementOperandSpan(tokens []exprToken, index int, prefix bool, start, end int, dialect Dialect) (bool, bool) {
	var operand []exprToken
	if start < 0 || end <= start || end > len(tokens) {
		return false, false
	}
	if prefix {
		operand = tokens[index:end]
	} else {
		operand = tokens[start:end]
	}
	if prefix {
		value := operand[1:]
		if len(value) > 0 && string(value[0].text) == "(" && unresolvedBuiltinIndexOperand(value, dialect) {
			return false, false
		}
		if groupedIncrementOperand(value, dialect) {
			return true, true
		}
		return invalidProvenIncrement(operand, value, dialect)
	}
	value := operand[:len(operand)-1]
	if groupedIncrementOperand(value, dialect) {
		return true, true
	}
	return invalidProvenIncrement(operand, value, dialect)
}

func groupedIncrementOperand(tokens []exprToken, dialect Dialect) bool {
	inner := stripExpressionParens(tokens)
	if len(inner) == len(tokens) || !incrementOperatorPresent(inner) {
		return false
	}
	if end, _, _, known := knownBuiltinAtomEnd(inner, 0, dialect); known && end == len(inner) {
		return false
	}
	return true
}

func isIncrementOperator(token exprToken) bool {
	return string(token.text) == "++" || string(token.text) == "--"
}
