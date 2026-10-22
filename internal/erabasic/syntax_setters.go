package erabasic

func setterHasAdjacentQuotedOperands(tokens []ExprToken) bool {
	for i := 1; i < len(tokens); i++ {
		if !ExpressionWordLike(tokens[i-1]) || !ExpressionWordLike(tokens[i]) {
			continue
		}
		if tokens[i-1].Kind == exprString || tokens[i].Kind == exprString {
			return true
		}
	}
	return false
}

func SetterCheckedFields(command string) int {
	switch command {
	case "CVARSET":
		return 5
	case "POWER":
		return 3
	default:
		return 4
	}
}

func setterMacroPossible(tokens []ExprToken) bool {
	return SetterMacroPossibleForDialect(tokens, CommonDialect)
}

func SetterMacroPossibleForDialect(tokens []ExprToken, dialect Dialect) bool {
	for n, token := range tokens {
		if token.Kind == ExprForm {
			return true
		}
		if token.Kind != ExprIdentifier {
			continue
		}
		if !setterKnownNameForDialect(token.Text, dialect) || n+1 < len(tokens) && string(tokens[n+1].Text) == "(" {
			return true
		}
	}
	return false
}

func setterLiteralDestination(tokens []ExprToken) bool {
	tokens = bitLiteralTokens(tokens)
	if setterNumericLiteralExpression(tokens) {
		return true
	}
	if len(tokens) == 2 && tokens[0].Kind == ExprOperator && (string(tokens[0].Text) == "+" || string(tokens[0].Text) == "-") {
		return tokens[1].Kind == exprNumber
	}
	if len(tokens) != 1 {
		return false
	}
	switch tokens[0].Kind {
	case exprNumber, exprString, ExprForm:
		return true
	default:
		return false
	}
}

func setterNumericLiteralExpression(tokens []ExprToken) bool {
	hasNumber := false
	for _, token := range tokens {
		switch token.Kind {
		case exprNumber:
			hasNumber = true
		case ExprOperator:
		case exprSymbol:
			if string(token.Text) != "(" && string(token.Text) != ")" {
				return false
			}
		default:
			return false
		}
	}
	return hasNumber
}

func setterInvalidDestination(tokens []ExprToken, dialect Dialect) bool {
	tokens = bitLiteralTokens(tokens)
	if len(tokens) == 0 {
		return false
	}
	if variableShapeInvalid(tokens) {
		return true
	}
	if BuiltinVariableReadOnly(tokens, dialect) {
		return true
	}
	if !SetterMacroPossibleForDialect(tokens, dialect) {
		if setterTernaryDestination(tokens) || setterVariableSyntaxInvalid(tokens) {
			return true
		}
	}
	if tokens[0].Kind == ExprOperator && string(tokens[0].Text) == "+" {
		if knownVariableType(tokens[1:]) == KnownValueString {
			return true
		}
	}
	if tokens[0].Kind == ExprOperator {
		switch string(tokens[0].Text) {
		case "+":
			return setterInvalidDestination(tokens[1:], dialect)
		case "-":
			return setterKnownDestinationPrefix(tokens[1:]) || setterBuiltinCall(tokens[1:], dialect)
		}
	}
	if setterBuiltinCall(tokens, dialect) {
		return true
	}
	if len(tokens) > 1 && tokens[0].Kind == ExprIdentifier && setterKnownNameForDialect(tokens[0].Text, dialect) && tokens[1].Kind == ExprOperator {
		return true
	}
	if tokens[0].Text[0] == '(' {
		close := setterMatchingParen(tokens)
		if close > 0 && close+1 < len(tokens) && setterKnownVariable(bitLiteralTokens(tokens[1:close])) && tokens[close+1].Kind == ExprOperator {
			return true
		}
	}
	return false
}

func setterVariableSyntaxInvalid(tokens []ExprToken) bool {
	tokens = stripExpressionParens(tokens)
	if len(tokens) > 0 && tokens[0].Kind == ExprOperator && string(tokens[0].Text) == "+" {
		tokens = stripExpressionParens(tokens[1:])
	}
	if len(tokens) == 0 || tokens[0].Kind != ExprIdentifier {
		return false
	}
	next := 1
	if next < len(tokens) && string(tokens[next].Text) == "@" {
		if next+1 >= len(tokens) || tokens[next+1].Kind != ExprIdentifier {
			return false
		}
		next += 2
	}
	if next >= len(tokens) || (string(tokens[next].Text) != ":" && string(tokens[next].Text) != "[") {
		return false
	}
	return !VariableArgumentTokens(tokens)
}

func setterTernaryDestination(tokens []ExprToken) bool {
	depth := 0
	for _, token := range tokens {
		switch string(token.Text) {
		case "(":
			depth++
		case ")":
			if depth > 0 {
				depth--
			}
		case "?", "#":
			if depth == 0 {
				return true
			}
		}
	}
	return false
}

func setterKnownVariable(tokens []ExprToken) bool {
	return len(tokens) == 1 && tokens[0].Kind == ExprIdentifier && setterKnownName(tokens[0].Text)
}

func setterKnownDestinationPrefix(tokens []ExprToken) bool {
	if len(tokens) == 0 {
		return false
	}
	if tokens[0].Kind == ExprIdentifier && setterKnownName(tokens[0].Text) {
		return true
	}
	if tokens[0].Kind != exprSymbol || string(tokens[0].Text) != "(" {
		return false
	}
	close := setterMatchingParen(tokens)
	return close > 0 && setterKnownDestinationPrefix(bitLiteralTokens(tokens[1:close]))
}

func setterKnownName(name []byte) bool {
	return setterKnownNameForDialect(name, CommonDialect)
}

func setterKnownNameForDialect(name []byte, dialect Dialect) bool {
	_, known := BuiltinVariableInfoByName(string(name), dialect)
	return known
}

func setterBuiltinCall(tokens []ExprToken, dialect Dialect) bool {
	if len(tokens) < 2 || tokens[0].Kind != ExprIdentifier || string(tokens[1].Text) != "(" {
		return false
	}
	mask, ok := builtinMethodMasks[string(tokens[0].Text)]
	return ok && methodDialect(mask, dialect)
}

func setterMatchingParen(tokens []ExprToken) int {
	if len(tokens) == 0 || tokens[0].Kind != exprSymbol || string(tokens[0].Text) != "(" {
		return -1
	}
	depth := 0
	for i, token := range tokens {
		if token.Kind != exprSymbol {
			continue
		}
		switch string(token.Text) {
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
