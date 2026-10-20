package format

func scanSyntaxForm(src []byte, start, end int, mode formMode, sessions []*expressionSession) (int, error) {
	if len(sessions) == 0 || sessions[0] == nil {
		return scanForm(src[start:end], mode)
	}
	next, err := sessions[0].scanFormAt(expressionSourceOriginal, start, end, mode)
	return next - start, err
}

func (p *formatter) scanSession(src []byte, start, end int) *expressionSession {
	s := newExpressionSessionWithSpace(src[start:end], p.allowFullwidthSpace)
	s.dialect = p.dialect
	s.onResource = func(offset int, message string) { p.syntaxResource(start+offset, message) }
	if len(src) > 0 && len(p.src) > 0 && &src[0] == &p.src[0] {
		s.onResource = func(offset int, message string) { p.resourceProblem(p.parseLine, start+offset, message) }
	}
	return s
}

func (p *formatter) syntaxAtomEnd(src []byte, start, end int) (int, bool) {
	if start < 0 || start >= end || end > len(src) {
		return -1, false
	}
	if src[start] != '@' && src[start] != '\\' {
		return syntaxAtomEnd(src, start, end)
	}
	next, atom := syntaxAtomEnd(src[start:end], 0, end-start, p.scanSession(src, start, end))
	if next >= 0 {
		next += start
	}
	return next, atom
}

func (p *formatter) syntaxCommentEnd(src []byte, start, end int) (int, bool) {
	next, found := syntaxCommentEnd(src[start:end], 0, end-start, p.scanSession(src, start, end))
	return start + next, found
}

func (p *formatter) splitTopLevel(src []byte, start, end int, separator byte) []syntaxSpan {
	spans := splitTopLevel(src[start:end], 0, end-start, separator, p.scanSession(src, start, end))
	for i := range spans {
		spans[i].start += start
		spans[i].end += start
	}
	return spans
}

func (p *formatter) topEqual(src []byte, start, end int) (int, bool) {
	next, found := topEqual(src[start:end], 0, end-start, p.scanSession(src, start, end))
	return start + next, found
}

func (p *formatter) matchingDelimiter(src []byte, start, end int, open, close byte) int {
	next := matchingDelimiter(src[start:end], 0, end-start, open, close, p.scanSession(src, start, end))
	if next >= 0 {
		next += start
	}
	return next
}

func (p *formatter) assignmentBoundary(src []byte) (int, assignmentOp, bool) {
	return assignmentBoundary(src, p.scanSession(src, 0, len(src)))
}

func (p *formatter) instructionBody(head []byte, wordEnd int) ([]byte, int, int, int, bool) {
	return instructionBodyWithSpace(head, wordEnd, p.allowFullwidthSpace, p.scanSession(head, 0, len(head)))
}

func (p *formatter) registeredBody(head []byte, wordEnd int) (int, int, int, bool) {
	return registeredBodyWithSpace(head, wordEnd, p.allowFullwidthSpace, p.scanSession(head, 0, len(head)))
}
