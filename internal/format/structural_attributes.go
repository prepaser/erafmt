package format

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"
)

func (p *formatter) resetFunctionAttributes() {
	p.privateDeclarations = make(map[string]bool)
	p.sharpAttributes = 0
	p.sharpFunctionKind = ""
	p.sharpStateUnknown = false
	p.localSizeSeen = [2]bool{}
}

func (p *formatter) auditSharpDeclaration(i int, head []byte, base int, optional, marked, continued bool) {
	if optional || marked || continued || p.conditional() || bytesHasAlias(head) {
		return
	}
	name := declarationName(head)
	if wordEnd := declarationWordEnd(head); wordEnd <= 1 || string(head[1:wordEnd]) != name {
		p.sharpStateUnknown = true
		return
	}
	switch name {
	case "LOCALSIZE", "LOCALSSIZE":
		p.auditLocalSize(i, head, base, name)
	case "SINGLE", "LATER", "PRI", "ONLY":
		p.auditEventAttribute(i, head, base, name)
	case "FUNCTION", "FUNCTIONS":
		p.auditSharpFunction(i, head, base, name)
	case "DEFINE", "DIM", "DIMS":
		return
	default:
		p.sharpStateUnknown = true
	}
}

func (p *formatter) auditLocalSize(i int, head []byte, base int, name string) {
	if p.sharpStateUnknown {
		return
	}
	index := 0
	if name == "LOCALSSIZE" {
		index = 1
	}
	wordEnd := declarationWordEnd(head)
	end, _ := p.syntaxCommentEnd(head, 0, len(head))
	valueStart := trimInputLeft(head, wordEnd, end, p.allowFullwidthSpace)
	if valueStart == end {
		return
	}
	if event, exact := p.eventFunctionStatus(); event {
		if exact {
			p.warning(i, base+wordEnd, "event_local_size", name+" is ignored in event functions")
		}
		return
	}
	tokens, err := p.lexSyntaxRange(base+valueStart, base+end)
	if err != nil {
		return
	}
	value, ok := constantIntegerValue(tokens)
	if !ok || value <= 0 || value >= 1<<31-1 {
		return
	}
	if p.localSizeSeen[index] {
		p.warning(i, base+wordEnd, "duplicate_local_size", name+" is duplicated in this function")
	}
	p.localSizeSeen[index] = true
}

func (p *formatter) registerGlobalDeclaration(i, offset int, tokens []exprToken) bool {
	if p.sharpStateUnknown || len(tokens) != 1 || tokens[0].kind != exprIdentifier || !bytes.Equal(tokens[0].text, bytes.ToUpper(tokens[0].text)) {
		return true
	}
	name := string(tokens[0].text)
	if p.privateDeclarations[name] {
		p.warning(i, offset, "duplicate_declaration", "global declaration name is duplicated: "+name)
		return false
	}
	if p.inContinuationProxy {
		p.pendingDeclaration = name
	} else {
		p.privateDeclarations[name] = true
	}
	return true
}

func (p *formatter) auditEventAttribute(i int, head []byte, base int, name string) {
	if p.sharpStateUnknown {
		return
	}
	event, exact := p.eventFunctionStatus()
	if event && !exact {
		return
	}
	if !event {
		p.warning(i, base+1, "event_attribute_ignored", "#"+name+" is ignored outside event functions")
		return
	}
	attribute := sharpAttributeFlag(name)
	if p.sharpAttributes&attribute != 0 {
		p.warning(i, base+1, "duplicate_attribute", "#"+name+" is duplicated")
	}
	if name != "ONLY" && p.sharpAttributes&sharpAttributeOnly != 0 {
		p.warning(i, base+1, "conflicting_attribute", "#ONLY prevents #"+name+" from taking effect")
		return
	}
	if name == "ONLY" && p.sharpAttributes&(sharpAttributePri|sharpAttributeLater|sharpAttributeSingle) != 0 {
		p.warning(i, base+1, "conflicting_attribute", "#ONLY overrides earlier event attributes")
		p.sharpAttributes &^= sharpAttributePri | sharpAttributeLater | sharpAttributeSingle
	}
	if (name == "PRI" && p.sharpAttributes&sharpAttributeLater != 0) || (name == "LATER" && p.sharpAttributes&sharpAttributePri != 0) {
		p.warning(i, base+1, "conflicting_attribute", "#PRI and #LATER cannot be used together")
	}
	p.sharpAttributes |= attribute
}

func (p *formatter) auditSharpFunction(i int, head []byte, base int, name string) {
	if p.sharpStateUnknown {
		return
	}
	if !bareSharpFunction(head, name, p.allowFullwidthSpace) {
		p.sharpStateUnknown = true
		return
	}
	firstRune, _ := utf8.DecodeRuneInString(p.functionName)
	if firstRune <= 0xffff && unicode.IsDigit(firstRune) {
		p.problem(i, base+1, "unsupported_declaration", "digit-leading function cannot declare #"+name, Unsupported)
		p.syntaxOpaque(i)
		return
	}
	if p.systemFunctionHeader() {
		p.problem(i, base+1, "unsupported_declaration", "system function cannot declare #"+name, Unsupported)
		p.syntaxOpaque(i)
		return
	}
	if p.sharpFunctionKind == "" {
		p.sharpFunctionKind = name
		return
	}
	if p.sharpFunctionKind == name {
		p.warning(i, base+1, "duplicate_declaration", "#"+name+" is duplicated")
		return
	}
	p.problem(i, base+1, "unsupported_declaration", "#FUNCTION and #FUNCTIONS cannot be combined", Unsupported)
	p.syntaxOpaque(i)
}

