package erabasic

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"
)

func (c *StructuralContext) ResetFunctionAttributes() {
	c.Function.PrivateDeclarations = make(map[string]bool)
	c.Function.SharpAttributes = 0
	c.Function.SharpFunctionKind = ""
	c.Function.SharpStateUnknown = false
	c.Function.LocalSizeSeen = [2]bool{}
}

func (c *StructuralContext) AuditSharpDeclaration(i int, head []byte, base int, optional, marked, continued bool) {
	if optional || marked || continued || c.Conditional || BytesHasAlias(head) {
		return
	}
	name := declarationName(head)
	if wordEnd := DeclarationWordEnd(head); wordEnd <= 1 || string(head[1:wordEnd]) != name {
		c.Function.SharpStateUnknown = true
		return
	}
	switch name {
	case "LOCALSIZE", "LOCALSSIZE":
		c.auditLocalSize(i, head, base, name)
	case "SINGLE", "LATER", "PRI", "ONLY":
		c.auditEventAttribute(i, head, base, name)
	case "FUNCTION", "FUNCTIONS":
		c.auditSharpFunction(i, head, base, name)
	case "DEFINE", "DIM", "DIMS":
		return
	default:
		c.Function.SharpStateUnknown = true
	}
}

func (c *StructuralContext) auditLocalSize(i int, head []byte, base int, name string) {
	if c.Function.SharpStateUnknown {
		return
	}
	index := 0
	if name == "LOCALSSIZE" {
		index = 1
	}
	wordEnd := DeclarationWordEnd(head)
	end, _ := SyntaxCommentEnd(head, 0, len(head), c.session(head))
	valueStart := TrimInputLeft(head, wordEnd, end, c.AllowFullwidthSpace)
	if valueStart == end {
		return
	}
	if event, exact := c.eventFunctionStatus(); event {
		if exact {
			c.report(i, base+wordEnd, "event_local_size", name+" is ignored in event functions", structuralWarning, false)
		}
		return
	}
	tokens, err := c.lex(base+valueStart, base+end)
	if err != nil {
		return
	}
	value, ok := constantIntegerValue(tokens)
	if !ok || value <= 0 || value >= 1<<31-1 {
		return
	}
	if c.Function.LocalSizeSeen[index] {
		c.report(i, base+wordEnd, "duplicate_local_size", name+" is duplicated in this function", structuralWarning, false)
	}
	c.Function.LocalSizeSeen[index] = true
}

func (c *StructuralContext) RegisterGlobalDeclaration(i, offset int, tokens []ExprToken) bool {
	if c.Function.SharpStateUnknown || len(tokens) != 1 || tokens[0].Kind != ExprIdentifier || !bytes.Equal(tokens[0].Text, bytes.ToUpper(tokens[0].Text)) {
		return true
	}
	name := string(tokens[0].Text)
	if c.Function.PrivateDeclarations[name] {
		c.report(i, offset, "duplicate_declaration", "global declaration name is duplicated: "+name, structuralWarning, false)
		return false
	}
	if c.InContinuationProxy {
		c.PendingDeclaration = name
	} else {
		c.Function.PrivateDeclarations[name] = true
	}
	return true
}

func (c *StructuralContext) auditEventAttribute(i int, head []byte, base int, name string) {
	if c.Function.SharpStateUnknown {
		return
	}
	event, exact := c.eventFunctionStatus()
	if event && !exact {
		return
	}
	if !event {
		c.report(i, base+1, "event_attribute_ignored", "#"+name+" is ignored outside event functions", structuralWarning, false)
		return
	}
	attribute := sharpAttributeFlag(name)
	if c.Function.SharpAttributes&attribute != 0 {
		c.report(i, base+1, "duplicate_attribute", "#"+name+" is duplicated", structuralWarning, false)
	}
	if name != "ONLY" && c.Function.SharpAttributes&sharpAttributeOnly != 0 {
		c.report(i, base+1, "conflicting_attribute", "#ONLY prevents #"+name+" from taking effect", structuralWarning, false)
		return
	}
	if name == "ONLY" && c.Function.SharpAttributes&(sharpAttributePri|sharpAttributeLater|SharpAttributeSingle) != 0 {
		c.report(i, base+1, "conflicting_attribute", "#ONLY overrides earlier event attributes", structuralWarning, false)
		c.Function.SharpAttributes &^= sharpAttributePri | sharpAttributeLater | SharpAttributeSingle
	}
	if (name == "PRI" && c.Function.SharpAttributes&sharpAttributeLater != 0) || (name == "LATER" && c.Function.SharpAttributes&sharpAttributePri != 0) {
		c.report(i, base+1, "conflicting_attribute", "#PRI and #LATER cannot be used together", structuralWarning, false)
	}
	c.Function.SharpAttributes |= attribute
}

