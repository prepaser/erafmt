package format

import (
	"bytes"
	"unicode"
	"unicode/utf8"
)

func (p *formatter) syntaxHeader(i int, head []byte, base int) int {
	mark := len(p.edits)
	fail := func() int {
		if len(p.edits) > mark {
			p.truncateEdits(mark)
		}
		p.syntaxOpaque(i)
		return -1
	}
	if len(head) < 2 || bytes.Contains(head, []byte("[[")) {
		return fail()
	}
	end, _ := p.syntaxCommentEnd(head, 1, len(head))
	if end == len(head) {
		end = syntaxHeaderMarkerEnd(head)
	}
	head = head[:end]
	_, delimiter := headerPrefix(head, 1)
	if delimiter < 0 {
		p.setCodeEnd(i, base+len(head))
		return base + len(head)
	}
	if p.rules[DelimiterSpacing] && (head[delimiter] == '[' || head[delimiter] == '(') {
		gapStart := delimiter
		gapStart = trimInputRight(head, 1, gapStart, p.allowFullwidthSpace)
		p.addEdit(i, base+gapStart, base+delimiter, nil, DelimiterSpacing)
	}
	if head[delimiter] == '[' {
		close := p.matchingDelimiter(head, delimiter, len(head), '[', ']')
		if close < 0 || !p.syntaxExpression(i, base+delimiter+1, base+close, false) {
			return fail()
		}
		p.syntaxDeclarationDelimiter(i, base+delimiter, base+close)
		delimiter = close + 1
		gapStart := delimiter
		for delimiter < len(head) {
			size := inputSpaceSize(head, delimiter, p.allowFullwidthSpace)
			if size == 0 {
				break
			}
			delimiter += size
		}
		if p.rules[DelimiterSpacing] && delimiter < len(head) && head[delimiter] == '(' {
			p.addEdit(i, base+gapStart, base+delimiter, nil, DelimiterSpacing)
		}
		if delimiter >= len(head) {
			p.setCodeEnd(i, base+len(head))
			return base + len(head)
		}
	}
	if head[delimiter] == '(' {
		close := p.matchingDelimiter(head, delimiter, len(head), '(', ')')
		if close < 0 || !p.syntaxSignatureArgs(i, base+delimiter+1, base+close) {
			return fail()
		}
		p.syntaxDeclarationDelimiter(i, base+delimiter, base+close)
	} else if head[delimiter] == ',' {
		if !p.syntaxSignatureArgs(i, base+delimiter+1, base+len(head)) {
			return fail()
		}
		p.syntaxCommaGap(i, base+delimiter, base+len(head))
	} else {
		return fail()
	}
	p.setCodeEnd(i, base+len(head))
	return base + len(head)
}

func syntaxHeaderMarkerEnd(head []byte) int {
	for i := 1; i < len(head); {
		if next, atom := syntaxAtomEnd(head, i, len(head)); atom {
			if next < 0 {
				return len(head)
			}
			i = next
			continue
		}
		if executableMarker(head, i) {
			return i
		}
		i++
	}
	return len(head)
}

func (p *formatter) syntaxSignatureArgs(i, start, end int) bool {
	mark := len(p.edits)
	fail := func() bool {
		p.truncateEdits(mark)
		return false
	}
	spans := p.splitTopLevel(p.src, start, end, ',')
	for index, span := range spans {
		if trimInputLeft(p.src, span.start, span.end, p.allowFullwidthSpace) >= trimInputRight(p.src, span.start, span.end, p.allowFullwidthSpace) {
			if len(spans) == 1 || index == len(spans)-1 && index > 0 {
				continue
			}
			return fail()
		}
		if eq, ok := p.topEqual(p.src, span.start, span.end); ok {
			if trimInputLeft(p.src, span.start, eq, p.allowFullwidthSpace) >= trimInputRight(p.src, span.start, eq, p.allowFullwidthSpace) || trimInputLeft(p.src, eq+1, span.end, p.allowFullwidthSpace) >= trimInputRight(p.src, eq+1, span.end, p.allowFullwidthSpace) {
				return fail()
			}
			if p.rules[DeclarationDefaultSpacing] {
				p.addEdit(i, trimInputRight(p.src, span.start, eq, p.allowFullwidthSpace), eq, []byte{' '}, DeclarationDefaultSpacing)
				p.addEdit(i, eq+1, trimInputLeft(p.src, eq+1, span.end, p.allowFullwidthSpace), []byte{' '}, DeclarationDefaultSpacing)
			}
			if !p.syntaxExpression(i, span.start, eq, false) || !p.syntaxExpression(i, eq+1, span.end, false) {
				return fail()
			}
		} else if !p.syntaxExpression(i, span.start, span.end, false) {
			return fail()
		}
	}
	p.syntaxDeclarationCommas(i, start, end)
	return true
}

