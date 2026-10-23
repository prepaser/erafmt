package erabasic

type SemanticContext struct {
	Src                 []byte
	Dialect             Dialect
	AllowFullwidthSpace bool
	ParseExpressions    bool
	Session             *ExpressionSession
	Base                int
	OnResource          func(int, string)
	OnAbsoluteResource  func(int, string)
}

func (p *SemanticContext) expressionParseEnabled() bool {
	return p.ParseExpressions
}

func (p *SemanticContext) ExpressionSessionFor(start, end int) (*ExpressionSession, int) {
	if p.Session != nil && start >= p.Base && end-p.Base <= len(p.Session.Sources[ExpressionSourceOriginal]) {
		return p.Session, start - p.Base
	}
	session := NewExpressionSessionWithSpace(p.Src[start:end], p.AllowFullwidthSpace)
	session.BuiltinTypes = true
	session.Dialect = p.Dialect
	resource := p.OnAbsoluteResource
	if resource == nil {
		resource = p.OnResource
	}
	if resource != nil {
		session.OnResource = func(offset int, message string) {
			resource(start+offset, message)
		}
	}
	return session, 0
}

func (p *SemanticContext) LexSyntaxRange(start, end int) ([]ExprToken, error) {
	session, offset := p.ExpressionSessionFor(start, end)
	return session.LexRange(ExpressionSourceOriginal, offset, offset+end-start, start)
}

func (p *SemanticContext) SplitTopLevel(src []byte, start, end int, separator byte) []SyntaxSpan {
	session := p.ScanSession(src, start, end)
	spans := SplitTopLevel(src[start:end], 0, end-start, separator, session)
	for i := range spans {
		spans[i].Start += start
		spans[i].End += start
	}
	return spans
}

func (p *SemanticContext) ScanSession(src []byte, start, end int) *ExpressionSession {
	session := p.newSourceSession(src[start:end])
	session.BuiltinTypes = false
	if p.resourceUsesAbsoluteSource(src) {
		resource := p.OnAbsoluteResource
		if resource == nil {
			resource = p.OnResource
		}
		if resource != nil {
			session.OnResource = func(offset int, message string) {
				resource(start+offset, message)
			}
		}
		return session
	}
	if p.OnResource != nil {
		session.OnResource = func(offset int, message string) {
			p.OnResource(start+offset, message)
		}
	}
	return session
}

func (p *SemanticContext) newSourceSession(src []byte) *ExpressionSession {
	session := NewExpressionSessionWithSpace(src, p.AllowFullwidthSpace)
	session.BuiltinTypes = true
	session.Dialect = p.Dialect
	if p.OnResource != nil {
		session.OnResource = p.OnResource
	}
	return session
}

func (p *SemanticContext) resourceUsesAbsoluteSource(src []byte) bool {
	return len(src) != 0 && len(p.Src) != 0 && &src[0] == &p.Src[0]
}

func (p *SemanticContext) isVariableArgument(left, right int) bool {
	return p.variableArgument(left, right, true)
}

func (p *SemanticContext) VariableLHS(src []byte) (string, bool) {
	session := p.newSourceSession(src)
	tokens, err := session.LexRange(ExpressionSourceOriginal, 0, len(src), 0)
	return VariableLHSFromTokens(tokens, err)
}
