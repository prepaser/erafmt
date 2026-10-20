package format

// provenExpressionShape checks only grammar errors that are decidable from
// exact builtin names. Unknown identifiers and macro-capable fields defer.
func provenExpressionShape(tokens []exprToken, dialect Dialect, caseMode bool) error {
	if len(tokens) == 0 || containsExpressionMarker(tokens) {
		return nil
	}
	if err := provenClosedOperandOperators(tokens, dialect); err != nil {
		return err
	}
	if err := provenQualifiedBuiltinReference(tokens, dialect); err != nil {
		return err
	}
	if err := provenClosedOperandAdjacency(tokens, dialect); err != nil {
		return err
	}
	if caseMode {
		return provenCaseShape(tokens, dialect)
	}
	if !expressionShapeIdentifiersKnown(tokens, dialect) {
		return nil
	}
	if err := provenKnownExpressionBoundaries(tokens, dialect); err != nil {
		return err
	}
	if err := provenIncrementOperatorState(tokens, dialect); err != nil {
		return err
	}
	if err := reservedExpressionKeyword(tokens, caseMode); err != nil {
		return err
	}
	return provenOrdinaryExpressionShape(tokens, dialect)
}

func provenClosedOperandOperators(tokens []exprToken, dialect Dialect) error {
	for close := 0; close+1 < len(tokens); close++ {
		if !isClosingExpressionSymbol(tokens[close]) {
			continue
		}
		next := tokens[close+1]
		if next.kind != exprOperator && !isIncrementOperator(next) {
			continue
		}
		open, ok := matchingExpressionOpen(tokens, close)
		if !ok || open > 0 && string(tokens[open-1].text) == ":" {
			continue
		}
		if !provenClosedOperandAt(tokens, close, open, dialect) {
			continue
		}
		if next.kind == exprOperator && !isIncrementOperator(next) && string(next.text) != "!" && string(next.text) != "~" {
			if err := provenClosedOperatorChain(tokens, close); err != nil {
				return err
			}
		}
		if isIncrementOperator(next) {
			if closedOperandIsBuiltinMethod(tokens, open, close, dialect) {
				return expressionShapeError(next, "expression has an invalid increment")
			}
			invalid, proven := invalidProvenIncrement(tokens[open:close+2], tokens[open:close+1], dialect)
			if proven && invalid {
				return expressionShapeError(next, "expression has an invalid increment")
			}
			continue
		}
		if string(next.text) == "!" || string(next.text) == "~" || expressionOperatorAtEnd(tokens, close+1) {
			return expressionShapeError(next, "expression has an invalid operator")
		}
	}
	if err := provenPrefixClosedOperandIncrement(tokens, dialect); err != nil {
		return err
	}
	return nil
}

func provenPrefixClosedOperandIncrement(tokens []exprToken, dialect Dialect) error {
	for i, token := range tokens {
		if token.kind != exprOperator || !isIncrementOperator(token) || i+1 >= len(tokens) {
			continue
		}
		start, open := i+1, i+1
		method := false
		if tokens[start].kind == exprIdentifier && start+1 < len(tokens) && string(tokens[start+1].text) == "(" {
			open, method = start+1, true
		} else if tokens[start].kind != exprSymbol || string(tokens[start].text) != "(" {
			continue
		}
		if open > 0 && string(tokens[open-1].text) == ":" {
			continue
		}
		close, matched := boundedExpressionMatchingParen(tokens, open)
		if !matched || !provenClosedOperandAt(tokens, close, open, dialect) {
			continue
		}
		if method || closedOperandIsBuiltinMethod(tokens, open, close, dialect) {
			return expressionShapeError(token, "expression has an invalid increment")
		}
		if unresolvedBuiltinIndexOperand(tokens[i+1:close+1], dialect) {
			continue
		}
		invalid, proven := invalidProvenIncrement(tokens[i:close+1], tokens[i+1:close+1], dialect)
		if proven && invalid {
			return expressionShapeError(token, "expression has an invalid increment")
		}
	}
	return nil
}

