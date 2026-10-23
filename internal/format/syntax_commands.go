package format

import (
	"bytes"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func (p *formatter) syntaxRegistered(i int, head []byte, base int, command string, wordEnd int) (int, bool) {
	spec, ok := erabasic.RegisteredCommandSpec(command, p.Dialect)
	if !ok {
		return -1, false
	}
	switch spec.Mode {
	case erabasic.RegisteredRaw:
		p.syntaxOpaque(i)
		return -1, true
	case erabasic.RegisteredNoArg:
		return p.syntaxRegisteredNoArg(i, head, base, wordEnd), true
	case erabasic.RegisteredForm:
		return p.syntaxRegisteredForm(i, head, base, wordEnd, spec), true
	case erabasic.RegisteredPrintV:
		return p.syntaxRegisteredPrintV(i, head, base, wordEnd), true
	case erabasic.RegisteredInputS:
		return p.syntaxRegisteredInputS(i, head, base, wordEnd), true
	case erabasic.RegisteredArraySort:
		return p.syntaxRegisteredArraySort(i, head, base, wordEnd, spec), true
	case erabasic.RegisteredSortChara:
		return p.syntaxRegisteredSortChara(i, head, base, wordEnd, spec), true
	case erabasic.RegisteredHTMLTagSplit:
		return p.syntaxRegisteredHTMLTagSplit(i, head, base, wordEnd, spec), true
	case erabasic.RegisteredVariable:
		return p.syntaxRegisteredVariable(i, head, base, wordEnd), true
	case erabasic.RegisteredDTColumnOptions:
		return p.syntaxRegisteredDTColumnOptions(i, head, base, wordEnd), true
	case erabasic.RegisteredFormAny:
		return p.syntaxRegisteredFormAny(i, head, base, wordEnd), true
	case erabasic.RegisteredBit:
		return p.syntaxBits(i, head, base, wordEnd), true
	case erabasic.RegisteredSetter:
		return p.syntaxSetters(i, head, base, wordEnd, command), true
	case erabasic.RegisteredFields, erabasic.RegisteredScalar, erabasic.RegisteredInput, erabasic.RegisteredTimedInput:
		if command == "SAVEVAR" {
			return p.syntaxSAVEVAR(i, head, base, wordEnd), true
		}
		return p.syntaxRegisteredNamedFields(i, head, base, wordEnd, command, spec), true
	default:
		return -1, true
	}
}

func (p *formatter) syntaxRegisteredFormAny(i int, head []byte, base, wordEnd int) int {
	start, end, ok := erabasic.RegisteredFormAnyBody(head, wordEnd, p.AllowFullwidthSpace)
	if !ok || start >= end {
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, base+wordEnd, "command requires a FORM argument")
		}
		p.syntaxOpaque(i)
		return -1
	}
	if p.rules[CommandSpacing] && head[wordEnd] != ';' {
		p.addEdit(i, base+wordEnd, base+start, []byte{' '}, CommandSpacing)
	}
	p.syntaxReturnForm(i, base+start, base+end)
	return -1
}

func (p *formatter) syntaxRegisteredVariable(i int, head []byte, base, wordEnd int) int {
	analysis := p.commandAnalysisContext()
	start, end, comment, ok := analysis.RegisteredBody(head, wordEnd)
	if !ok || !erabasic.RegisteredHasArgumentWithSpace(head, start, end, p.AllowFullwidthSpace) {
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, base+wordEnd, "VARSIZE requires an array identifier")
		}
		p.syntaxOpaque(i)
		return -1
	}
	codeEnd := erabasic.RegisteredCodeEnd(base, end, comment)
	if erabasic.CommandFieldHasDeferredMarker(p.Src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	if erabasic.CommandFieldHasMandatoryMarker(p.Src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	left := erabasic.TrimInputLeft(p.Src, base+start, codeEnd, p.AllowFullwidthSpace)
	right := erabasic.TrimInputRight(p.Src, left, codeEnd, p.AllowFullwidthSpace)
	session, offset := analysis.ExpressionSessionFor(left, right)
	prefixEnd, prefixOK := erabasic.LabelIdentifierEndWithSession(p.Src[left:right], 0, session)
	prefixEnd += left
	if !prefixOK {
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, left, "VARSIZE requires a bare array identifier")
		}
		p.syntaxOpaque(i)
		return -1
	}
	tokens, err := session.LexRange(erabasic.ExpressionSourceOriginal, offset, offset+prefixEnd-left, left)
	if err != nil || len(tokens) != 1 || tokens[0].Kind != erabasic.ExprIdentifier {
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, left, "VARSIZE requires a bare array identifier")
		}
		p.syntaxOpaque(i)
		return -1
	}
	info, known := erabasic.BuiltinVariableInfoForDialect(tokens, analysis.Dialect)
	if p.expressionParseEnabled() {
		if known && (info.Rank == 0 || string(tokens[0].Text) == "RAND") {
			p.syntaxProblem(i, left, "VARSIZE requires an array identifier")
			p.syntaxOpaque(i)
			return -1
		}
	}
	if prefixEnd != right {
		if known && info.Rank > 0 && string(tokens[0].Text) != "RAND" && p.expressionParseEnabled() {
			p.ignoredCommandArguments(i, codeEnd, "VARSIZE")
		}
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	p.setCodeEnd(i, right)
	return right
}

