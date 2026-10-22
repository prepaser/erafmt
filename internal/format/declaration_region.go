package format

import "github.com/prepaser/erafmt/internal/erabasic"

func (p *formatter) declarationRegionAnalysis() *erabasic.DeclarationRegionAnalysis {
	return &erabasic.DeclarationRegionAnalysis{Source: p.Src, Dialect: p.Dialect, AllowFullwidthSpace: p.AllowFullwidthSpace, NewSession: p.newExpressionSession}
}

func (p *formatter) declarationRegionHeaderValid(head []byte) bool {
	return p.declarationRegionAnalysis().HeaderValid(head)
}

func (p *formatter) declarationRegionBodyStart(start, limit int) (int, bool) {
	if start < 0 || start >= len(p.Lines) || limit < start || limit > len(p.Lines) {
		return 0, false
	}
	return p.declarationRegionAnalysis().BodyStart(func(yield func(erabasic.DeclarationRegionLine) bool) {
		for i := start; i < limit; i++ {
			fact, ok := p.declarationRegionLine(i)
			if !ok || !yield(fact) {
				return
			}
		}
	})
}

func (p *formatter) declarationRegionLine(i int) (erabasic.DeclarationRegionLine, bool) {
	if i < 0 || i >= len(p.Lines) {
		return erabasic.DeclarationRegionLine{}, false
	}
	line := p.Lines[i]
	head, _, optional, marked := p.headOf(erabasic.LBody(p.Src, line))
	_, codeEnd := p.CodeEnds[i]
	return erabasic.DeclarationRegionLine{
		Line:            i,
		Head:            head,
		Optional:        optional,
		Marked:          marked,
		Opaque:          p.lineOpaque(i),
		Blank:           p.ordinaryBlank(i),
		OrdinaryComment: p.isOrdinaryComment(i),
		CodeEnd:         codeEnd,
		BodyStart:       i < len(p.BodyStarts) && p.BodyStarts[i],
	}, true
}

func (p *formatter) declarationBodyStart(start, limit int) (int, bool) {
	return p.declarationRegionBodyStart(start, limit)
}

func (p *formatter) declarationPrologueLine(i int, sawDim *bool) bool {
	fact, ok := p.declarationRegionLine(i)
	return ok && p.declarationRegionAnalysis().Prologue(fact, sawDim)
}

func (p *formatter) provenDeclarationBody(i int) bool {
	fact, ok := p.declarationRegionLine(i)
	return ok && p.declarationRegionAnalysis().ProvenBody(fact)
}