func closedOperandIsBuiltinMethod(tokens []exprToken, open, close int, dialect Dialect) bool {
	if open < 0 || close < open || close >= len(tokens) {
		return false
	}
	if open > 0 && builtinMethodCall(tokens, open-1, dialect) {
		return true
	}
	operand := stripExpressionParens(tokens[open : close+1])
	return len(operand) > 1 && operand[0].kind == exprIdentifier && builtinMethodCall(operand, 0, dialect)
}

func provenClosedOperatorChain(tokens []exprToken, close int) error {
	wantOperand := false
	i := close + 1
	for ; i < len(tokens) && tokens[i].kind == exprOperator; i++ {
		token := tokens[i]
		op := string(token.text)
		if wantOperand {
			switch {
			case op == "+" || op == "-" || op == "!" || op == "~" || isIncrementOperator(token):
				continue
			default:
				return expressionShapeError(token, "expression has an invalid operator")
			}
		}
		switch {
		case isIncrementOperator(token):
			return expressionShapeError(token, "expression has an invalid increment")
		case op == "?":
			wantOperand = true
		case op == "#":
			wantOperand = true
		case op == "=" || constantPriority(op) == 0:
			return expressionShapeError(token, "expression has an invalid operator")
		default:
			wantOperand = true
		}
	}
	if wantOperand && (i == len(tokens) || expressionOperandBoundary(tokens[i])) {
		return expressionShapeError(tokens[i-1], "expression has an invalid operator")
	}
	return nil
}

func expressionOperandBoundary(token exprToken) bool {
	if token.kind != exprSymbol {
		return false
	}
	switch string(token.text) {
	case ")", "]", ",", ":":
		return true
	default:
		return false
	}
}

func provenKnownExpressionBoundaries(tokens []exprToken, dialect Dialect) error {
	if len(tokens) > maxBuiltinProofTokens || containsExpressionMarker(tokens) || containsExpressionForm(tokens) || !expressionShapeIdentifiersKnown(tokens, dialect) || expressionShapeDepthExceeds(tokens) || containsScopedQualifier(tokens) {
		return nil
	}
	for i, token := range tokens {
		if token.kind == exprOperator {
			if i+1 >= len(tokens) {
				if !isIncrementOperator(token) {
					return expressionShapeError(token, "expression has an invalid operator")
				}
				continue
			}
			next := tokens[i+1]
			if next.kind == exprSymbol && invalidExpressionOperandSymbol(string(next.text)) &&
				!(isIncrementOperator(token) && expressionPostfixBoundary(tokens, i, next)) {
				return expressionShapeError(next, "expression has an invalid operator")
			}
			continue
		}
		if token.kind != exprSymbol {
			continue
		}
		symbol := string(token.text)
		switch symbol {
		case "[":
			if expressionOperandBoundaryBefore(tokens, i) {
				return expressionShapeError(token, "invalid builtin variable expression")
			}
		case ".":
			return expressionShapeError(token, "invalid builtin variable expression")
		case ":":
			if i == 0 || i+1 >= len(tokens) || expressionOperandBoundaryBefore(tokens, i) || invalidExpressionIndexStart(tokens[i+1]) || !expressionColonOwnedByBuiltin(tokens, i, dialect) {
				return expressionShapeError(token, "invalid builtin variable expression")
			}
		}
	}
	return nil
}

func expressionShapeDepthExceeds(tokens []exprToken) bool {
	depth := 0
	for _, token := range tokens {
		if token.kind != exprSymbol {
			continue
		}
		switch string(token.text) {
		case "(", "[":
			depth++
			if depth > maxBuiltinProofDepth {
				return true
			}
		case ")", "]":
			if depth > 0 {
				depth--
			}
		}
	}
	return false
}

func containsScopedQualifier(tokens []exprToken) bool {
	for i := 1; i+1 < len(tokens); i++ {
		if tokens[i].kind == exprSymbol && string(tokens[i].text) == "@" && tokens[i-1].kind == exprIdentifier && tokens[i+1].kind == exprIdentifier {
			return true
		}
	}
	return false
}

