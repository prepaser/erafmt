package format

import (
	"bytes"
	"strings"
)

type headerValidationStatus uint8

const (
	headerValid headerValidationStatus = iota
	headerUnsupported
	headerInvalid
)

type headerValidation struct {
	status  headerValidationStatus
	message string
}

func validateHeaderWithSession(head []byte, session *expressionSession) headerValidation {
	if session == nil || !bytes.Equal(session.sources[expressionSourceOriginal], head) {
		session = newExpressionSession(head)
	}
	if len(head) < 2 || head[0] != '@' {
		return invalidHeader("function header requires a name")
	}
	if bytes.Contains(head, []byte("[[")) {
		return unsupportedHeader("function header contains an unresolved alias")
	}

	start := trimInputLeftWithSession(head, 1, len(head), session)
	nameEnd, ok := labelIdentifierEndWithSession(head, start, session)
	if !ok {
		if start < len(head) && head[start] >= '0' && head[start] <= '9' {
			return invalidHeader("function name cannot start with a digit")
		}
		return invalidHeader("function header requires a valid name")
	}
	if headerFullwidthSpaceWithSession(head, start, nameEnd, session) {
		return unsupportedHeader("function header uses runtime-dependent full-width whitespace")
	}

	pos := skipHeaderSpaceWithSession(head, nameEnd, session)
	if headerFullwidthAtWithSession(head, pos, session) {
		return unsupportedHeader("function header uses runtime-dependent full-width whitespace")
	}
	event, exact := eventFunctionContext(string(head[start:nameEnd]))
	if event && exact && pos < len(head) && head[pos] != ';' {
		return invalidHeader("event function cannot have arguments")
	}
	if pos == len(head) || head[pos] == ';' {
		return validateHeaderComment(head, pos, session)
	}
	if head[pos] == '[' {
		return invalidHeader("function header square-bracket subnames are unsupported")
	}

	switch head[pos] {
	case '(':
		close, result := scanHeaderDelimiterWithSession(head, pos, session)
		if result.status != headerValid {
			return result
		}
		if result := validateHeaderArgumentsWithSession(head, pos+1, close, session); result.status != headerValid {
			return result
		}
		return validateHeaderComment(head, skipHeaderSpaceWithSession(head, close+1, session), session)
	case ',':
		end, result := scanHeaderCodeEndWithSession(head, pos+1, session)
		if result.status != headerValid {
			return result
		}
		if result := validateHeaderArgumentsWithSession(head, pos+1, end, session); result.status != headerValid {
			return result
		}
		return validateHeaderComment(head, end, session)
	default:
		return invalidHeader("unexpected text after function name")
	}
}

func validateHeaderArgumentsWithSession(src []byte, start, end int, session *expressionSession) headerValidation {
	spans := splitHeaderTopLevelWithSession(src, start, end, ',', session)
	for index, span := range spans {
		spanStart, spanEnd := trimHeaderSpanWithSession(src, span.start, span.end, session)
		if spanStart == spanEnd {
			if len(spans) == 1 || index == len(spans)-1 && index > 0 {
				continue
			}
			return invalidHeader("function signature contains an empty argument")
		}
		if equal, ok := topHeaderEqualWithSession(src, spanStart, spanEnd, session); ok {
			leftStart, leftEnd := trimHeaderSpanWithSession(src, spanStart, equal, session)
			rightStart, rightEnd := trimHeaderSpanWithSession(src, equal+1, spanEnd, session)
			if leftStart == leftEnd || rightStart == rightEnd {
				return invalidHeader("function signature contains an empty default")
			}
			if result := validateHeaderExpressionWithSession(src, leftStart, leftEnd, session); result.status != headerValid {
				return result
			}
			if result := validateHeaderExpressionWithSession(src, rightStart, rightEnd, session); result.status != headerValid {
				return result
			}
			continue
		}
		if headerTopComparisonWithSession(src, spanStart, spanEnd, session) {
			return invalidHeader("function signature defaults require a single equals sign")
		}
		if result := validateHeaderExpressionWithSession(src, spanStart, spanEnd, session); result.status != headerValid {
			return result
		}
	}
	return headerValidation{status: headerValid}
}

func headerTopComparisonWithSession(src []byte, start, end int, session *expressionSession) bool {
	tokens, err := session.lexRange(expressionSourceOriginal, start, end, start)
	if err != nil {
		return false
	}
	depth := 0
	for _, token := range tokens {
		switch string(token.text) {
		case "(", "[":
			depth++
		case ")", "]":
			depth--
		case "==":
			if depth == 0 {
				return true
			}
		}
	}
	return false
}

