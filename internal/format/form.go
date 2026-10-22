package format

import (
	"bytes"

	"github.com/prepaser/erafmt/internal/erabasic"
)

type formFormatter struct {
	erabasic.FormAnalyzer
	rules  ruleSet
	edits  []edit
	verify bool
}

func formEditsWithSession(session *erabasic.ExpressionSession, source erabasic.ExpressionSourceID, src []byte, cacheStart int, rules ruleSet, mode erabasic.FormMode, verify bool) ([]edit, error) {
	if !session.ValidUTF8(source) {
		return nil, &erabasic.ExprError{Msg: "invalid UTF-8"}
	}
	if session.SkipFormTypes {
		return nil, nil
	}
	if session.HasAlias(source, cacheStart, cacheStart+len(src)) {
		return nil, nil
	}
	formatter := &formFormatter{FormAnalyzer: erabasic.FormAnalyzer{Src: src, Validate: rules != nil, Session: session, Source: source, Base: cacheStart}, rules: rules, verify: verify}
	formatter.Visitor = formatter.consumeEvent
	end, err := formatter.Scan(mode)
	if err != nil {
		return nil, err
	}
	if (mode == erabasic.FormQuoted || mode == erabasic.FormConditional) && end != len(src) {
		name := "quoted"
		if mode == erabasic.FormConditional {
			name = "conditional"
		}
		return nil, &erabasic.ExprError{Offset: end, Msg: "characters after " + name + " FORM"}
	}
	return formatter.edits, formatter.FirstIslandErr
}

func (p *formFormatter) consumeEvent(event erabasic.FormEvent) error {
	if event.Skip {
		return nil
	}
	switch event.Kind {
	case erabasic.FormEventIsland:
		return p.formatIsland(event)
	case erabasic.FormEventCondition:
		return p.formatCondition(event.Start, event.End)
	default:
		return nil
	}
}

func (p *formFormatter) formatCondition(start, end int) error {
	session := p.SessionOrNew()
	edits, err := expressionEditsWithSession(session, p.Source, p.Src[start:end], p.Base+start, false, p.rules, p.verify)
	if err != nil {
		return &erabasic.FormIslandError{Err: erabasic.ShiftExpressionError(err, start)}
	}
	for _, edit := range edits {
		edit.start += start
		edit.end += start
		p.edits = append(p.edits, edit)
	}
	if p.rules[FormConditionSpacing] && !p.FormExpressionHasMandatoryMarker(start, end) && conditionalSpacingAllowed(session, p.Src, start, end) {
		p.formatConditionalSpacing(start, end)
	}
	return nil
}

func conditionalSpacingAllowed(session *erabasic.ExpressionSession, src []byte, start, end int) bool {
	if start >= end || erabasic.LeadingTriviaEndWithSession(session, src[start:end]) >= erabasic.TrailingTriviaStartWithSession(session, src[start:end]) {
		return false
	}
	return !erabasic.ExpressionExcludedWithSession(session, src[start:end])
}

func (p *formFormatter) formatIsland(event erabasic.FormEvent) error {
	start, end := event.Start, event.End
	innerStart, innerEnd := start, end
	editBase := innerStart
	session := p.SessionOrNew()
	cacheStart := p.Base + innerStart
	edits, err := expressionEditsWithSession(session, p.Source, p.Src[innerStart:innerEnd], cacheStart, false, p.rules, p.verify)
	if err != nil {
		return &erabasic.FormIslandError{Err: erabasic.ShiftExpressionError(err, innerStart)}
	}
	if event.SuspiciousWidth {
		return nil
	}
	var local []edit
	if p.rules[FormPadding] {
		innerStart = erabasic.TrimInputLeftWithSession(p.Src, innerStart, innerEnd, session)
		innerEnd = erabasic.TrimInputRight(p.Src, innerStart, innerEnd, session.AllowFullwidthSpace)
		if innerStart != start {
			local = append(local, edit{start, innerStart, nil, Change{Rule: FormPadding}})
		}
		if innerEnd != end {
			local = append(local, edit{innerEnd, end, nil, Change{Rule: FormPadding}})
		}
	}
	for _, e := range edits {
		e.start += editBase
		e.end += editBase
		local = append(local, e)
	}
	p.edits = append(p.edits, local...)
	return nil
}

func (p *formFormatter) formatConditionalSpacing(start, end int) {
	session := p.SessionOrNew()
	left := start + erabasic.LeadingTriviaEndWithSession(session, p.Src[start:end])
	trailing := erabasic.TrailingTriviaStartWithSession(session, p.Src[start:end])
	right := start + trailing
	if left > start {
		if !bytes.Equal(p.Src[start:left], []byte{' '}) {
			p.edits = append(p.edits, edit{start, left, []byte{' '}, Change{Rule: FormConditionSpacing}})
		}
	} else {
		p.edits = append(p.edits, edit{start, start, []byte{' '}, Change{Rule: FormConditionSpacing}})
	}
	if right < end {
		if !bytes.Equal(p.Src[right:end], []byte{' '}) {
			p.edits = append(p.edits, edit{right, end, []byte{' '}, Change{Rule: FormConditionSpacing}})
		}
	} else {
		p.edits = append(p.edits, edit{end, end, []byte{' '}, Change{Rule: FormConditionSpacing}})
	}
}
