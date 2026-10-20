package format

func commandFieldType(command string, index int) knownValueType {
	switch command {
	case "BAR", "BARL", "SETCOLOR", "SETBGCOLOR", "TWAIT", "SWAPCHARA", "COPYCHARA",
		"ADDCHARA", "ADDSPCHARA", "ADDCOPYCHARA", "DELCHARA", "PICKUPCHARA", "TOOLTIP_SETCOLOR",
		"INPUT", "ONEINPUT", "BINPUT", "ONEBINPUT", "FONTSTYLE", "LOADDATA", "ASSERT",
		"TOOLTIP_SETDELAY", "TOOLTIP_SETDURATION", "INPUTMOUSEKEY", "RANDOMIZE",
		"SETBGMVOLUME", "SETSOUNDVOLUME", "TOOLTIP_SETFONTSIZE", "TOOLTIP_CUSTOM", "TOOLTIP_FORMAT", "TOOLTIP_IMG",
		"PRINTCPERLINE", "SAVENOS":
		return knownValueNumeric
	case "SETFONT", "LOADCHARA", "LOADVAR", "PLAYBGM", "TOOLTIP_SETFONT", "PRINTSN", "PRINTFORMSN", "ARRAYCOPY":
		return knownValueString
	case "SAVEDATA":
		if index == 1 {
			return knownValueString
		}
		return knownValueNumeric
	case "SAVECHARA":
		if index < 2 {
			return knownValueString
		}
		return knownValueNumeric
	case "TINPUT", "TONEINPUT":
		if index == 3 {
			return knownValueString
		}
		return knownValueNumeric
	case "TINPUTS", "TONEINPUTS":
		if index == 1 || index == 3 {
			return knownValueString
		}
		return knownValueNumeric
	case "PRINTBUTTON", "PRINTBUTTONC", "PRINTBUTTONLC":
		if index == 0 {
			return knownValueString
		}
	case "ARRAYREMOVE":
		if index > 0 {
			return knownValueNumeric
		}
	case "ARRAYSHIFT":
		if index == 1 || index >= 3 {
			return knownValueNumeric
		}
	case "ARRAYSORT":
		if index >= 2 {
			return knownValueNumeric
		}
	case "HTML_TAGSPLIT":
		if index < 2 {
			return knownValueString
		}
		return knownValueNumeric
	case "HTML_PRINT":
		if index == 0 {
			return knownValueString
		}
		return knownValueNumeric
	case "HTML_PRINT_ISLAND", "PLAYSOUND":
		if index == 0 {
			return knownValueString
		}
		return knownValueNumeric
	}
	return knownValueUnknown
}

func commandTypeMatches(expected, actual knownValueType) bool {
	return expected == knownValueUnknown || actual == knownValueUnknown || expected == actual
}

type commandRoles struct {
	macro       bool
	destination knownValueType
}

func (p *formatter) commandFieldTypeValid(command string, index, left, right int, roles *commandRoles) bool {
	if !p.expressionParseEnabled() || roles.macro {
		return true
	}
	tokens, err := p.commandFieldTokens(left, right)
	if err != nil {
		return true
	}
	if commandMacroPossibleForDialect(tokens, p.dialect) {
		roles.macro = true
		return true
	}
	if !commandFieldHasMandatoryMarker(p.src[left:right]) {
		scalarField := !(index == 0 && (command == "ARRAYREMOVE" || command == "ARRAYSHIFT" || command == "SORTCHARA") || command == "HTML_TAGSPLIT" && index == 1)
		if scalarField && builtinScalarIndexesMissing(tokens, p.dialect) {
			return false
		}
	}
	actual := knownExpressionTypeForDialect(tokens, p.dialect)
	if index == 0 {
		roles.destination = actual
	}
	expected := commandFieldType(command, index)
	if command == "HTML_PRINT" && p.dialect != EMEE && index > 0 {
		expected = knownValueUnknown
	}
	if command == "ARRAYSHIFT" && index == 2 {
		expected = roles.destination
	}
	if !commandTypeMatches(expected, actual) {
		return false
	}
	mutable := index == 0 && (command == "ARRAYSHIFT" || command == "ARRAYREMOVE") || command == "HTML_TAGSPLIT" && index >= 1
	if mutable && builtinVariableReadOnly(tokens, p.dialect) {
		return false
	}
	if info, known := builtinVariableRoleInfo(tokens, p.dialect); known {
		if (command == "ARRAYREMOVE" || command == "ARRAYSHIFT") && index == 0 && info.rank != 1 {
			return false
		}
		if command == "HTML_TAGSPLIT" && index == 1 && (info.character || info.rank != 1) {
			return false
		}
		if command == "SORTCHARA" && index == 0 && !info.character {
			return false
		}
	}
	if name, exact := variableSystemName(tokens); exact {
		if command == "SORTCHARA" && index == 0 && (name == "RESULT" || name == "RESULTS") {
			return false
		}
		if command == "HTML_TAGSPLIT" && index == 1 && (name == "CFLAG" || name == "CSTR") {
			return false
		}
	}
	return true
}

