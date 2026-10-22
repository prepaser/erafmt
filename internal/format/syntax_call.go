package format

import (
	"bytes"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func (p *formatter) syntaxCall(i int, command string, start, end int) (ok bool) {
	mark := len(p.edits)
	defer func() {
		if !ok {
			p.truncateEdits(mark)
		}
	}()
	if start >= end {
		return true
	}
	formName := erabasic.CallUsesForm(command)
	if !p.expressionParseEnabled() || formName && !p.expressionRulesEnabled() {
		return true
	}
	analysis := p.statementAnalysis().Call(command, start, end)
	if analysis.Issue != nil {
		p.syntaxProblem(i, analysis.Issue.Offset, analysis.Issue.Message)
		return false
	}
	if formName && !p.syntaxForm(i, analysis.NameStart, analysis.NameEnd, erabasic.FormCall) {
		return false
	}
	if analysis.SubnameOpen >= 0 {
		p.syntaxCallDelimiter(i, analysis.NameEnd, analysis.SubnameOpen, analysis.SubnameClose)
		if !p.syntaxOrdinaryExpression(i, analysis.SubnameOpen+1, analysis.SubnameClose, false) {
			return false
		}
	}
	if analysis.ArgumentOpen >= 0 {
		p.syntaxCallDelimiter(i, analysis.BeforeOpen, analysis.ArgumentOpen, analysis.ArgumentClose)
		if !p.syntaxOrdinaryExpression(i, analysis.ArgumentOpen+1, analysis.ArgumentClose, false) {
			return false
		}
		return p.syntaxCallTail(i, analysis.ArgumentsStart, analysis.ArgumentClose, end)
	}
	if analysis.Delimiter >= 0 && analysis.Delimiter < end && p.Src[analysis.Delimiter] == ',' {
		p.syntaxRegisteredCommaGap(i, analysis.Delimiter, end, analysis.BeforeOpen)
		return p.syntaxOrdinaryExpression(i, analysis.Delimiter+1, end, false)
	}
	return true
}

func (p *formatter) syntaxCallTail(i, start, close, end int) bool {
	issue := p.statementAnalysis().CallTailIssue(start, close, end)
	if issue == nil {
		return true
	}
	p.syntaxProblem(i, issue.Offset, issue.Message)
	return false
}

func (p *formatter) tryGotoListFuncAllowed(i, start, end, delimiter int) bool {
	issue := p.statementAnalysis().TryGotoListFuncIssue(start, end, delimiter)
	if issue == nil {
		return true
	}
	p.syntaxProblem(i, issue.Offset, issue.Message)
	return false
}

func (p *formatter) setCallCodeEnd(i, start, end int) int {
	if p.CodeEnds == nil || i < 0 || i >= len(p.Lines) {
		return end
	}
	l := p.Lines[i]
	if bytes.IndexByte(p.Src[l.Start:l.BodyEnd], 0) >= 0 {
		p.syntaxOpaque(i)
		return end
	}
	end, remove := erabasic.CallCodeEndValue(p.Src, l, start, end, p.AllowFullwidthSpace, p.scanSession(p.Src, start, end))
	if remove {
		delete(p.CodeEnds, i)
	} else {
		p.CodeEnds[i] = end
	}
	return end
}

func (p *formatter) setCodeEnd(i, end int) {
	if p.CodeEnds == nil || i < 0 || i >= len(p.Lines) {
		return
	}
	l := p.Lines[i]
	if bytes.IndexByte(p.Src[l.Start:l.BodyEnd], 0) >= 0 {
		p.syntaxOpaque(i)
		return
	}
	end, remove := erabasic.CodeEndValue(p.Src, l, end, p.AllowFullwidthSpace, p.scanSession(p.Src, l.IndentEnd, end))
	if remove {
		delete(p.CodeEnds, i)
		return
	}
	p.recordCodeEnd(i, end)
}

func (p *formatter) recordCodeEnd(i, end int) {
	if i < 0 || i >= len(p.Lines) {
		return
	}
	l := p.Lines[i]
	end, remove := erabasic.RecordCodeEndValue(p.Src, l, end, p.AllowFullwidthSpace)
	if remove {
		delete(p.CodeEnds, i)
		return
	}
	p.CodeEnds[i] = end
}