func (p *formatter) syntaxDeclaration(i int, head []byte, base int) int {
	mark := len(p.edits)
	diagnosticMark := len(p.diagnostics)
	fail := func() int {
		if len(p.edits) > mark {
			p.truncateEdits(mark)
		}
		if len(p.diagnostics) > diagnosticMark {
			p.removeLineRule(i, DeclarationIndent)
		}
		p.syntaxOpaque(i)
		return -1
	}
	if bytes.Contains(head, []byte("[[")) {
		return fail()
	}
	end, _ := p.syntaxCommentEnd(head, 0, len(head))
	head = head[:end]
	name := declarationName(head)
	if name == "DEFINE" {
		if p.kind == ERH && !p.sharpStateUnknown && functionStyleDefine(head) {
			p.preserveFile = true
			p.problem(i, base+declarationWordEnd(head), "unsupported_declaration", "function-style #DEFINE is unsupported", Unsupported)
			p.syntaxOpaque(i)
			return fail()
		}
		if p.kind == ERH && !p.sharpStateUnknown {
			p.syntaxObjectDefine(i, head, base)
			return -1
		}
		return fail()
	}
	if name == "FUNCTION" || name == "FUNCTIONS" {
		if p.kind == ERB && bareSharpFunction(head, name, p.allowFullwidthSpace) {
			p.setCodeEnd(i, base+len(head))
			return base + len(head)
		}
		return fail()
	}
	if name == "SINGLE" || name == "LATER" || name == "PRI" || name == "ONLY" {
		p.setCodeEnd(i, base+len(head))
		return base + len(head)
	}
	wordEnd := declarationWordEnd(head)
	if wordEnd < 0 {
		return fail()
	}
	bodyStart := wordEnd
	for bodyStart < len(head) {
		size := inputSpaceSize(head, bodyStart, p.allowFullwidthSpace)
		if size == 0 {
			break
		}
		bodyStart += size
	}
	if bodyStart >= len(head) {
		if name == "DIM" || name == "DIMS" {
			p.syntaxProblem(i, base+bodyStart, "declaration requires an identifier")
			return fail()
		}
		if (name == "LOCALSIZE" || name == "LOCALSSIZE") && p.expressionParseEnabled() {
			p.warning(i, base+wordEnd, "missing_local_size", name+" requires a value")
		}
		p.setCodeEnd(i, base+len(head))
		return base + len(head)
	}
	p.addDeclarationWordGaps(i, head, base, wordEnd, &bodyStart)
	if bodyStart >= len(head) && (name == "DIM" || name == "DIMS") {
		p.syntaxProblem(i, base+bodyStart, "declaration requires an identifier")
		return fail()
	}
	if name == "LOCALSIZE" || name == "LOCALSSIZE" {
		if p.eventFunction() {
			p.setCodeEnd(i, base+len(head))
			return base + len(head)
		}
		if !p.syntaxExpression(i, base+bodyStart, base+len(head), false) {
			return fail()
		}
		if p.expressionParseEnabled() && !p.sharpStateUnknown && !p.optional && !p.conditional() {
			left := base + bodyStart
			right := base + len(head)
			if tokens, err := p.lexSyntaxRange(left, right); err == nil && !commandMacroPossible(tokens) {
				status, value := localSizeConstantProof(tokens)
				if status == localSizeConstantInvalid {
					p.warning(i, left, "suspicious_local_size", "local size expression is not a valid integer constant")
					return fail()
				}
				if status == localSizeConstantValid && (value <= 0 || value >= 1<<31-1) {
					p.warning(i, left, "suspicious_local_size", "constant local size is ignored because it is outside the range 1..2147483646")
					return fail()
				}
			}
		}
		p.setCodeEnd(i, base+len(head))
		return base + len(head)
	}
	if name != "DIM" && name != "DIMS" {
		return fail()
	}
	modifiers, modifierKnown := declarationModifierInfo(p.src, base+wordEnd, base+bodyStart, p.allowFullwidthSpace)
	end = base + len(head)
	eq, hasEqual := p.topEqual(p.src, base+bodyStart, end)
	leftEnd := end
	if hasEqual {
		leftEnd = eq
	}
	nameSpans := p.splitTopLevel(p.src, base+bodyStart, leftEnd, ',')
	if p.aborted {
		return fail()
	}
	modifierIssue := declarationModifierIssue(p.kind, modifiers)
	if len(nameSpans) > 0 && declarationMacroNameSpan(p, nameSpans[0]) && modifierIssue == "" && !(p.kind == ERH && modifiers.ref) {
		return fail()
	}
	if p.kind == ERB && !p.optional && !p.conditional() {
		if p.privateDeclarationNameMacroPossible(nameSpans) {
			return fail()
		}
	}
	if declarationReservedName(p, base+bodyStart, leftEnd) {
		if p.kind == ERB {
			p.problem(i, base+bodyStart, "unsupported_syntax", "declaration name is reserved", Unsupported)
		} else {
			p.syntaxProblem(i, base+bodyStart, "declaration name is reserved")
		}
		return fail()
	}
	if p.kind == ERH && modifierKnown && modifiers.ref {
		p.preserveFile = true
		p.syntaxProblem(i, base+wordEnd, "ERH REF declarations are unsupported")
		return fail()
	}
	if hasEqual {
		if trimInputLeft(p.src, base+bodyStart, eq, p.allowFullwidthSpace) >= trimInputRight(p.src, base+bodyStart, eq, p.allowFullwidthSpace) || trimInputLeft(p.src, eq+1, end, p.allowFullwidthSpace) >= trimInputRight(p.src, eq+1, end, p.allowFullwidthSpace) {
			if p.expressionParseEnabled() && modifierKnown {
				p.warning(i, eq+1, "suspicious_initializer", "declaration initializer is empty")
			}
			return fail()
		}
		if p.rules[DeclarationDefaultSpacing] {
			p.addEdit(i, trimInputRight(p.src, base+bodyStart, eq, p.allowFullwidthSpace), eq, []byte{' '}, DeclarationDefaultSpacing)
			p.addEdit(i, eq+1, trimInputLeft(p.src, eq+1, end, p.allowFullwidthSpace), []byte{' '}, DeclarationDefaultSpacing)
		}
		if !p.syntaxDeclarationInitializerExpression(i, eq+1, end) {
			return fail()
		}
	}
	if p.expressionParseEnabled() && modifierKnown {
		if message := declarationModifierIssue(p.kind, modifiers); message != "" {
			p.syntaxProblem(i, base+wordEnd, message)
			return fail()
		}
		if !hasEqual && modifiers.isConst {
			p.warning(i, base+bodyStart, "suspicious_initializer", "CONST declaration has no initializer; macro expansion may change its role")
			return fail()
		}
		if hasEqual && declarationInitializerSuspicious(p, i, name == "DIMS", modifiers, p.src, base+bodyStart, leftEnd, eq+1, end) {
			return fail()
		}
	}
	allowRefOmittedDimensions := p.kind == ERB && p.base > 0 && p.state.declarations && !hasEqual && declarationModifiersAllowRefOmissionWithSpace(p.src, base+wordEnd, base+bodyStart, p.allowFullwidthSpace)
	initializerStart, initializerEnd := -1, -1
	if hasEqual {
		initializerStart, initializerEnd = eq+1, end
	}
	if !p.syntaxDeclarationDimensions(i, base+wordEnd, base+bodyStart, leftEnd, allowRefOmittedDimensions, initializerStart, initializerEnd, name == "DIMS") {
		return fail()
	}
	p.setCodeEnd(i, end)
	return end
}

