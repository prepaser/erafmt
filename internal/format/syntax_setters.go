package format

func (p *formatter) syntaxSetters(i int, head []byte, base, wordEnd int, command string) int {
	start, end, comment, ok := p.registeredBody(head, wordEnd)
	if !ok {
		p.syntaxOpaque(i)
		return -1
	}
	codeEnd := registeredCodeEnd(base, end, comment)
	mark := len(p.edits)
	fail := func(offset int, message string) int {
		p.truncateEdits(mark)
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, offset, message)
		}
		p.syntaxOpaque(i)
		return -1
	}

	if start >= end {
		return fail(base+wordEnd, "setter command requires arguments")
	}
	if !p.expressionParseEnabled() {
		p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
		codeEnd = p.trimInputRight(base+start, codeEnd)
		p.setCodeEnd(i, codeEnd)
		return codeEnd
	}

	bodyStart := base + start
	if expressionExcludedWithSession(p.newExpressionSession(head), p.src[bodyStart:codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	if commandFieldHasDeferredMarker(p.src[bodyStart:codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	parts := p.splitTopLevel(p.src, bodyStart, codeEnd, ',')
	checked := setterCheckedFields(command)
	markerPresent := commandFieldHasMandatoryMarker(p.src[bodyStart:codeEnd])
	if command == "POWER" && !markerPresent && len(parts) < 3 {
		if tokens, err := p.lexSyntaxRange(bodyStart, codeEnd); err == nil && !setterMacroPossibleForDialect(tokens, p.dialect) {
			return fail(codeEnd, "POWER requires three arguments")
		}
	}
	macroPrefix := false
	destinationType := knownValueUnknown
	for n, field := range parts {
		if n >= checked {
			continue
		}
		left := p.trimInputLeft(field.start, field.end)
		right := p.trimInputRight(left, field.end)
		if left == right {
			if n == 0 {
				return fail(field.start, "setter command contains an empty destination")
			}
			if command == "POWER" && n < 3 && !macroPrefix && !markerPresent {
				return fail(field.start, "POWER requires three arguments")
			}
			if n == len(parts)-1 {
				continue
			}
			if macroPrefix {
				p.truncateEdits(mark)
				p.syntaxOpaque(i)
				return -1
			}
			return fail(field.start, "setter command contains an empty argument")
		}
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil {
			p.truncateEdits(mark)
			p.expressionProblem(i, 0, err)
			p.syntaxOpaque(i)
			return -1
		}
		if macroPrefix && setterHasAdjacentQuotedOperands(tokens) {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
		potentialMacro := setterMacroPossibleForDialect(tokens, p.dialect)
		if !markerPresent && !macroPrefix && !potentialMacro && builtinScalarIndexesMissing(tokens, p.dialect) {
			if n == 0 && command == "POWER" {
				return fail(left, "setter destination must be a scalar variable")
			}
			if n > 0 {
				return fail(left, "setter argument has an incompatible type")
			}
		}
		if n == 0 && !macroPrefix {
			if setterLiteralDestination(tokens) {
				return fail(left, "setter destination cannot be a literal")
			}
			if command == "CVARSET" && !markerPresent && !potentialMacro {
				if info, known := builtinVariableRoleInfo(tokens, p.dialect); known && (!info.character || info.rank > 1) {
					return fail(left, "CVARSET destination must be a scalar or one-dimensional character variable")
				}
				if name, exact := variableSystemName(tokens); exact && (name == "RESULT" || name == "RESULTS") {
					return fail(left, "CVARSET destination must be a character variable")
				}
			}
			destinationType = knownExpressionTypeForDialect(tokens, p.dialect)
			if command == "POWER" && destinationType == knownValueString {
				return fail(left, "setter destination must be numeric")
			}
			if setterInvalidDestination(tokens, p.dialect) {
				return fail(left, "setter destination cannot be assigned")
			}
		} else if !macroPrefix && !potentialMacro {
			valueType := knownExpressionTypeForDialect(tokens, p.dialect)
			invalid := false
			switch command {
			case "POWER":
				invalid = valueType == knownValueString
			case "VARSET", "CVARSET":
				if command == "CVARSET" && n == 1 {
					break
				}
				if (command == "VARSET" && n >= 2) || (command == "CVARSET" && n >= 3) {
					invalid = valueType == knownValueString
				} else {
					invalid = destinationType != knownValueUnknown && valueType != knownValueUnknown && valueType != destinationType
				}
			}
			if invalid {
				return fail(left, "setter argument has an incompatible type")
			}
		}
		macroPrefix = macroPrefix || potentialMacro
		valid := false
		if macroPrefix {
			valid = p.syntaxExpression(i, left, right, false)
		} else {
			valid = p.syntaxOrdinaryExpression(i, left, right, false)
		}
		if !valid {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
	}
	effective := parts
	if len(effective) > 0 {
		last := effective[len(effective)-1]
		left := p.trimInputLeft(last.start, last.end)
		if left >= p.trimInputRight(left, last.end) {
			effective = effective[:len(effective)-1]
		}
	}
	if len(effective) > checked && !markerPresent {
		uncertain := macroPrefix
		for _, field := range effective[checked:] {
			left := p.trimInputLeft(field.start, field.end)
			right := p.trimInputRight(left, field.end)
			if left >= right {
				continue
			}
			tokens, err := p.lexSyntaxRange(left, right)
			if err != nil || setterMacroPossibleForDialect(tokens, p.dialect) {
				uncertain = true
				break
			}
		}
		if uncertain {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
		p.ignoredCommandArguments(i, codeEnd, command)
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	p.syntaxRegisteredCommaGaps(i, parts, codeEnd)
	codeEnd = p.trimInputRight(bodyStart, codeEnd)
	p.setCodeEnd(i, codeEnd)
	return codeEnd
}

func setterHasAdjacentQuotedOperands(tokens []exprToken) bool {
	for i := 1; i < len(tokens); i++ {
		if !expressionWordLike(tokens[i-1]) || !expressionWordLike(tokens[i]) {
			continue
		}
		if tokens[i-1].kind == exprString || tokens[i].kind == exprString {
			return true
		}
	}
	return false
}

func (p *formatter) destinationRoleHasMarker(parts []syntaxSpan) bool {
	for _, field := range parts {
		left := p.trimInputLeft(field.start, field.end)
		right := p.trimInputRight(left, field.end)
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil {
			continue
		}
		for _, token := range tokens {
			if token.kind == exprMarker {
				return true
			}
		}
	}
	return false
}

func setterCheckedFields(command string) int {
	switch command {
	case "CVARSET":
		return 5
	case "POWER":
		return 3
	default:
		return 4
	}
}

func setterMacroPossible(tokens []exprToken) bool {
	return setterMacroPossibleForDialect(tokens, CommonDialect)
}

func setterMacroPossibleForDialect(tokens []exprToken, dialect Dialect) bool {
	for n, token := range tokens {
		if token.kind == exprForm {
			return true
		}
		if token.kind != exprIdentifier {
			continue
		}
		if !setterKnownNameForDialect(token.text, dialect) || n+1 < len(tokens) && string(tokens[n+1].text) == "(" {
			return true
		}
	}
	return false
}

func setterLiteralDestination(tokens []exprToken) bool {
	tokens = bitLiteralTokens(tokens)
	if setterNumericLiteralExpression(tokens) {
		return true
	}
	if len(tokens) == 2 && tokens[0].kind == exprOperator &&
		(string(tokens[0].text) == "+" || string(tokens[0].text) == "-") {
		return tokens[1].kind == exprNumber
	}
	if len(tokens) != 1 {
		return false
	}
	switch tokens[0].kind {
	case exprNumber, exprString, exprForm:
		return true
	default:
		return false
	}
}

func setterNumericLiteralExpression(tokens []exprToken) bool {
	hasNumber := false
	for _, token := range tokens {
		switch token.kind {
		case exprNumber:
			hasNumber = true
		case exprOperator:
		case exprSymbol:
			if string(token.text) != "(" && string(token.text) != ")" {
				return false
			}
		default:
			return false
		}
	}
	return hasNumber
}

func setterInvalidDestination(tokens []exprToken, dialect Dialect) bool {
	tokens = bitLiteralTokens(tokens)
	if len(tokens) == 0 {
		return false
	}
	if variableShapeInvalid(tokens) {
		return true
	}
	if builtinVariableReadOnly(tokens, dialect) {
		return true
	}
	if !setterMacroPossibleForDialect(tokens, dialect) {
		if setterTernaryDestination(tokens) || setterVariableSyntaxInvalid(tokens) {
			return true
		}
	}
	if tokens[0].kind == exprOperator && string(tokens[0].text) == "+" {
		if knownVariableType(tokens[1:]) == knownValueString {
			return true
		}
	}
	if tokens[0].kind == exprOperator {
		switch string(tokens[0].text) {
		case "+":
			return setterInvalidDestination(tokens[1:], dialect)
		case "-":
			return setterKnownDestinationPrefix(tokens[1:]) || setterBuiltinCall(tokens[1:], dialect)
		}
	}
	if setterBuiltinCall(tokens, dialect) {
		return true
	}
	if len(tokens) > 1 && tokens[0].kind == exprIdentifier && setterKnownNameForDialect(tokens[0].text, dialect) && tokens[1].kind == exprOperator {
		return true
	}
	if tokens[0].text[0] == '(' {
		close := setterMatchingParen(tokens)
		if close > 0 && close+1 < len(tokens) && setterKnownVariable(bitLiteralTokens(tokens[1:close])) && tokens[close+1].kind == exprOperator {
			return true
		}
	}
	return false
}

func setterVariableSyntaxInvalid(tokens []exprToken) bool {
	tokens = stripExpressionParens(tokens)
	if len(tokens) > 0 && tokens[0].kind == exprOperator && string(tokens[0].text) == "+" {
		tokens = stripExpressionParens(tokens[1:])
	}
	if len(tokens) == 0 || tokens[0].kind != exprIdentifier {
		return false
	}
	next := 1
	if next < len(tokens) && string(tokens[next].text) == "@" {
		if next+1 >= len(tokens) || tokens[next+1].kind != exprIdentifier {
			return false
		}
		next += 2
	}
	if next >= len(tokens) || (string(tokens[next].text) != ":" && string(tokens[next].text) != "[") {
		return false
	}
	return !variableArgumentTokens(tokens)
}

func setterTernaryDestination(tokens []exprToken) bool {
	depth := 0
	for _, token := range tokens {
		switch string(token.text) {
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

func setterKnownVariable(tokens []exprToken) bool {
	return len(tokens) == 1 && tokens[0].kind == exprIdentifier && setterKnownName(tokens[0].text)
}

func setterKnownDestinationPrefix(tokens []exprToken) bool {
	if len(tokens) == 0 {
		return false
	}
	if tokens[0].kind == exprIdentifier && setterKnownName(tokens[0].text) {
		return true
	}
	if tokens[0].kind != exprSymbol || string(tokens[0].text) != "(" {
		return false
	}
	close := setterMatchingParen(tokens)
	return close > 0 && setterKnownDestinationPrefix(bitLiteralTokens(tokens[1:close]))
}

func setterKnownName(name []byte) bool {
	return setterKnownNameForDialect(name, CommonDialect)
}

func setterKnownNameForDialect(name []byte, dialect Dialect) bool {
	_, known := builtinVariableInfoByName(string(name), dialect)
	return known
}

func setterBuiltinCall(tokens []exprToken, dialect Dialect) bool {
	if len(tokens) < 2 || tokens[0].kind != exprIdentifier || string(tokens[1].text) != "(" {
		return false
	}
	mask, ok := builtinMethodMasks[string(tokens[0].text)]
	return ok && methodDialect(mask, dialect)
}

func setterMatchingParen(tokens []exprToken) int {
	if len(tokens) == 0 || tokens[0].kind != exprSymbol || string(tokens[0].text) != "(" {
		return -1
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
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
