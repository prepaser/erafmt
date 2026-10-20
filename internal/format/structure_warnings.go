package format

func (p *formatter) auditDollarLabel(i int, head []byte, base int, uncertain bool) {
	if p.base == 0 || !p.expressionParseEnabled() {
		return
	}
	if uncertain || p.conditional() || bytesHasAlias(head) {
		p.labelsUnknown = true
		return
	}
	if p.labelsUnknown {
		return
	}
	start := trimInputLeft(head, 1, len(head), p.allowFullwidthSpace)
	end, valid := labelIdentifierEndWithSession(head, start, p.newExpressionSession(head))
	if !valid || !labelTailWithSpace(head, '$', end, p.allowFullwidthSpace) {
		p.labelsUnknown = true
		return
	}
	name := string(head[start:end])
	if p.dollarLabels[name] {
		p.warning(i, base+start, "duplicate_label", "label is duplicated in this function: "+name)
		return
	}
	if p.dollarLabels == nil {
		p.dollarLabels = make(map[string]bool)
	}
	if p.inContinuationProxy {
		p.pendingLabel = name
	} else {
		p.dollarLabels[name] = true
	}
}

func (p *formatter) auditRepeatCount(i int, head []byte, wordEnd, base int) {
	if !p.expressionParseEnabled() {
		return
	}
	_, start, end, comment, hasArg := p.instructionBody(head, wordEnd)
	if comment >= 0 {
		end = comment
	}
	if !hasArg || len(trimInput(head[start:end], p.allowFullwidthSpace)) == 0 {
		p.warning(i, base+wordEnd, "nonpositive_repeat", "REPEAT count is zero; its body is skipped")
		return
	}
	parts := p.splitTopLevel(head, start, end, ',')
	if len(parts) == 0 {
		return
	}
	left, right := extraFieldBoundsWithSpace(head, parts[0], p.allowFullwidthSpace)
	tokens, err := p.lexSyntaxRange(base+left, base+right)
	if err != nil || expressionNeedsDeferral(tokens, p.dialect) {
		return
	}
	if value, known := constantIntegerValue(tokens); known && value <= 0 {
		p.warning(i, base+left, "nonpositive_repeat", "REPEAT count is nonpositive; its body is skipped")
	}
}

func (p *formatter) provenRepeatPositive(head []byte, wordEnd, base int) bool {
	if !p.expressionParseEnabled() {
		return false
	}
	_, start, end, comment, hasArg := p.instructionBody(head, wordEnd)
	if comment >= 0 {
		end = comment
	}
	if !hasArg || len(trimInput(head[start:end], p.allowFullwidthSpace)) == 0 {
		return false
	}
	parts := p.splitTopLevel(head, start, end, ',')
	if len(parts) == 0 {
		return false
	}
	left, right := extraFieldBoundsWithSpace(head, parts[0], p.allowFullwidthSpace)
	tokens, err := p.lexSyntaxRange(base+left, base+right)
	if err != nil || expressionNeedsDeferral(tokens, p.dialect) {
		return false
	}
	value, known := constantIntegerValue(tokens)
	return known && value > 0
}
