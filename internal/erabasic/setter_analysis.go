package erabasic

func (p *SemanticContext) setterHasAdjacentQuotedOperands(tokens []ExprToken) bool {
	return setterHasAdjacentQuotedOperands(tokens)
}

func (p *SemanticContext) SetterMacroPossibleForDialect(tokens []ExprToken) bool {
	return SetterMacroPossibleForDialect(tokens, p.Dialect)
}

func (p *SemanticContext) setterLiteralDestination(tokens []ExprToken) bool {
	return setterLiteralDestination(tokens)
}

func (p *SemanticContext) setterInvalidDestination(tokens []ExprToken) bool {
	return setterInvalidDestination(tokens, p.Dialect)
}

func (p *SemanticContext) setterKnownName(name []byte) bool {
	return setterKnownNameForDialect(name, p.Dialect)
}

func (p *SemanticContext) setterBuiltinCall(tokens []ExprToken) bool {
	return setterBuiltinCall(tokens, p.Dialect)
}

type SetterValidationState struct {
	MacroPrefix     bool
	DestinationType KnownValueType
}

type setterFieldValidation struct {
	Valid           bool
	Opaque          bool
	ExprErr         error
	Offset          int
	Message         string
	MacroPrefix     bool
	DestinationType KnownValueType
}

func (p *SemanticContext) ValidateSetterField(command string, index int, left, right, fieldCount int, markerPresent bool, state SetterValidationState) setterFieldValidation {
	result := setterFieldValidation{Valid: true, MacroPrefix: state.MacroPrefix, DestinationType: state.DestinationType}
	if left == right {
		if index == 0 {
			return setterFieldValidation{Offset: left, Message: "setter command contains an empty destination"}
		}
		if command == "POWER" && index < 3 && !state.MacroPrefix && !markerPresent {
			return setterFieldValidation{Offset: left, Message: "POWER requires three arguments"}
		}
		if index == fieldCount-1 {
			return result
		}
		if state.MacroPrefix {
			return setterFieldValidation{Opaque: true}
		}
		return setterFieldValidation{Offset: left, Message: "setter command contains an empty argument"}
	}
	tokens, err := p.LexSyntaxRange(left, right)
	if err != nil {
		return setterFieldValidation{ExprErr: err}
	}
	if state.MacroPrefix && p.setterHasAdjacentQuotedOperands(tokens) {
		return setterFieldValidation{Opaque: true}
	}
	potentialMacro := p.SetterMacroPossibleForDialect(tokens)
	if !markerPresent && !state.MacroPrefix && !potentialMacro && BuiltinScalarIndexesMissing(tokens, p.Dialect) {
		if index == 0 && command == "POWER" {
			return setterFieldValidation{Offset: left, Message: "setter destination must be a scalar variable"}
		}
		if index > 0 {
			return setterFieldValidation{Offset: left, Message: "setter argument has an incompatible type"}
		}
	}
	if index == 0 && !state.MacroPrefix {
		if p.setterLiteralDestination(tokens) {
			return setterFieldValidation{Offset: left, Message: "setter destination cannot be a literal"}
		}
		if command == "CVARSET" && !markerPresent && !potentialMacro {
			if info, known := BuiltinVariableRoleInfo(tokens, p.Dialect); known && (!info.Character || info.Rank > 1) {
				return setterFieldValidation{Offset: left, Message: "CVARSET destination must be a scalar or one-dimensional character variable"}
			}
			if name, exact := VariableSystemName(tokens); exact && (name == "RESULT" || name == "RESULTS") {
				return setterFieldValidation{Offset: left, Message: "CVARSET destination must be a character variable"}
			}
		}
		result.DestinationType = KnownExpressionTypeForDialect(tokens, p.Dialect)
		if command == "POWER" && result.DestinationType == KnownValueString {
			return setterFieldValidation{Offset: left, Message: "setter destination must be numeric"}
		}
		if p.setterInvalidDestination(tokens) {
			return setterFieldValidation{Offset: left, Message: "setter destination cannot be assigned"}
		}
	} else if !state.MacroPrefix && !potentialMacro {
		valueType := KnownExpressionTypeForDialect(tokens, p.Dialect)
		invalid := false
		switch command {
		case "POWER":
			invalid = valueType == KnownValueString
		case "VARSET", "CVARSET":
			if command == "CVARSET" && index == 1 {
				break
			}
			if (command == "VARSET" && index >= 2) || (command == "CVARSET" && index >= 3) {
				invalid = valueType == KnownValueString
			} else {
				invalid = result.DestinationType != KnownValueUnknown && valueType != KnownValueUnknown && valueType != result.DestinationType
			}
		}
		if invalid {
			return setterFieldValidation{Offset: left, Message: "setter argument has an incompatible type"}
		}
	}
	result.MacroPrefix = state.MacroPrefix || potentialMacro
	return result
}

func (p *SemanticContext) SetterSurplusUncertain(parts []SyntaxSpan, checked int, macroPrefix bool) bool {
	uncertain := macroPrefix
	for _, field := range parts[checked:] {
		left := TrimInputLeft(p.Src, field.Start, field.End, p.AllowFullwidthSpace)
		right := TrimInputRight(p.Src, left, field.End, p.AllowFullwidthSpace)
		if left >= right {
			continue
		}
		tokens, err := p.LexSyntaxRange(left, right)
		if err != nil || p.SetterMacroPossibleForDialect(tokens) {
			uncertain = true
			break
		}
	}
	return uncertain
}