func validateHeaderExpressionWithSession(src []byte, start, end int, session *expressionSession) headerValidation {
	start, end = trimHeaderSpanWithSession(src, start, end, session)
	if start == end {
		return invalidHeader("function header expression is empty")
	}
	if bytes.Contains(src[start:end], []byte("[[")) {
		return unsupportedHeader("function header expression contains an unresolved alias")
	}
	if headerFullwidthSpaceWithSession(src, start, end, session) {
		return unsupportedHeader("function header expression uses runtime-dependent full-width whitespace")
	}
	if _, err := session.lexRange(expressionSourceOriginal, start, end, start); err != nil {
		return unsupportedHeader("function header expression cannot be proven")
	}
	return headerValidation{status: headerValid}
}

func validateHeaderComment(src []byte, pos int, session *expressionSession) headerValidation {
	if pos == len(src) {
		return headerValidation{status: headerValid}
	}
	if headerFullwidthAtWithSession(src, pos, session) {
		return unsupportedHeader("function header uses runtime-dependent full-width whitespace")
	}
	if src[pos] == ';' && executableMarker(src, pos) {
		end := pos
		for {
			end = skipHeaderSpaceWithSession(src, end, session)
			if end == len(src) {
				return headerValidation{status: headerValid}
			}
			if executableMarker(src, end) {
				end += 3
				continue
			}
			if src[end] == ';' {
				return headerValidation{status: headerValid}
			}
			return unsupportedHeader("function header marker tail cannot be proven")
		}
	}
	if src[pos] == ';' {
		return headerValidation{status: headerValid}
	}
	return invalidHeader("function header has a non-comment suffix")
}

func scanHeaderDelimiterWithSession(src []byte, open int, session *expressionSession) (int, headerValidation) {
	if open < 0 || open >= len(src) || (src[open] != '(' && src[open] != '[') {
		return -1, invalidHeader("function header has an invalid delimiter")
	}
	stack := []byte{headerCloseFor(src[open])}
	for i := open + 1; i < len(src); {
		next, atom, result := scanHeaderAtomWithSession(src, i, len(src), session)
		if result.status != headerValid {
			return -1, result
		}
		if atom {
			i = next
			continue
		}
		if size := inputSpaceSize(src, i, session.allowFullwidthSpace); size > 0 {
			i += size
			continue
		}
		if headerFullwidthAtWithSession(src, i, session) {
			return -1, unsupportedHeader("function header uses runtime-dependent full-width whitespace")
		}
		switch src[i] {
		case '(', '[':
			stack = append(stack, headerCloseFor(src[i]))
		case ')', ']':
			if len(stack) == 0 || stack[len(stack)-1] != src[i] {
				return -1, invalidHeader("function header delimiters are misnested")
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i, headerValidation{status: headerValid}
			}
		case ';':
			return -1, invalidHeader("function header delimiter is not closed before the comment")
		}
		i++
	}
	return -1, invalidHeader("function header delimiter is not closed")
}

func scanHeaderCodeEndWithSession(src []byte, start int, session *expressionSession) (int, headerValidation) {
	stack := make([]byte, 0, 2)
	for i := start; i < len(src); {
		next, atom, result := scanHeaderAtomWithSession(src, i, len(src), session)
		if result.status != headerValid {
			return -1, result
		}
		if atom {
			i = next
			continue
		}
		if size := inputSpaceSize(src, i, session.allowFullwidthSpace); size > 0 {
			i += size
			continue
		}
		if headerFullwidthAtWithSession(src, i, session) {
			return -1, unsupportedHeader("function header uses runtime-dependent full-width whitespace")
		}
		switch src[i] {
		case '(', '[':
			stack = append(stack, headerCloseFor(src[i]))
		case ')', ']':
			if len(stack) == 0 || stack[len(stack)-1] != src[i] {
				return -1, invalidHeader("function signature delimiters are misnested")
			}
			stack = stack[:len(stack)-1]
		case ';':
			if len(stack) != 0 {
				return -1, invalidHeader("function signature delimiter is not closed before the comment")
			}
			return i, headerValidation{status: headerValid}
		}
		i++
	}
	if len(stack) != 0 {
		return -1, invalidHeader("function signature delimiters are not closed")
	}
	return len(src), headerValidation{status: headerValid}
}