func (p *formatter) syntaxDeclarationInitializerExpression(i, start, end int) bool {
	if p.expressionParseEnabled() {
		tokens, err := p.lexSyntaxRange(start, end)
		if err == nil && !expressionNeedsDeferral(tokens, p.dialect) {
			if shapeErr := provenExpressionShape(tokens, p.dialect, false); shapeErr != nil {
				p.expressionProblem(i, start, shiftExpressionError(shapeErr, -start))
				return false
			}
		}
	}
	return p.syntaxExpression(i, start, end, false)
}

type localSizeConstantStatus uint8

const (
	localSizeConstantUnknown localSizeConstantStatus = iota
	localSizeConstantValid
	localSizeConstantInvalid
)

func localSizeConstantProof(tokens []exprToken) (localSizeConstantStatus, int64) {
	if len(tokens) == 0 || len(tokens) > 4096 {
		return localSizeConstantUnknown, 0
	}
	depth := 0
	hasString := false
	for _, token := range tokens {
		switch token.kind {
		case exprIdentifier, exprForm, exprMarker:
			return localSizeConstantUnknown, 0
		case exprString:
			hasString = true
		case exprNumber:
			if _, ok := numericLiteralValue(token.text); !ok {
				return localSizeConstantUnknown, 0
			}
		case exprOperator:
			op := string(token.text)
			if constantPriority(op) == 0 && op != "+" && op != "-" && op != "!" && op != "~" {
				return localSizeConstantInvalid, 0
			}
		case exprSymbol:
			switch string(token.text) {
			case "(":
				depth++
				if depth > 256 {
					return localSizeConstantUnknown, 0
				}
			case ")":
				if depth == 0 {
					return localSizeConstantInvalid, 0
				}
				depth--
			default:
				return localSizeConstantInvalid, 0
			}
		default:
			return localSizeConstantUnknown, 0
		}
	}
	if depth != 0 {
		return localSizeConstantInvalid, 0
	}
	valueType, valid, known := constantExpressionType(tokens)
	if !known || !valid || valueType != knownValueNumeric {
		return localSizeConstantInvalid, 0
	}
	if hasString {
		return localSizeConstantUnknown, 0
	}
	value, ok := constantIntegerValue(tokens)
	if !ok {
		return localSizeConstantInvalid, 0
	}
	return localSizeConstantValid, value
}

func functionStyleDefine(head []byte) bool {
	wordEnd := declarationWordEnd(head)
	if wordEnd != len("#DEFINE") || !bytes.Equal(head[:wordEnd], []byte("#DEFINE")) {
		return false
	}
	pos := wordEnd
	for pos < len(head) {
		size := inputSpaceSize(head, pos, false)
		if size == 0 {
			break
		}
		pos += size
	}
	nameEnd := expressionIdentifierEnd(head, pos)
	return nameEnd > pos && nameEnd < len(head) && head[nameEnd] == '(' && !bytes.Contains(head, []byte("[["))
}

