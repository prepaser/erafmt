package format

import "bytes"

type registeredMode uint8

const (
	registeredScalar registeredMode = iota
	registeredFields
	registeredForm
	registeredPrintV
	registeredRaw
	registeredNoArg
	registeredInput
	registeredTimedInput
	registeredInputS
	registeredArraySort
	registeredSortChara
	registeredHTMLTagSplit
	registeredVariable
	registeredDTColumnOptions
	registeredFormAny
	registeredBit
	registeredSetter
)

const (
	dialectCommonMask uint8 = 1 << iota
	dialectEmueraMask
	dialectEMEEMask
)

type registeredSpec struct {
	mode      registeredMode
	min       uint8
	max       uint8
	dialects  uint8
	unbounded bool
}

var registeredCommandSpecs = buildRegisteredCommandSpecs()

func buildRegisteredCommandSpecs() map[string]registeredSpec {
	m := make(map[string]registeredSpec)
	add := func(mode registeredMode, min, max uint8, dialects uint8, names ...string) {
		for _, name := range names {
			m[name] = registeredSpec{mode: mode, min: min, max: max, dialects: dialects}
		}
	}
	addUnbounded := func(mode registeredMode, min uint8, dialects uint8, names ...string) {
		for _, name := range names {
			m[name] = registeredSpec{mode: mode, min: min, dialects: dialects, unbounded: true}
		}
	}
	all := dialectCommonMask | dialectEmueraMask | dialectEMEEMask
	add(registeredScalar, 0, 1, all, "SETFONT", "FONTSTYLE")
	add(registeredScalar, 1, 1, all, "LOADDATA", "ASSERT", "LOADCHARA", "LOADVAR", "TOOLTIP_SETDELAY", "TOOLTIP_SETDURATION")
	add(registeredFields, 0, 1, all, "PRINTCPERLINE", "SAVENOS")
	add(registeredFields, 1, 2, all, "HTML_PRINT")
	add(registeredFields, 1, 3, all, "SETCOLOR", "SETBGCOLOR")
	add(registeredFields, 2, 2, all, "PRINTBUTTON", "PRINTBUTTONC", "PRINTBUTTONLC")
	add(registeredFields, 3, 3, all, "BAR", "BARL")
	add(registeredFields, 2, 2, all, "TWAIT", "SWAPCHARA", "COPYCHARA")
	addUnbounded(registeredFields, 1, all, "ADDCHARA", "ADDSPCHARA", "ADDCOPYCHARA", "PICKUPCHARA", "DELCHARA")
	add(registeredFields, 2, 2, all, "SAVEDATA")
	addUnbounded(registeredFields, 3, all, "SAVECHARA")
	add(registeredFields, 3, 255, all, "SAVEVAR")
	add(registeredFields, 3, 3, all, "ARRAYREMOVE")
	add(registeredFields, 3, 5, all, "ARRAYSHIFT")
	add(registeredFields, 2, 2, all, "ARRAYCOPY")
	add(registeredArraySort, 1, 4, all, "ARRAYSORT")
	add(registeredSortChara, 0, 2, all, "SORTCHARA")
	add(registeredHTMLTagSplit, 1, 3, all, "HTML_TAGSPLIT")
	add(registeredInput, 0, 3, all, "INPUT", "ONEINPUT")
	add(registeredTimedInput, 2, 6, all, "TINPUT", "TINPUTS", "TONEINPUT", "TONEINPUTS")
	add(registeredInputS, 0, 2, all, "INPUTS", "ONEINPUTS")
	add(registeredInput, 0, 3, dialectEMEEMask, "BINPUT", "ONEBINPUT")
	add(registeredInputS, 0, 2, dialectEMEEMask, "BINPUTS", "ONEBINPUTS")
	add(registeredForm, 0, 0, all, "THROW", "PUTFORM", "PRINTPLAINFORM", "DEBUGPRINTFORM", "DEBUGPRINTFORML", "STRLENFORM", "STRLENFORMU", "ENCODETOUNI", "REUSELASTLINE")
	add(registeredBit, 2, 255, all, "SETBIT", "CLEARBIT", "INVERTBIT")
	add(registeredSetter, 0, 255, all, "VARSET", "CVARSET", "POWER")
	add(registeredForm, 1, 1, all, "DRAWLINEFORM")
	add(registeredNoArg, 0, 0, all, "WAIT", "FORCEWAIT", "WAITANYKEY")
	add(registeredRaw, 0, 0, dialectEMEEMask, "PRINTN")
	add(registeredPrintV, 0, 0, dialectEMEEMask, "PRINTVN")
	add(registeredScalar, 1, 1, dialectEMEEMask, "PRINTSN")
	add(registeredForm, 0, 0, dialectEMEEMask, "PRINTFORMN")
	add(registeredScalar, 1, 1, dialectEMEEMask, "PRINTFORMSN")
	add(registeredNoArg, 0, 0, all, "RESETCOLOR", "RESETBGCOLOR", "FONTBOLD", "FONTITALIC", "FONTREGULAR", "SAVEGLOBAL", "LOADGLOBAL", "RESETDATA", "RESETGLOBAL", "DEBUGCLEAR", "DUMPRAND", "INITRAND", "RESTART", "SAVEGAME", "LOADGAME", "ADDVOIDCHARA")
	add(registeredRaw, 0, 0, all, "BEGIN", "CALLEVENT", "ALIGNMENT", "SETCOLORBYNAME", "SETBGCOLORBYNAME", "STRLEN", "STRLENU")
	add(registeredRaw, 0, 0, dialectEMEEMask, "FORCE_BEGIN")
	add(registeredScalar, 0, 1, all, "AWAIT", "INPUTMOUSEKEY", "RANDOMIZE")
	add(registeredNoArg, 0, 0, dialectEMEEMask, "INPUTANY", "STOPBGM", "STOPSOUND", "UPDATECHECK", "HTML_PRINT_ISLAND_CLEAR", "CLEARBGIMAGE")
	add(registeredScalar, 0, 1, dialectEMEEMask, "BREAKBUTTON")
	add(registeredScalar, 1, 1, dialectEMEEMask, "SETBGMVOLUME", "SETSOUNDVOLUME", "TOOLTIP_SETFONTSIZE", "TOOLTIP_CUSTOM", "TOOLTIP_FORMAT", "TOOLTIP_IMG", "PLAYBGM", "TOOLTIP_SETFONT")
	add(registeredFields, 1, 2, dialectEMEEMask, "PLAYSOUND", "HTML_PRINT_ISLAND")
	add(registeredFields, 2, 2, all, "TOOLTIP_SETCOLOR")
	add(registeredVariable, 1, 1, all, "VARSIZE")
	add(registeredDTColumnOptions, 4, 255, dialectEMEEMask, "DT_COLUMN_OPTIONS")
	add(registeredFormAny, 1, 255, dialectEMEEMask, "SETBGIMAGE", "REMOVEBGIMAGE")
	return m
}

