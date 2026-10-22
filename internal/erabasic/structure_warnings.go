package erabasic

func (c *StructuralContext) AuditDollarLabel(i int, head []byte, base int, uncertain bool) {
	if c.Base == 0 || !c.ExpressionEnabled {
		return
	}
	if uncertain || c.Conditional || BytesHasAlias(head) {
		c.Function.LabelsUnknown = true
		return
	}
	if c.Function.LabelsUnknown {
		return
	}
	start := TrimInputLeft(head, 1, len(head), c.AllowFullwidthSpace)
	end, valid := LabelIdentifierEndWithSession(head, start, c.session(head))
	if !valid || !labelTailWithSpace(head, '$', end, c.AllowFullwidthSpace) {
		c.Function.LabelsUnknown = true
		return
	}
	name := string(head[start:end])
	if c.Function.DollarLabels[name] {
		c.report(i, base+start, "duplicate_label", "label is duplicated in this function: "+name, structuralWarning, false)
		return
	}
	if c.Function.DollarLabels == nil {
		c.Function.DollarLabels = make(map[string]bool)
	}
	if c.InContinuationProxy {
		c.PendingLabel = name
	} else {
		c.Function.DollarLabels[name] = true
	}
}

func (c *StructuralContext) AuditRepeatCount(i int, head []byte, wordEnd, base int) {
	if !c.ExpressionEnabled {
		return
	}
	_, start, end, comment, hasArg := c.instructionBody(head, wordEnd)
	if comment >= 0 {
		end = comment
	}
	if !hasArg || len(TrimInput(head[start:end], c.AllowFullwidthSpace)) == 0 {
		c.report(i, base+wordEnd, "nonpositive_repeat", "REPEAT count is zero; its body is skipped", structuralWarning, false)
		return
	}
	parts := c.split(base+start, base+end, ',')
	if len(parts) == 0 {
		return
	}
	left, right := ExtraFieldBoundsWithSpace(c.Source, parts[0], c.AllowFullwidthSpace)
	tokens, err := c.lex(left, right)
	if err != nil || ExpressionNeedsDeferral(tokens, c.Dialect) {
		return
	}
	if value, known := constantIntegerValue(tokens); known && value <= 0 {
		c.report(i, base+left, "nonpositive_repeat", "REPEAT count is nonpositive; its body is skipped", structuralWarning, false)
	}
}

func (c *StructuralContext) ProvenRepeatPositive(head []byte, wordEnd, base int) bool {
	if !c.ExpressionEnabled {
		return false
	}
	_, start, end, comment, hasArg := c.instructionBody(head, wordEnd)
	if comment >= 0 {
		end = comment
	}
	if !hasArg || len(TrimInput(head[start:end], c.AllowFullwidthSpace)) == 0 {
		return false
	}
	parts := c.split(base+start, base+end, ',')
	if len(parts) == 0 {
		return false
	}
	left, right := ExtraFieldBoundsWithSpace(c.Source, parts[0], c.AllowFullwidthSpace)
	tokens, err := c.lex(left, right)
	if err != nil || ExpressionNeedsDeferral(tokens, c.Dialect) {
		return false
	}
	value, known := constantIntegerValue(tokens)
	return known && value > 0
}
