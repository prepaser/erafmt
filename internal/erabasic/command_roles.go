package erabasic

func commandFieldType(command string, index int) KnownValueType {
	switch command {
	case "BAR", "BARL", "SETCOLOR", "SETBGCOLOR", "TWAIT", "SWAPCHARA", "COPYCHARA", "ADDCHARA", "ADDSPCHARA", "ADDCOPYCHARA", "DELCHARA", "PICKUPCHARA", "TOOLTIP_SETCOLOR", "INPUT", "ONEINPUT", "BINPUT", "ONEBINPUT", "FONTSTYLE", "LOADDATA", "ASSERT", "TOOLTIP_SETDELAY", "TOOLTIP_SETDURATION", "INPUTMOUSEKEY", "RANDOMIZE", "SETBGMVOLUME", "SETSOUNDVOLUME", "TOOLTIP_SETFONTSIZE", "TOOLTIP_CUSTOM", "TOOLTIP_FORMAT", "TOOLTIP_IMG", "PRINTCPERLINE", "SAVENOS":
		return KnownValueNumeric
	case "SETFONT", "LOADCHARA", "LOADVAR", "PLAYBGM", "TOOLTIP_SETFONT", "PRINTSN", "PRINTFORMSN", "ARRAYCOPY":
		return KnownValueString
	case "SAVEDATA":
		if index == 1 {
			return KnownValueString
		}
		return KnownValueNumeric
	case "SAVECHARA":
		if index < 2 {
			return KnownValueString
		}
		return KnownValueNumeric
	case "TINPUT", "TONEINPUT":
		if index == 3 {
			return KnownValueString
		}
		return KnownValueNumeric
	case "TINPUTS", "TONEINPUTS":
		if index == 1 || index == 3 {
			return KnownValueString
		}
		return KnownValueNumeric
	case "PRINTBUTTON", "PRINTBUTTONC", "PRINTBUTTONLC":
		if index == 0 {
			return KnownValueString
		}
	case "ARRAYREMOVE":
		if index > 0 {
			return KnownValueNumeric
		}
	case "ARRAYSHIFT":
		if index == 1 || index >= 3 {
			return KnownValueNumeric
		}
	case "ARRAYSORT":
		if index >= 2 {
			return KnownValueNumeric
		}
	case "HTML_TAGSPLIT":
		if index < 2 {
			return KnownValueString
		}
		return KnownValueNumeric
	case "HTML_PRINT":
		if index == 0 {
			return KnownValueString
		}
		return KnownValueNumeric
	case "HTML_PRINT_ISLAND", "PLAYSOUND":
		if index == 0 {
			return KnownValueString
		}
		return KnownValueNumeric
	}
	return KnownValueUnknown
}

func CommandTypeMatches(expected, actual KnownValueType) bool {
	return expected == KnownValueUnknown || actual == KnownValueUnknown || expected == actual
}

type CommandRoles struct {
	Macro       bool
	Destination KnownValueType
}

func (p *SemanticContext) CommandFieldTypeValid(command string, index, left, right int, roles *CommandRoles) bool {
	if !p.expressionParseEnabled() || roles.Macro {
		return true
	}
	tokens, err := p.CommandFieldTokens(left, right)
	if err != nil {
		return true
	}
	if CommandMacroPossibleForDialect(tokens, p.Dialect) {
		roles.Macro = true
		return true
	}
	if !CommandFieldHasMandatoryMarker(p.Src[left:right]) {
		scalarField := !(index == 0 && (command == "ARRAYREMOVE" || command == "ARRAYSHIFT" || command == "SORTCHARA") || command == "HTML_TAGSPLIT" && index == 1)
		if scalarField && BuiltinScalarIndexesMissing(tokens, p.Dialect) {
			return false
		}
	}
	actual := KnownExpressionTypeForDialect(tokens, p.Dialect)
	if index == 0 {
		roles.Destination = actual
	}
	expected := commandFieldType(command, index)
	if command == "HTML_PRINT" && p.Dialect != EMEE && index > 0 {
		expected = KnownValueUnknown
	}
	if command == "ARRAYSHIFT" && index == 2 {
		expected = roles.Destination
	}
	if !CommandTypeMatches(expected, actual) {
		return false
	}
	mutable := index == 0 && (command == "ARRAYSHIFT" || command == "ARRAYREMOVE") || command == "HTML_TAGSPLIT" && index >= 1
	if mutable && BuiltinVariableReadOnly(tokens, p.Dialect) {
		return false
	}
	if info, known := BuiltinVariableRoleInfo(tokens, p.Dialect); known {
		if (command == "ARRAYREMOVE" || command == "ARRAYSHIFT") && index == 0 && info.Rank != 1 {
			return false
		}
		if command == "HTML_TAGSPLIT" && index == 1 && (info.Character || info.Rank != 1) {
			return false
		}
		if command == "SORTCHARA" && index == 0 && !info.Character {
			return false
		}
	}
	if name, exact := VariableSystemName(tokens); exact {
		if command == "SORTCHARA" && index == 0 && (name == "RESULT" || name == "RESULTS") {
			return false
		}
		if command == "HTML_TAGSPLIT" && index == 1 && (name == "CFLAG" || name == "CSTR") {
			return false
		}
	}
	return true
}