func (p *formatter) syntaxRegistered(i int, head []byte, base int, command string, wordEnd int) (int, bool) {
	spec, ok := registeredCommandSpecs[command]
	if !ok || !registeredDialect(spec.dialects, p.dialect) {
		return -1, false
	}

	switch spec.mode {
	case registeredRaw:
		p.syntaxOpaque(i)
		return -1, true
	case registeredNoArg:
		return p.syntaxRegisteredNoArg(i, head, base, wordEnd), true
	case registeredForm:
		return p.syntaxRegisteredForm(i, head, base, wordEnd, spec), true
	case registeredPrintV:
		return p.syntaxRegisteredPrintV(i, head, base, wordEnd), true
	case registeredInputS:
		return p.syntaxRegisteredInputS(i, head, base, wordEnd), true
	case registeredArraySort:
		return p.syntaxRegisteredArraySort(i, head, base, wordEnd, spec), true
	case registeredSortChara:
		return p.syntaxRegisteredSortChara(i, head, base, wordEnd, spec), true
	case registeredHTMLTagSplit:
		return p.syntaxRegisteredHTMLTagSplit(i, head, base, wordEnd, spec), true
	case registeredVariable:
		return p.syntaxRegisteredVariable(i, head, base, wordEnd), true
	case registeredDTColumnOptions:
		return p.syntaxRegisteredDTColumnOptions(i, head, base, wordEnd), true
	case registeredFormAny:
		return p.syntaxRegisteredFormAny(i, head, base, wordEnd), true
	case registeredBit:
		return p.syntaxBits(i, head, base, wordEnd), true
	case registeredSetter:
		return p.syntaxSetters(i, head, base, wordEnd, command), true
	case registeredFields, registeredScalar, registeredInput, registeredTimedInput:
		if command == "SAVEVAR" {
			return p.syntaxSAVEVAR(i, head, base, wordEnd), true
		}
		return p.syntaxRegisteredNamedFields(i, head, base, wordEnd, command, spec), true
	default:
		return -1, true
	}
}

