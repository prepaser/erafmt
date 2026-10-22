package format

import "github.com/prepaser/erafmt/internal/erabasic"

func (p *formatter) scanSession(src []byte, start, end int) *erabasic.ExpressionSession {
	s := erabasic.NewExpressionSessionWithSpace(src[start:end], p.AllowFullwidthSpace)
	s.Dialect = p.Dialect
	s.OnResource = func(offset int, message string) {
		p.syntaxResource(start+offset, message)
	}
	if len(src) > 0 && len(p.Src) > 0 && &src[0] == &p.Src[0] {
		s.OnResource = func(offset int, message string) {
			p.resourceProblem(p.ParseLine, start+offset, message)
		}
	}
	return s
}

func (p *formatter) syntaxAtomEnd(src []byte, start, end int) (int, bool) {
	if start < 0 || start >= end || end > len(src) {
		return -1, false
	}
	if src[start] != '@' && src[start] != '\\' {
		return erabasic.SyntaxAtomEnd(src, start, end)
	}
	next, atom := erabasic.SyntaxAtomEnd(src[start:end], 0, end-start, p.scanSession(src, start, end))
	if next >= 0 {
		next += start
	}
	return next, atom
}

func (p *formatter) syntaxCommentEnd(src []byte, start, end int) (int, bool) {
	next, found := erabasic.SyntaxCommentEnd(src[start:end], 0, end-start, p.scanSession(src, start, end))
	return start + next, found
}

func (p *formatter) splitTopLevel(src []byte, start, end int, separator byte) []erabasic.SyntaxSpan {
	spans := erabasic.SplitTopLevel(src[start:end], 0, end-start, separator, p.scanSession(src, start, end))
	for i := range spans {
		spans[i].Start += start
		spans[i].End += start
	}
	return spans
}

func (p *formatter) topEqual(src []byte, start, end int) (int, bool) {
	next, found := erabasic.TopEqual(src[start:end], 0, end-start, p.scanSession(src, start, end))
	return start + next, found
}

func (p *formatter) matchingDelimiter(src []byte, start, end int, open, close byte) int {
	next := erabasic.MatchingDelimiter(src[start:end], 0, end-start, open, close, p.scanSession(src, start, end))
	if next >= 0 {
		next += start
	}
	return next
}

func (p *formatter) assignmentBoundary(src []byte) (int, erabasic.AssignmentOp, bool) {
	return erabasic.AssignmentBoundary(src, p.scanSession(src, 0, len(src)))
}

func (p *formatter) instructionBody(head []byte, wordEnd int) ([]byte, int, int, int, bool) {
	return erabasic.InstructionBodyWithSpace(head, wordEnd, p.AllowFullwidthSpace, p.scanSession(head, 0, len(head)))
}

func (p *formatter) registeredBody(head []byte, wordEnd int) (int, int, int, bool) {
	return erabasic.RegisteredBodyWithSpace(head, wordEnd, p.AllowFullwidthSpace, p.scanSession(head, 0, len(head)))
}
