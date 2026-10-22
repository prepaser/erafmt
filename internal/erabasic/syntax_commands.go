package erabasic

func SavevarBuiltinTargetForbidden(tokens []ExprToken, dialect Dialect) bool {
	name, proven := BuiltinVariableIdentity(tokens, dialect)
	if !proven {
		return false
	}
	switch name {
	case "ARG", "ARGS", "LOCAL", "LOCALS", "WINDOW_TITLE":
		return true
	default:
		return false
	}
}

func RegisteredCodeEnd(base, end, comment int) int {
	if comment >= 0 {
		return base + comment
	}
	return base + end
}

func RegisteredBodyWithSpace(head []byte, wordEnd int, allowFullwidthSpace bool, sessions ...*ExpressionSession) (start, end, comment int, ok bool) {
	if wordEnd >= len(head) {
		return wordEnd, wordEnd, -1, true
	}
	if head[wordEnd] == ';' {
		return 0, 0, -1, false
	}
	if !InputSpaceAt(head, wordEnd, allowFullwidthSpace) {
		return 0, 0, -1, false
	}
	start = wordEnd + InputSpaceSize(head, wordEnd, allowFullwidthSpace)
	end, found := SyntaxCommentEnd(head, start, len(head), sessions...)
	if !found {
		return start, end, -1, true
	}
	return start, end, end, true
}

func RegisteredRawBodyWithSpace(head []byte, wordEnd int, allowFullwidthSpace bool) (start, end int, ok bool) {
	if wordEnd >= len(head) {
		return wordEnd, wordEnd, true
	}
	if !InputSpaceAt(head, wordEnd, allowFullwidthSpace) && head[wordEnd] != ';' {
		return 0, 0, false
	}
	return wordEnd + max(1, InputSpaceSize(head, wordEnd, allowFullwidthSpace)), len(head), true
}

func RegisteredFormAnyBody(head []byte, wordEnd int, allowFullwidthSpace bool) (start, end int, ok bool) {
	start, end, ok = RegisteredRawBodyWithSpace(head, wordEnd, allowFullwidthSpace)
	if !ok {
		return 0, 0, false
	}
	for start < len(head) && head[start] == ' ' {
		start++
	}
	return start, end, true
}

func RegisteredHasArgumentWithSpace(head []byte, start, end int, allowFullwidthSpace bool) bool {
	return start < end && TrimInputLeft(head, start, end, allowFullwidthSpace) < TrimInputRight(head, start, end, allowFullwidthSpace)
}
