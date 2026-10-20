package format

import (
	"bytes"
	"strings"
)

type graphicsFieldKind uint8

const (
	graphicsInvalid graphicsFieldKind = iota
	graphicsString
	graphicsNumeric
	graphicsNumericCertain
	graphicsNumericUnknown
)

type graphicsExpressionGroup struct {
	open byte
	call bool
}

func (p *formatter) syntaxGraphics(i int, head []byte, base int, command string, wordEnd int) (int, bool) {
	if command != "PRINT_IMG" && command != "PRINT_RECT" && command != "PRINT_SPACE" {
		return -1, false
	}
	start, end, comment, ok := p.registeredBody(head, wordEnd)
	if !ok {
		p.syntaxOpaque(i)
		return -1, true
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	mark := len(p.edits)
	diagMark := len(p.diagnostics)
	if start >= end || trimInputLeft(head, start, end, p.allowFullwidthSpace) >= trimInputRight(head, start, end, p.allowFullwidthSpace) {
		return p.graphicsInvalid(i, base+wordEnd, mark, diagMark, "missing graphics argument")
	}
	codeEnd := registeredCodeEnd(base, end, comment)
	session, offset := p.expressionSessionFor(base+start, codeEnd)
	defer p.addFormAdvisories(i, session, offset, offset+codeEnd-base-start, base+start)
	parts := p.splitTopLevel(p.src, base+start, codeEnd, ',')
	if p.graphicsHasDebugMarker(parts) {
		p.syntaxOpaque(i)
		return -1, true
	}
	if !p.graphicsFields(i, command, parts) {
		return p.graphicsInvalid(i, base+start, mark, diagMark, "unsupported graphics argument shape")
	}
	if p.graphicsLiteralRangeWarnings(i, command, parts) {
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1, true
	}
	if !p.graphicsCommaFormattingDeferred(command, parts) {
		p.syntaxRegisteredCommaGaps(i, parts, codeEnd)
	}
	codeEnd = trimInputRight(p.src, base+start, codeEnd, p.allowFullwidthSpace)
	p.setCodeEnd(i, codeEnd)
	return codeEnd, true
}

func (p *formatter) graphicsLiteralRangeWarnings(i int, command string, parts []syntaxSpan) bool {
	if p.dialect == EMEE || command == "PRINT_IMG" || !p.expressionParseEnabled() {
		return false
	}
	warned := false
	for _, field := range graphicsEffectiveFields(p.src, parts, p.allowFullwidthSpace) {
		left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
		tokens, opaque, _ := p.graphicsFieldTokens(left, right)
		if opaque {
			continue
		}
		value, kind := directIntegerLiteral(tokens)
		if kind == knownValueNumeric && (value < 0 || value > 1<<31-1) {
			p.warning(i, left, "suspicious_graphics_range", command+" literal is outside the base rendering range 0..2147483647")
			warned = true
		}
	}
	return warned
}

func (p *formatter) graphicsHasDebugMarker(parts []syntaxSpan) bool {
	for _, field := range parts {
		left := p.trimInputLeft(field.start, field.end)
		right := p.trimInputRight(left, field.end)
		if left < right && p.graphicsDebugMarker(left, right) {
			return true
		}
	}
	return false
}

func (p *formatter) graphicsCommaFormattingDeferred(command string, parts []syntaxSpan) bool {
	if command != "PRINT_IMG" || p.dialect != EMEE {
		return false
	}
	parts = graphicsEffectiveFields(p.src, parts, p.allowFullwidthSpace)
	for _, field := range parts[1:] {
		if p.graphicsFieldType(field) == graphicsNumericUnknown {
			return true
		}
	}
	return false
}

func (p *formatter) graphicsInvalid(i, offset, mark, diagMark int, message string) (int, bool) {
	p.truncateEdits(mark)
	if (p.expressionParseEnabled() || p.rules[PixelUnitSpacing] && p.dialect == EMEE) && len(p.diagnostics) == diagMark {
		p.syntaxProblem(i, offset, message)
	}
	p.syntaxOpaque(i)
	return -1, true
}

func (p *formatter) graphicsFields(i int, command string, parts []syntaxSpan) bool {
	parts = graphicsEffectiveFields(p.src, parts, p.allowFullwidthSpace)
	switch command {
	case "PRINT_IMG":
		return p.graphicsImageFields(i, parts)
	case "PRINT_RECT":
		if len(parts) != 1 && len(parts) != 4 {
			return false
		}
		for _, field := range parts {
			if !p.graphicsNumericField(i, field, p.dialect == EMEE) {
				return false
			}
		}
		return true
	case "PRINT_SPACE":
		return len(parts) == 1 && p.graphicsNumericField(i, parts[0], p.dialect == EMEE)
	default:
		return false
	}
}

func (p *formatter) graphicsImageFields(i int, parts []syntaxSpan) bool {
	editMark := len(p.edits)
	if len(parts) == 0 {
		return false
	}
	if p.dialect != EMEE {
		return len(parts) == 1 && p.graphicsKnownStringField(i, parts[0])
	}
	if len(parts) > 6 {
		return false
	}
	if !p.graphicsKnownStringField(i, parts[0]) {
		return false
	}
	names, numerics := 1, 0
	numeric := false
	for _, field := range parts[1:] {
		kind := p.graphicsFieldType(field)
		switch kind {
		case graphicsString:
			if numeric || names == 3 {
				return false
			}
			if !p.graphicsStringField(i, field) {
				return false
			}
			names++
		case graphicsNumericCertain:
			if numerics == 3 {
				return false
			}
			numeric = true
			numerics++
			if !p.graphicsNumericField(i, field, true) {
				return false
			}
		case graphicsNumericUnknown:
			p.truncateEdits(editMark)
			return true
		default:
			return false
		}
	}
	return names <= 3
}

func graphicsEffectiveFields(src []byte, parts []syntaxSpan, allowFullwidthSpace bool) []syntaxSpan {
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		left := trimInputLeft(src, last.start, last.end, allowFullwidthSpace)
		right := trimInputRight(src, left, last.end, allowFullwidthSpace)
		if left == right {
			return parts[:len(parts)-1]
		}
	}
	return parts
}

