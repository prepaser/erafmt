package format

func (p *formatter) syntaxExtra(i int, head []byte, base int, command string, wordEnd int) (int, bool) {
	switch command {
	case "TIMES":
		return p.syntaxExtraTimes(i, head, base, wordEnd), true
	case "PRINTDATA", "PRINTDATAL", "PRINTDATAW", "PRINTDATAK", "PRINTDATAKL", "PRINTDATAKW", "PRINTDATAD", "PRINTDATADL", "PRINTDATADW", "STRDATA":
		return p.syntaxExtraVariableCommand(i, head, base, wordEnd, command), true
	case "SPLIT":
		return p.syntaxExtraSplit(i, head, base, wordEnd), true
	case "SWAP":
		return p.syntaxExtraSwap(i, head, base, wordEnd), true
	default:
		return -1, false
	}
}

func (p *formatter) syntaxExtraTimes(i int, head []byte, base, wordEnd int) int {
	mark := len(p.edits)
	fail := func(offset int, message string) int {
		p.truncateEdits(mark)
		if p.expressionRulesEnabled() {
			p.syntaxProblem(i, offset, message)
		}
		p.syntaxOpaque(i)
		return -1
	}
	start, end, comment, ok := p.registeredBody(head, wordEnd)
	if !ok {
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	if !registeredHasArgumentWithSpace(head, start, end, p.allowFullwidthSpace) {
		return fail(base+wordEnd, "TIMES expects two arguments")
	}
	codeEnd := registeredCodeEnd(base, end, comment)
	if commandFieldHasDeferredMarker(p.src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	parts := p.splitTopLevel(p.src, base+start, codeEnd, ',')
	parts, overflow := commandPrefixParts(p.src, parts, 2, 2, p.allowFullwidthSpace, true)
	if len(parts) != 2 {
		return fail(base+start, "TIMES expects two arguments")
	}
	left, right := extraFieldBoundsWithSpace(p.src, parts[0], p.allowFullwidthSpace)
	firstMarked := commandFieldHasMandatoryMarker(p.src[left:right])
	firstVariable := p.isVariableArgument(p.src[left:right])
	if firstMarked {
		firstVariable = commandFieldVariableArgument(p, left, right)
	}
	if left == right || !firstVariable {
		return fail(left, "TIMES requires a variable as its first argument")
	}
	if p.expressionParseEnabled() {
		if tokens, err := p.commandFieldTokens(left, right); err == nil && !commandMacroPossibleForDialect(tokens, p.dialect) {
			if !firstMarked && builtinScalarIndexesMissing(tokens, p.dialect) {
				return fail(left, "scalar variable requires indexes")
			}
			if builtinVariableReadOnly(tokens, p.dialect) {
				return fail(left, "TIMES requires a mutable variable as its first argument")
			}
			if knownExpressionTypeForDialect(tokens, p.dialect) == knownValueString {
				return fail(left, "TIMES requires a numeric variable as its first argument")
			}
		}
	}
	coefficientLeft, coefficientRight := extraFieldBoundsWithSpace(p.src, parts[1], p.allowFullwidthSpace)
	coefficientMarked := commandFieldHasMandatoryMarker(p.src[coefficientLeft:coefficientRight])
	coefficient := p.src[coefficientLeft:coefficientRight]
	if coefficientLeft == coefficientRight || (!validTimesCoefficient(coefficient) && !timesCoefficientRuntimeWarning(coefficient) && !coefficientMarked) {
		return fail(coefficientLeft, "TIMES requires a decimal coefficient")
	}
	if timesCoefficientRuntimeWarning(coefficient) && !coefficientMarked {
		if p.expressionParseEnabled() {
			p.warning(i, coefficientLeft, "runtime_numeric", "TIMES coefficient is only partially parsed by the runtime")
		}
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	if p.expressionRulesEnabled() && !p.syntaxExpression(i, left, right, false) {
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	if overflow {
		if p.expressionParseEnabled() {
			p.ignoredCommandArguments(i, codeEnd, "TIMES")
		}
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	p.syntaxRegisteredCommaGaps(i, parts, codeEnd)
	p.setCodeEnd(i, trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace))
	return codeEnd
}

func (p *formatter) syntaxExtraVariableCommand(i int, head []byte, base, wordEnd int, command string) int {
	mark := len(p.edits)
	fail := func(offset int, message string) int {
		p.truncateEdits(mark)
		if p.expressionRulesEnabled() {
			p.syntaxProblem(i, offset, message)
		}
		p.syntaxOpaque(i)
		return -1
	}
	start, end, comment, ok := p.registeredBody(head, wordEnd)
	if !ok {
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	codeEnd := registeredCodeEnd(base, end, comment)
	if commandFieldHasDeferredMarker(p.src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	if !registeredHasArgumentWithSpace(head, start, end, p.allowFullwidthSpace) {
		p.setCodeEnd(i, base+wordEnd)
		return base + wordEnd
	}
	parts := p.splitTopLevel(p.src, base+start, codeEnd, ',')
	parts, overflow := commandPrefixParts(p.src, parts, 1, 1, p.allowFullwidthSpace, true)
	if len(parts) != 1 {
		return fail(base+start, "command expects at most one variable argument")
	}
	left, right := extraFieldBoundsWithSpace(p.src, parts[0], p.allowFullwidthSpace)
	variable := p.isVariableArgument(p.src[left:right])
	if commandFieldHasMandatoryMarker(p.src[left:right]) {
		variable = commandFieldVariableArgument(p, left, right)
	}
	if left == right || !variable {
		return fail(left, "command requires a variable argument")
	}
	if p.expressionParseEnabled() {
		tokens, err := p.commandFieldTokens(left, right)
		if err == nil && !commandMacroPossibleForDialect(tokens, p.dialect) {
			if !commandFieldHasMandatoryMarker(p.src[left:right]) && builtinScalarIndexesMissing(tokens, p.dialect) {
				return fail(left, "scalar variable requires indexes")
			}
			expected := knownValueNumeric
			if command == "STRDATA" {
				expected = knownValueString
			}
			if builtinVariableReadOnly(tokens, p.dialect) {
				p.truncateEdits(mark)
				p.syntaxProblem(i, left, "command requires a mutable variable argument")
				p.syntaxOpaque(i)
				return -1
			}
			if !commandTypeMatches(expected, knownExpressionTypeForDialect(tokens, p.dialect)) {
				p.truncateEdits(mark)
				p.syntaxProblem(i, left, "command argument has an incompatible type")
				p.syntaxOpaque(i)
				return -1
			}
		}
	}
	if p.expressionRulesEnabled() && !p.syntaxExpression(i, left, right, false) {
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	if overflow {
		if p.expressionParseEnabled() {
			p.ignoredCommandArguments(i, codeEnd, command)
		}
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	p.setCodeEnd(i, trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace))
	return codeEnd
}

type extraFieldValidation struct {
	valid bool
	quiet bool
}

func (p *formatter) syntaxExtraSplit(i int, head []byte, base, wordEnd int) int {
	macroPrefix := false
	return p.syntaxExtraFields(i, head, base, wordEnd, 3, 4, "SPLIT", func(n int, left, right int) extraFieldValidation {
		if left == right {
			return extraFieldValidation{}
		}
		tokens, err := p.commandFieldTokens(left, right)
		potentialMacro := err == nil && commandMacroPossibleForDialect(tokens, p.dialect)
		valid := n < 2 || err == nil && variableArgumentTokens(tokens)
		if err == nil {
			if n != 2 && !macroPrefix && !potentialMacro && !commandFieldHasMandatoryMarker(p.src[left:right]) && builtinScalarIndexesMissing(tokens, p.dialect) {
				valid = false
			}
			if n >= 2 && !macroPrefix && !potentialMacro && builtinVariableReadOnly(tokens, p.dialect) {
				valid = false
			}
			typeOfValue := knownExpressionTypeForDialect(tokens, p.dialect)
			if n < 2 && typeOfValue == knownValueNumeric {
				valid = false
			}
			if n == 2 && typeOfValue == knownValueNumeric {
				valid = false
			}
			if n == 3 && typeOfValue == knownValueString {
				valid = false
			}
			if n == 2 {
				if known, array := builtinArrayRole(tokens); known && !array {
					valid = false
				}
			}
		}
		if !valid {
			return extraFieldValidation{quiet: macroPrefix || potentialMacro}
		}
		macroPrefix = macroPrefix || potentialMacro
		return extraFieldValidation{valid: true}
	})
}

func (p *formatter) syntaxExtraSwap(i int, head []byte, base, wordEnd int) int {
	macroPrefix := false
	var destination knownValueType
	return p.syntaxExtraFields(i, head, base, wordEnd, 2, 2, "SWAP", func(n int, left, right int) extraFieldValidation {
		if left >= right {
			return extraFieldValidation{}
		}
		tokens, err := p.commandFieldTokens(left, right)
		potentialMacro := err == nil && commandMacroPossibleForDialect(tokens, p.dialect)
		valid := err == nil && variableArgumentTokens(tokens)
		if !valid {
			return extraFieldValidation{quiet: macroPrefix || potentialMacro}
		}
		if !potentialMacro && builtinVariableReadOnly(tokens, p.dialect) {
			return extraFieldValidation{}
		}
		if !macroPrefix && !potentialMacro {
			if !commandFieldHasMandatoryMarker(p.src[left:right]) && builtinScalarIndexesMissing(tokens, p.dialect) {
				return extraFieldValidation{}
			}
			actual := knownExpressionTypeForDialect(tokens, p.dialect)
			if n == 0 {
				destination = actual
			} else if !commandTypeMatches(destination, actual) {
				return extraFieldValidation{}
			}
		}
		macroPrefix = macroPrefix || potentialMacro
		return extraFieldValidation{valid: true}
	})
}

func (p *formatter) syntaxExtraFields(i int, head []byte, base, wordEnd, min, max int, command string, valid func(int, int, int) extraFieldValidation) int {
	mark := len(p.edits)
	fail := func(offset int, message string) int {
		p.truncateEdits(mark)
		if p.expressionRulesEnabled() {
			p.syntaxProblem(i, offset, message)
		}
		p.syntaxOpaque(i)
		return -1
	}
	start, end, comment, ok := p.registeredBody(head, wordEnd)
	if !ok {
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	if !registeredHasArgumentWithSpace(head, start, end, p.allowFullwidthSpace) {
		return fail(base+wordEnd, command+" expects command arguments")
	}
	codeEnd := registeredCodeEnd(base, end, comment)
	if commandFieldHasDeferredMarker(p.src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	parts := p.splitTopLevel(p.src, base+start, codeEnd, ',')
	parts, overflow := commandPrefixParts(p.src, parts, max, min, p.allowFullwidthSpace, true)
	if len(parts) < min {
		if len(parts) < min && p.commandFieldsMayExpand(parts) {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
		return fail(base+start, command+" has an invalid argument count")
	}
	for n, field := range parts {
		left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
		result := valid(n, left, right)
		if !result.valid {
			if result.quiet {
				p.truncateEdits(mark)
				p.syntaxOpaque(i)
				return -1
			}
			return fail(left, command+" has an invalid argument")
		}
		if p.expressionRulesEnabled() && !p.syntaxExpression(i, left, right, false) {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
	}
	if overflow {
		if p.expressionParseEnabled() {
			p.ignoredCommandArguments(i, codeEnd, command)
		}
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	p.syntaxRegisteredCommaGaps(i, parts, codeEnd)
	p.setCodeEnd(i, trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace))
	return codeEnd
}

func extraFieldBoundsWithSpace(src []byte, field syntaxSpan, allowFullwidthSpace bool) (int, int) {
	left := field.start
	left = trimInputLeft(src, left, field.end, allowFullwidthSpace)
	right := field.end
	right = trimInputRight(src, left, right, allowFullwidthSpace)
	return left, right
}

func validTimesCoefficient(src []byte) bool {
	if len(src) == 0 {
		return false
	}
	i := 0
	if src[i] == '+' || src[i] == '-' {
		i++
		if i == len(src) {
			return false
		}
	}
	digits := 0
	for i < len(src) && src[i] >= '0' && src[i] <= '9' {
		i++
		digits++
	}
	if i < len(src) && src[i] == '.' {
		i++
		for i < len(src) && src[i] >= '0' && src[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return false
	}
	if i < len(src) && (src[i] == 'e' || src[i] == 'E') {
		i++
		if i < len(src) && src[i] == '-' {
			i++
		}
		exponent := i
		for i < len(src) && src[i] >= '0' && src[i] <= '9' {
			i++
		}
		if exponent == i {
			return false
		}
	}
	return i == len(src)
}

func timesCoefficientRuntimeWarning(src []byte) bool {
	if validTimesCoefficient(src) || len(src) == 0 {
		return false
	}
	i := 0
	if src[i] == '+' || src[i] == '-' {
		i++
		if i == len(src) {
			return false
		}
	}
	if i+1 < len(src) && src[i] == '0' && (src[i+1] == 'x' || src[i+1] == 'X' || src[i+1] == 'b' || src[i+1] == 'B') {
		return false
	}
	digits := 0
	for i < len(src) && src[i] >= '0' && src[i] <= '9' {
		i++
		digits++
	}
	if i < len(src) && src[i] == '.' {
		i++
		for i < len(src) && src[i] >= '0' && src[i] <= '9' {
			i++
			digits++
		}
	}
	return digits > 0
}