func (p *formatter) syntaxRegisteredDTColumnOptions(i int, head []byte, base, wordEnd int) int {
	analysis := p.commandAnalysisContext()
	mark := len(p.edits)
	fail := func(offset int, message string) int {
		p.truncateEdits(mark)
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, offset, message)
		}
		p.syntaxOpaque(i)
		return -1
	}
	start, end, comment, ok := analysis.RegisteredBody(head, wordEnd)
	if !ok || !erabasic.RegisteredHasArgumentWithSpace(head, start, end, p.AllowFullwidthSpace) {
		return fail(base+wordEnd, "DT_COLUMN_OPTIONS requires arguments")
	}
	codeEnd := erabasic.RegisteredCodeEnd(base, end, comment)
	parts := analysis.SplitTopLevel(p.Src, base+start, codeEnd, ',')
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		left := erabasic.TrimInputLeft(p.Src, last.Start, last.End, p.AllowFullwidthSpace)
		right := erabasic.TrimInputRight(p.Src, left, last.End, p.AllowFullwidthSpace)
		if left == right {
			parts = parts[:len(parts)-1]
		}
	}
	if len(parts) < 4 || (len(parts)-2)%2 != 0 {
		return fail(base+start, "DT_COLUMN_OPTIONS requires column options")
	}
	for n, field := range parts {
		left := erabasic.TrimInputLeft(p.Src, field.Start, field.End, p.AllowFullwidthSpace)
		right := erabasic.TrimInputRight(p.Src, left, field.End, p.AllowFullwidthSpace)
		if left == right {
			return fail(field.Start, "empty DT_COLUMN_OPTIONS argument")
		}
		if n >= 2 && n%2 == 0 {
			if !bytes.EqualFold(p.Src[left:right], []byte("DEFAULT")) {
				return fail(left, "DT_COLUMN_OPTIONS has an unknown option")
			}
			continue
		}
		if p.expressionParseEnabled() && !p.syntaxExpression(i, left, right, false) {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	p.syntaxRegisteredCommaGaps(i, parts, codeEnd)
	codeEnd = erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace)
	p.setCodeEnd(i, codeEnd)
	return codeEnd
}

func (p *formatter) syntaxRegisteredNoArg(i int, head []byte, base, wordEnd int) int {
	if wordEnd >= len(head) {
		p.setCodeEnd(i, base+wordEnd)
		return base + wordEnd
	}
	if head[wordEnd] == ';' {
		p.syntaxOpaque(i)
		return -1
	}
	if !p.inputSpaceIn(head, wordEnd) {
		p.syntaxOpaque(i)
		return -1
	}
	tail := wordEnd + erabasic.InputSpaceSize(head, wordEnd, p.AllowFullwidthSpace)
	for tail < len(head) {
		size := erabasic.InputSpaceSize(head, tail, p.AllowFullwidthSpace)
		if size == 0 {
			break
		}
		tail += size
	}
	if tail < len(head) && head[tail] != ';' {
		p.syntaxOpaque(i)
		return -1
	}
	p.setCodeEnd(i, base+wordEnd)
	return base + wordEnd
}

func (p *formatter) syntaxRegisteredForm(i int, head []byte, base, wordEnd int, spec erabasic.RegisteredSpec) int {
	if spec.Min > 0 && wordEnd+max(1, erabasic.InputSpaceSize(head, wordEnd, p.AllowFullwidthSpace)) >= len(head) {
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, base+wordEnd, "missing FORM argument")
		}
		p.syntaxOpaque(i)
		return -1
	}
	if wordEnd >= len(head) {
		return base + wordEnd
	}
	if !p.inputSpaceIn(head, wordEnd) && head[wordEnd] != ';' {
		p.syntaxOpaque(i)
		return -1
	}
	start := base + wordEnd + erabasic.InputSpaceSize(head, wordEnd, p.AllowFullwidthSpace)
	if start == base+wordEnd {
		start++
	}
	if !p.expressionRulesEnabled() {
		return -1
	}
	if !p.syntaxForm(i, start, base+len(head), erabasic.FormLine) {
		p.syntaxOpaque(i)
	}
	return -1
}

func (p *formatter) syntaxRegisteredPrintV(i int, head []byte, base, wordEnd int) int {
	if wordEnd >= len(head) {
		return -1
	}
	if !p.inputSpaceIn(head, wordEnd) {
		p.syntaxOpaque(i)
		return -1
	}
	start := wordEnd + erabasic.InputSpaceSize(head, wordEnd, p.AllowFullwidthSpace)
	arg := erabasic.TrimInputLeft(head, start, len(head), p.AllowFullwidthSpace)
	if p.rules[CommandSpacing] && arg < len(head) && head[arg] != ';' {
		p.addEdit(i, base+wordEnd, base+arg, []byte{' '}, CommandSpacing)
	}
	if arg >= len(head) || head[arg] == ';' {
		return -1
	}
	if p.expressionParseEnabled() && !p.syntaxPrintV(i, base+start, base+len(head)) {
		p.syntaxOpaque(i)
	}
	return -1
}