func (p *formatter) graphicsFieldType(field syntaxSpan) graphicsFieldKind {
	left := p.trimInputLeft(field.start, field.end)
	right := p.trimInputRight(left, field.end)
	if left >= right {
		return graphicsInvalid
	}
	exprEnd, _, hasUnit := p.graphicsUnitSpan(left, right)
	if !hasUnit {
		_, _, candidate := p.graphicsUnitCandidate(left, right)
		if candidate {
			return graphicsInvalid
		}
		exprEnd = right
	}
	tokens, opaque, marked := p.graphicsFieldTokens(left, exprEnd)
	if opaque {
		return graphicsNumericUnknown
	}
	if len(tokens) == 0 {
		return graphicsInvalid
	}
	if constantExpressionsValid(tokens) != nil {
		return graphicsInvalid
	}
	if value := graphicsKnownType(tokens); value == knownValueString {
		return graphicsString
	}
	if value := graphicsKnownType(tokens); value == knownValueNumeric {
		return graphicsNumericCertain
	}
	if !marked && p.graphicsStringSpan(left, exprEnd) {
		return graphicsString
	}
	if !p.graphicsExpressionComplete(left, exprEnd) {
		return graphicsInvalid
	}
	for _, token := range tokens {
		if token.kind == exprIdentifier {
			return graphicsNumericUnknown
		}
	}
	return graphicsNumericUnknown
}

func (p *formatter) graphicsStringField(i int, field syntaxSpan) bool {
	left := trimInputLeft(p.src, field.start, field.end, p.allowFullwidthSpace)
	right := trimInputRight(p.src, left, field.end, p.allowFullwidthSpace)
	if left >= right {
		return false
	}
	tokens, opaque, marked := p.graphicsFieldTokens(left, right)
	if opaque {
		return true
	}
	if len(tokens) == 0 || graphicsKnownType(tokens) != knownValueString {
		return false
	}
	if p.expressionParseEnabled() && !marked {
		if err := provenExpressionShape(tokens, p.dialect, false); err != nil {
			return false
		}
	}
	if !marked && !p.graphicsStringSpan(left, right) && p.dialect != EMEE {
		return false
	}
	if marked {
		return true
	}
	if p.expressionParseEnabled() && !p.graphicsStringSpan(left, right) {
		return p.syntaxExpression(i, left, right, false)
	}
	return true
}