func (p *formatter) syntaxObjectDefine(i int, head []byte, base int) {
	if !p.rules[DeclarationWordSpacing] || len(head) < len("#DEFINE") || !bytes.Equal(head[:len("#DEFINE")], []byte("#DEFINE")) {
		return
	}
	line := p.lines[i]
	if bytes.IndexByte(p.src[line.start:line.bodyEnd], 0) >= 0 || bytes.IndexByte(head, 0) >= 0 || bytes.Contains(head, []byte("[[")) || bytes.Contains(head, []byte(";!;")) || bytes.Contains(head, []byte(";#;")) {
		return
	}
	wordEnd := len("#DEFINE")
	nameStart := wordEnd
	for nameStart < len(head) {
		size := inputSpaceSize(head, nameStart, p.allowFullwidthSpace)
		if size == 0 {
			break
		}
		nameStart += size
	}
	if nameStart == wordEnd || nameStart >= len(head) {
		return
	}
	nameEnd := expressionIdentifierEnd(head, nameStart)
	if nameEnd == nameStart || nameEnd < 0 {
		return
	}
	tokens, err := p.lexSyntaxRange(base+nameStart, base+nameEnd)
	if err != nil || len(tokens) != 1 || tokens[0].kind != exprIdentifier {
		return
	}
	if nameEnd < len(head) && head[nameEnd] == '(' {
		return
	}
	if nameEnd < len(head) && inputSpaceSize(head, nameEnd, p.allowFullwidthSpace) == 0 {
		return
	}
	p.addEdit(i, base+wordEnd, base+nameStart, []byte{' '}, DeclarationWordSpacing)
	if nameEnd >= len(head) {
		return
	}
	bodyStart := nameEnd
	for bodyStart < len(head) {
		size := inputSpaceSize(head, bodyStart, p.allowFullwidthSpace)
		if size == 0 {
			break
		}
		bodyStart += size
	}
	if bodyStart == nameEnd || bodyStart >= len(head) {
		return
	}
	p.addEdit(i, base+nameEnd, base+bodyStart, []byte{' '}, DeclarationWordSpacing)
}

type declarationModifierSet struct {
	isConst bool
	dynamic bool
	static  bool
	save    bool
	chara   bool
	ref     bool
	global  bool
	issue   string
}

func declarationModifierInfo(src []byte, start, end int, allowFullwidthSpace bool) (declarationModifierSet, bool) {
	var modifiers declarationModifierSet
	for start < end {
		for start < end {
			size := inputSpaceSize(src, start, allowFullwidthSpace)
			if size == 0 {
				break
			}
			start += size
		}
		if start >= end {
			return modifiers, true
		}
		wordEnd := asciiWordEnd(src, start)
		if wordEnd == start || wordEnd > end {
			return declarationModifierSet{}, false
		}
		switch string(bytes.ToUpper(src[start:wordEnd])) {
		case "CONST":
			if modifiers.isConst {
				modifiers.issue = "CONST modifier is duplicated"
			}
			modifiers.isConst = true
		case "DYNAMIC":
			if modifiers.dynamic {
				modifiers.issue = "DYNAMIC modifier is duplicated"
			}
			modifiers.dynamic = true
		case "STATIC":
			if modifiers.static {
				modifiers.issue = "STATIC modifier is duplicated"
			}
			modifiers.static = true
		case "SAVEDATA":
			if modifiers.save {
				modifiers.issue = "SAVEDATA modifier is duplicated"
			}
			modifiers.save = true
		case "CHARADATA":
			if modifiers.chara {
				modifiers.issue = "CHARADATA modifier is duplicated"
			}
			modifiers.chara = true
		case "REF":
			if modifiers.ref {
				modifiers.issue = "REF modifier is duplicated"
			}
			modifiers.ref = true
		case "GLOBAL":
			modifiers.global = true
		default:
			return declarationModifierSet{}, false
		}
		start = wordEnd
	}
	return modifiers, true
}

func declarationModifierIssue(kind Kind, modifiers declarationModifierSet) string {
	if modifiers.issue != "" {
		return modifiers.issue
	}
	if kind == ERB {
		if modifiers.global {
			return "GLOBAL is not valid for a local declaration"
		}
		if modifiers.save {
			return "SAVEDATA is not valid for a local declaration"
		}
		if modifiers.chara {
			return "CHARADATA is not valid for a local declaration"
		}
	} else {
		if modifiers.dynamic {
			return "DYNAMIC is not valid for a global declaration"
		}
		if modifiers.static {
			return "STATIC is not valid for a global declaration"
		}
	}
	if modifiers.isConst && (modifiers.dynamic || modifiers.save || modifiers.chara || modifiers.ref || modifiers.global) {
		return "CONST modifier conflicts with another declaration modifier"
	}
	if modifiers.ref && (modifiers.static || modifiers.save || modifiers.chara || modifiers.global || modifiers.isConst) {
		return "REF modifier conflicts with another declaration modifier"
	}
	if modifiers.dynamic && modifiers.static {
		return "DYNAMIC and STATIC modifiers conflict"
	}
	if modifiers.static && (modifiers.save || modifiers.chara || modifiers.global) {
		return "STATIC modifier conflicts with another declaration modifier"
	}
	if modifiers.dynamic && modifiers.chara {
		return "DYNAMIC modifier conflicts with CHARADATA"
	}
	if modifiers.chara && modifiers.global {
		return "CHARADATA modifier conflicts with GLOBAL"
	}
	return ""
}