func (p *formatter) systemFunctionHeader() bool {
	if p.kind != ERB {
		return false
	}
	if p.functionName != "" {
		return p.systemFunctionKnown
	}
	if p.functionStart < 0 || p.functionStart >= len(p.lines) || p.inContinuationProxy {
		return false
	}
	head, _, _, _ := p.headOf(lBody(p.src, p.lines[p.functionStart]))
	start := trimInputLeft(head, 1, len(head), p.allowFullwidthSpace)
	end, _ := headerPrefix(head, start)
	end = trimInputRight(head, start, end, p.allowFullwidthSpace)
	if end <= start || end > len(head) || string(head[start:end]) != strings.ToUpper(string(head[start:end])) {
		return false
	}
	return systemFunctionName(string(head[start:end]))
}

func systemFunctionName(name string) bool {
	if event, exact := eventFunctionContext(name); event && exact {
		return true
	}
	switch name {
	case "SHOW_STATUS", "SHOW_USERCOM", "USERCOM", "SOURCE_CHECK", "CALLTRAINEND", "SHOW_JUEL", "SHOW_ABLUP_SELECT", "USERABLUP", "SHOW_SHOP", "SAVEINFO", "USERSHOP",
		"TITLE_LOADGAME", "SYSTEM_AUTOSAVE", "SYSTEM_TITLE", "SYSTEM_LOADEND":
		return true
	}
	if strings.HasPrefix(name, "COM") && allDigits(name[3:]) {
		return true
	}
	if strings.HasPrefix(name, "COM_ABLE") && allDigits(name[len("COM_ABLE"):]) {
		return true
	}
	return strings.HasPrefix(name, "ABLUP") && allDigits(name[len("ABLUP"):])
}

func functionHeaderName(head []byte, allowFullwidthSpace bool) string {
	if len(head) < 2 || head[0] != '@' {
		return ""
	}
	start := trimInputLeft(head, 1, len(head), allowFullwidthSpace)
	end, _ := headerPrefix(head, start)
	end = trimInputRight(head, start, end, allowFullwidthSpace)
	if end <= start || end > len(head) {
		return ""
	}
	return string(head[start:end])
}

func eventFunctionContext(name string) (bool, bool) {
	upper := strings.ToUpper(name)
	switch upper {
	case "EVENTFIRST", "EVENTTRAIN", "EVENTSHOP", "EVENTBUY", "EVENTCOM", "EVENTTURNEND", "EVENTCOMEND", "EVENTEND", "EVENTLOAD":
		return true, name == upper
	default:
		return false, false
	}
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (p *formatter) trustedReturnMode() string {
	if p.kind != ERB || p.functionStart < 0 || p.sharpStateUnknown {
		return ""
	}
	switch p.sharpFunctionKind {
	case "FUNCTION":
		return "numeric"
	case "FUNCTIONS":
		return "string"
	default:
		return "ordinary"
	}
}

func (p *formatter) trustedReturnExpressionType(tokens []exprToken) knownValueType {
	if expressionNeedsDeferral(tokens, p.dialect) {
		return knownValueUnknown
	}
	return knownExpressionTypeForDialect(tokens, p.dialect)
}

func (p *formatter) returnCommandMessage(command string, value knownValueType) string {
	switch p.trustedReturnMode() {
	case "numeric":
		switch command {
		case "RETURN":
			return "RETURN cannot be used in #FUNCTION"
		case "RETURNFORM":
			return "RETURNFORM cannot be used in #FUNCTION"
		case "RETURNF":
			if value == knownValueString {
				return "#FUNCTION requires a numeric RETURNF expression"
			}
		}
	case "string":
		switch command {
		case "RETURN":
			return "RETURN cannot be used in #FUNCTIONS"
		case "RETURNFORM":
			return "RETURNFORM cannot be used in #FUNCTIONS"
		case "RETURNF":
			if value == knownValueNumeric {
				return "#FUNCTIONS requires a string RETURNF expression"
			}
		}
	case "ordinary":
		if command == "RETURNF" && value != knownValueUnknown {
			return "RETURNF can only be used in #FUNCTION"
		}
	}
	return ""
}

func (p *formatter) registerPrivateDeclaration(i, offset int, name string) bool {
	if p.sharpStateUnknown || name == "" {
		return true
	}
	if p.privateDeclarations[name] {
		p.warning(i, offset, "duplicate_declaration", "declaration name is duplicated: "+name)
		return false
	}
	if p.inContinuationProxy {
		p.pendingDeclaration = name
	} else {
		p.privateDeclarations[name] = true
	}
	return true
}

func bytesHasAlias(head []byte) bool {
	return strings.Contains(string(head), "[[")
}