func (c *StructuralContext) auditSharpFunction(i int, head []byte, base int, name string) {
	if c.Function.SharpStateUnknown {
		return
	}
	if !BareSharpFunction(head, name, c.AllowFullwidthSpace) {
		c.Function.SharpStateUnknown = true
		return
	}
	firstRune, _ := utf8.DecodeRuneInString(c.Function.FunctionName)
	if firstRune <= 0xffff && unicode.IsDigit(firstRune) {
		c.report(i, base+1, "unsupported_declaration", "digit-leading function cannot declare #"+name, StructuralUnsupported, true)
		return
	}
	if c.systemFunctionHeader() {
		c.report(i, base+1, "unsupported_declaration", "system function cannot declare #"+name, StructuralUnsupported, true)
		return
	}
	if c.Function.SharpFunctionKind == "" {
		c.Function.SharpFunctionKind = name
		return
	}
	if c.Function.SharpFunctionKind == name {
		c.report(i, base+1, "duplicate_declaration", "#"+name+" is duplicated", structuralWarning, false)
		return
	}
	c.report(i, base+1, "unsupported_declaration", "#FUNCTION and #FUNCTIONS cannot be combined", StructuralUnsupported, true)
}

func systemFunctionName(name string) bool {
	if event, exact := EventFunctionContext(name); event && exact {
		return true
	}
	switch name {
	case "SHOW_STATUS", "SHOW_USERCOM", "USERCOM", "SOURCE_CHECK", "CALLTRAINEND", "SHOW_JUEL", "SHOW_ABLUP_SELECT", "USERABLUP", "SHOW_SHOP", "SAVEINFO", "USERSHOP", "TITLE_LOADGAME", "SYSTEM_AUTOSAVE", "SYSTEM_TITLE", "SYSTEM_LOADEND":
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
	start := TrimInputLeft(head, 1, len(head), allowFullwidthSpace)
	end, _ := HeaderPrefix(head, start)
	end = TrimInputRight(head, start, end, allowFullwidthSpace)
	if end <= start || end > len(head) {
		return ""
	}
	return string(head[start:end])
}

func EventFunctionContext(name string) (bool, bool) {
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

func (c *StructuralContext) trustedReturnMode() string {
	if c.Kind != ERB || c.Function == nil || c.Function.FunctionStart < 0 || c.Function.SharpStateUnknown {
		return ""
	}
	switch c.Function.SharpFunctionKind {
	case "FUNCTION":
		return "numeric"
	case "FUNCTIONS":
		return "string"
	default:
		return "ordinary"
	}
}

func (c *StructuralContext) TrustedReturnExpressionType(tokens []ExprToken) KnownValueType {
	if ExpressionNeedsDeferral(tokens, c.Dialect) {
		return KnownValueUnknown
	}
	return KnownExpressionTypeForDialect(tokens, c.Dialect)
}

func (c *StructuralContext) ReturnCommandMessage(command string, value KnownValueType) string {
	switch c.trustedReturnMode() {
	case "numeric":
		switch command {
		case "RETURN":
			return "RETURN cannot be used in #FUNCTION"
		case "RETURNFORM":
			return "RETURNFORM cannot be used in #FUNCTION"
		case "RETURNF":
			if value == KnownValueString {
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
			if value == KnownValueNumeric {
				return "#FUNCTIONS requires a string RETURNF expression"
			}
		}
	case "ordinary":
		if command == "RETURNF" && value != KnownValueUnknown {
			return "RETURNF can only be used in #FUNCTION"
		}
	}
	return ""
}

func (c *StructuralContext) RegisterPrivateDeclaration(i, offset int, name string) bool {
	if c.Function.SharpStateUnknown || name == "" {
		return true
	}
	if c.Function.PrivateDeclarations[name] {
		c.report(i, offset, "duplicate_declaration", "declaration name is duplicated: "+name, structuralWarning, false)
		return false
	}
	if c.InContinuationProxy {
		c.PendingDeclaration = name
	} else {
		c.Function.PrivateDeclarations[name] = true
	}
	return true
}

func BytesHasAlias(head []byte) bool {
	return bytes.Contains(head, []byte("[["))
}
