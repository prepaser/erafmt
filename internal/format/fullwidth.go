package format

import "github.com/prepaser/erafmt/internal/erabasic"

func (p *formatter) inputSpaceIn(src []byte, pos int) bool {
	return erabasic.InputSpaceAt(src, pos, p.AllowFullwidthSpace)
}

func (p *formatter) trimInputIn(src []byte) []byte {
	return erabasic.TrimInput(src, p.AllowFullwidthSpace)
}

func (p *formatter) trimInputLeft(start, end int) int {
	return erabasic.TrimInputLeft(p.Src, start, end, p.AllowFullwidthSpace)
}

func (p *formatter) trimInputRight(start, end int) int {
	return erabasic.TrimInputRight(p.Src, start, end, p.AllowFullwidthSpace)
}

func (p *formatter) headOf(body []byte) (head []byte, prefix int, optional bool, marked bool) {
	return erabasic.HeadOfWithSpaceDialect(body, p.AllowFullwidthSpace, p.Dialect)
}

func (p *formatter) commandOf(head []byte) (string, int) {
	return erabasic.CommandOfWithSpace(head, p.AllowFullwidthSpace)
}

func (p *formatter) newExpressionSession(src []byte) *erabasic.ExpressionSession {
	session := erabasic.NewExpressionSessionWithSpace(src, p.AllowFullwidthSpace)
	session.BuiltinTypes = true
	session.Dialect = p.Dialect
	session.OnResource = p.syntaxResource
	return session
}