func (p *formatter) syntaxRegisteredNamedFields(i int, head []byte, base, wordEnd int, command string, spec erabasic.RegisteredSpec) int {
	analysis := p.commandAnalysisContext()
	mark := len(p.edits)
	start, end, comment, ok := analysis.RegisteredBody(head, wordEnd)
	if !ok {
		p.syntaxOpaque(i)
		return -1
	}
	if !erabasic.RegisteredHasArgumentWithSpace(head, start, end, p.AllowFullwidthSpace) {
		if spec.Min == 0 {
			p.setCodeEnd(i, base+wordEnd)
			return base + wordEnd
		}
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, base+wordEnd, "missing command argument")
		}
		p.syntaxOpaque(i)
		return -1
	}
	arg := base + start
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	codeEnd := base + comment
	if comment < 0 {
		codeEnd = base + end
	}
	if erabasic.CommandFieldHasDeferredMarker(p.Src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	parts := analysis.SplitTopLevel(p.Src, arg, codeEnd, ',')
	if !p.expressionParseEnabled() {
		p.setCodeEnd(i, erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace))
		return erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace)
	}
	max := int(spec.Max)
	if p.Dialect == erabasic.Emuera1824 {
		switch spec.Mode {
		case erabasic.RegisteredInput:
			max = 1
		case erabasic.RegisteredTimedInput:
			max = 4
		}
	}
	if command == "HTML_PRINT" && p.Dialect == erabasic.Emuera1824 {
		max = 1
	}
	allowTrailing := spec.Mode == erabasic.RegisteredFields || spec.Mode == erabasic.RegisteredTimedInput
	if spec.Unbounded {
		max = len(parts)
	}
	parts, overflow := erabasic.CommandPrefixParts(p.Src, parts, max, int(spec.Min), p.AllowFullwidthSpace, allowTrailing)
	if spec.Mode == erabasic.RegisteredScalar && len(parts) < 1 && spec.Min > 0 {
		p.syntaxProblem(i, arg, "expected one command argument")
		p.syntaxOpaque(i)
		return -1
	}
	if command == "SETCOLOR" || command == "SETBGCOLOR" {
		if len(parts) != 1 && len(parts) != 3 {
			if len(parts) < 3 && analysis.CommandFieldsMayExpand(parts) {
				p.truncateEdits(mark)
				p.syntaxOpaque(i)
				return -1
			}
			p.syntaxProblem(i, arg, "expected one or three command arguments")
			p.syntaxOpaque(i)
			return -1
		}
	}
	if len(parts) < int(spec.Min) {
		if analysis.CommandFieldsMayExpand(parts) {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
		p.syntaxProblem(i, arg, "too few command arguments")
		p.syntaxOpaque(i)
		return -1
	}
	roles := erabasic.CommandRoles{}
	saveCharaIDs := make(map[int64]struct{})
	for n, field := range parts {
		left := erabasic.TrimInputLeft(p.Src, field.Start, field.End, p.AllowFullwidthSpace)
		right := erabasic.TrimInputRight(p.Src, left, field.End, p.AllowFullwidthSpace)
		if left == right {
			if spec.Mode != erabasic.RegisteredInput {
				p.truncateEdits(mark)
				if p.expressionParseEnabled() {
					p.syntaxProblem(i, field.Start, "empty command argument")
				}
				p.syntaxOpaque(i)
				return -1
			}
			continue
		}
		if !analysis.CommandFieldTypeValid(command, n, left, right, &roles) {
			p.truncateEdits(mark)
			p.syntaxProblem(i, left, "command argument has an incompatible type")
			p.syntaxOpaque(i)
			return -1
		}
		if (command == "SETCOLOR" || command == "SETBGCOLOR") && analysis.CommandFieldHasUnpairedTernaryHashAt(left, right) {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
		if command == "SAVECHARA" && n >= 2 && !roles.Macro {
			if tokens, err := analysis.CommandFieldTokens(left, right); err == nil && !erabasic.CommandMacroPossibleForDialect(tokens, analysis.Dialect) {
				if value, kind := erabasic.DirectIntegerLiteral(tokens); kind == erabasic.KnownValueNumeric {
					if value < 0 || value > 2147483647 {
						p.truncateEdits(mark)
						p.syntaxProblem(i, left, "SAVECHARA character ID is outside 0..2147483647")
						p.syntaxOpaque(i)
						return -1
					}
					if _, duplicate := saveCharaIDs[value]; duplicate {
						p.truncateEdits(mark)
						p.syntaxProblem(i, left, "SAVECHARA contains a duplicate character ID")
						p.syntaxOpaque(i)
						return -1
					}
					saveCharaIDs[value] = struct{}{}
				}
			}
		}
		fieldValid := analysis.RegisteredFieldValid(command, n, left, right)
		if n == 0 && erabasic.CommandFieldHasMandatoryMarker(p.Src[left:right]) && (command == "ARRAYSHIFT" || command == "ARRAYREMOVE") {
			fieldValid = analysis.CommandFieldVariableArgument(left, right)
			if command == "ARRAYSHIFT" {
				if tokens, err := analysis.CommandFieldTokens(left, right); err == nil {
					if info, known := erabasic.BuiltinVariableRoleInfo(tokens, analysis.Dialect); known && info.Rank != 1 {
						fieldValid = false
					}
				}
			}
		}
		if !fieldValid {
			p.truncateEdits(mark)
			if p.expressionParseEnabled() {
				p.syntaxProblem(i, left, "invalid command variable argument")
			}
			p.syntaxOpaque(i)
			return -1
		}
		valid := false
		if command == "PRINTFORMSN" && n == 0 {
			valid = p.syntaxPrintFormsLiteral(i, left, right)
		} else {
			valid = p.syntaxOrdinaryExpression(i, left, right, false)
		}
		if !valid {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
	}
	if overflow && p.expressionParseEnabled() {
		p.ignoredCommandArguments(i, codeEnd, command)
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommaGaps(i, parts, codeEnd)
	codeEnd = erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace)
	p.setCodeEnd(i, codeEnd)
	return codeEnd
}

func (p *formatter) syntaxSAVEVAR(i int, head []byte, base, wordEnd int) int {
	analysis := p.commandAnalysisContext()
	start, end, comment, ok := analysis.RegisteredBody(head, wordEnd)
	if !ok {
		p.syntaxOpaque(i)
		return -1
	}
	if !erabasic.RegisteredHasArgumentWithSpace(head, start, end, p.AllowFullwidthSpace) {
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, base+wordEnd, "SAVEVAR requires arguments")
		}
		p.syntaxOpaque(i)
		return -1
	}
	codeEnd := erabasic.RegisteredCodeEnd(base, end, comment)
	if erabasic.CommandFieldHasDeferredMarker(p.Src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	arg := base + start
	if bytes.Contains(p.Src[arg:codeEnd], []byte("[[")) {
		p.syntaxOpaque(i)
		return -1
	}
	parts := analysis.SplitTopLevel(p.Src, arg, codeEnd, ',')
	fields := parts
	if len(fields) > 1 {
		last := fields[len(fields)-1]
		if p.trimInputLeft(last.Start, last.End) >= p.trimInputRight(last.Start, last.End) {
			fields = fields[:len(fields)-1]
		}
	}
	if !p.expressionParseEnabled() {
		p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
		codeEnd = erabasic.TrimInputRight(p.Src, arg, codeEnd, p.AllowFullwidthSpace)
		p.setCodeEnd(i, codeEnd)
		return codeEnd
	}
	mark := len(p.edits)
	fail := func(offset int, message string) int {
		p.truncateEdits(mark)
		p.syntaxProblem(i, offset, message)
		p.syntaxOpaque(i)
		return -1
	}
	macroPrefix := false
	saveVarIDs := make(map[string]struct{})
	for n, field := range fields {
		left := p.trimInputLeft(field.Start, field.End)
		right := p.trimInputRight(left, field.End)
		if left == right {
			return fail(field.Start, "empty SAVEVAR argument")
		}
		tokens, err := analysis.LexSyntaxRange(left, right)
		if err != nil {
			p.truncateEdits(mark)
			p.expressionProblem(i, 0, err)
			p.syntaxOpaque(i)
			return -1
		}
		tokens = erabasic.StripCommandMarkers(tokens)
		potentialMacro := erabasic.SetterMacroPossibleForDialect(tokens, analysis.Dialect)
		if !macroPrefix && !potentialMacro {
			if n < 2 {
				if erabasic.KnownExpressionTypeForDialect(tokens, analysis.Dialect) == erabasic.KnownValueNumeric {
					return fail(left, "SAVEVAR requires string arguments")
				}
			} else {
				if !erabasic.VariableArgumentTokens(tokens) {
					return fail(left, "SAVEVAR requires a variable argument")
				}
				if erabasic.SavevarBuiltinTargetForbidden(tokens, analysis.Dialect) {
					return fail(left, "SAVEVAR cannot save a local or calculated variable")
				}
				if erabasic.BuiltinVariableReadOnly(tokens, analysis.Dialect) {
					return fail(left, "SAVEVAR cannot save a read-only variable")
				}
				if name, exact := erabasic.VariableSystemName(tokens); exact {
					if info, known := erabasic.BuiltinVariableInfoByName(name, analysis.Dialect); known && info.Character {
						return fail(left, "SAVEVAR does not accept character variables")
					}
				}
				if info, known := erabasic.BuiltinVariableInfoForDialect(tokens, analysis.Dialect); known && info.Character {
					return fail(left, "SAVEVAR does not accept character variables")
				}
				if identity, proven := erabasic.BuiltinVariableIdentity(tokens, analysis.Dialect); proven {
					if _, duplicate := saveVarIDs[identity]; duplicate {
						return fail(left, "SAVEVAR contains a duplicate variable")
					}
					saveVarIDs[identity] = struct{}{}
				}
			}
		}
		if !p.syntaxExpression(i, left, right, false) {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
		macroPrefix = macroPrefix || potentialMacro
	}
	if len(fields) < 3 && !macroPrefix {
		return fail(arg, "SAVEVAR requires two string arguments and a variable")
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	p.syntaxRegisteredCommaGaps(i, parts, codeEnd)
	codeEnd = erabasic.TrimInputRight(p.Src, arg, codeEnd, p.AllowFullwidthSpace)
	p.setCodeEnd(i, codeEnd)
	return codeEnd
}

func (p *formatter) syntaxRegisteredInputS(i int, head []byte, base, wordEnd int) int {
	analysis := p.commandAnalysisContext()
	start, end, ok := erabasic.RegisteredRawBodyWithSpace(head, wordEnd, p.AllowFullwidthSpace)
	if !ok {
		p.syntaxOpaque(i)
		return -1
	}
	if wordEnd == len(head) {
		p.setCodeEnd(i, base+wordEnd)
		return base + wordEnd
	}
	if erabasic.CommandFieldHasDeferredMarker(head[start:end]) {
		p.syntaxOpaque(i)
		return -1
	}
	if p.Dialect == erabasic.EMEE {
		mark := len(p.edits)
		bodyStart := base + start
		session, offset := analysis.ExpressionSessionFor(bodyStart, base+end)
		formEnd, err := session.ScanFormAt(erabasic.ExpressionSourceOriginal, offset, offset+base+end-bodyStart, erabasic.FormComma)
		n := formEnd - offset
		if err != nil {
			p.truncateEdits(mark)
			if p.expressionParseEnabled() {
				p.expressionProblem(i, bodyStart, err)
			}
			p.syntaxOpaque(i)
			return -1
		}
		firstEnd := bodyStart + n
		if p.expressionRulesEnabled() && !p.syntaxForm(i, bodyStart, firstEnd, erabasic.FormComma) {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
		if firstEnd < base+end {
			if p.Src[firstEnd] != ',' {
				p.syntaxOpaque(i)
				return -1
			}
			parts := analysis.SplitTopLevel(p.Src, firstEnd+1, base+end, ',')
			parts, overflow := erabasic.CommandPrefixParts(p.Src, parts, 2, 0, p.AllowFullwidthSpace, true)
			for _, field := range parts {
				left := erabasic.TrimInputLeft(p.Src, field.Start, field.End, p.AllowFullwidthSpace)
				right := erabasic.TrimInputRight(p.Src, left, field.End, p.AllowFullwidthSpace)
				if left == right {
					continue
				}
				if erabasic.CommandFieldHasDeferredMarker(p.Src[left:right]) {
					p.truncateEdits(mark)
					p.syntaxOpaque(i)
					return -1
				}
				if !p.syntaxExpression(i, left, right, false) {
					p.truncateEdits(mark)
					p.syntaxOpaque(i)
					return -1
				}
			}
			if overflow {
				if p.expressionParseEnabled() {
					p.ignoredCommandArguments(i, base+end, "INPUTS")
				}
				p.truncateEdits(mark)
				p.syntaxOpaque(i)
				return -1
			}
			p.syntaxRegisteredCommaGap(i, firstEnd, base+end, firstEnd)
			p.syntaxRegisteredCommaGaps(i, parts, base+end)
		}
		return -1
	}
	if p.Dialect == erabasic.CommonDialect {
		bodyStart, bodyEnd := base+start, base+end
		session, offset := analysis.ExpressionSessionFor(bodyStart, bodyEnd)
		formEnd, err := session.ScanFormAt(erabasic.ExpressionSourceOriginal, offset, offset+bodyEnd-bodyStart, erabasic.FormComma)
		n := formEnd - offset
		if err == nil && n < bodyEnd-bodyStart && p.Src[bodyStart+n] == ',' {
			if p.expressionParseEnabled() {
				p.syntaxProblem(i, base+start, "INPUTS comma-separated arguments require a runtime dialect")
			}
			p.syntaxOpaque(i)
			return -1
		}
	}
	if p.expressionRulesEnabled() && !p.syntaxForm(i, base+start, base+end, erabasic.FormLine) {
		p.syntaxOpaque(i)
	}
	return -1
}

func (p *formatter) syntaxRegisteredArraySort(i int, head []byte, base, wordEnd int, spec erabasic.RegisteredSpec) int {
	analysis := p.commandAnalysisContext()
	mark := len(p.edits)
	start, end, comment, ok := analysis.RegisteredBody(head, wordEnd)
	if !ok {
		p.syntaxOpaque(i)
		return -1
	}
	if !erabasic.RegisteredHasArgumentWithSpace(head, start, end, p.AllowFullwidthSpace) {
		if spec.Min == 0 {
			p.setCodeEnd(i, base+wordEnd)
			return base + wordEnd
		}
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, base+wordEnd, "missing command argument")
		}
		p.syntaxOpaque(i)
		return -1
	}
	codeEnd := erabasic.RegisteredCodeEnd(base, end, comment)
	if erabasic.CommandFieldHasDeferredMarker(p.Src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	if !p.expressionParseEnabled() {
		codeEnd = erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace)
		p.setCodeEnd(i, codeEnd)
		return codeEnd
	}
	parts := analysis.SplitTopLevel(p.Src, base+start, codeEnd, ',')
	parts, overflow := erabasic.CommandPrefixParts(p.Src, parts, int(spec.Max), int(spec.Min), p.AllowFullwidthSpace, true)
	if len(parts) > 1 {
		first := parts[0]
		firstLeft := erabasic.TrimInputLeft(p.Src, first.Start, first.End, p.AllowFullwidthSpace)
		firstRight := erabasic.TrimInputRight(p.Src, firstLeft, first.End, p.AllowFullwidthSpace)
		second := parts[1]
		left := erabasic.TrimInputLeft(p.Src, second.Start, second.End, p.AllowFullwidthSpace)
		right := erabasic.TrimInputRight(p.Src, left, second.End, p.AllowFullwidthSpace)
		if !analysis.CommandFieldMayExpand(firstLeft, firstRight) && !analysis.CommandFieldMayExpand(left, right) && !analysis.CommandFieldKeyword(left, right, "FORWARD") && !analysis.CommandFieldKeyword(left, right, "BACK") {
			if p.expressionParseEnabled() {
				p.syntaxProblem(i, left, "array sort range requires FORWARD or BACK")
			}
			p.syntaxOpaque(i)
			return -1
		}
	}
	macroPrefix := false
	directionKeyword := false
	for n, field := range parts {
		left := erabasic.TrimInputLeft(p.Src, field.Start, field.End, p.AllowFullwidthSpace)
		right := erabasic.TrimInputRight(p.Src, left, field.End, p.AllowFullwidthSpace)
		if left == right {
			p.truncateEdits(mark)
			if p.expressionParseEnabled() {
				p.syntaxProblem(i, field.Start, "empty command argument")
			}
			p.syntaxOpaque(i)
			return -1
		}
		fieldMayExpand := analysis.CommandFieldMayExpand(left, right)
		if n == 1 {
			keyword := analysis.CommandFieldKeyword(left, right, "FORWARD") || analysis.CommandFieldKeyword(left, right, "BACK")
			if !keyword && !fieldMayExpand {
				p.truncateEdits(mark)
				p.syntaxOpaque(i)
				return -1
			}
			directionKeyword = keyword && !macroPrefix
			macroPrefix = macroPrefix || fieldMayExpand
			continue
		}
		if n == 0 && !commandFieldVariableArgument(p, left, right) {
			if fieldMayExpand {
				p.truncateEdits(mark)
				p.syntaxOpaque(i)
				return -1
			}
			p.truncateEdits(mark)
			if p.expressionParseEnabled() {
				p.syntaxProblem(i, left, "invalid array variable argument")
			}
			p.syntaxOpaque(i)
			return -1
		}
		if n == 0 && !fieldMayExpand {
			if tokens, err := analysis.CommandFieldTokens(left, right); err == nil {
				if erabasic.BuiltinVariableReadOnly(tokens, analysis.Dialect) {
					p.truncateEdits(mark)
					p.syntaxProblem(i, left, "ARRAYSORT requires a mutable array variable")
					p.syntaxOpaque(i)
					return -1
				}
				if info, known := erabasic.BuiltinVariableRoleInfo(tokens, analysis.Dialect); known && info.Rank != 1 {
					p.truncateEdits(mark)
					p.syntaxProblem(i, left, "invalid array variable argument")
					p.syntaxOpaque(i)
					return -1
				}
			}
		}
		if n >= 2 && !analysis.CommandRawTypeValid(erabasic.KnownValueNumeric, left, right) {
			p.truncateEdits(mark)
			if directionKeyword && !fieldMayExpand {
				if tokens, err := analysis.CommandFieldTokens(left, right); err == nil && erabasic.BuiltinScalarIndexesMissing(tokens, analysis.Dialect) {
					p.syntaxProblem(i, left, "scalar variable requires indexes")
					p.syntaxOpaque(i)
					return -1
				}
			}
			if macroPrefix || fieldMayExpand {
				p.syntaxOpaque(i)
				return -1
			}
			p.syntaxProblem(i, left, "invalid ARRAYSORT argument type")
			p.syntaxOpaque(i)
			return -1
		}
		macroPrefix = macroPrefix || fieldMayExpand
		if macroPrefix && n >= 2 {
			tokens, err := analysis.LexSyntaxRange(left, right)
			if err == nil && erabasic.ConstantExpressionsValid(tokens) != nil {
				p.truncateEdits(mark)
				p.syntaxOpaque(i)
				return -1
			}
		}
		if !p.syntaxExpression(i, left, right, false) {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
	}
	if overflow && p.expressionParseEnabled() {
		p.ignoredCommandArguments(i, codeEnd, "ARRAYSORT")
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommaGaps(i, parts, codeEnd)
	p.setCodeEnd(i, erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace))
	return erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace)
}

func (p *formatter) syntaxRegisteredSortChara(i int, head []byte, base, wordEnd int, spec erabasic.RegisteredSpec) int {
	analysis := p.commandAnalysisContext()
	mark := len(p.edits)
	start, end, comment, ok := analysis.RegisteredBody(head, wordEnd)
	if !ok {
		p.syntaxOpaque(i)
		return -1
	}
	if !erabasic.RegisteredHasArgumentWithSpace(head, start, end, p.AllowFullwidthSpace) {
		p.setCodeEnd(i, base+wordEnd)
		return base + wordEnd
	}
	codeEnd := erabasic.RegisteredCodeEnd(base, end, comment)
	if erabasic.CommandFieldHasDeferredMarker(p.Src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	if !p.expressionParseEnabled() {
		codeEnd = erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace)
		p.setCodeEnd(i, codeEnd)
		return codeEnd
	}
	parts := analysis.SplitTopLevel(p.Src, base+start, codeEnd, ',')
	parts, overflow := erabasic.CommandPrefixParts(p.Src, parts, int(spec.Max), int(spec.Min), p.AllowFullwidthSpace, true)
	macroPrefix := false
	for n, field := range parts {
		left := erabasic.TrimInputLeft(p.Src, field.Start, field.End, p.AllowFullwidthSpace)
		right := erabasic.TrimInputRight(p.Src, left, field.End, p.AllowFullwidthSpace)
		if left == right {
			p.truncateEdits(mark)
			if p.expressionParseEnabled() {
				p.syntaxProblem(i, field.Start, "empty command argument")
			}
			p.syntaxOpaque(i)
			return -1
		}
		fieldMayExpand := analysis.CommandFieldMayExpand(left, right)
		if (n == len(parts)-1 || len(parts) == 1) && (analysis.CommandFieldKeyword(left, right, "FORWARD") || analysis.CommandFieldKeyword(left, right, "BACK")) {
			continue
		}
		if len(parts) == 2 && n == 1 && !analysis.CommandFieldKeyword(left, right, "FORWARD") && !analysis.CommandFieldKeyword(left, right, "BACK") && !fieldMayExpand {
			p.truncateEdits(mark)
			if macroPrefix {
				p.syntaxOpaque(i)
				return -1
			}
			if p.expressionParseEnabled() {
				p.syntaxProblem(i, left, "SORTCHARA requires FORWARD or BACK")
			}
			p.syntaxOpaque(i)
			return -1
		}
		if n == 0 && (len(parts) == 1 || len(parts) == 2) && !analysis.CommandFieldVariableArgument(left, right) {
			p.truncateEdits(mark)
			if fieldMayExpand {
				p.syntaxOpaque(i)
				return -1
			}
			if p.expressionParseEnabled() {
				p.syntaxProblem(i, left, "invalid SORTCHARA variable argument")
			}
			p.syntaxOpaque(i)
			return -1
		}
		if n == 0 {
			if tokens, err := analysis.CommandFieldTokens(left, right); err == nil && !erabasic.CommandMacroPossibleForDialect(tokens, analysis.Dialect) {
				if erabasic.BuiltinVariableReadOnly(tokens, analysis.Dialect) {
					p.truncateEdits(mark)
					p.syntaxProblem(i, left, "SORTCHARA requires a mutable character variable")
					p.syntaxOpaque(i)
					return -1
				}
				if info, known := erabasic.BuiltinVariableRoleInfo(tokens, analysis.Dialect); known && !info.Character {
					p.truncateEdits(mark)
					p.syntaxProblem(i, left, "invalid SORTCHARA argument type")
					p.syntaxOpaque(i)
					return -1
				}
			}
		}
		rawRoles := erabasic.CommandRoles{}
		if !analysis.CommandFieldTypeValid("SORTCHARA", n, left, right, &rawRoles) {
			p.truncateEdits(mark)
			if macroPrefix || fieldMayExpand {
				p.syntaxOpaque(i)
				return -1
			}
			p.syntaxProblem(i, left, "invalid SORTCHARA argument type")
			p.syntaxOpaque(i)
			return -1
		}
		macroPrefix = macroPrefix || fieldMayExpand
		if !p.syntaxExpression(i, left, right, false) {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
	}
	if overflow && p.expressionParseEnabled() {
		p.ignoredCommandArguments(i, codeEnd, "SORTCHARA")
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommaGaps(i, parts, codeEnd)
	p.setCodeEnd(i, erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace))
	return erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace)
}

func (p *formatter) syntaxRegisteredHTMLTagSplit(i int, head []byte, base, wordEnd int, spec erabasic.RegisteredSpec) int {
	analysis := p.commandAnalysisContext()
	mark := len(p.edits)
	start, end, comment, ok := analysis.RegisteredBody(head, wordEnd)
	if !ok || !erabasic.RegisteredHasArgumentWithSpace(head, start, end, p.AllowFullwidthSpace) {
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, base+wordEnd, "missing HTML_TAGSPLIT argument")
		}
		p.syntaxOpaque(i)
		return -1
	}
	codeEnd := erabasic.RegisteredCodeEnd(base, end, comment)
	if erabasic.CommandFieldHasDeferredMarker(p.Src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	if !p.expressionParseEnabled() {
		codeEnd = erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace)
		p.setCodeEnd(i, codeEnd)
		return codeEnd
	}
	parts := analysis.SplitTopLevel(p.Src, base+start, codeEnd, ',')
	parts, overflow := erabasic.CommandPrefixParts(p.Src, parts, int(spec.Max), int(spec.Min), p.AllowFullwidthSpace, true)
	if len(parts) < int(spec.Min) {
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, base+start, "HTML_TAGSPLIT expects one to three arguments")
		}
		p.syntaxOpaque(i)
		return -1
	}
	macroPrefix := false
	for n, field := range parts {
		left := erabasic.TrimInputLeft(p.Src, field.Start, field.End, p.AllowFullwidthSpace)
		right := erabasic.TrimInputRight(p.Src, left, field.End, p.AllowFullwidthSpace)
		if left == right {
			p.truncateEdits(mark)
			if p.expressionParseEnabled() {
				p.syntaxProblem(i, left, "invalid HTML_TAGSPLIT argument")
			}
			p.syntaxOpaque(i)
			return -1
		}
		fieldMayExpand := analysis.CommandFieldMayExpand(left, right)
		if n > 0 && !analysis.CommandFieldVariableArgument(left, right) {
			p.truncateEdits(mark)
			if macroPrefix || fieldMayExpand {
				p.syntaxOpaque(i)
				return -1
			}
			if p.expressionParseEnabled() {
				p.syntaxProblem(i, left, "invalid HTML_TAGSPLIT argument")
			}
			p.syntaxOpaque(i)
			return -1
		}
		rawRoles := erabasic.CommandRoles{}
		if !analysis.CommandFieldTypeValid("HTML_TAGSPLIT", n, left, right, &rawRoles) {
			p.truncateEdits(mark)
			if macroPrefix || fieldMayExpand {
				p.syntaxOpaque(i)
				return -1
			}
			p.syntaxProblem(i, left, "invalid HTML_TAGSPLIT argument type")
			p.syntaxOpaque(i)
			return -1
		}
		macroPrefix = macroPrefix || fieldMayExpand
		if !p.syntaxExpression(i, left, right, false) {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
	}
	if overflow && p.expressionParseEnabled() {
		p.ignoredCommandArguments(i, codeEnd, "HTML_TAGSPLIT")
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommaGaps(i, parts, codeEnd)
	p.setCodeEnd(i, erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace))
	return erabasic.TrimInputRight(p.Src, base+start, codeEnd, p.AllowFullwidthSpace)
}