func expressionColonOwnedByBuiltin(tokens []exprToken, index int, dialect Dialect) bool {
	for index > 0 {
		previous := index - 1
		switch tokens[previous].kind {
		case exprIdentifier:
			info, known := builtinVariableInfoByName(string(tokens[previous].text), dialect)
			return known && (info.rank > 0 || info.character)
		case exprNumber, exprString:
		case exprSymbol:
			if string(tokens[previous].text) != ")" {
				return false
			}
			open, matched := matchingExpressionOpen(tokens, previous)
			if !matched {
				return false
			}
			previous = open
		default:
			return false
		}
		if previous == 0 || string(tokens[previous-1].text) != ":" {
			return false
		}
		index = previous - 1
	}
	return false
}

func expressionPostfixBoundary(tokens []exprToken, index int, next exprToken) bool {
	if string(next.text) != ")" && string(next.text) != "]" && string(next.text) != "," {
		return false
	}
	return index > 0 && (expressionWordLike(tokens[index-1]) || isClosingExpressionSymbol(tokens[index-1]))
}

func expressionOperandBoundaryBefore(tokens []exprToken, index int) bool {
	if index == 0 {
		return true
	}
	previous := tokens[index-1]
	if previous.kind == exprOperator {
		return true
	}
	if previous.kind != exprSymbol {
		return false
	}
	switch string(previous.text) {
	case "(", "[", ",", ":":
		return true
	default:
		return false
	}
}

func invalidExpressionOperandSymbol(symbol string) bool {
	switch symbol {
	case ".", ":", "[", ")", "]", ",":
		return true
	default:
		return false
	}
}

func invalidExpressionIndexStart(token exprToken) bool {
	return token.kind == exprOperator || token.kind == exprSymbol && invalidExpressionOperandSymbol(string(token.text))
}

func invalidProvenIncrement(tokens, operand []exprToken, dialect Dialect) (invalid, proven bool) {
	_, valid, known := builtinIncrementType(tokens, dialect)
	if known {
		return !valid, true
	}
	if unresolvedBuiltinIndexOperand(operand, dialect) {
		return false, false
	}
	if _, _, known := constantExpressionType(operand); known {
		return true, true
	}
	_, _, known = builtinExpressionType(operand, dialect)
	return known, known
}

func unresolvedBuiltinIndexOperand(tokens []exprToken, dialect Dialect) bool {
	tokens = stripExpressionParens(tokens)
	if len(tokens) < 3 || tokens[0].kind != exprIdentifier || string(tokens[1].text) != ":" {
		return false
	}
	_, known := builtinVariableInfoByName(string(tokens[0].text), dialect)
	return known
}

func expressionOperatorAtEnd(tokens []exprToken, operator int) bool {
	if operator+1 >= len(tokens) {
		return true
	}
	next := tokens[operator+1]
	return next.kind == exprSymbol && (string(next.text) == "," || string(next.text) == ")" || string(next.text) == "]")
}

func provenQualifiedBuiltinReference(tokens []exprToken, dialect Dialect) error {
	for i := range tokens {
		_, qualified, _, known := knownBuiltinAtomEnd(tokens, i, dialect)
		if !known || !qualified {
			continue
		}
		return expressionShapeError(tokens[i+2], "non-local builtin variable reference")
	}
	return nil
}

func reservedExpressionKeyword(tokens []exprToken, caseMode bool) error {
	if caseMode {
		return nil
	}
	for i, token := range tokens {
		if token.kind != exprIdentifier || !isCaseKeyword(token.text) {
			continue
		}
		if i > 0 && string(tokens[i-1].text) == "@" {
			continue
		}
		return expressionShapeError(token, "reserved CASE keyword in expression")
	}
	return nil
}

func provenClosedOperandAdjacency(tokens []exprToken, dialect Dialect) error {
	for close := 0; close+1 < len(tokens); close++ {
		if !isClosingExpressionSymbol(tokens[close]) {
			continue
		}
		if !invalidClosedOperandSuffix(tokens, close, dialect) && !provenExpressionStart(tokens, close+1, dialect) {
			continue
		}
		if provenClosedOperand(tokens, close, dialect) {
			return expressionShapeError(tokens[close+1], "adjacent operands in expression")
		}
	}
	return nil
}

func invalidClosedOperandSuffix(tokens []exprToken, close int, dialect Dialect) bool {
	next := tokens[close+1]
	if next.kind == exprForm {
		return true
	}
	if next.kind != exprSymbol {
		return false
	}
	switch next.text[0] {
	case '(', '[', '.':
		return true
	case ':':
		return !expressionColonOwnedByBuiltin(tokens, close+1, dialect)
	default:
		return false
	}
}