func declarationInitializerSuspicious(p *formatter, line int, stringType bool, modifiers declarationModifierSet, src []byte, fieldsStart, fieldsEnd, initStart, initEnd int) bool {
	if modifiers.ref {
		p.warning(line, initStart, "suspicious_initializer", "reference declaration cannot have an initializer")
		return true
	}
	if modifiers.chara {
		p.warning(line, initStart, "suspicious_initializer", "CHARADATA declaration cannot have an initializer")
		return true
	}
	dimensions := p.splitTopLevel(src, fieldsStart, fieldsEnd, ',')
	if len(dimensions) > 1 && !declarationInitializerDimensionsKnown(p, dimensions[1:]) {
		return false
	}
	initializer := p.declarationInitializerFields(initStart, initEnd)
	if len(initializer) == 0 {
		p.warning(line, initStart, "suspicious_initializer", "declaration initializer is empty")
		return true
	}
	expected := knownValueNumeric
	if stringType {
		expected = knownValueString
	}
	for _, field := range initializer {
		left := trimInputLeft(src, field.start, field.end, p.allowFullwidthSpace)
		right := trimInputRight(src, left, field.end, p.allowFullwidthSpace)
		if left == right {
			p.warning(line, field.start, "suspicious_initializer", "initializer contains an empty value")
			return true
		}
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil || commandMacroPossible(tokens) {
			return false
		}
		actual := declarationInitializerType(tokens)
		if actual == knownValueUnknown {
			return false
		}
		if actual != expected {
			p.warning(line, left, "suspicious_initializer", "initializer type does not match declaration type; macro expansion may change its role")
			return true
		}
	}
	if len(dimensions) > 1 {
		knownSize, size := declarationInitializerSize(p, dimensions[1:])
		if !knownSize {
			return false
		}
		if len(dimensions)-1 >= 2 {
			p.warning(line, initStart, "suspicious_initializer", "multi-dimensional declaration cannot have an initializer; macro expansion may change its role")
			return true
		}
		if size > 0 && (int64(len(initializer)) > size || modifiers.isConst && int64(len(initializer)) != size) {
			p.warning(line, initStart, "suspicious_initializer", "initializer count does not match the declaration size; macro expansion may change its role")
			return true
		}
	}
	return false
}

func declarationInitializerDimensionsKnown(p *formatter, spans []syntaxSpan) bool {
	for _, span := range spans {
		left := trimInputLeft(p.src, span.start, span.end, p.allowFullwidthSpace)
		right := trimInputRight(p.src, left, span.end, p.allowFullwidthSpace)
		if left == right {
			return false
		}
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil || commandMacroPossible(tokens) {
			return false
		}
	}
	return true
}

func declarationInitializerType(tokens []exprToken) knownValueType {
	value, valid, known := constantExpressionType(tokens)
	if known && valid {
		return value
	}
	return knownValueUnknown
}

func declarationInitializerSize(p *formatter, spans []syntaxSpan) (bool, int64) {
	if len(spans) == 0 {
		return true, 0
	}
	if len(spans) > 1 {
		for _, span := range spans {
			left := trimInputLeft(p.src, span.start, span.end, p.allowFullwidthSpace)
			right := trimInputRight(p.src, left, span.end, p.allowFullwidthSpace)
			tokens, err := p.lexSyntaxRange(left, right)
			if err != nil || commandMacroPossible(tokens) {
				return false, 0
			}
			if _, valid := constantIntegerValue(tokens); !valid {
				return false, 0
			}
		}
		return true, 0
	}
	left := trimInputLeft(p.src, spans[0].start, spans[0].end, p.allowFullwidthSpace)
	right := trimInputRight(p.src, left, spans[0].end, p.allowFullwidthSpace)
	tokens, err := p.lexSyntaxRange(left, right)
	if err != nil || commandMacroPossible(tokens) {
		return false, 0
	}
	value, valid := constantIntegerValue(tokens)
	return valid, value
}

func (p *formatter) declarationInitializerFields(start, end int) []syntaxSpan {
	fields := p.splitTopLevel(p.src, start, end, ',')
	if len(fields) > 1 {
		left, right := extraFieldBoundsWithSpace(p.src, fields[len(fields)-1], p.allowFullwidthSpace)
		if left == right {
			return fields[:len(fields)-1]
		}
	}
	return fields
}

func declarationMacroNameSpan(p *formatter, span syntaxSpan) bool {
	left := trimInputLeft(p.src, span.start, span.end, p.allowFullwidthSpace)
	right := trimInputRight(p.src, left, span.end, p.allowFullwidthSpace)
	tokens, err := p.lexSyntaxRange(left, right)
	if err != nil || len(tokens) < 2 || tokens[0].kind != exprIdentifier {
		return false
	}
	for _, token := range tokens {
		if token.kind != exprIdentifier {
			return false
		}
	}
	name := string(bytes.ToUpper(tokens[0].text))
	if setterKnownNameForDialect(tokens[0].text, p.dialect) || reservedInstructions[name] && !commandUnavailable(name, p.dialect) {
		return false
	}
	mask, ok := builtinMethodMasks[name]
	return !ok || !methodDialect(mask, p.dialect)
}

func declarationReservedName(p *formatter, start, end int) bool {
	spans := p.splitTopLevel(p.src, start, end, ',')
	if len(spans) == 0 {
		return false
	}
	left := trimInputLeft(p.src, spans[0].start, spans[0].end, p.allowFullwidthSpace)
	right := trimInputRight(p.src, left, spans[0].end, p.allowFullwidthSpace)
	tokens, err := p.lexSyntaxRange(left, right)
	if err != nil || len(tokens) != 1 || tokens[0].kind != exprIdentifier || !bytes.Equal(tokens[0].text, bytes.ToUpper(tokens[0].text)) {
		return false
	}
	name := string(tokens[0].text)
	if reservedInstructions[name] && !commandUnavailable(name, p.dialect) || setterKnownNameForDialect(tokens[0].text, p.dialect) {
		return true
	}
	if reservedDeclarationNames[name] {
		return true
	}
	mask, ok := builtinMethodMasks[name]
	return ok && methodDialect(mask, p.dialect)
}