func (p *formatter) syntaxRegisteredCommaGap(i, comma, end, leftBound int) {
	if !p.rules[CommaSpacing] {
		return
	}
	left := erabasic.TrimInputRight(p.Src, leftBound, comma, p.AllowFullwidthSpace)
	right := erabasic.TrimInputLeft(p.Src, comma+1, end, p.AllowFullwidthSpace)
	if erabasic.MandatoryInlineMarker(p.Src[:left], left-3) {
		left = comma
	}
	if erabasic.MandatoryInlineMarker(p.Src, right) {
		p.addEdit(i, left, comma, nil, CommaSpacing)
		return
	}
	if right < end && p.Src[right] != ',' && p.Src[right] != ')' && p.Src[right] != ']' {
		p.addEdit(i, left, right, []byte{',', ' '}, CommaSpacing)
	} else if left < comma {
		p.addEdit(i, left, comma, nil, CommaSpacing)
	}
}

func (p *formatter) syntaxRegisteredCommandGap(i int, head []byte, base, wordEnd, start, end int) {
	if !p.rules[CommandSpacing] {
		return
	}
	first := erabasic.TrimInputLeft(head, start, end, p.AllowFullwidthSpace)
	if first < end && head[first] != ';' && head[first] != ',' {
		p.addEdit(i, base+wordEnd, base+first, []byte{' '}, CommandSpacing)
	}
}

func (p *formatter) syntaxRegisteredCommaGaps(i int, parts []erabasic.SyntaxSpan, end int) {
	for n := 0; n+1 < len(parts); n++ {
		comma := parts[n].End
		for comma < parts[n+1].Start && p.Src[comma] != ',' {
			comma++
		}
		if comma >= parts[n+1].Start || p.Src[comma] != ',' {
			continue
		}
		leftBound := parts[n].Start
		fieldLeft := erabasic.TrimInputLeft(p.Src, parts[n].Start, parts[n].End, p.AllowFullwidthSpace)
		fieldRight := erabasic.TrimInputRight(p.Src, fieldLeft, parts[n].End, p.AllowFullwidthSpace)
		if fieldLeft >= fieldRight {
			leftBound = comma
		}
		p.syntaxRegisteredCommaGap(i, comma, end, leftBound)
	}
}

func (p *formatter) ignoredCommandArguments(i, offset int, command string) {
	p.warning(i, offset, "ignored_arguments", command+" has runtime-ignored arguments")
}
