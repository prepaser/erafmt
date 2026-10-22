package erabasic

type KnownValueType uint8

const (
	KnownValueUnknown KnownValueType = iota
	KnownValueNumeric
	KnownValueString
)

func variableArgumentShape(tokens []ExprToken) bool {
	if VariableArgumentTokens(tokens) {
		return true
	}
	if !setterMacroPossible(tokens) || variableShapeInvalid(tokens) {
		return false
	}
	tokens = stripExpressionParens(tokens)
	if len(tokens) > 0 && string(tokens[0].Text) == "+" {
		tokens = stripExpressionParens(tokens[1:])
		if knownVariableType(tokens) == KnownValueString {
			return false
		}
	}
	if len(tokens) < 2 || tokens[0].Kind != ExprIdentifier {
		return false
	}
	next := 1
	if len(tokens) > 3 && string(tokens[1].Text) == "@" && tokens[2].Kind == ExprIdentifier {
		next = 3
	}
	return string(tokens[next].Text) == ":"
}

func VariableArgumentTokens(tokens []ExprToken) bool {
	if len(tokens) == 0 {
		return false
	}
	tokens = stripExpressionParens(tokens)
	unaryPlus := false
	if len(tokens) > 0 && tokens[0].Kind == ExprOperator && string(tokens[0].Text) == "+" {
		unaryPlus = true
		tokens = stripExpressionParens(tokens[1:])
	}
	if len(tokens) == 0 || tokens[0].Kind != ExprIdentifier {
		return false
	}
	if unaryPlus {
		if knownVariableType(tokens) == KnownValueString {
			return false
		}
	}
	next := 1
	if next < len(tokens) && string(tokens[next].Text) == "@" {
		if next+1 >= len(tokens) || tokens[next+1].Kind != ExprIdentifier {
			return false
		}
		next += 2
	}
	if next == len(tokens) {
		return true
	}
	if string(tokens[next].Text) == "[" {
		return false
	}
	if string(tokens[next].Text) != ":" {
		return false
	}
	return validVariableIndexes(tokens[next+1:])
}

func stripExpressionParens(tokens []ExprToken) []ExprToken {
	if len(tokens) < 2 || tokens[0].Kind != exprSymbol || string(tokens[0].Text) != "(" {
		return tokens
	}
	outer := 0
	for outer < len(tokens) && tokens[outer].Kind == exprSymbol && string(tokens[outer].Text) == "(" {
		outer++
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
			if depth < outer && i != len(tokens)-1-depth {
				outer = depth
				if outer <= 0 {
					return tokens
				}
			}
		}
	}
	if depth != 0 {
		return tokens
	}
	return tokens[outer : len(tokens)-outer]
}

func validVariableIndexes(tokens []ExprToken) bool {
	start, depth := 0, 0
	for i, token := range tokens {
		switch string(token.Text) {
		case "(":
			depth++
		case ")":
			if depth > 0 {
				depth--
			}
		case ":":
			if depth == 0 {
				if !validVariableIndex(tokens[start:i]) {
					return false
				}
				start = i + 1
			}
		}
	}
	return validVariableIndex(tokens[start:])
}

func validVariableIndex(tokens []ExprToken) bool {
	if len(tokens) == 0 {
		return false
	}
	if len(tokens) == 1 {
		switch tokens[0].Kind {
		case ExprIdentifier, exprNumber, exprString, ExprForm:
			return true
		}
		return false
	}
	if string(tokens[0].Text) == "(" && setterMatchingParen(tokens) == len(tokens)-1 {
		return true
	}
	return tokens[0].Kind == ExprIdentifier && string(tokens[1].Text) == "(" && setterMatchingParen(tokens[1:]) == len(tokens)-2
}

func KnownExpressionType(tokens []ExprToken) KnownValueType {
	tokens = stripExpressionParens(tokens)
	if len(tokens) == 0 {
		return KnownValueUnknown
	}
	if value, valid, known := constantExpressionType(tokens); known {
		if valid {
			return value
		}
		return KnownValueUnknown
	}
	if VariableArgumentTokens(tokens) {
		if name, exact := VariableSystemName(tokens); exact {
			switch name {
			case "RESULT", "CFLAG":
				return KnownValueNumeric
			case "RESULTS", "CSTR":
				return KnownValueString
			}
		}
	}
	if info, known := builtinVariableInfo(tokens); known {
		return info.Value
	}
	if len(tokens) == 1 {
		switch tokens[0].Kind {
		case exprNumber:
			return KnownValueNumeric
		case exprString, ExprForm:
			return KnownValueString
		}
		return KnownValueUnknown
	}
	if len(tokens) == 2 && tokens[0].Kind == ExprOperator && (string(tokens[0].Text) == "+" || string(tokens[0].Text) == "-") {
		return KnownExpressionType(tokens[1:])
	}
	if allKnownNumericExpression(tokens) {
		return KnownValueNumeric
	}
	return knownCompoundVariableTypeForDialect(tokens, CommonDialect)
}

func KnownExpressionTypeForDialect(tokens []ExprToken, dialect Dialect) KnownValueType {
	value := KnownExpressionType(tokens)
	if value != KnownValueUnknown {
		return value
	}
	trimmed := stripExpressionParens(tokens)
	if value := directBuiltinValueType(trimmed, dialect); value != KnownValueUnknown {
		return value
	}
	if info, known := BuiltinVariableInfoForDialect(trimmed, dialect); known {
		return info.Value
	}
	if value, valid, proven := indexedBuiltinType(trimmed, dialect); proven && valid {
		return value
	}
	if value, valid, proven := BuiltinExpressionType(trimmed, dialect); proven && valid {
		return value
	}
	return knownCompoundVariableTypeForDialect(tokens, dialect)
}