func commandMacroPossible(tokens []exprToken) bool {
	return commandMacroPossibleForDialect(tokens, CommonDialect)
}

func commandMacroPossibleForDialect(tokens []exprToken, dialect Dialect) bool {
	for n, token := range tokens {
		if token.kind == exprIdentifier && (!setterKnownNameForDialect(token.text, dialect) || n+1 < len(tokens) && string(tokens[n+1].text) == "(") {
			return true
		}
	}
	return false
}

func (p *formatter) commandFieldsMayExpand(parts []syntaxSpan) bool {
	for _, field := range parts {
		left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
		tokens, err := p.lexSyntaxRange(left, right)
		if err == nil && commandMacroPossibleForDialect(tokens, p.dialect) {
			return true
		}
	}
	return false
}

func (p *formatter) genericCommandRoleValid(command string, start, end int) (int, string, bool) {
	if !p.expressionParseEnabled() || !numericControlCommand(command) && !simpleIntegerCommand(command) && !simpleStringCommand(command) && command != "FOR" && command != "SELECTCASE" && command != "CASE" {
		return 0, "", true
	}
	if tokens, err := p.lexSyntaxRange(start, end); err == nil && command != "FOR" {
		for _, token := range tokens {
			if token.kind == exprMarker {
				return 0, "", true
			}
		}
	}
	parts := p.splitTopLevel(p.src, start, end, ',')
	if command == "FOR" && len(parts) > 1 {
		left, right := extraFieldBoundsWithSpace(p.src, parts[len(parts)-1], p.allowFullwidthSpace)
		if left == right {
			parts = parts[:len(parts)-1]
		}
	}
	macro := false
	for n, field := range parts {
		left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
		if left == right {
			if command != "FOR" && !macro && (n == 0 || n < len(parts)-1 && (command == "RETURN" || command == "CASE")) {
				return left, "command contains an omitted expression", false
			}
			if command == "FOR" && n == 0 {
				return left, "FOR requires a mutable counter", false
			}
			if command == "FOR" && n == 2 && !macro {
				return left, "FOR requires an end expression", false
			}
			if command == "FOR" && n == 3 && !macro {
				return left, "FOR contains an explicitly omitted step", false
			}
			continue
		}
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil {
			continue
		}
		if command != "FOR" {
			for _, token := range tokens {
				if token.kind == exprMarker {
					return 0, "", true
				}
			}
		}
		tokens = stripCommandMarkers(tokens)
		mayExpand := commandMacroPossibleForDialect(tokens, p.dialect)
		if command == "CASE" {
			classifyOperators(tokens, true)
			mayExpand = caseCommandMacroPossible(tokens, p.dialect)
		}
		if macro || mayExpand {
			macro = true
			continue
		}
		scalarField := n == 0 || command == "RETURN" || command == "CASE" || command == "FOR" && n < 4
		if scalarField && !commandFieldHasMandatoryMarker(p.src[left:right]) && builtinScalarIndexesMissing(tokens, p.dialect) {
			return left, "command argument has an incompatible type", false
		}
		actual := knownExpressionTypeForDialect(tokens, p.dialect)
		switch {
		case numericControlCommand(command):
			if n == 0 && actual == knownValueString {
				return left, "command argument has an incompatible type", false
			}
		case simpleIntegerCommand(command):
			if command == "RETURN" || n == 0 {
				if actual == knownValueString {
					return left, "command argument has an incompatible type", false
				}
			}
		case simpleStringCommand(command) && command != "RETURNF":
			if n == 0 && actual == knownValueNumeric {
				return left, "command argument has an incompatible type", false
			}
		case command == "FOR":
			if n == 0 {
				variable := p.isVariableArgument(p.src[left:right])
				if commandFieldHasMandatoryMarker(p.src[left:right]) {
					variable = commandFieldVariableArgument(p, left, right)
				}
				if !variable {
					return left, "FOR counter must be a mutable variable", false
				}
				if actual == knownValueString {
					return left, "FOR counter must be numeric", false
				}
				if builtinVariableReadOnly(tokens, p.dialect) {
					return left, "FOR counter must be a mutable variable", false
				}
				if info, known := builtinVariableRoleInfo(tokens, p.dialect); known && info.character {
					return left, "FOR counter cannot be a character variable", false
				}
				if name, exact := variableSystemName(tokens); exact && name == "CFLAG" {
					return left, "FOR counter cannot be a character variable", false
				}
			} else if n >= 1 && n <= 3 && actual == knownValueString {
				return left, "FOR range argument has an incompatible type", false
			}
		}
	}
	if command == "FOR" && len(parts) < 3 && !macro {
		return end, "FOR requires an end expression", false
	}
	return 0, "", true
}

func requiredExpressionCommand(command string) bool {
	return simpleStringCommand(command) && command != "RETURNF" || command == "SELECTCASE" || command == "CASE"
}

func numericControlCommand(command string) bool {
	switch command {
	case "IF", "ELSEIF", "SIF", "WHILE", "REPEAT", "LOOP":
		return true
	default:
		return false
	}
}