var reservedDeclarationNames = map[string]bool{
	"IS": true, "TO": true, "INT": true, "REFFUNC": true,
	"PRIVATE": true, "__DEBUG__": true, "__SKIP__": true, "_": true,
}

func (p *formatter) eventFunction() bool {
	event, _ := p.eventFunctionStatus()
	return event
}

func (p *formatter) eventFunctionStatus() (bool, bool) {
	if p.kind != ERB {
		return false, false
	}
	if p.functionName != "" {
		return p.eventFunctionKnown, p.eventFunctionExact
	}
	if p.functionStart < 0 || p.functionStart >= len(p.lines) {
		return false, false
	}
	head, _, _, _ := p.headOf(lBody(p.src, p.lines[p.functionStart]))
	start := trimInputLeft(head, 1, len(head), p.allowFullwidthSpace)
	end, _ := headerPrefix(head, start)
	end = trimInputRight(head, start, end, p.allowFullwidthSpace)
	name := string(head[start:end])
	return eventFunctionContext(name)
}

func (p *formatter) addDeclarationWordGaps(i int, head []byte, base, wordEnd int, bodyStart *int) {
	pos := *bodyStart
	if p.rules[DeclarationWordSpacing] && pos > wordEnd && pos < len(head) {
		p.addEdit(i, base+wordEnd, base+pos, []byte{' '}, DeclarationWordSpacing)
	}
	for {
		wordEnd := asciiWordEnd(head, pos)
		separator := wordEnd < len(head) && inputSpaceAt(head, wordEnd, p.allowFullwidthSpace)
		if wordEnd == pos || wordEnd < len(head) && !separator && !isDelimiter(rune(head[wordEnd])) || !declarationModifier(string(bytes.ToUpper(head[pos:wordEnd]))) {
			break
		}
		next := wordEnd
		for next < len(head) {
			size := inputSpaceSize(head, next, p.allowFullwidthSpace)
			if size == 0 {
				break
			}
			next += size
		}
		if p.rules[DeclarationWordSpacing] && next > wordEnd && next < len(head) {
			p.addEdit(i, base+wordEnd, base+next, []byte{' '}, DeclarationWordSpacing)
		}
		pos = next
	}
	*bodyStart = pos
}

func (p *formatter) syntaxDeclarationDimensions(i, modifiersStart, start, end int, allowRefOmitted bool, initializerStart, initializerEnd int, stringType bool) bool {
	spans := p.splitTopLevel(p.src, start, end, ',')
	modifiers, modifiersKnown := declarationModifierInfo(p.src, modifiersStart, start, p.allowFullwidthSpace)
	if declarationMacroNameSpan(p, spans[0]) {
		return false
	}
	left := trimInputLeft(p.src, spans[0].start, spans[0].end, p.allowFullwidthSpace)
	right := trimInputRight(p.src, left, spans[0].end, p.allowFullwidthSpace)
	name, err := p.lexSyntaxRange(left, right)
	if err != nil || len(name) != 1 || name[0].kind != exprIdentifier {
		p.syntaxProblem(i, start, "declaration requires an identifier before its dimensions or initializer")
		return false
	}
	hasOmitted := false
	for _, span := range spans[1:] {
		if trimInputLeft(p.src, span.start, span.end, p.allowFullwidthSpace) >= trimInputRight(p.src, span.start, span.end, p.allowFullwidthSpace) {
			hasOmitted = true
			continue
		}
		if !p.syntaxExpression(i, span.start, span.end, false) {
			return false
		}
	}
	if p.expressionParseEnabled() && p.kind == ERH && modifiersKnown && modifiers.chara {
		if third, ok := declarationCharadataDimensionsProven(p, spans); ok {
			p.warning(i, third.start, "suspicious_dimension", "CHARADATA declarations support at most two dimensions; macro expansion may change its role")
			return false
		}
	}
	skipDirectValidation := declarationHasRefModifier(p.src, modifiersStart, start, p.allowFullwidthSpace) && !allowRefOmitted
	if p.expressionParseEnabled() && !skipDirectValidation {
		total := int64(1)
		dimensionCount := 0
		capacityKnown := true
		for _, span := range spans[1:] {
			left := trimInputLeft(p.src, span.start, span.end, p.allowFullwidthSpace)
			right := trimInputRight(p.src, left, span.end, p.allowFullwidthSpace)
			if left == right {
				continue
			}
			tokens, err := p.lexSyntaxRange(left, right)
			if err != nil {
				continue
			}
			if commandMacroPossible(tokens) {
				break
			}
			dimensionCount++
			if !allowRefOmitted && dimensionCount > 3 {
				p.warning(i, left, "suspicious_dimension", "declaration has more than three dimensions; macro expansion may change its role")
				return false
			}
			value, kind := directIntegerLiteral(tokens)
			if kind == knownValueUnknown {
				capacityKnown = false
				continue
			}
			if kind == knownValueString {
				if allowRefOmitted {
					p.syntaxProblem(i, left, "reference declaration dimensions must be omitted or zero")
				} else {
					p.warning(i, left, "suspicious_dimension", "string literal in a dimension field; macro expansion may change its role")
				}
				return false
			}
			if allowRefOmitted {
				if value != 0 {
					p.syntaxProblem(i, left, "reference declaration dimensions must be omitted or zero")
					return false
				}
				continue
			}
			if value <= 0 || value > 1000000 {
				p.warning(i, left, "suspicious_dimension", "literal is outside dimension range 1..1000000; macro expansion may change its role")
				return false
			}
			if capacityKnown && total > 1000000/value {
				p.warning(i, left, "suspicious_dimension", "declaration exceeds the total dimension capacity of 1000000; macro expansion may change its role")
				return false
			}
			total *= value
		}
	}
	if hasOmitted {
		if !allowRefOmitted || len(spans)-1 > 3 {
			if p.expressionParseEnabled() && declarationOmittedDimensionProvenOrdinary(p, spans, modifiersStart, start) {
				for _, span := range spans[1:] {
					left := trimInputLeft(p.src, span.start, span.end, p.allowFullwidthSpace)
					right := trimInputRight(p.src, left, span.end, p.allowFullwidthSpace)
					if left == right {
						p.syntaxProblem(i, span.start, "ordinary declaration dimensions cannot be omitted")
						break
					}
				}
			}
			return false
		}
		for _, span := range spans[1:] {
			left := trimInputLeft(p.src, span.start, span.end, p.allowFullwidthSpace)
			right := trimInputRight(p.src, left, span.end, p.allowFullwidthSpace)
			if left == right {
				continue
			}
			if !p.literalZero(left, right) {
				return false
			}
		}
	}
	valuesProven := modifiersKnown && declarationModifierIssue(p.kind, modifiers) == "" && declarationValuesProven(p, start, end, initializerStart, initializerEnd, stringType, modifiers)
	if p.kind == ERH && valuesProven {
		if !p.registerGlobalDeclaration(i, left, name) {
			return false
		}
	}
	if p.kind == ERB && p.base > 0 && valuesProven {
		if !p.registerPrivateDeclaration(i, left, string(name[0].text)) {
			return false
		}
	}
	p.syntaxDeclarationCommas(i, start, end)
	return true
}

