package format

import (
	"bytes"
	"unicode/utf8"
)

func (p *formatter) syntaxLabel(i int, head []byte, base int) {
	if i < 0 || i >= len(p.lines) || len(head) < 2 || (head[0] != '@' && head[0] != '$') {
		return
	}
	if bytes.Contains(head, []byte("[[")) {
		p.syntaxOpaque(i)
		return
	}
	if syntaxHasProtectedMetadataTail(head) {
		defer p.syntaxOpaque(i)
	}

	start := 1
	start = trimInputLeft(head, start, len(head), p.allowFullwidthSpace)
	nameEnd, ok := labelIdentifierEndWithSession(head, start, p.newExpressionSession(head))
	if !ok || !labelTailWithSpace(head, head[0], nameEnd, p.allowFullwidthSpace) {
		p.syntaxOpaque(i)
		return
	}
	if p.rules[LabelMarkerSpacing] && start > 1 {
		p.addEdit(i, base+1, base+start, nil, LabelMarkerSpacing)
	}
	if head[0] == '$' {
		p.setCodeEnd(i, base+nameEnd)
	}
}

func labelIdentifierEndWithSession(src []byte, start int, session *expressionSession) (int, bool) {
	if session == nil || !bytes.Equal(session.sources[expressionSourceOriginal], src) {
		session = newExpressionSessionWithSpace(src, session != nil && session.allowFullwidthSpace)
	}
	if start >= len(src) {
		return start, false
	}
	first, _ := utf8.DecodeRune(src[start:])
	if isASCIIDigit(first) {
		return start, false
	}
	i := start
	for i < len(src) {
		r, size := utf8.DecodeRune(src[i:])
		if r == utf8.RuneError && size == 1 {
			return i, false
		}
		if isDelimiterWithSpace(r, session.allowFullwidthSpace) {
			break
		}
		i += size
	}
	if i == start {
		return i, false
	}
	tokens, err := session.lexRange(expressionSourceOriginal, start, i, start)
	return i, err == nil && len(tokens) == 1 && tokens[0].kind == exprIdentifier
}

func labelTailWithSpace(src []byte, marker byte, nameEnd int, allowFullwidthSpace bool) bool {
	i := nameEnd
	i = trimInputLeft(src, i, len(src), allowFullwidthSpace)
	if marker == '$' {
		return i == len(src) || src[i] == ';'
	}
	if i == len(src) || src[i] == ';' || src[i] == '[' || src[i] == '(' || src[i] == ',' {
		return true
	}
	return false
}