func (p *formatter) graphicsKnownStringField(i int, field syntaxSpan) bool {
	left := trimInputLeft(p.src, field.start, field.end, p.allowFullwidthSpace)
	right := trimInputRight(p.src, left, field.end, p.allowFullwidthSpace)
	if left >= right {
		return false
	}
	tokens, opaque, marked := p.graphicsFieldTokens(left, right)
	if opaque {
		return true
	}
	if len(tokens) == 0 {
		return false
	}
	if p.expressionParseEnabled() && !marked {
		if err := provenExpressionShape(tokens, p.dialect, false); err != nil {
			return false
		}
	}
	if p.expressionParseEnabled() && constantExpressionsValid(tokens) != nil {
		return false
	}
	if p.dialect != EMEE && p.expressionParseEnabled() && graphicsKnownType(tokens) == knownValueNumeric {
		return false
	}
	if p.expressionRulesEnabled() && !marked {
		session, offset := p.expressionSessionFor(left, right)
		edits, err := expressionEditsWithSession(session, expressionSourceOriginal, p.src[left:right], offset, false, p.rules, true)
		if err != nil {
			return false
		}
		p.addSyntaxEdits(i, left, edits)
	}
	return true
}

func (p *formatter) graphicsStringSpan(left, right int) bool {
	if left >= right {
		return false
	}
	src := p.src[left:right]
	if src[0] == '"' {
		end := quotedEnd(src, 0)
		return end == len(src)
	}
	mode := formMode(0)
	switch {
	case bytes.HasPrefix(src, []byte(`@"`)):
		mode = formQuoted
	case bytes.HasPrefix(src, []byte(`\@`)):
		mode = formConditional
	default:
		return false
	}
	session, offset := p.expressionSessionFor(left, right)
	end, err := session.scanFormAt(expressionSourceOriginal, offset, offset+right-left, mode)
	return err == nil && end == offset+right-left
}

func (p *formatter) graphicsNumericField(i int, field syntaxSpan, allowUnit bool) bool {
	left := trimInputLeft(p.src, field.start, field.end, p.allowFullwidthSpace)
	right := trimInputRight(p.src, left, field.end, p.allowFullwidthSpace)
	if left >= right {
		return false
	}
	exprEnd, unitStart, hasUnit := p.graphicsUnitSpan(left, right)
	if hasUnit && !allowUnit {
		return false
	}
	_, _, candidate := p.graphicsUnitCandidate(left, right)
	if !hasUnit && candidate {
		return false
	}
	if exprEnd <= left {
		return false
	}
	tokens, opaque, marked := p.graphicsFieldTokens(left, exprEnd)
	if opaque {
		return true
	}
	if len(tokens) == 0 {
		return false
	}
	if p.expressionParseEnabled() && !marked {
		if err := provenExpressionShape(tokens, p.dialect, false); err != nil {
			return false
		}
	}
	if provenClosedOperandAdjacency(tokens, p.dialect) != nil {
		return false
	}
	if provenClosedOperandOperators(tokens, p.dialect) != nil || provenQualifiedBuiltinReference(tokens, p.dialect) != nil {
		return false
	}
	if err := adjacentExpressionOperands(tokens, p.dialect); err != nil {
		return false
	}
	if (p.expressionParseEnabled() || p.rules[PixelUnitSpacing] && allowUnit) && constantExpressionsValid(tokens) != nil {
		return false
	}
	known := graphicsKnownType(tokens)
	if p.dialect != EMEE && known != knownValueNumeric && !graphicsNumericOperandsValid(tokens) {
		if known != knownValueUnknown || !graphicsExpressionMayBeUnknown(tokens) || !p.graphicsExpressionComplete(left, exprEnd) {
			return false
		}
	}
	if graphicsConstantSyntax(tokens) && !p.graphicsExpressionComplete(left, exprEnd) {
		return false
	}
	if p.dialect != EMEE && p.expressionParseEnabled() && known == knownValueString {
		return false
	}
	last := tokens[len(tokens)-1]
	if last.kind == exprOperator && string(last.text) != "++" && string(last.text) != "--" {
		return false
	}
	if p.expressionRulesEnabled() && !marked {
		session, offset := p.expressionSessionFor(left, exprEnd)
		edits, err := expressionEditsWithSession(session, expressionSourceOriginal, p.src[left:exprEnd], offset, false, p.rules, true)
		if err != nil {
			return false
		}
		p.addSyntaxEdits(i, left, edits)
	}
	if hasUnit && p.rules[PixelUnitSpacing] && !mandatoryInlineMarker(p.src[:exprEnd], exprEnd-3) {
		p.addEdit(i, exprEnd, unitStart, []byte{' '}, PixelUnitSpacing)
	}
	return true
}