func scanHeaderAtomWithSession(src []byte, pos, limit int, session *expressionSession) (next int, atom bool, result headerValidation) {
	if limit < 0 || limit > len(src) {
		limit = len(src)
	}
	if pos >= limit {
		return pos, false, headerValidation{status: headerValid}
	}
	switch src[pos] {
	case '"':
		end := quotedEnd(src[pos:limit], 0)
		if end < 0 {
			return 0, false, invalidHeader("function header contains an unterminated string")
		}
		return pos + end, true, headerValidation{status: headerValid}
	case '@':
		if pos+1 < limit && src[pos+1] == '"' {
			end, err := session.scanFormAt(expressionSourceOriginal, pos, limit, formQuoted)
			if err != nil {
				return 0, false, classifyHeaderScanError(err)
			}
			return end, true, headerValidation{status: headerValid}
		}
	case '\\':
		if pos+1 < limit && src[pos+1] == '@' {
			end, err := session.scanFormAt(expressionSourceOriginal, pos, limit, formConditional)
			if err != nil {
				return 0, false, classifyHeaderScanError(err)
			}
			return end, true, headerValidation{status: headerValid}
		}
	}
	return pos, false, headerValidation{status: headerValid}
}

func classifyHeaderScanError(err error) headerValidation {
	message := ""
	if err != nil {
		message = err.Error()
	}
	if strings.Contains(message, "alias") {
		return unsupportedHeader("function header contains an unresolved alias")
	}
	if definitelyMalformedHeaderForm(message) {
		return invalidHeader("function header contains an invalid FORM")
	}
	return unsupportedHeader("function header FORM cannot be proven")
}

func definitelyMalformedHeaderForm(message string) bool {
	for _, marker := range []string{
		"unterminated",
		"missing FORM",
		"missing closing FORM",
		"misnested FORM",
		"FORM expression reaches a comment",
		"nested FORM brace",
		"missing character after FORM",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func splitHeaderTopLevelWithSession(src []byte, start, end int, separator byte, session *expressionSession) []syntaxSpan {
	var out []syntaxSpan
	part, depth := start, 0
	for i := start; i < end; i++ {
		if next, atom, result := scanHeaderAtomWithSession(src, i, end, session); atom {
			if result.status != headerValid {
				return []syntaxSpan{{start, end}}
			}
			i = next - 1
			continue
		} else if result.status != headerValid {
			return []syntaxSpan{{start, end}}
		}
		switch src[i] {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case separator:
			if depth == 0 {
				if len(out) >= maxExpressionTokens-1 {
					session.resource(i, "header field count exceeds the formatter resource limit")
					return []syntaxSpan{{start, end}}
				}
				out = append(out, syntaxSpan{part, i})
				part = i + 1
			}
		}
	}
	return append(out, syntaxSpan{part, end})
}

func topHeaderEqualWithSession(src []byte, start, end int, session *expressionSession) (int, bool) {
	depth := 0
	for i := start; i < end; i++ {
		if next, atom, result := scanHeaderAtomWithSession(src, i, end, session); atom {
			if result.status != headerValid {
				return 0, false
			}
			i = next - 1
			continue
		} else if result.status != headerValid {
			return 0, false
		}
		switch src[i] {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case ';':
			if depth == 0 {
				return 0, false
			}
		}
		if src[i] == '=' && depth == 0 && (i+1 >= end || src[i+1] != '=') && defaultEqualBoundary(src, start, i) {
			return i, true
		}
	}
	return 0, false
}

func trimInputLeftWithSession(src []byte, start, end int, session *expressionSession) int {
	return trimInputLeft(src, start, end, session != nil && session.allowFullwidthSpace)
}

func trimHeaderSpanWithSession(src []byte, start, end int, session *expressionSession) (int, int) {
	start = trimInputLeftWithSession(src, start, end, session)
	return start, trimInputRight(src, start, end, session != nil && session.allowFullwidthSpace)
}

func skipHeaderSpaceWithSession(src []byte, pos int, session *expressionSession) int {
	return trimInputLeftWithSession(src, pos, len(src), session)
}

func headerFullwidthSpaceWithSession(src []byte, start, end int, session *expressionSession) bool {
	if session != nil && session.allowFullwidthSpace {
		return false
	}
	for i := start; i+2 < end; {
		next, atom, result := scanHeaderAtomWithSession(src, i, end, session)
		if result.status != headerValid {
			return false
		}
		if atom {
			i = next
			continue
		}
		if src[i] == 0xe3 && src[i+1] == 0x80 && src[i+2] == 0x80 {
			return true
		}
		i++
	}
	return false
}

func headerFullwidthAtWithSession(src []byte, pos int, session *expressionSession) bool {
	return (session == nil || !session.allowFullwidthSpace) && pos >= 0 && pos+2 < len(src) && src[pos] == 0xe3 && src[pos+1] == 0x80 && src[pos+2] == 0x80
}

func headerCloseFor(open byte) byte {
	if open == '[' {
		return ']'
	}
	return ')'
}

func invalidHeader(message string) headerValidation {
	return headerValidation{status: headerInvalid, message: message}
}

func unsupportedHeader(message string) headerValidation {
	return headerValidation{status: headerUnsupported, message: message}
}