func (p *formatter) privateDeclarationNameMacroPossible(spans []syntaxSpan) bool {
	if len(spans) == 0 {
		return false
	}
	left := trimInputLeft(p.src, spans[0].start, spans[0].end, p.allowFullwidthSpace)
	right := trimInputRight(p.src, left, spans[0].end, p.allowFullwidthSpace)
	tokens, err := p.lexSyntaxRange(left, right)
	if err != nil || len(tokens) != 1 || tokens[0].kind != exprIdentifier {
		return false
	}
	r, _ := utf8.DecodeRune(tokens[0].text)
	return r <= 0xffff && unicode.IsDigit(r) && (r < '0' || r > '9')
}

func declarationOmittedDimensionProvenOrdinary(p *formatter, spans []syntaxSpan, modifiersStart, start int) bool {
	modifiers, known := declarationModifierInfo(p.src, modifiersStart, start, p.allowFullwidthSpace)
	if known && modifiers.ref {
		return false
	}
	forcedOrdinary := known && (modifiers.isConst || modifiers.static || modifiers.save || modifiers.chara || modifiers.global)
	positive := false
	for _, span := range spans[1:] {
		left := trimInputLeft(p.src, span.start, span.end, p.allowFullwidthSpace)
		right := trimInputRight(p.src, left, span.end, p.allowFullwidthSpace)
		if left == right {
			continue
		}
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil || commandMacroPossible(tokens) {
			return false
		}
		value, valid := constantIntegerValue(tokens)
		if !valid {
			return false
		}
		if value <= 0 {
			return false
		}
		positive = true
	}
	return positive || forcedOrdinary
}

func declarationCharadataDimensionsProven(p *formatter, spans []syntaxSpan) (syntaxSpan, bool) {
	count := 0
	var third syntaxSpan
	for _, span := range spans[1:] {
		left := trimInputLeft(p.src, span.start, span.end, p.allowFullwidthSpace)
		right := trimInputRight(p.src, left, span.end, p.allowFullwidthSpace)
		if left == right {
			return syntaxSpan{}, false
		}
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil || commandMacroPossibleForDialect(tokens, p.dialect) {
			return syntaxSpan{}, false
		}
		value, kind := directIntegerLiteral(tokens)
		if kind != knownValueNumeric || value <= 0 {
			return syntaxSpan{}, false
		}
		count++
		if count == 3 {
			third = span
		}
	}
	return third, count > 2
}

func declarationValuesProven(p *formatter, start, end, initializerStart, initializerEnd int, stringType bool, modifiers declarationModifierSet) bool {
	fields := p.splitTopLevel(p.src, start, end, ',')
	if len(fields) > 4 {
		return false
	}
	total := int64(1)
	for _, field := range fields[1:] {
		left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
		if left == right && modifiers.ref {
			continue
		}
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil {
			return false
		}
		value, valid := constantIntegerValue(tokens)
		if !valid {
			return false
		}
		if modifiers.ref {
			if value != 0 {
				return false
			}
			continue
		}
		if value <= 0 || value > 1000000 || total > 1000000/value {
			return false
		}
		total *= value
	}
	if initializerStart < 0 {
		return !modifiers.isConst
	}
	if modifiers.ref || modifiers.chara || len(fields) > 2 {
		return false
	}
	initializer := p.declarationInitializerFields(initializerStart, initializerEnd)
	count := int64(len(initializer))
	if count > 1000000 || len(fields) == 2 && (count > total || modifiers.isConst && count != total) {
		return false
	}
	for _, field := range initializer {
		left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil {
			return false
		}
		if stringType {
			for len(tokens) >= 3 && string(tokens[0].text) == "(" && string(tokens[len(tokens)-1].text) == ")" {
				tokens = tokens[1 : len(tokens)-1]
			}
			if len(tokens) != 1 || tokens[0].kind != exprString {
				return false
			}
		} else if _, valid := constantIntegerValue(tokens); !valid {
			return false
		}
	}
	return true
}