func graphicsExpressionMayBeUnknown(tokens []exprToken) bool {
	for _, token := range tokens {
		if token.kind == exprIdentifier || token.kind == exprForm {
			return true
		}
	}
	return false
}

func graphicsKnownType(tokens []exprToken) knownValueType {
	for _, token := range tokens {
		if token.kind == exprMarker {
			return knownValueUnknown
		}
	}
	if commandMacroPossible(tokens) {
		return knownValueUnknown
	}
	return knownExpressionType(tokens)
}

func (p *formatter) graphicsFieldTokens(left, right int) ([]exprToken, bool, bool) {
	raw := p.src[left:right]
	if !bytes.Contains(raw, []byte(";!;")) && !bytes.Contains(raw, []byte(";#;")) {
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil {
			return nil, false, false
		}
		return tokens, false, false
	}
	if p.graphicsDebugMarker(left, right) {
		return nil, true, false
	}
	tokens, err := p.lexSyntaxRange(left, right)
	if err != nil {
		return nil, false, false
	}
	marked := false
	for _, token := range tokens {
		if token.kind == exprMarker {
			marked = true
			break
		}
	}
	if !marked {
		return tokens, false, false
	}
	return stripCommandMarkers(tokens), false, true
}

func (p *formatter) graphicsDebugMarker(left, right int) bool {
	for i := left; i < right; {
		next, atom := p.syntaxAtomEnd(p.src, i, right)
		if atom {
			if next < 0 {
				return false
			}
			i = next
			continue
		}
		if i+3 <= right && string(p.src[i:i+3]) == ";#;" {
			return true
		}
		i++
	}
	return false
}

func graphicsNumericOperandsValid(tokens []exprToken) bool {
	groups := make([]graphicsExpressionGroup, 0, 2)
	wantOperand := true
	for _, token := range tokens {
		switch token.kind {
		case exprIdentifier, exprNumber:
			wantOperand = false
		case exprString, exprForm:
			if !graphicsInsideCall(groups) {
				return false
			}
			wantOperand = false
		case exprOperator:
			op := string(token.text)
			if (op == "++" || op == "--") && !wantOperand {
				continue
			}
			wantOperand = true
		case exprSymbol:
			switch token.text[0] {
			case '(', '[':
				groups = append(groups, graphicsExpressionGroup{open: token.text[0], call: token.text[0] == '(' && !wantOperand})
				wantOperand = true
			case ')', ']':
				if len(groups) == 0 {
					return false
				}
				current := groups[len(groups)-1]
				if (current.open != '(' || token.text[0] != ')') && (current.open != '[' || token.text[0] != ']') {
					return false
				}
				groups = groups[:len(groups)-1]
				wantOperand = false
			case ',':
				wantOperand = true
			case ':', '@', '.':
				wantOperand = true
			default:
				wantOperand = true
			}
		}
	}
	return len(groups) == 0
}

func graphicsInsideCall(groups []graphicsExpressionGroup) bool {
	for _, current := range groups {
		if current.call {
			return true
		}
	}
	return false
}

func (p *formatter) graphicsUnitSpan(left, right int) (exprEnd, unitStart int, ok bool) {
	session, offset := p.expressionSessionFor(left, right)
	return graphicsUnitSpanWithSession(p.src, left, right, p.allowFullwidthSpace, session, offset)
}

func graphicsUnitSpanWithSession(src []byte, left, right int, allowFullwidthSpace bool, session *expressionSession, offset int) (exprEnd, unitStart int, ok bool) {
	exprEnd, unitStart, ok = graphicsUnitCandidateWithSession(src, left, right, allowFullwidthSpace, session, offset)
	if !ok || !graphicsExpressionCompleteWithSession(src, left, exprEnd, allowFullwidthSpace, session, offset, left) {
		return right, right, false
	}
	return exprEnd, unitStart, true
}

func (p *formatter) graphicsUnitCandidate(left, right int) (exprEnd, unitStart int, ok bool) {
	session, offset := p.expressionSessionFor(left, right)
	return graphicsUnitCandidateWithSession(p.src, left, right, p.allowFullwidthSpace, session, offset)
}

