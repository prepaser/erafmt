package erabasic

type formEventKind uint8

const (
	FormEventIsland formEventKind = iota
	FormEventCondition
)

type formEventVisitor func(FormEvent) error
type FormEvent struct {
	Kind            formEventKind
	Start           int
	End             int
	SuspiciousWidth bool
	Skip            bool
}

func (p *FormAnalyzer) analyzeIsland(start, end int, valueType KnownValueType) (FormEvent, error) {
	event := FormEvent{Kind: FormEventIsland, Start: start, End: end}
	session := p.SessionOrNew()
	if ExpressionExcludedWithSession(session, p.Src[start:end]) || p.FormExpressionHasMandatoryMarker(start, end) {
		event.Skip = true
		return event, nil
	}
	fields := p.splitTopLevel(start, end)
	event.SuspiciousWidth = p.noteIslandWidth(valueType, fields)
	if err := p.validateIslandShape(fields); err != nil {
		return FormEvent{}, &FormIslandError{Err: err}
	}
	if tokens, err := session.LexRange(p.Source, p.Base+start, p.Base+end, start); err != nil {
		return FormEvent{}, &FormIslandError{Err: err}
	} else if err := formExpressionPrefix(tokens); err != nil {
		return FormEvent{}, &FormIslandError{Err: err}
	} else if err := ProvenExpressionShape(tokens, session.Dialect, false); err != nil {
		return FormEvent{}, &FormIslandError{Err: err}
	}
	if err := p.validateIslandTypes(valueType, fields); err != nil {
		return FormEvent{}, &FormIslandError{Err: err}
	}
	if err := p.validateIslandAlignment(fields); err != nil {
		return FormEvent{}, &FormIslandError{Err: err}
	}
	if LeadingTriviaEndWithSession(session, p.Src[start:end]) >= TrailingTriviaStartWithSession(session, p.Src[start:end]) {
		return FormEvent{}, &FormIslandError{Err: &ExprError{Offset: start, Msg: "empty FORM expression"}}
	}
	return event, nil
}

func (p *FormAnalyzer) analyzeCondition(start, end int) (FormEvent, error) {
	event := FormEvent{Kind: FormEventCondition, Start: start, End: end}
	session := p.SessionOrNew()
	if p.FormExpressionHasMandatoryMarker(start, end) {
		event.Skip = true
		return event, nil
	}
	if start == end || LeadingTriviaEndWithSession(session, p.Src[start:end]) >= TrailingTriviaStartWithSession(session, p.Src[start:end]) {
		return FormEvent{}, &FormIslandError{Err: &ExprError{Offset: start, Msg: "empty FORM conditional expression"}}
	}
	if !session.SkipFormTypes {
		if p.scalarIndexesMissing(start, end) {
			return FormEvent{}, &FormIslandError{Err: &ExprError{Offset: start, Msg: "scalar variable requires indexes"}}
		}
		actual, _ := p.formFieldType(start, end)
		if actual == KnownValueString {
			return FormEvent{}, &FormIslandError{Err: &ExprError{Offset: start, Msg: "FORM conditional requires a numeric expression"}}
		}
	}
	if tokens, err := session.LexRange(p.Source, p.Base+start, p.Base+end, start); err != nil {
		return FormEvent{}, &FormIslandError{Err: err}
	} else if err := formExpressionPrefix(tokens); err != nil {
		return FormEvent{}, &FormIslandError{Err: err}
	} else if !ExpressionNeedsDeferral(tokens, session.Dialect) && len(p.splitTopLevel(start, end)) != 1 {
		return FormEvent{}, &FormIslandError{Err: &ExprError{Offset: start, Msg: "FORM conditional requires one expression"}}
	} else if err := ProvenExpressionShape(tokens, session.Dialect, false); err != nil {
		return FormEvent{}, &FormIslandError{Err: err}
	}
	return event, nil
}
