package format

type knownValueType uint8

const (
	knownValueUnknown knownValueType = iota
	knownValueNumeric
	knownValueString
)

func variableArgumentShape(tokens []exprToken) bool {
	if variableArgumentTokens(tokens) {
		return true
	}
	if !setterMacroPossible(tokens) || variableShapeInvalid(tokens) {
		return false
	}
	tokens = stripExpressionParens(tokens)
	if len(tokens) > 0 && string(tokens[0].text) == "+" {
		tokens = stripExpressionParens(tokens[1:])
		if knownVariableType(tokens) == knownValueString {
			return false
		}
	}
	if len(tokens) < 2 || tokens[0].kind != exprIdentifier {
		return false
	}
	next := 1
	if len(tokens) > 3 && string(tokens[1].text) == "@" && tokens[2].kind == exprIdentifier {
		next = 3
	}
	return string(tokens[next].text) == ":"
}

func variableArgumentTokens(tokens []exprToken) bool {
	if len(tokens) == 0 {
		return false
	}
	tokens = stripExpressionParens(tokens)
	unaryPlus := false
	if len(tokens) > 0 && tokens[0].kind == exprOperator && string(tokens[0].text) == "+" {
		unaryPlus = true
		tokens = stripExpressionParens(tokens[1:])
	}
	if len(tokens) == 0 || tokens[0].kind != exprIdentifier {
		return false
	}
	if unaryPlus {
		if knownVariableType(tokens) == knownValueString {
			return false
		}
	}
	next := 1
	if next < len(tokens) && string(tokens[next].text) == "@" {
		if next+1 >= len(tokens) || tokens[next+1].kind != exprIdentifier {
			return false
		}
		next += 2
	}
	if next == len(tokens) {
		return true
	}
	if string(tokens[next].text) == "[" {
		return false
	}
	if string(tokens[next].text) != ":" {
		return false
	}
	return validVariableIndexes(tokens[next+1:])
}

