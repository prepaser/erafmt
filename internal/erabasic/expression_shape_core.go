package erabasic

// ProvenExpressionShape checks only grammar errors that are decidable from
// exact builtin names. Unknown identifiers and macro-capable fields defer.
func ProvenExpressionShape(tokens []ExprToken, dialect Dialect, caseMode bool) error {
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
	if err := ProvenKnownExpressionBoundaries(tokens, dialect); err != nil {
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

func provenClosedOperandOperators(tokens []ExprToken, dialect Dialect) error {
	for close := 0; close+1 < len(tokens); close++ {
		if !isClosingExpressionSymbol(tokens[close]) {
			continue
		}
		next := tokens[close+1]
		if next.Kind != ExprOperator && !isIncrementOperator(next) {
			continue
		}
		open, ok := matchingExpressionOpen(tokens, close)
		if !ok || open > 0 && string(tokens[open-1].Text) == ":" {
			continue
		}
		if !provenClosedOperandAt(tokens, close, open, dialect) {
			continue
		}
		if next.Kind == ExprOperator && !isIncrementOperator(next) && string(next.Text) != "!" && string(next.Text) != "~" {
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
		if string(next.Text) == "!" || string(next.Text) == "~" || expressionOperatorAtEnd(tokens, close+1) {
			return expressionShapeError(next, "expression has an invalid operator")
		}
	}
	if err := provenPrefixClosedOperandIncrement(tokens, dialect); err != nil {
		return err
	}
	return nil
}

func provenPrefixClosedOperandIncrement(tokens []ExprToken, dialect Dialect) error {
	for i, token := range tokens {
		if token.Kind != ExprOperator || !isIncrementOperator(token) || i+1 >= len(tokens) {
			continue
		}
		start, open := i+1, i+1
		method := false
		if tokens[start].Kind == ExprIdentifier && start+1 < len(tokens) && string(tokens[start+1].Text) == "(" {
			open, method = start+1, true
		} else if tokens[start].Kind != exprSymbol || string(tokens[start].Text) != "(" {
			continue
		}
		if open > 0 && string(tokens[open-1].Text) == ":" {
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

func closedOperandIsBuiltinMethod(tokens []ExprToken, open, close int, dialect Dialect) bool {
	if open < 0 || close < open || close >= len(tokens) {
		return false
	}
	if open > 0 && builtinMethodCall(tokens, open-1, dialect) {
		return true
	}
	operand := stripExpressionParens(tokens[open : close+1])
	return len(operand) > 1 && operand[0].Kind == ExprIdentifier && builtinMethodCall(operand, 0, dialect)
}

func provenClosedOperatorChain(tokens []ExprToken, close int) error {
	wantOperand := false
	i := close + 1
	for ; i < len(tokens) && tokens[i].Kind == ExprOperator; i++ {
		token := tokens[i]
		op := string(token.Text)
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

func expressionOperandBoundary(token ExprToken) bool {
	if token.Kind != exprSymbol {
		return false
	}
	switch string(token.Text) {
	case ")", "]", ",", ":":
		return true
	default:
		return false
	}
}

func ProvenKnownExpressionBoundaries(tokens []ExprToken, dialect Dialect) error {
	if len(tokens) > maxBuiltinProofTokens || containsExpressionMarker(tokens) || containsExpressionForm(tokens) || !expressionShapeIdentifiersKnown(tokens, dialect) || expressionShapeDepthExceeds(tokens) || containsScopedQualifier(tokens) {
		return nil
	}
	for i, token := range tokens {
		if token.Kind == ExprOperator {
			if i+1 >= len(tokens) {
				if !isIncrementOperator(token) {
					return expressionShapeError(token, "expression has an invalid operator")
				}
				continue
			}
			next := tokens[i+1]
			if next.Kind == exprSymbol && invalidExpressionOperandSymbol(string(next.Text)) && !(isIncrementOperator(token) && expressionPostfixBoundary(tokens, i, next)) {
				return expressionShapeError(next, "expression has an invalid operator")
			}
			continue
		}
		if token.Kind != exprSymbol {
			continue
		}
		symbol := string(token.Text)
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

func expressionShapeDepthExceeds(tokens []ExprToken) bool {
	depth := 0
	for _, token := range tokens {
		if token.Kind != exprSymbol {
			continue
		}
		switch string(token.Text) {
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

func containsScopedQualifier(tokens []ExprToken) bool {
	for i := 1; i+1 < len(tokens); i++ {
		if tokens[i].Kind == exprSymbol && string(tokens[i].Text) == "@" && tokens[i-1].Kind == ExprIdentifier && tokens[i+1].Kind == ExprIdentifier {
			return true
		}
	}
	return false
}

func expressionColonOwnedByBuiltin(tokens []ExprToken, index int, dialect Dialect) bool {
	for index > 0 {
		previous := index - 1
		switch tokens[previous].Kind {
		case ExprIdentifier:
			info, known := BuiltinVariableInfoByName(string(tokens[previous].Text), dialect)
			return known && (info.Rank > 0 || info.Character)
		case exprNumber, exprString:
		case exprSymbol:
			if string(tokens[previous].Text) != ")" {
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
		if previous == 0 || string(tokens[previous-1].Text) != ":" {
			return false
		}
		index = previous - 1
	}
	return false
}

func expressionPostfixBoundary(tokens []ExprToken, index int, next ExprToken) bool {
	if string(next.Text) != ")" && string(next.Text) != "]" && string(next.Text) != "," {
		return false
	}
	return index > 0 && (ExpressionWordLike(tokens[index-1]) || isClosingExpressionSymbol(tokens[index-1]))
}

func expressionOperandBoundaryBefore(tokens []ExprToken, index int) bool {
	if index == 0 {
		return true
	}
	previous := tokens[index-1]
	if previous.Kind == ExprOperator {
		return true
	}
	if previous.Kind != exprSymbol {
		return false
	}
	switch string(previous.Text) {
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

func invalidExpressionIndexStart(token ExprToken) bool {
	return token.Kind == ExprOperator || token.Kind == exprSymbol && invalidExpressionOperandSymbol(string(token.Text))
}

func invalidProvenIncrement(tokens, operand []ExprToken, dialect Dialect) (invalid, proven bool) {
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
	_, _, known = BuiltinExpressionType(operand, dialect)
	return known, known
}

func unresolvedBuiltinIndexOperand(tokens []ExprToken, dialect Dialect) bool {
	tokens = stripExpressionParens(tokens)
	if len(tokens) < 3 || tokens[0].Kind != ExprIdentifier || string(tokens[1].Text) != ":" {
		return false
	}
	_, known := BuiltinVariableInfoByName(string(tokens[0].Text), dialect)
	return known
}

func expressionOperatorAtEnd(tokens []ExprToken, operator int) bool {
	if operator+1 >= len(tokens) {
		return true
	}
	next := tokens[operator+1]
	return next.Kind == exprSymbol && (string(next.Text) == "," || string(next.Text) == ")" || string(next.Text) == "]")
}

func provenQualifiedBuiltinReference(tokens []ExprToken, dialect Dialect) error {
	for i := range tokens {
		_, qualified, _, known := knownBuiltinAtomEnd(tokens, i, dialect)
		if !known || !qualified {
			continue
		}
		return expressionShapeError(tokens[i+2], "non-local builtin variable reference")
	}
	return nil
}

func reservedExpressionKeyword(tokens []ExprToken, caseMode bool) error {
	if caseMode {
		return nil
	}
	for i, token := range tokens {
		if token.Kind != ExprIdentifier || !IsCaseKeyword(token.Text) {
			continue
		}
		if i > 0 && string(tokens[i-1].Text) == "@" {
			continue
		}
		return expressionShapeError(token, "reserved CASE keyword in expression")
	}
	return nil
}

func provenClosedOperandAdjacency(tokens []ExprToken, dialect Dialect) error {
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

func invalidClosedOperandSuffix(tokens []ExprToken, close int, dialect Dialect) bool {
	next := tokens[close+1]
	if next.Kind == ExprForm {
		return true
	}
	if next.Kind != exprSymbol {
		return false
	}
	switch next.Text[0] {
	case '(', '[', '.':
		return true
	case ':':
		return !expressionColonOwnedByBuiltin(tokens, close+1, dialect)
	default:
		return false
	}
}

func isClosingExpressionSymbol(token ExprToken) bool {
	return token.Kind == exprSymbol && string(token.Text) == ")"
}

func provenClosedOperand(tokens []ExprToken, close int, dialect Dialect) bool {
	open, ok := matchingExpressionOpen(tokens, close)
	if !ok {
		return false
	}
	return provenClosedOperandAt(tokens, close, open, dialect)
}

func provenClosedOperandAt(tokens []ExprToken, close, open int, dialect Dialect) bool {
	if tokens[open].Text[0] == '(' && open > 1 && string(tokens[open-2].Text) == "@" {
		return false
	}
	if tokens[open].Text[0] == '(' && open > 0 && tokens[open-1].Kind == ExprIdentifier {
		if !builtinMethodCall(tokens, open-1, dialect) {
			return false
		}
	}
	if open >= 2 && string(tokens[open-1].Text) == ":" && tokens[open-2].Kind == ExprIdentifier {
		if _, known := BuiltinVariableInfoByName(string(tokens[open-2].Text), dialect); known {
			index := tokens[open+1 : close]
			if CommandMacroPossibleForDialect(index, dialect) || containsExpressionForm(index) {
				return false
			}
		}
	}
	return provenExpressionAtoms(tokens[open+1:close], dialect)
}

func matchingExpressionOpen(tokens []ExprToken, close int) (int, bool) {
	want, open := byte(')'), byte('(')
	depth := 0
	for i := close; i >= 0; i-- {
		if tokens[i].Kind != exprSymbol {
			continue
		}
		text := tokens[i].Text
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

func builtinMethodCall(tokens []ExprToken, start int, dialect Dialect) bool {
	if start < 0 || start+1 >= len(tokens) || tokens[start].Kind != ExprIdentifier || string(tokens[start+1].Text) != "(" {
		return false
	}
	mask, known := builtinMethodMasks[string(tokens[start].Text)]
	if !known || !methodDialect(mask, dialect) {
		return false
	}
	close, matched := boundedExpressionMatchingParen(tokens, start+1)
	return matched && close > start+1
}

func provenExpressionStart(tokens []ExprToken, start int, dialect Dialect) bool {
	if start < 0 || start >= len(tokens) {
		return false
	}
	token := tokens[start]
	switch token.Kind {
	case exprNumber, exprString, ExprForm:
		return true
	case ExprIdentifier:
		if start+1 < len(tokens) && string(tokens[start+1].Text) == "@" {
			return false
		}
		if _, known := BuiltinVariableInfoByName(string(token.Text), dialect); known {
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

func provenExpressionAtoms(tokens []ExprToken, dialect Dialect) bool {
	if len(tokens) == 0 || containsExpressionMarker(tokens) || containsExpressionForm(tokens) {
		return false
	}
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token.Kind != ExprIdentifier {
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
		if _, known := BuiltinVariableInfoByName(string(token.Text), dialect); known {
			if i+1 < len(tokens) && string(tokens[i+1].Text) == "(" {
				return false
			}
			continue
		}
		if i >= 2 && string(tokens[i-1].Text) == "@" && tokens[i-2].Kind == ExprIdentifier {
			name := string(tokens[i-2].Text)
			if _, builtin := BuiltinVariableInfoByName(name, dialect); builtin && name != "ARG" && name != "ARGS" && name != "LOCAL" && name != "LOCALS" {
				continue
			}
		}
		return false
	}
	return true
}

func expressionShapeError(token ExprToken, message string) error {
	return &ExprError{Offset: token.Off, Msg: message}
}
