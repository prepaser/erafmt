package erabasic

import (
	"bytes"
	"unicode/utf8"
)

func LabelIdentifierEndWithSession(src []byte, start int, session *ExpressionSession) (int, bool) {
	if session == nil || !bytes.Equal(session.Sources[ExpressionSourceOriginal], src) {
		session = NewExpressionSessionWithSpace(src, session != nil && session.AllowFullwidthSpace)
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
		if isDelimiterWithSpace(r, session.AllowFullwidthSpace) {
			break
		}
		i += size
	}
	if i == start {
		return i, false
	}
	tokens, err := session.LexRange(ExpressionSourceOriginal, start, i, start)
	return i, err == nil && len(tokens) == 1 && tokens[0].Kind == ExprIdentifier
}

func labelTailWithSpace(src []byte, marker byte, nameEnd int, allowFullwidthSpace bool) bool {
	i := nameEnd
	i = TrimInputLeft(src, i, len(src), allowFullwidthSpace)
	if marker == '$' {
		return i == len(src) || src[i] == ';'
	}
	if i == len(src) || src[i] == ';' || src[i] == '[' || src[i] == '(' || src[i] == ',' {
		return true
	}
	return false
}
