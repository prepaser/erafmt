package format

import "github.com/prepaser/erafmt/internal/erabasic"

func (p *formatter) syntaxBits(i int, head []byte, base, wordEnd int) int {
	start, end, comment, ok := p.registeredBody(head, wordEnd)
	if !ok {
		p.syntaxOpaque(i)
		return -1
	}
	codeEnd := erabasic.RegisteredCodeEnd(base, end, comment)
	fail := func(offset int, message string) int {
		p.syntaxProblem(i, offset, message)
		p.syntaxOpaque(i)
		return -1
	}
	if p.expressionParseEnabled() {
		validation := p.semanticContext().ValidateBitArguments(head, base, start, end, codeEnd)
		if validation.Excluded {
			p.syntaxOpaque(i)
			return -1
		}
		if validation.ExprErr != nil {
			p.expressionProblem(i, 0, validation.ExprErr)
			p.syntaxOpaque(i)
			return -1
		}
		if !validation.Valid {
			return fail(validation.Offset, validation.Message)
		}
	}
	mark := len(p.edits)
	if !p.syntaxExpression(i, base+start, codeEnd, false) {
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1
	}
	p.syntaxRegisteredCommandGap(i, head, base, wordEnd, start, end)
	p.setCodeEnd(i, p.trimInputRight(base+start, codeEnd))
	return codeEnd
}