func CommandMacroPossible(tokens []ExprToken) bool {
	return CommandMacroPossibleForDialect(tokens, CommonDialect)
}

func CommandMacroPossibleForDialect(tokens []ExprToken, dialect Dialect) bool {
	for n, token := range tokens {
		if token.Kind == ExprIdentifier && (!setterKnownNameForDialect(token.Text, dialect) || n+1 < len(tokens) && string(tokens[n+1].Text) == "(") {
			return true
		}
	}
	return false
}

func (p *SemanticContext) CommandFieldsMayExpand(parts []SyntaxSpan) bool {
	for _, field := range parts {
		left, right := ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
		tokens, err := p.LexSyntaxRange(left, right)
		if err == nil && CommandMacroPossibleForDialect(tokens, p.Dialect) {
			return true
		}
	}
	return false
}

func (p *SemanticContext) GenericCommandRoleValid(command string, start, end int) (int, string, bool) {
	if !p.expressionParseEnabled() || !numericControlCommand(command) && !SimpleIntegerCommand(command) && !SimpleStringCommand(command) && command != "FOR" && command != "SELECTCASE" && command != "CASE" {
		return 0, "", true
	}
	if tokens, err := p.LexSyntaxRange(start, end); err == nil && command != "FOR" {
		for _, token := range tokens {
			if token.Kind == ExprMarker {
				return 0, "", true
			}
		}
	}
	parts := p.SplitTopLevel(p.Src, start, end, ',')
	if command == "FOR" && len(parts) > 1 {
		left, right := ExtraFieldBoundsWithSpace(p.Src, parts[len(parts)-1], p.AllowFullwidthSpace)
		if left == right {
			parts = parts[:len(parts)-1]
		}
	}
	macro := false
	for n, field := range parts {
		left, right := ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
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
		tokens, err := p.LexSyntaxRange(left, right)
		if err != nil {
			continue
		}
		if command != "FOR" {
			for _, token := range tokens {
				if token.Kind == ExprMarker {
					return 0, "", true
				}
			}
		}
		tokens = StripCommandMarkers(tokens)
		mayExpand := CommandMacroPossibleForDialect(tokens, p.Dialect)
		if command == "CASE" {
			classifyOperators(tokens, true)
			mayExpand = caseCommandMacroPossible(tokens, p.Dialect)
		}
		if macro || mayExpand {
			macro = true
			continue
		}
		scalarField := n == 0 || command == "RETURN" || command == "CASE" || command == "FOR" && n < 4
		if scalarField && !CommandFieldHasMandatoryMarker(p.Src[left:right]) && BuiltinScalarIndexesMissing(tokens, p.Dialect) {
			return left, "command argument has an incompatible type", false
		}
		actual := KnownExpressionTypeForDialect(tokens, p.Dialect)
		switch {
		case numericControlCommand(command):
			if n == 0 && actual == KnownValueString {
				return left, "command argument has an incompatible type", false
			}
		case SimpleIntegerCommand(command):
			if command == "RETURN" || n == 0 {
				if actual == KnownValueString {
					return left, "command argument has an incompatible type", false
				}
			}
		case SimpleStringCommand(command) && command != "RETURNF":
			if n == 0 && actual == KnownValueNumeric {
				return left, "command argument has an incompatible type", false
			}
		case command == "FOR":
			if n == 0 {
				variable := p.isVariableArgument(left, right)
				if CommandFieldHasMandatoryMarker(p.Src[left:right]) {
					variable = p.CommandFieldVariableArgument(left, right)
				}
				if !variable {
					return left, "FOR counter must be a mutable variable", false
				}
				if actual == KnownValueString {
					return left, "FOR counter must be numeric", false
				}
				if BuiltinVariableReadOnly(tokens, p.Dialect) {
					return left, "FOR counter must be a mutable variable", false
				}
				if info, known := BuiltinVariableRoleInfo(tokens, p.Dialect); known && info.Character {
					return left, "FOR counter cannot be a character variable", false
				}
				if name, exact := VariableSystemName(tokens); exact && name == "CFLAG" {
					return left, "FOR counter cannot be a character variable", false
				}
			} else if n >= 1 && n <= 3 && actual == KnownValueString {
				return left, "FOR range argument has an incompatible type", false
			}
		}
	}
	if command == "FOR" && len(parts) < 3 && !macro {
		return end, "FOR requires an end expression", false
	}
	return 0, "", true
}

func RequiredExpressionCommand(command string) bool {
	return SimpleStringCommand(command) && command != "RETURNF" || command == "SELECTCASE" || command == "CASE"
}

func numericControlCommand(command string) bool {
	switch command {
	case "IF", "ELSEIF", "SIF", "WHILE", "REPEAT", "LOOP":
		return true
	default:
		return false
	}
}