func graphicsUnitCandidateWithSession(src []byte, left, right int, allowFullwidthSpace bool, session *expressionSession, offset int) (exprEnd, unitStart int, ok bool) {
	if right-left < 2 || !strings.EqualFold(string(src[right-2:right]), "px") {
		return right, right, false
	}
	unitStart = right - 2
	if unitStart <= left {
		return right, right, false
	}
	tokens, err := session.lexRange(expressionSourceOriginal, offset, offset+right-left, left)
	if err != nil || len(tokens) < 2 {
		return right, right, false
	}
	unit := tokens[len(tokens)-1]
	if unit.kind != exprIdentifier || unit.off != unitStart || len(unit.text) != 2 {
		return right, right, false
	}
	exprEnd = trimInputRight(src, left, unitStart, allowFullwidthSpace)
	return exprEnd, unitStart, exprEnd > left
}

func graphicsConstantSyntax(tokens []exprToken) bool {
	for _, t := range tokens {
		if t.kind != exprNumber && t.kind != exprOperator && t.kind != exprSymbol {
			return false
		}
	}
	return true
}

func (p *formatter) graphicsExpressionComplete(left, right int) bool {
	session, offset := p.expressionSessionFor(left, right)
	return graphicsExpressionCompleteWithSession(p.src, left, right, p.allowFullwidthSpace, session, offset, left)
}

func graphicsExpressionCompleteWithSession(src []byte, left, right int, allowFullwidthSpace bool, session *expressionSession, offset, base int) bool {
	tokens, err := session.lexRange(expressionSourceOriginal, offset, offset+right-left, base)
	if err != nil || len(tokens) == 0 {
		return false
	}
	strict := graphicsConstantSyntax(tokens)
	type group struct {
		open    byte
		call    bool
		content bool
		ternary int
	}
	var groups []group
	wantOperand := true
	ternary := 0
	for _, token := range tokens {
		switch token.kind {
		case exprIdentifier, exprNumber, exprString, exprForm:
			if !wantOperand {
				return false
			}
			wantOperand = false
			if len(groups) > 0 {
				groups[len(groups)-1].content = true
			}
		case exprOperator:
			op := string(token.text)
			if len(groups) > 0 {
				groups[len(groups)-1].content = true
			}
			if wantOperand {
				switch op {
				case "+", "-", "!", "~", "++", "--":
				default:
					return false
				}
				continue
			}
			if op == "++" || op == "--" {
				continue
			}
			if op == "!" || op == "~" {
				return false
			}
			switch op {
			case "+", "-", "*", "/", "%", "==", ">", "<", ">=", "<=", "!=", "&&", "||", "^^", "!&", "!|", "&", "|", "^", ">>", "<<":
				wantOperand = true
			case "?":
				ternary++
				wantOperand = true
			case "#":
				boundary := 0
				if len(groups) > 0 {
					boundary = groups[len(groups)-1].ternary
				}
				if ternary == 0 || strict && ternary <= boundary {
					return false
				}
				ternary--
				wantOperand = true
			default:
				return false
			}
		case exprSymbol:
			switch token.text[0] {
			case '(', '[':
				if len(groups) > 0 {
					groups[len(groups)-1].content = true
				}
				groups = append(groups, group{open: token.text[0], call: token.text[0] == '(' && !wantOperand, ternary: ternary})
				wantOperand = true
			case ')', ']':
				if len(groups) == 0 || (token.text[0] == ')' && groups[len(groups)-1].open != '(') || (token.text[0] == ']' && groups[len(groups)-1].open != '[') {
					return false
				}
				current := groups[len(groups)-1]
				if strict && ternary != current.ternary {
					return false
				}
				if wantOperand && !(token.text[0] == ')' && current.call && !current.content) {
					return false
				}
				groups = groups[:len(groups)-1]
				wantOperand = false
				if len(groups) > 0 {
					groups[len(groups)-1].content = true
				}
			case ',':
				if len(groups) == 0 || groups[len(groups)-1].open != '(' || !groups[len(groups)-1].call || strict && ternary != groups[len(groups)-1].ternary || (wantOperand && groups[len(groups)-1].content) {
					return false
				}
				groups[len(groups)-1].content = false
				wantOperand = true
			case ':', '@':
				if wantOperand {
					return false
				}
				if len(groups) > 0 {
					groups[len(groups)-1].content = true
				}
				wantOperand = true
			default:
				return false
			}
		}
	}
	return !wantOperand && len(groups) == 0 && ternary == 0
}