func stripExpressionParens(tokens []exprToken) []exprToken {
	if len(tokens) < 2 || tokens[0].kind != exprSymbol || string(tokens[0].text) != "(" {
		return tokens
	}
	outer := 0
	for outer < len(tokens) && tokens[outer].kind == exprSymbol && string(tokens[outer].text) == "(" {
		outer++
	}
	depth := 0
	for i, token := range tokens {
		if token.kind != exprSymbol {
			continue
		}
		switch string(token.text) {
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

func validVariableIndexes(tokens []exprToken) bool {
	start, depth := 0, 0
	for i, token := range tokens {
		switch string(token.text) {
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

func validVariableIndex(tokens []exprToken) bool {
	if len(tokens) == 0 {
		return false
	}
	if len(tokens) == 1 {
		switch tokens[0].kind {
		case exprIdentifier, exprNumber, exprString, exprForm:
			return true
		}
		return false
	}
	if string(tokens[0].text) == "(" && setterMatchingParen(tokens) == len(tokens)-1 {
		return true
	}
	return tokens[0].kind == exprIdentifier && string(tokens[1].text) == "(" && setterMatchingParen(tokens[1:]) == len(tokens)-2
}

func knownExpressionType(tokens []exprToken) knownValueType {
	tokens = stripExpressionParens(tokens)
	if len(tokens) == 0 {
		return knownValueUnknown
	}
	if value, valid, known := constantExpressionType(tokens); known {
		if valid {
			return value
		}
		return knownValueUnknown
	}
	if variableArgumentTokens(tokens) {
		if name, exact := variableSystemName(tokens); exact {
			switch name {
			case "RESULT", "CFLAG":
				return knownValueNumeric
			case "RESULTS", "CSTR":
				return knownValueString
			}
		}
	}
	if info, known := builtinVariableInfo(tokens); known {
		return info.value
	}
	if len(tokens) == 1 {
		switch tokens[0].kind {
		case exprNumber:
			return knownValueNumeric
		case exprString, exprForm:
			return knownValueString
		}
		return knownValueUnknown
	}
	if len(tokens) == 2 && tokens[0].kind == exprOperator && (string(tokens[0].text) == "+" || string(tokens[0].text) == "-") {
		return knownExpressionType(tokens[1:])
	}
	if allKnownNumericExpression(tokens) {
		return knownValueNumeric
	}
	return knownCompoundVariableTypeForDialect(tokens, CommonDialect)
}

func knownExpressionTypeForDialect(tokens []exprToken, dialect Dialect) knownValueType {
	value := knownExpressionType(tokens)
	if value != knownValueUnknown {
		return value
	}
	trimmed := stripExpressionParens(tokens)
	if value := directBuiltinValueType(trimmed, dialect); value != knownValueUnknown {
		return value
	}
	if info, known := builtinVariableInfoForDialect(trimmed, dialect); known {
		return info.value
	}
	if value, valid, proven := indexedBuiltinType(trimmed, dialect); proven && valid {
		return value
	}
	if value, valid, proven := builtinExpressionType(trimmed, dialect); proven && valid {
		return value
	}
	return knownCompoundVariableTypeForDialect(tokens, dialect)
}

func builtinScalarIndexesMissing(tokens []exprToken, dialect Dialect) bool {
	if len(tokens) > 256 || containsExpressionMarker(tokens) || containsExpressionForm(tokens) || commandMacroPossibleForDialect(tokens, dialect) {
		return false
	}
	for i, token := range tokens {
		if token.kind != exprIdentifier || i > 0 && string(tokens[i-1].text) == "@" {
			continue
		}
		info, known := builtinVariableInfoByName(string(token.text), dialect)
		if !known || info.rank < 2 {
			continue
		}
		if i+1 < len(tokens) && (string(tokens[i+1].text) == ":" || string(tokens[i+1].text) == "@") {
			continue
		}
		return true
	}
	return false
}

func indexedBuiltinType(tokens []exprToken, dialect Dialect) (knownValueType, bool, bool) {
	if len(tokens) < 3 || tokens[0].kind != exprIdentifier || string(tokens[1].text) != ":" {
		return knownValueUnknown, false, false
	}
	info, known := builtinVariableInfoForDialect(tokens[:1], dialect)
	if !known {
		return knownValueUnknown, false, false
	}
	indexes := 0
	indexKinds := make([]exprTokenKind, 0, 3)
	for pos := 2; pos < len(tokens); {
		if tokens[pos].kind != exprNumber && tokens[pos].kind != exprString {
			return knownValueUnknown, false, false
		}
		indexes++
		indexKinds = append(indexKinds, tokens[pos].kind)
		pos++
		if pos == len(tokens) {
			break
		}
		if string(tokens[pos].text) != ":" {
			return knownValueUnknown, false, false
		}
		pos++
		if pos == len(tokens) {
			return knownValueUnknown, false, true
		}
	}
	if !indexedBuiltinArityAllowed(info, indexes) {
		return knownValueUnknown, false, true
	}
	for index, kind := range indexKinds {
		if kind == exprString && !builtinStringIndexAllowed(string(tokens[0].text), info, index, indexes, dialect) {
			return knownValueUnknown, false, true
		}
	}
	return info.value, true, true
}

func indexedBuiltinArityAllowed(info builtinVariable, indexes int) bool {
	if info.character {
		switch info.rank {
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
	if info.rank == 0 {
		return indexes == 0
	}
	return indexes == 0 || indexes == int(info.rank)
}

func knownCompoundVariableTypeForDialect(tokens []exprToken, dialect Dialect) knownValueType {
	var typed []exprToken
	for n, token := range tokens {
		if token.kind != exprIdentifier {
			continue
		}
		info, ok := builtinVariableInfoForDialect([]exprToken{token}, dialect)
		if !ok {
			return knownValueUnknown
		}
		if typed == nil {
			typed = append([]exprToken(nil), tokens...)
		}
		if info.value == knownValueString {
			typed[n].kind = exprString
		} else {
			typed[n].kind = exprNumber
		}
	}
	if typed == nil {
		return knownValueUnknown
	}
	if value, valid, known := constantExpressionType(typed); known && valid {
		return value
	}
	return knownValueUnknown
}

func variableSystemName(tokens []exprToken) (string, bool) {
	tokens = stripExpressionParens(tokens)
	if len(tokens) > 0 && tokens[0].kind == exprOperator && string(tokens[0].text) == "+" {
		tokens = stripExpressionParens(tokens[1:])
	}
	if len(tokens) == 0 || tokens[0].kind != exprIdentifier {
		return "", false
	}
	next := 1
	if next < len(tokens) && string(tokens[next].text) == "@" {
		return "", false
	}
	if next < len(tokens) && string(tokens[next].text) != ":" {
		return "", false
	}
	return string(tokens[0].text), true
}

func allKnownNumericExpression(tokens []exprToken) bool {
	hasValue := false
	for _, token := range tokens {
		switch token.kind {
		case exprNumber:
			hasValue = true
		case exprIdentifier:
			if string(token.text) != "RESULT" && string(token.text) != "CFLAG" {
				return false
			}
			hasValue = true
		case exprOperator:
		case exprSymbol:
			if string(token.text) != "(" && string(token.text) != ")" {
				return false
			}
		default:
			return false
		}
	}
	return hasValue
}

func variableShapeInvalid(tokens []exprToken) bool {
	if len(tokens) == 0 {
		return false
	}
	for _, token := range tokens {
		if token.kind == exprForm {
			return false
		}
	}
	for _, token := range tokens {
		if string(token.text) == "[" || string(token.text) == "]" {
			return true
		}
	}
	return false
}

func directBuiltinValueType(tokens []exprToken, dialect Dialect) knownValueType {
	if len(tokens) > 4096 || commandMacroPossibleForDialect(tokens, dialect) || !variableArgumentTokens(tokens) {
		return knownValueUnknown
	}
	for _, token := range tokens {
		if token.kind == exprMarker || token.kind == exprForm || string(token.text) == "@" {
			return knownValueUnknown
		}
	}
	name, exact := variableSystemName(tokens)
	if !exact {
		return knownValueUnknown
	}
	info, known := builtinVariableInfoByName(name, dialect)
	if !known {
		return knownValueUnknown
	}
	return info.value
}
