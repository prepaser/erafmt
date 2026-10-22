package format

import "github.com/prepaser/erafmt/internal/erabasic"

func (p *formatter) syntaxExtra(i int, head []byte, base int, command string, wordEnd int) (int, bool) {
	switch erabasic.ExtraCommand(command) {
	case erabasic.ExtraTimes:
		return p.syntaxExtraTimes(i, head, base, wordEnd), true
	case erabasic.ExtraVariable:
		return p.syntaxExtraVariableCommand(i, head, base, wordEnd, command), true
	case erabasic.ExtraSplit:
		return p.syntaxExtraSplit(i, head, base, wordEnd), true
	case erabasic.ExtraSwap:
		return p.syntaxExtraSwap(i, head, base, wordEnd), true
	default:
		return -1, false
	}
}

func (p *formatter) syntaxExtraTimes(i int, head []byte, base, wordEnd int) int {
	analysis := p.commandAnalysisContext()
	mark := len(p.edits)
	fail := func(offset int, message string) int {
		p.truncateEdits(mark)
		if p.expressionRulesEnabled() {
			p.syntaxProblem(i, offset, message)
		}
		p.syntaxOpaque(i)
		return -1
	}
	start, end, comment, ok := analysis.RegisteredBody(head, wordEnd)
	if !ok {
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	if !erabasic.RegisteredHasArgumentWithSpace(head, start, end, p.AllowFullwidthSpace) {
		return fail(base+wordEnd, "TIMES expects two arguments")
	}
	codeEnd := erabasic.RegisteredCodeEnd(base, end, comment)
	if erabasic.CommandFieldHasDeferredMarker(p.Src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	parts := analysis.SplitTopLevel(p.Src, base+start, codeEnd, ',')
	parts, overflow := erabasic.CommandPrefixParts(p.Src, parts, 2, 2, p.AllowFullwidthSpace, true)
	if len(parts) != 2 {
		return fail(base+start, "TIMES expects two arguments")
	}
	left, right := erabasic.ExtraFieldBoundsWithSpace(p.Src, parts[0], p.AllowFullwidthSpace)
	firstMarked := erabasic.CommandFieldHasMandatoryMarker(p.Src[left:right])
	firstVariable := analysis.VariableArgument(p.Src[left:right])
	if firstMarked {
		firstVariable = analysis.CommandFieldVariableArgument(left, right)
	}
	if left == right || !firstVariable {
		return fail(left, "TIMES requires a variable as its first argument")
	}
	if p.expressionParseEnabled() {
		if tokens, err := analysis.CommandFieldTokens(left, right); err == nil && !erabasic.CommandMacroPossibleForDialect(tokens, analysis.Dialect) {
			if !firstMarked && erabasic.BuiltinScalarIndexesMissing(tokens, p.Dialect) {
				return fail(left, "scalar variable requires indexes")
			}
			if erabasic.BuiltinVariableReadOnly(tokens, p.Dialect) {
				return fail(left, "TIMES requires a mutable variable as its first argument")
			}
			if erabasic.KnownExpressionTypeForDialect(tokens, p.Dialect) == erabasic.KnownValueString {
				return fail(left, "TIMES requires a numeric variable as its first argument")
			}
		}
	}
	coefficientLeft, coefficientRight := erabasic.ExtraFieldBoundsWithSpace(p.Src, parts[1], p.AllowFullwidthSpace)
	coefficientMarked := erabasic.CommandFieldHasMandatoryMarker(p.Src[coefficientLeft:coefficientRight])
	coefficient := p.Src[coefficientLeft:coefficientRight]
	if coefficientLeft == coefficientRight || (!erabasic.ValidTimesCoefficient(coefficient) && !erabasic.TimesCoefficientRuntimeWarning(coefficient) && !coefficientMarked) {
		return fail(coefficientLeft, "TIMES requires a decimal coefficient")
	}
	if erabasic.TimesCoefficientRuntimeWarning(coefficient) && !coefficientMarked {
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
	p.setCodeEnd(i, erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace))
	return codeEnd
}

func (p *formatter) syntaxExtraVariableCommand(i int, head []byte, base, wordEnd int, command string) int {
	analysis := p.commandAnalysisContext()
	mark := len(p.edits)
	fail := func(offset int, message string) int {
		p.truncateEdits(mark)
		if p.expressionRulesEnabled() {
			p.syntaxProblem(i, offset, message)
		}
		p.syntaxOpaque(i)
		return -1
	}
	start, end, comment, ok := analysis.RegisteredBody(head, wordEnd)
	if !ok {
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	codeEnd := erabasic.RegisteredCodeEnd(base, end, comment)
	if erabasic.CommandFieldHasDeferredMarker(p.Src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	if !erabasic.RegisteredHasArgumentWithSpace(head, start, end, p.AllowFullwidthSpace) {
		p.setCodeEnd(i, base+wordEnd)
		return base + wordEnd
	}
	parts := analysis.SplitTopLevel(p.Src, base+start, codeEnd, ',')
	parts, overflow := erabasic.CommandPrefixParts(p.Src, parts, 1, 1, p.AllowFullwidthSpace, true)
	if len(parts) != 1 {
		return fail(base+start, "command expects at most one variable argument")
	}
	left, right := erabasic.ExtraFieldBoundsWithSpace(p.Src, parts[0], p.AllowFullwidthSpace)
	variable := analysis.VariableArgument(p.Src[left:right])
	if erabasic.CommandFieldHasMandatoryMarker(p.Src[left:right]) {
		variable = analysis.CommandFieldVariableArgument(left, right)
	}
	if left == right || !variable {
		return fail(left, "command requires a variable argument")
	}
	if p.expressionParseEnabled() {
		tokens, err := analysis.CommandFieldTokens(left, right)
		if err == nil && !erabasic.CommandMacroPossibleForDialect(tokens, analysis.Dialect) {
			if !erabasic.CommandFieldHasMandatoryMarker(p.Src[left:right]) && erabasic.BuiltinScalarIndexesMissing(tokens, p.Dialect) {
				return fail(left, "scalar variable requires indexes")
			}
			expected := erabasic.KnownValueNumeric
			if command == "STRDATA" {
				expected = erabasic.KnownValueString
			}
			if erabasic.BuiltinVariableReadOnly(tokens, p.Dialect) {
				p.truncateEdits(mark)
				p.syntaxProblem(i, left, "command requires a mutable variable argument")
				p.syntaxOpaque(i)
				return -1
			}
			if !erabasic.CommandTypeMatches(expected, erabasic.KnownExpressionTypeForDialect(tokens, p.Dialect)) {
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
	p.setCodeEnd(i, erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace))
	return codeEnd
}

func (p *formatter) syntaxExtraSplit(i int, head []byte, base, wordEnd int) int {
	analysis := p.commandAnalysisContext()
	macroPrefix := false
	return p.syntaxExtraFields(i, head, base, wordEnd, 3, 4, "SPLIT", func(n int, left, right int) erabasic.ExtraFieldValidation {
		if left == right {
			return erabasic.ExtraFieldValidation{}
		}
		tokens, err := analysis.CommandFieldTokens(left, right)
		potentialMacro := err == nil && erabasic.CommandMacroPossibleForDialect(tokens, analysis.Dialect)
		valid := n < 2 || err == nil && erabasic.VariableArgumentTokens(tokens)
		if err == nil {
			if n != 2 && !macroPrefix && !potentialMacro && !erabasic.CommandFieldHasMandatoryMarker(p.Src[left:right]) && erabasic.BuiltinScalarIndexesMissing(tokens, analysis.Dialect) {
				valid = false
			}
			if n >= 2 && !macroPrefix && !potentialMacro && erabasic.BuiltinVariableReadOnly(tokens, analysis.Dialect) {
				valid = false
			}
			typeOfValue := erabasic.KnownExpressionTypeForDialect(tokens, analysis.Dialect)
			if n < 2 && typeOfValue == erabasic.KnownValueNumeric {
				valid = false
			}
			if n == 2 && typeOfValue == erabasic.KnownValueNumeric {
				valid = false
			}
			if n == 3 && typeOfValue == erabasic.KnownValueString {
				valid = false
			}
			if n == 2 {
				if known, array := erabasic.BuiltinArrayRole(tokens); known && !array {
					valid = false
				}
			}
		}
		if !valid {
			return erabasic.ExtraFieldValidation{Quiet: macroPrefix || potentialMacro}
		}
		macroPrefix = macroPrefix || potentialMacro
		return erabasic.ExtraFieldValidation{Valid: true}
	})
}

func (p *formatter) syntaxExtraSwap(i int, head []byte, base, wordEnd int) int {
	analysis := p.commandAnalysisContext()
	macroPrefix := false
	var destination erabasic.KnownValueType
	return p.syntaxExtraFields(i, head, base, wordEnd, 2, 2, "SWAP", func(n int, left, right int) erabasic.ExtraFieldValidation {
		if left >= right {
			return erabasic.ExtraFieldValidation{}
		}
		tokens, err := analysis.CommandFieldTokens(left, right)
		potentialMacro := err == nil && erabasic.CommandMacroPossibleForDialect(tokens, analysis.Dialect)
		valid := err == nil && erabasic.VariableArgumentTokens(tokens)
		if !valid {
			return erabasic.ExtraFieldValidation{Quiet: macroPrefix || potentialMacro}
		}
		if !potentialMacro && erabasic.BuiltinVariableReadOnly(tokens, analysis.Dialect) {
			return erabasic.ExtraFieldValidation{}
		}
		if !macroPrefix && !potentialMacro {
			if !erabasic.CommandFieldHasMandatoryMarker(p.Src[left:right]) && erabasic.BuiltinScalarIndexesMissing(tokens, analysis.Dialect) {
				return erabasic.ExtraFieldValidation{}
			}
			actual := erabasic.KnownExpressionTypeForDialect(tokens, analysis.Dialect)
			if n == 0 {
				destination = actual
			} else if !erabasic.CommandTypeMatches(destination, actual) {
				return erabasic.ExtraFieldValidation{}
			}
		}
		macroPrefix = macroPrefix || potentialMacro
		return erabasic.ExtraFieldValidation{Valid: true}
	})
}

func (p *formatter) syntaxExtraFields(i int, head []byte, base, wordEnd, min, max int, command string, valid func(int, int, int) erabasic.ExtraFieldValidation) int {
	analysis := p.commandAnalysisContext()
	mark := len(p.edits)
	fail := func(offset int, message string) int {
		p.truncateEdits(mark)
		if p.expressionRulesEnabled() {
			p.syntaxProblem(i, offset, message)
		}
		p.syntaxOpaque(i)
		return -1
	}
	start, end, comment, ok := analysis.RegisteredBody(head, wordEnd)
	if !ok {
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	if !erabasic.RegisteredHasArgumentWithSpace(head, start, end, p.AllowFullwidthSpace) {
		return fail(base+wordEnd, command+" expects command arguments")
	}
	codeEnd := erabasic.RegisteredCodeEnd(base, end, comment)
	if erabasic.CommandFieldHasDeferredMarker(p.Src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	parts := analysis.SplitTopLevel(p.Src, base+start, codeEnd, ',')
	parts, overflow := erabasic.CommandPrefixParts(p.Src, parts, max, min, p.AllowFullwidthSpace, true)
	if len(parts) < min {
		if len(parts) < min && analysis.CommandFieldsMayExpand(parts) {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
		return fail(base+start, command+" has an invalid argument count")
	}
	for n, field := range parts {
		left, right := erabasic.ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
		result := valid(n, left, right)
		if !result.Valid {
			if result.Quiet {
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
	p.setCodeEnd(i, erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace))
	return codeEnd
}