func isClosingExpressionSymbol(token exprToken) bool {
	return token.kind == exprSymbol && string(token.text) == ")"
}

func provenClosedOperand(tokens []exprToken, close int, dialect Dialect) bool {
	open, ok := matchingExpressionOpen(tokens, close)
	if !ok {
		return false
	}
	return provenClosedOperandAt(tokens, close, open, dialect)
}

func provenClosedOperandAt(tokens []exprToken, close, open int, dialect Dialect) bool {
	if tokens[open].text[0] == '(' && open > 1 && string(tokens[open-2].text) == "@" {
		return false
	}
	if tokens[open].text[0] == '(' && open > 0 && tokens[open-1].kind == exprIdentifier {
		if !builtinMethodCall(tokens, open-1, dialect) {
			return false
		}
	}
	if open >= 2 && string(tokens[open-1].text) == ":" && tokens[open-2].kind == exprIdentifier {
		if _, known := builtinVariableInfoByName(string(tokens[open-2].text), dialect); known {
			index := tokens[open+1 : close]
			if commandMacroPossibleForDialect(index, dialect) || containsExpressionForm(index) {
				return false
			}
		}
	}
	return provenExpressionAtoms(tokens[open+1:close], dialect)
}

func matchingExpressionOpen(tokens []exprToken, close int) (int, bool) {
	want, open := byte(')'), byte('(')
	depth := 0
	for i := close; i >= 0; i-- {
		if tokens[i].kind != exprSymbol {
			continue
		}
		text := tokens[i].text
		if len(text) == 0 {
			continue
		}
		switch text[0] {
		case want:
			depth++
		case open:
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return -1, false
}

func builtinMethodCall(tokens []exprToken, start int, dialect Dialect) bool {
	if start < 0 || start+1 >= len(tokens) || tokens[start].kind != exprIdentifier || string(tokens[start+1].text) != "(" {
		return false
	}
	mask, known := builtinMethodMasks[string(tokens[start].text)]
	if !known || !methodDialect(mask, dialect) {
		return false
	}
	close, matched := boundedExpressionMatchingParen(tokens, start+1)
	return matched && close > start+1
}

func provenExpressionStart(tokens []exprToken, start int, dialect Dialect) bool {
	if start < 0 || start >= len(tokens) {
		return false
	}
	token := tokens[start]
	switch token.kind {
	case exprNumber, exprString, exprForm:
		return true
	case exprIdentifier:
		if start+1 < len(tokens) && string(tokens[start+1].text) == "@" {
			return false
		}
		if _, known := builtinVariableInfoByName(string(token.text), dialect); known {
			return true
		}
		if !builtinMethodCall(tokens, start, dialect) {
			return false
		}
		close, _ := boundedExpressionMatchingParen(tokens, start+1)
		return provenExpressionAtoms(tokens[start+2:close], dialect)
	default:
		return false
	}
}

func provenExpressionAtoms(tokens []exprToken, dialect Dialect) bool {
	if len(tokens) == 0 || containsExpressionMarker(tokens) || containsExpressionForm(tokens) {
		return false
	}
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token.kind != exprIdentifier {
			continue
		}
		if builtinMethodCall(tokens, i, dialect) {
			close, _ := boundedExpressionMatchingParen(tokens, i+1)
			if !provenExpressionAtoms(tokens[i+2:close], dialect) {
				return false
			}
			i = close
			continue
		}
		if _, known := builtinVariableInfoByName(string(token.text), dialect); known {
			if i+1 < len(tokens) && string(tokens[i+1].text) == "(" {
				return false
			}
			continue
		}
		if i >= 2 && string(tokens[i-1].text) == "@" && tokens[i-2].kind == exprIdentifier {
			name := string(tokens[i-2].text)
			if _, builtin := builtinVariableInfoByName(name, dialect); builtin && name != "ARG" && name != "ARGS" && name != "LOCAL" && name != "LOCALS" {
				continue
			}
		}
		return false
	}
	return true
}

func expressionShapeError(token exprToken, message string) error {
	return &exprError{offset: token.off, msg: message}
}
