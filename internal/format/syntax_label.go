package format

import (
	"bytes"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func (p *formatter) syntaxLabel(i int, head []byte, base int) {
	if i < 0 || i >= len(p.Lines) || len(head) < 2 || (head[0] != '@' && head[0] != '$') {
		return
	}
	if bytes.Contains(head, []byte("[[")) {
		p.syntaxOpaque(i)
		return
	}
	if erabasic.SyntaxHasProtectedMetadataTail(head) {
		defer p.syntaxOpaque(i)
	}
	start := 1
	start = erabasic.TrimInputLeft(head, start, len(head), p.AllowFullwidthSpace)
	analysis := p.labelAnalysis(head, head[0], start)
	if !analysis.Valid || !analysis.Tail {
		p.syntaxOpaque(i)
		return
	}
	if p.rules[LabelMarkerSpacing] && start > 1 {
		p.addEdit(i, base+1, base+start, nil, LabelMarkerSpacing)
	}
	if head[0] == '$' {
		p.setCodeEnd(i, base+analysis.End)
	}
}