func (p *formatter) syntaxRegisteredFormAny(i int, head []byte, base, wordEnd int) int {
	start, end, ok := registeredFormAnyBody(head, wordEnd, p.allowFullwidthSpace)
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
	start, end, comment, ok := p.registeredBody(head, wordEnd)
	if !ok || !registeredHasArgumentWithSpace(head, start, end, p.allowFullwidthSpace) {
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, base+wordEnd, "VARSIZE requires an array identifier")
		}
		p.syntaxOpaque(i)
		return -1
	}
	codeEnd := registeredCodeEnd(base, end, comment)
	if commandFieldHasDeferredMarker(p.src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	if commandFieldHasMandatoryMarker(p.src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	left := trimInputLeft(p.src, base+start, codeEnd, p.allowFullwidthSpace)
	right := trimInputRight(p.src, left, codeEnd, p.allowFullwidthSpace)
	session := p.scanSession(p.src, left, right)
	prefixEnd, prefixOK := labelIdentifierEndWithSession(p.src[left:right], 0, session)
	prefixEnd += left
	if !prefixOK {
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, left, "VARSIZE requires a bare array identifier")
		}
		p.syntaxOpaque(i)
		return -1
	}
	tokens, err := session.lexRange(expressionSourceOriginal, 0, prefixEnd-left, left)
	if err != nil || len(tokens) != 1 || tokens[0].kind != exprIdentifier {
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, left, "VARSIZE requires a bare array identifier")
		}
		p.syntaxOpaque(i)
		return -1
	}
	info, known := builtinVariableInfoForDialect(tokens, p.dialect)
	if p.expressionParseEnabled() {
		if known && (info.rank == 0 || string(tokens[0].text) == "RAND") {
			p.syntaxProblem(i, left, "VARSIZE requires an array identifier")
			p.syntaxOpaque(i)
			return -1
		}
	}
	if prefixEnd != right {
		if known && info.rank > 0 && string(tokens[0].text) != "RAND" && p.expressionParseEnabled() {
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
	mark := len(p.edits)
	fail := func(offset int, message string) int {
		p.truncateEdits(mark)
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, offset, message)
		}
		p.syntaxOpaque(i)
		return -1
	}
	start, end, comment, ok := p.registeredBody(head, wordEnd)
	if !ok || !registeredHasArgumentWithSpace(head, start, end, p.allowFullwidthSpace) {
		return fail(base+wordEnd, "DT_COLUMN_OPTIONS requires arguments")
	}
	codeEnd := registeredCodeEnd(base, end, comment)
	parts := p.splitTopLevel(p.src, base+start, codeEnd, ',')
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		left := trimInputLeft(p.src, last.start, last.end, p.allowFullwidthSpace)
		right := trimInputRight(p.src, left, last.end, p.allowFullwidthSpace)
		if left == right {
			parts = parts[:len(parts)-1]
		}
	}
	if len(parts) < 4 || (len(parts)-2)%2 != 0 {
		return fail(base+start, "DT_COLUMN_OPTIONS requires column options")
	}
	for n, field := range parts {
		left := trimInputLeft(p.src, field.start, field.end, p.allowFullwidthSpace)
		right := trimInputRight(p.src, left, field.end, p.allowFullwidthSpace)
		if left == right {
			return fail(field.start, "empty DT_COLUMN_OPTIONS argument")
		}
		if n >= 2 && n%2 == 0 {
			if !bytes.EqualFold(p.src[left:right], []byte("DEFAULT")) {
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
	codeEnd = trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace)
	p.setCodeEnd(i, codeEnd)
	return codeEnd
}

func registeredDialect(mask uint8, dialect Dialect) bool {
	switch dialect {
	case Emuera1824:
		return mask&dialectEmueraMask != 0
	case EMEE:
		return mask&dialectEMEEMask != 0
	default:
		return mask&dialectCommonMask != 0
	}
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
	tail := wordEnd + inputSpaceSize(head, wordEnd, p.allowFullwidthSpace)
	for tail < len(head) {
		size := inputSpaceSize(head, tail, p.allowFullwidthSpace)
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

func (p *formatter) syntaxRegisteredForm(i int, head []byte, base, wordEnd int, spec registeredSpec) int {
	if spec.min > 0 && wordEnd+max(1, inputSpaceSize(head, wordEnd, p.allowFullwidthSpace)) >= len(head) {
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
	start := base + wordEnd + inputSpaceSize(head, wordEnd, p.allowFullwidthSpace)
	if start == base+wordEnd {
		start++
	}
	if !p.expressionRulesEnabled() {
		return -1
	}
	if !p.syntaxForm(i, start, base+len(head), formLine) {
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
	start := wordEnd + inputSpaceSize(head, wordEnd, p.allowFullwidthSpace)
	arg := trimInputLeft(head, start, len(head), p.allowFullwidthSpace)
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

func (p *formatter) syntaxRegisteredNamedFields(i int, head []byte, base, wordEnd int, command string, spec registeredSpec) int {
	mark := len(p.edits)
	start, end, comment, ok := p.registeredBody(head, wordEnd)
	if !ok {
		p.syntaxOpaque(i)
		return -1
	}
	if !registeredHasArgumentWithSpace(head, start, end, p.allowFullwidthSpace) {
		if spec.min == 0 {
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
	if commandFieldHasDeferredMarker(p.src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	parts := p.splitTopLevel(p.src, arg, codeEnd, ',')
	if !p.expressionParseEnabled() {
		p.setCodeEnd(i, trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace))
		return trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace)
	}
	max := int(spec.max)
	if p.dialect == Emuera1824 {
		switch spec.mode {
		case registeredInput:
			max = 1
		case registeredTimedInput:
			max = 4
		}
	}
	if command == "HTML_PRINT" && p.dialect == Emuera1824 {
		max = 1
	}
	allowTrailing := spec.mode == registeredFields || spec.mode == registeredTimedInput
	if spec.unbounded {
		max = len(parts)
	}
	parts, overflow := commandPrefixParts(p.src, parts, max, int(spec.min), p.allowFullwidthSpace, allowTrailing)
	if spec.mode == registeredScalar && len(parts) < 1 && spec.min > 0 {
		p.syntaxProblem(i, arg, "expected one command argument")
		p.syntaxOpaque(i)
		return -1
	}
	if command == "SETCOLOR" || command == "SETBGCOLOR" {
		if len(parts) != 1 && len(parts) != 3 {
			if len(parts) < 3 && p.commandFieldsMayExpand(parts) {
				p.truncateEdits(mark)
				p.syntaxOpaque(i)
				return -1
			}
			p.syntaxProblem(i, arg, "expected one or three command arguments")
			p.syntaxOpaque(i)
			return -1
		}
	}
	if len(parts) < int(spec.min) {
		if p.commandFieldsMayExpand(parts) {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
		p.syntaxProblem(i, arg, "too few command arguments")
		p.syntaxOpaque(i)
		return -1
	}
	roles := commandRoles{}
	saveCharaIDs := make(map[int64]struct{})
	for n, field := range parts {
		left := trimInputLeft(p.src, field.start, field.end, p.allowFullwidthSpace)
		right := trimInputRight(p.src, left, field.end, p.allowFullwidthSpace)
		if left == right {
			if spec.mode != registeredInput {
				p.truncateEdits(mark)
				if p.expressionParseEnabled() {
					p.syntaxProblem(i, field.start, "empty command argument")
				}
				p.syntaxOpaque(i)
				return -1
			}
			continue
		}
		if !p.commandFieldTypeValid(command, n, left, right, &roles) {
			p.truncateEdits(mark)
			p.syntaxProblem(i, left, "command argument has an incompatible type")
			p.syntaxOpaque(i)
			return -1
		}
		if (command == "SETCOLOR" || command == "SETBGCOLOR") && p.commandFieldHasUnpairedTernaryHashAt(left, right) {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
		if command == "SAVECHARA" && n >= 2 && !roles.macro {
			if tokens, err := p.commandFieldTokens(left, right); err == nil && !commandMacroPossibleForDialect(tokens, p.dialect) {
				if value, kind := directIntegerLiteral(tokens); kind == knownValueNumeric {
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
		fieldValid := p.registeredFieldValid(command, n, p.src[left:right])
		if n == 0 && commandFieldHasMandatoryMarker(p.src[left:right]) && (command == "ARRAYSHIFT" || command == "ARRAYREMOVE") {
			fieldValid = commandFieldVariableArgument(p, left, right)
			if command == "ARRAYSHIFT" {
				if tokens, err := p.commandFieldTokens(left, right); err == nil {
					if info, known := builtinVariableRoleInfo(tokens, p.dialect); known && info.rank != 1 {
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
	codeEnd = trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace)
	p.setCodeEnd(i, codeEnd)
	return codeEnd
}

func (p *formatter) syntaxSAVEVAR(i int, head []byte, base, wordEnd int) int {
	start, end, comment, ok := p.registeredBody(head, wordEnd)
	if !ok {
		p.syntaxOpaque(i)
		return -1
	}
	if !registeredHasArgumentWithSpace(head, start, end, p.allowFullwidthSpace) {
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, base+wordEnd, "SAVEVAR requires arguments")
		}
		p.syntaxOpaque(i)
		return -1
	}
	codeEnd := registeredCodeEnd(base, end, comment)
	if commandFieldHasDeferredMarker(p.src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	arg := base + start
	if bytes.Contains(p.src[arg:codeEnd], []byte("[[")) {
		p.syntaxOpaque(i)
		return -1
	}
	parts := p.splitTopLevel(p.src, arg, codeEnd, ',')
	fields := parts
	if len(fields) > 1 {
		last := fields[len(fields)-1]
		if p.trimInputLeft(last.start, last.end) >= p.trimInputRight(last.start, last.end) {
			fields = fields[:len(fields)-1]
		}
	}
	if !p.expressionParseEnabled() {
		p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
		codeEnd = trimInputRight(p.src, arg, codeEnd, p.allowFullwidthSpace)
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
		left := p.trimInputLeft(field.start, field.end)
		right := p.trimInputRight(left, field.end)
		if left == right {
			return fail(field.start, "empty SAVEVAR argument")
		}
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil {
			p.truncateEdits(mark)
			p.expressionProblem(i, 0, err)
			p.syntaxOpaque(i)
			return -1
		}
		tokens = stripCommandMarkers(tokens)
		potentialMacro := setterMacroPossibleForDialect(tokens, p.dialect)
		if !macroPrefix && !potentialMacro {
			if n < 2 {
				if knownExpressionTypeForDialect(tokens, p.dialect) == knownValueNumeric {
					return fail(left, "SAVEVAR requires string arguments")
				}
			} else {
				if !variableArgumentTokens(tokens) {
					return fail(left, "SAVEVAR requires a variable argument")
				}
				if savevarBuiltinTargetForbidden(tokens, p.dialect) {
					return fail(left, "SAVEVAR cannot save a local or calculated variable")
				}
				if builtinVariableReadOnly(tokens, p.dialect) {
					return fail(left, "SAVEVAR cannot save a read-only variable")
				}
				if name, exact := variableSystemName(tokens); exact {
					if info, known := builtinVariableInfoByName(name, p.dialect); known && info.character {
						return fail(left, "SAVEVAR does not accept character variables")
					}
				}
				if info, known := builtinVariableInfoForDialect(tokens, p.dialect); known && info.character {
					return fail(left, "SAVEVAR does not accept character variables")
				}
				if identity, proven := builtinVariableIdentity(tokens, p.dialect); proven {
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
	codeEnd = trimInputRight(p.src, arg, codeEnd, p.allowFullwidthSpace)
	p.setCodeEnd(i, codeEnd)
	return codeEnd
}

func savevarBuiltinTargetForbidden(tokens []exprToken, dialect Dialect) bool {
	name, proven := builtinVariableIdentity(tokens, dialect)
	if !proven {
		return false
	}
	switch name {
	case "ARG", "ARGS", "LOCAL", "LOCALS", "WINDOW_TITLE":
		return true
	default:
		return false
	}
}

func (p *formatter) syntaxRegisteredInputS(i int, head []byte, base, wordEnd int) int {
	start, end, ok := registeredRawBodyWithSpace(head, wordEnd, p.allowFullwidthSpace)
	if !ok {
		p.syntaxOpaque(i)
		return -1
	}
	if wordEnd == len(head) {
		p.setCodeEnd(i, base+wordEnd)
		return base + wordEnd
	}
	if commandFieldHasDeferredMarker(head[start:end]) {
		p.syntaxOpaque(i)
		return -1
	}
	if p.dialect == EMEE {
		mark := len(p.edits)
		bodyStart := base + start
		session, offset := p.expressionSessionFor(bodyStart, base+end)
		formEnd, err := session.scanFormAt(expressionSourceOriginal, offset, offset+base+end-bodyStart, formComma)
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
		if p.expressionRulesEnabled() && !p.syntaxForm(i, bodyStart, firstEnd, formComma) {
			p.truncateEdits(mark)
			p.syntaxOpaque(i)
			return -1
		}
		if firstEnd < base+end {
			if p.src[firstEnd] != ',' {
				p.syntaxOpaque(i)
				return -1
			}
			parts := p.splitTopLevel(p.src, firstEnd+1, base+end, ',')
			parts, overflow := commandPrefixParts(p.src, parts, 2, 0, p.allowFullwidthSpace, true)
			for _, field := range parts {
				left := trimInputLeft(p.src, field.start, field.end, p.allowFullwidthSpace)
				right := trimInputRight(p.src, left, field.end, p.allowFullwidthSpace)
				if left == right {
					continue
				}
				if commandFieldHasDeferredMarker(p.src[left:right]) {
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
	if p.dialect == CommonDialect {
		bodyStart, bodyEnd := base+start, base+end
		session, offset := p.expressionSessionFor(bodyStart, bodyEnd)
		formEnd, err := session.scanFormAt(expressionSourceOriginal, offset, offset+bodyEnd-bodyStart, formComma)
		n := formEnd - offset
		if err == nil && n < bodyEnd-bodyStart && p.src[bodyStart+n] == ',' {
			if p.expressionParseEnabled() {
				p.syntaxProblem(i, base+start, "INPUTS comma-separated arguments require a runtime dialect")
			}
			p.syntaxOpaque(i)
			return -1
		}
	}
	if p.expressionRulesEnabled() && !p.syntaxForm(i, base+start, base+end, formLine) {
		p.syntaxOpaque(i)
	}
	return -1
}

func (p *formatter) syntaxRegisteredArraySort(i int, head []byte, base, wordEnd int, spec registeredSpec) int {
	mark := len(p.edits)
	start, end, comment, ok := p.registeredBody(head, wordEnd)
	if !ok {
		p.syntaxOpaque(i)
		return -1
	}
	if !registeredHasArgumentWithSpace(head, start, end, p.allowFullwidthSpace) {
		if spec.min == 0 {
			p.setCodeEnd(i, base+wordEnd)
			return base + wordEnd
		}
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, base+wordEnd, "missing command argument")
		}
		p.syntaxOpaque(i)
		return -1
	}
	codeEnd := registeredCodeEnd(base, end, comment)
	if commandFieldHasDeferredMarker(p.src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	if !p.expressionParseEnabled() {
		codeEnd = trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace)
		p.setCodeEnd(i, codeEnd)
		return codeEnd
	}
	parts := p.splitTopLevel(p.src, base+start, codeEnd, ',')
	parts, overflow := commandPrefixParts(p.src, parts, int(spec.max), int(spec.min), p.allowFullwidthSpace, true)
	if len(parts) > 1 {
		first := parts[0]
		firstLeft := trimInputLeft(p.src, first.start, first.end, p.allowFullwidthSpace)
		firstRight := trimInputRight(p.src, firstLeft, first.end, p.allowFullwidthSpace)
		second := parts[1]
		left := trimInputLeft(p.src, second.start, second.end, p.allowFullwidthSpace)
		right := trimInputRight(p.src, left, second.end, p.allowFullwidthSpace)
		if !p.commandFieldMayExpand(firstLeft, firstRight) && !p.commandFieldMayExpand(left, right) && !commandFieldKeyword(p, left, right, "FORWARD") && !commandFieldKeyword(p, left, right, "BACK") {
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
		left := trimInputLeft(p.src, field.start, field.end, p.allowFullwidthSpace)
		right := trimInputRight(p.src, left, field.end, p.allowFullwidthSpace)
		if left == right {
			p.truncateEdits(mark)
			if p.expressionParseEnabled() {
				p.syntaxProblem(i, field.start, "empty command argument")
			}
			p.syntaxOpaque(i)
			return -1
		}
		fieldMayExpand := p.commandFieldMayExpand(left, right)
		if n == 1 {
			keyword := commandFieldKeyword(p, left, right, "FORWARD") || commandFieldKeyword(p, left, right, "BACK")
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
			if tokens, err := p.commandFieldTokens(left, right); err == nil {
				if builtinVariableReadOnly(tokens, p.dialect) {
					p.truncateEdits(mark)
					p.syntaxProblem(i, left, "ARRAYSORT requires a mutable array variable")
					p.syntaxOpaque(i)
					return -1
				}
				if info, known := builtinVariableRoleInfo(tokens, p.dialect); known && info.rank != 1 {
					p.truncateEdits(mark)
					p.syntaxProblem(i, left, "invalid array variable argument")
					p.syntaxOpaque(i)
					return -1
				}
			}
		}
		if n >= 2 && !p.commandRawTypeValid(knownValueNumeric, left, right) {
			p.truncateEdits(mark)
			if directionKeyword && !fieldMayExpand {
				if tokens, err := p.commandFieldTokens(left, right); err == nil && builtinScalarIndexesMissing(tokens, p.dialect) {
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
			tokens, err := p.lexSyntaxRange(left, right)
			if err == nil && constantExpressionsValid(tokens) != nil {
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
	p.setCodeEnd(i, trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace))
	return trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace)
}

func (p *formatter) registeredFieldValid(command string, index int, src []byte) bool {
	switch command {
	case "ARRAYREMOVE", "ARRAYSHIFT":
		return index != 0 || p.isVariableArgument(src)
	case "PRINTCPERLINE", "SAVENOS":
		return p.isVariableArgument(src)
	default:
		return true
	}
}

func (p *formatter) commandFieldMayExpand(left, right int) bool {
	tokens, err := p.commandFieldTokens(left, right)
	return err == nil && commandMacroPossibleForDialect(tokens, p.dialect)
}

func (p *formatter) commandRawTypeValid(expected knownValueType, left, right int) bool {
	tokens, err := p.commandFieldTokens(left, right)
	if err != nil || commandMacroPossibleForDialect(tokens, p.dialect) || setterMacroPossibleForDialect(tokens, p.dialect) {
		return true
	}
	if builtinScalarIndexesMissing(tokens, p.dialect) {
		return false
	}
	return commandTypeMatches(expected, knownExpressionTypeForDialect(tokens, p.dialect))
}

func (p *formatter) syntaxRegisteredSortChara(i int, head []byte, base, wordEnd int, spec registeredSpec) int {
	mark := len(p.edits)
	start, end, comment, ok := p.registeredBody(head, wordEnd)
	if !ok {
		p.syntaxOpaque(i)
		return -1
	}
	if !registeredHasArgumentWithSpace(head, start, end, p.allowFullwidthSpace) {
		p.setCodeEnd(i, base+wordEnd)
		return base + wordEnd
	}
	codeEnd := registeredCodeEnd(base, end, comment)
	if commandFieldHasDeferredMarker(p.src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	if !p.expressionParseEnabled() {
		codeEnd = trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace)
		p.setCodeEnd(i, codeEnd)
		return codeEnd
	}
	parts := p.splitTopLevel(p.src, base+start, codeEnd, ',')
	parts, overflow := commandPrefixParts(p.src, parts, int(spec.max), int(spec.min), p.allowFullwidthSpace, true)
	macroPrefix := false
	for n, field := range parts {
		left := trimInputLeft(p.src, field.start, field.end, p.allowFullwidthSpace)
		right := trimInputRight(p.src, left, field.end, p.allowFullwidthSpace)
		if left == right {
			p.truncateEdits(mark)
			if p.expressionParseEnabled() {
				p.syntaxProblem(i, field.start, "empty command argument")
			}
			p.syntaxOpaque(i)
			return -1
		}
		fieldMayExpand := p.commandFieldMayExpand(left, right)
		if (n == len(parts)-1 || len(parts) == 1) && (commandFieldKeyword(p, left, right, "FORWARD") || commandFieldKeyword(p, left, right, "BACK")) {
			continue
		}
		if len(parts) == 2 && n == 1 && !commandFieldKeyword(p, left, right, "FORWARD") && !commandFieldKeyword(p, left, right, "BACK") && !fieldMayExpand {
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
		if n == 0 && (len(parts) == 1 || len(parts) == 2) && !commandFieldVariableArgument(p, left, right) {
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
			if tokens, err := p.commandFieldTokens(left, right); err == nil && !commandMacroPossibleForDialect(tokens, p.dialect) {
				if builtinVariableReadOnly(tokens, p.dialect) {
					p.truncateEdits(mark)
					p.syntaxProblem(i, left, "SORTCHARA requires a mutable character variable")
					p.syntaxOpaque(i)
					return -1
				}
				if info, known := builtinVariableRoleInfo(tokens, p.dialect); known && !info.character {
					p.truncateEdits(mark)
					p.syntaxProblem(i, left, "invalid SORTCHARA argument type")
					p.syntaxOpaque(i)
					return -1
				}
			}
		}
		rawRoles := commandRoles{}
		if !p.commandFieldTypeValid("SORTCHARA", n, left, right, &rawRoles) {
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
	p.setCodeEnd(i, trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace))
	return trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace)
}

func (p *formatter) syntaxRegisteredHTMLTagSplit(i int, head []byte, base, wordEnd int, spec registeredSpec) int {
	mark := len(p.edits)
	start, end, comment, ok := p.registeredBody(head, wordEnd)
	if !ok || !registeredHasArgumentWithSpace(head, start, end, p.allowFullwidthSpace) {
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, base+wordEnd, "missing HTML_TAGSPLIT argument")
		}
		p.syntaxOpaque(i)
		return -1
	}
	codeEnd := registeredCodeEnd(base, end, comment)
	if commandFieldHasDeferredMarker(p.src[base+start : codeEnd]) {
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	if !p.expressionParseEnabled() {
		codeEnd = trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace)
		p.setCodeEnd(i, codeEnd)
		return codeEnd
	}
	parts := p.splitTopLevel(p.src, base+start, codeEnd, ',')
	parts, overflow := commandPrefixParts(p.src, parts, int(spec.max), int(spec.min), p.allowFullwidthSpace, true)
	if len(parts) < int(spec.min) {
		if p.expressionParseEnabled() {
			p.syntaxProblem(i, base+start, "HTML_TAGSPLIT expects one to three arguments")
		}
		p.syntaxOpaque(i)
		return -1
	}
	macroPrefix := false
	for n, field := range parts {
		left := trimInputLeft(p.src, field.start, field.end, p.allowFullwidthSpace)
		right := trimInputRight(p.src, left, field.end, p.allowFullwidthSpace)
		if left == right {
			p.truncateEdits(mark)
			if p.expressionParseEnabled() {
				p.syntaxProblem(i, left, "invalid HTML_TAGSPLIT argument")
			}
			p.syntaxOpaque(i)
			return -1
		}
		fieldMayExpand := p.commandFieldMayExpand(left, right)
		if n > 0 && !commandFieldVariableArgument(p, left, right) {
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
		rawRoles := commandRoles{}
		if !p.commandFieldTypeValid("HTML_TAGSPLIT", n, left, right, &rawRoles) {
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
	p.setCodeEnd(i, trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace))
	return trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace)
}

func registeredCodeEnd(base, end, comment int) int {
	if comment >= 0 {
		return base + comment
	}
	return base + end
}

func registeredBodyWithSpace(head []byte, wordEnd int, allowFullwidthSpace bool, sessions ...*expressionSession) (start, end, comment int, ok bool) {
	if wordEnd >= len(head) {
		return wordEnd, wordEnd, -1, true
	}
	if head[wordEnd] == ';' {
		return 0, 0, -1, false
	}
	if !inputSpaceAt(head, wordEnd, allowFullwidthSpace) {
		return 0, 0, -1, false
	}
	start = wordEnd + inputSpaceSize(head, wordEnd, allowFullwidthSpace)
	end, found := syntaxCommentEnd(head, start, len(head), sessions...)
	if !found {
		return start, end, -1, true
	}
	return start, end, end, true
}

func registeredRawBodyWithSpace(head []byte, wordEnd int, allowFullwidthSpace bool) (start, end int, ok bool) {
	if wordEnd >= len(head) {
		return wordEnd, wordEnd, true
	}
	if !inputSpaceAt(head, wordEnd, allowFullwidthSpace) && head[wordEnd] != ';' {
		return 0, 0, false
	}
	return wordEnd + max(1, inputSpaceSize(head, wordEnd, allowFullwidthSpace)), len(head), true
}

func registeredFormAnyBody(head []byte, wordEnd int, allowFullwidthSpace bool) (start, end int, ok bool) {
	start, end, ok = registeredRawBodyWithSpace(head, wordEnd, allowFullwidthSpace)
	if !ok {
		return 0, 0, false
	}
	for start < len(head) && head[start] == ' ' {
		start++
	}
	return start, end, true
}

func (p *formatter) syntaxRegisteredCommaGap(i, comma, end, leftBound int) {
	if !p.rules[CommaSpacing] {
		return
	}
	left := trimInputRight(p.src, leftBound, comma, p.allowFullwidthSpace)
	right := trimInputLeft(p.src, comma+1, end, p.allowFullwidthSpace)
	if mandatoryInlineMarker(p.src[:left], left-3) {
		left = comma
	}
	if mandatoryInlineMarker(p.src, right) {
		p.addEdit(i, left, comma, nil, CommaSpacing)
		return
	}
	if right < end && p.src[right] != ',' && p.src[right] != ')' && p.src[right] != ']' {
		p.addEdit(i, left, right, []byte{',', ' '}, CommaSpacing)
	} else if left < comma {
		p.addEdit(i, left, comma, nil, CommaSpacing)
	}
}

func (p *formatter) syntaxRegisteredCommandGap(i int, head []byte, base, wordEnd, start, end int) {
	if !p.rules[CommandSpacing] {
		return
	}
	first := trimInputLeft(head, start, end, p.allowFullwidthSpace)
	if first < end && head[first] != ';' && head[first] != ',' {
		p.addEdit(i, base+wordEnd, base+first, []byte{' '}, CommandSpacing)
	}
}

func (p *formatter) syntaxRegisteredCommaGaps(i int, parts []syntaxSpan, end int) {
	for n := 0; n+1 < len(parts); n++ {
		comma := parts[n].end
		for comma < parts[n+1].start && p.src[comma] != ',' {
			comma++
		}
		if comma >= parts[n+1].start || p.src[comma] != ',' {
			continue
		}
		leftBound := parts[n].start
		fieldLeft := trimInputLeft(p.src, parts[n].start, parts[n].end, p.allowFullwidthSpace)
		fieldRight := trimInputRight(p.src, fieldLeft, parts[n].end, p.allowFullwidthSpace)
		if fieldLeft >= fieldRight {
			leftBound = comma
		}
		p.syntaxRegisteredCommaGap(i, comma, end, leftBound)
	}
}

func registeredHasArgumentWithSpace(head []byte, start, end int, allowFullwidthSpace bool) bool {
	return start < end && trimInputLeft(head, start, end, allowFullwidthSpace) < trimInputRight(head, start, end, allowFullwidthSpace)
}

func (p *formatter) isVariableArgument(src []byte) bool {
	src = trimInput(src, p.allowFullwidthSpace)
	if bytes.Contains(src, []byte("[[")) {
		return true
	}
	tokens, err := p.newExpressionSession(src).lexRange(expressionSourceOriginal, 0, len(src), 0)
	return err == nil && variableArgumentShape(tokens)
}