func BuiltinScalarIndexesMissing(tokens []ExprToken, dialect Dialect) bool {
	if len(tokens) > 256 || containsExpressionMarker(tokens) || containsExpressionForm(tokens) || CommandMacroPossibleForDialect(tokens, dialect) {
		return false
	}
	for i, token := range tokens {
		if token.Kind != ExprIdentifier || i > 0 && string(tokens[i-1].Text) == "@" {
			continue
		}
		info, known := BuiltinVariableInfoByName(string(token.Text), dialect)
		if !known || info.Rank < 2 {
			continue
		}
		if i+1 < len(tokens) && (string(tokens[i+1].Text) == ":" || string(tokens[i+1].Text) == "@") {
			continue
		}
		return true
	}
	return false
}

func indexedBuiltinType(tokens []ExprToken, dialect Dialect) (KnownValueType, bool, bool) {
	if len(tokens) < 3 || tokens[0].Kind != ExprIdentifier || string(tokens[1].Text) != ":" {
		return KnownValueUnknown, false, false
	}
	info, known := BuiltinVariableInfoForDialect(tokens[:1], dialect)
	if !known {
		return KnownValueUnknown, false, false
	}
	indexes := 0
	indexKinds := make([]exprTokenKind, 0, 3)
	for pos := 2; pos < len(tokens); {
		if tokens[pos].Kind != exprNumber && tokens[pos].Kind != exprString {
			return KnownValueUnknown, false, false
		}
		indexes++
		indexKinds = append(indexKinds, tokens[pos].Kind)
		pos++
		if pos == len(tokens) {
			break
		}
		if string(tokens[pos].Text) != ":" {
			return KnownValueUnknown, false, false
		}
		pos++
		if pos == len(tokens) {
			return KnownValueUnknown, false, true
		}
	}
	if !indexedBuiltinArityAllowed(info, indexes) {
		return KnownValueUnknown, false, true
	}
	for index, kind := range indexKinds {
		if kind == exprString && !builtinStringIndexAllowed(string(tokens[0].Text), info, index, indexes, dialect) {
			return KnownValueUnknown, false, true
		}
	}
	return info.Value, true, true
}

func indexedBuiltinArityAllowed(info BuiltinVariable, indexes int) bool {
	if info.Character {
		switch info.Rank {
		case 0:
			return indexes <= 1
		case 1:
			return indexes <= 2
		case 2:
			return indexes == 0 || indexes == 3
		default:
			return false
		}
	}
	if info.Rank == 0 {
		return indexes == 0
	}
	return indexes == 0 || indexes == int(info.Rank)
}

func knownCompoundVariableTypeForDialect(tokens []ExprToken, dialect Dialect) KnownValueType {
	var typed []ExprToken
	for n, token := range tokens {
		if token.Kind != ExprIdentifier {
			continue
		}
		info, ok := BuiltinVariableInfoForDialect([]ExprToken{token}, dialect)
		if !ok {
			return KnownValueUnknown
		}
		if typed == nil {
			typed = append([]ExprToken(nil), tokens...)
		}
		if info.Value == KnownValueString {
			typed[n].Kind = exprString
		} else {
			typed[n].Kind = exprNumber
		}
	}
	if typed == nil {
		return KnownValueUnknown
	}
	if value, valid, known := constantExpressionType(typed); known && valid {
		return value
	}
	return KnownValueUnknown
}

func VariableSystemName(tokens []ExprToken) (string, bool) {
	tokens = stripExpressionParens(tokens)
	if len(tokens) > 0 && tokens[0].Kind == ExprOperator && string(tokens[0].Text) == "+" {
		tokens = stripExpressionParens(tokens[1:])
	}
	if len(tokens) == 0 || tokens[0].Kind != ExprIdentifier {
		return "", false
	}
	next := 1
	if next < len(tokens) && string(tokens[next].Text) == "@" {
		return "", false
	}
	if next < len(tokens) && string(tokens[next].Text) != ":" {
		return "", false
	}
	return string(tokens[0].Text), true
}

func allKnownNumericExpression(tokens []ExprToken) bool {
	hasValue := false
	for _, token := range tokens {
		switch token.Kind {
		case exprNumber:
			hasValue = true
		case ExprIdentifier:
			if string(token.Text) != "RESULT" && string(token.Text) != "CFLAG" {
				return false
			}
			hasValue = true
		case ExprOperator:
		case exprSymbol:
			if string(token.Text) != "(" && string(token.Text) != ")" {
				return false
			}
		default:
			return false
		}
	}
	return hasValue
}

func variableShapeInvalid(tokens []ExprToken) bool {
	if len(tokens) == 0 {
		return false
	}
	for _, token := range tokens {
		if token.Kind == ExprForm {
			return false
		}
	}
	for _, token := range tokens {
		if string(token.Text) == "[" || string(token.Text) == "]" {
			return true
		}
	}
	return false
}

func directBuiltinValueType(tokens []ExprToken, dialect Dialect) KnownValueType {
	if len(tokens) > 4096 || CommandMacroPossibleForDialect(tokens, dialect) || !VariableArgumentTokens(tokens) {
		return KnownValueUnknown
	}
	for _, token := range tokens {
		if token.Kind == ExprMarker || token.Kind == ExprForm || string(token.Text) == "@" {
			return KnownValueUnknown
		}
	}
	name, exact := VariableSystemName(tokens)
	if !exact {
		return KnownValueUnknown
	}
	info, known := BuiltinVariableInfoByName(name, dialect)
	if !known {
		return KnownValueUnknown
	}
	return info.Value
}