func declarationHasRefModifier(src []byte, start, end int, allowFullwidthSpace bool) bool {
	for start < end {
		for start < end {
			size := inputSpaceSize(src, start, allowFullwidthSpace)
			if size == 0 {
				break
			}
			start += size
		}
		if start >= end {
			return false
		}
		wordEnd := asciiWordEnd(src, start)
		if wordEnd == start || wordEnd > end {
			return false
		}
		word := string(bytes.ToUpper(src[start:wordEnd]))
		if word == "REF" {
			if wordEnd == end || inputSpaceSize(src, wordEnd, allowFullwidthSpace) != 0 || isDelimiter(rune(src[wordEnd])) {
				return true
			}
			return false
		}
		if !declarationModifier(word) || wordEnd >= end || inputSpaceSize(src, wordEnd, allowFullwidthSpace) == 0 {
			return false
		}
		start = wordEnd
	}
	return false
}

func declarationModifiersAllowRefOmissionWithSpace(src []byte, start, end int, allowFullwidthSpace bool) bool {
	hasRef := false
	hasDynamic := false
	for start < end {
		for start < end {
			size := inputSpaceSize(src, start, allowFullwidthSpace)
			if size == 0 {
				break
			}
			start += size
		}
		if start >= end {
			break
		}
		wordEnd := asciiWordEnd(src, start)
		if wordEnd == start || wordEnd > end {
			return false
		}
		switch string(bytes.ToUpper(src[start:wordEnd])) {
		case "REF":
			if hasRef {
				return false
			}
			hasRef = true
		case "DYNAMIC":
			if hasDynamic {
				return false
			}
			hasDynamic = true
		default:
			return false
		}
		start = wordEnd
	}
	return hasRef
}

func (p *formatter) literalZero(start, end int) bool {
	tokens, err := p.lexSyntaxRange(start, end)
	if err != nil {
		return false
	}
	value, kind := directIntegerLiteral(tokens)
	return kind == knownValueNumeric && value == 0
}

func (p *formatter) syntaxDeclarationCommas(i, start, end int) {
	spans := p.splitTopLevel(p.src, start, end, ',')
	for _, span := range spans[:max(0, len(spans)-1)] {
		p.syntaxCommaGap(i, span.end, end)
	}
}

func (p *formatter) syntaxDeclarationDelimiter(i, open, close int) {
	if !p.rules[DelimiterSpacing] || close <= open+1 {
		return
	}
	innerStart, innerEnd := open+1, close
	leftEnd := trimInputLeft(p.src, innerStart, innerEnd, p.allowFullwidthSpace)
	rightStart := trimInputRight(p.src, innerStart, innerEnd, p.allowFullwidthSpace)
	if leftEnd > rightStart {
		p.addEdit(i, innerStart, innerEnd, nil, DelimiterSpacing)
		return
	}
	if !mandatoryInlineMarker(p.src, leftEnd) {
		p.addEdit(i, innerStart, leftEnd, nil, DelimiterSpacing)
	}
	if !mandatoryInlineMarker(p.src[:rightStart], rightStart-3) {
		p.addEdit(i, rightStart, innerEnd, nil, DelimiterSpacing)
	}
}

func asciiWordEnd(src []byte, start int) int {
	for start < len(src) && ((src[start] >= 'A' && src[start] <= 'Z') || (src[start] >= 'a' && src[start] <= 'z')) {
		start++
	}
	return start
}

func declarationModifier(word string) bool {
	switch word {
	case "CONST", "DYNAMIC", "STATIC", "SAVEDATA", "CHARADATA", "REF", "GLOBAL":
		return true
	default:
		return false
	}
}

func headerPrefix(head []byte, start int) (int, int) {
	for i := start; i < len(head); i++ {
		if head[i] == '[' || head[i] == '(' || head[i] == ',' || head[i] == ';' {
			return i, i
		}
	}
	return len(head), -1
}

func declarationName(head []byte) string {
	if len(head) < 2 || head[0] != '#' {
		return ""
	}
	end := declarationWordEnd(head)
	if end < 0 {
		return ""
	}
	return string(bytes.ToUpper(head[1:end]))
}

func declarationWordEnd(head []byte) int {
	if len(head) < 2 || head[0] != '#' {
		return -1
	}
	for pos := 1; pos < len(head); pos++ {
		if halfSpace(head[pos]) || bytes.HasPrefix(head[pos:], fullwidthSpace) {
			return pos
		}
		switch head[pos] {
		case '.', '+', '-', '*', '/', '%', '=', '!', '<', '>', '|', '&', '^', '~', '#', '?', ')', '}', ']', ',', ':', '(', '{', '[', '$', '\\', '\'', '"', '@', ';':
			return pos
		}
	}
	return len(head)
}

func bareSharpFunction(head []byte, name string, allowFullwidthSpace bool) bool {
	wordEnd := declarationWordEnd(head)
	if wordEnd != len(name)+1 || !bytes.Equal(head[:wordEnd], []byte("#"+name)) {
		return false
	}
	return len(trimInput(head[wordEnd:], allowFullwidthSpace)) == 0
}
