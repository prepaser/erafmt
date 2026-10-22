package format

import "github.com/prepaser/erafmt/internal/erabasic"

func (p *formatter) syntaxMethod(i int, head []byte, base int, command string, wordEnd int) (int, bool) {
	if !erabasic.BuiltinMethodAvailable(command, p.Dialect) {
		return -1, false
	}
	if wordEnd < len(head) && head[wordEnd] == ';' {
		p.syntaxOpaque(i)
		return -1, true
	}
	analysis := p.commandAnalysisContext()
	mark := len(p.edits)
	body, bodyStart, end, comment, hasArg := analysis.InstructionBody(head, wordEnd)
	if !hasArg {
		if offset, message, valid := analysis.SimpleMethodArgumentsValid(command, base+wordEnd, base+wordEnd); !valid {
			p.syntaxProblem(i, offset, message)
			p.syntaxOpaque(i)
			return -1, true
		}
		p.setCodeEnd(i, base+wordEnd)
		return base + wordEnd, true
	}
	if comment >= 0 {
		end = comment
	}
	if bodyStart >= len(head) || len(erabasic.TrimInput(body, p.AllowFullwidthSpace)) == 0 {
		if offset, message, valid := analysis.SimpleMethodArgumentsValid(command, base+end, base+end); !valid {
			p.syntaxProblem(i, offset, message)
			p.syntaxOpaque(i)
			return -1, true
		}
		p.setCodeEnd(i, base+wordEnd)
		return base + wordEnd, true
	}
	if p.rules[CommandSpacing] && wordEnd < len(head) && p.inputSpaceIn(head, wordEnd) {
		arg := erabasic.TrimInputLeft(head, bodyStart, end, p.AllowFullwidthSpace)
		if arg < end && head[arg] != ';' {
			p.addEdit(i, base+wordEnd, base+arg, []byte{' '}, CommandSpacing)
		}
	}
	if offset, message, valid := analysis.SimpleMethodArgumentsValid(command, base+bodyStart, base+end); !valid {
		p.truncateEdits(mark)
		p.syntaxProblem(i, offset, message)
		p.syntaxOpaque(i)
		return -1, true
	}
	var formatted bool
	if command == "STRFORM" {
		formatted = p.syntaxPrintFormsLiteral(i, base+bodyStart, base+end)
	} else {
		formatted = p.syntaxOrdinaryExpression(i, base+bodyStart, base+end, false)
	}
	if !formatted {
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1, true
	}
	codeEnd := erabasic.TrimInputRight(p.Src, base+bodyStart, base+end, p.AllowFullwidthSpace)
	p.setCodeEnd(i, codeEnd)
	return codeEnd, true
}
