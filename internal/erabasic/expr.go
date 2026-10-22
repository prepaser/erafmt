package erabasic

import (
	"bytes"
	"fmt"
	"unicode"
	"unicode/utf8"
)

type ExprError struct {
	Offset   int
	Msg      string
	Resource bool
}

func (e *ExprError) Error() string {
	return fmt.Sprintf("expression at byte %d: %s", e.Offset, e.Msg)
}

type exprTokenKind uint8

const (
	ExprIdentifier exprTokenKind = iota
	exprNumber
	exprString
	ExprOperator
	exprSymbol
	ExprForm
	ExprMarker
)

type ExprToken struct {
	Kind        exprTokenKind
	Text        []byte
	Off         int
	Binary      bool
	CaseKeyword bool
	FormMode    FormMode
	Fingerprint []byte
}

func expressionBodyEndWithSession(session *ExpressionSession, source ExpressionSourceID, src []byte, cacheStart int) (int, bool, error) {
	if session.HasAlias(source, cacheStart, cacheStart+len(src)) {
		return len(src), true, nil
	}
	quoted := false
	mandatorySeen := false
	for i := 0; i < len(src); {
		b := src[i]
		if quoted {
			if b == 0 {
				return 0, false, &ExprError{Offset: i, Msg: "unterminated string literal"}
			}
			if b == '\\' {
				if i+1 >= len(src) || src[i+1] == 0 {
					return 0, false, &ExprError{Offset: i, Msg: "missing character after escape"}
				}
				i += 2
				continue
			}
			if b == '"' {
				quoted = false
			}
			i++
			continue
		}
		switch b {
		case 0:
			return i, false, nil
		case '"':
			quoted = true
			i++
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				end, err := session.ScanFormAt(source, cacheStart+i, cacheStart+len(src), FormQuoted)
				if err != nil {
					return 0, false, ShiftExpressionError(err, -cacheStart)
				}
				i = end - cacheStart
				continue
			}
			i++
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				end, err := session.ScanFormAt(source, cacheStart+i, cacheStart+len(src), FormConditional)
				if err != nil {
					return 0, false, ShiftExpressionError(err, -cacheStart)
				}
				i = end - cacheStart
				continue
			}
			i++
		case '[':
			if i+1 < len(src) && src[i+1] == '[' {
				return len(src), true, nil
			}
			i++
		case ';':
			if MandatoryInlineMarker(src, i) {
				if mandatorySeen {
					return len(src), true, nil
				}
				mandatorySeen = true
				i += 3
				continue
			}
			if ExecutableMarker(src, i) {
				return len(src), true, nil
			}
			return i, false, nil
		default:
			r, size := utf8.DecodeRune(src[i:])
			if r == '\u3000' && !session.AllowFullwidthSpace {
				return len(src), true, nil
			}
			if isASCIIDigit(r) {
				end, err := scanNumber(src, i)
				if err != nil {
					return 0, false, err
				}
				i = end
				continue
			}
			if !isDelimiterWithSpace(r, session.AllowFullwidthSpace) {
				i = expressionIdentifierEnd(src, i)
				continue
			}
			i += size
		}
	}
	if quoted {
		return 0, false, &ExprError{Offset: len(src), Msg: "unterminated string literal"}
	}
	return len(src), false, nil
}

func ExpressionExcludedWithSession(session *ExpressionSession, src []byte) bool {
	excluded, _ := ExpressionExclusionWithSession(session, src)
	return excluded
}

func ExpressionExclusionWithSession(session *ExpressionSession, src []byte) (bool, error) {
	nested := NewExpressionSessionWithSpace(src, session != nil && session.AllowFullwidthSpace)
	_, excluded, err := expressionBodyEndWithSession(nested, ExpressionSourceOriginal, src, 0)
	if e, ok := err.(*ExprError); ok && ExecutableMarker(src, e.Offset) && !MandatoryInlineMarker(src, e.Offset) {
		return true, nil
	}
	return excluded, err
}

func expressionIdentifierEnd(src []byte, start int) int {
	for start < len(src) {
		r, size := utf8.DecodeRune(src[start:])
		if IsDelimiter(r) || r == '\u3000' {
			break
		}
		start += size
	}
	return start
}

func ExecutableMarker(src []byte, off int) bool {
	if off+3 > len(src) {
		return false
	}
	return (src[off] == ';' && src[off+1] == '!' && src[off+2] == ';') || (src[off] == ';' && src[off+1] == '#' && src[off+2] == ';')
}

func LeadingTriviaEndWithSession(session *ExpressionSession, src []byte) int {
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRune(src[i:])
		if !session.inputSpace(r) {
			return i
		}
		i += size
	}
	return len(src)
}

func TrailingTriviaStartWithSession(session *ExpressionSession, src []byte) int {
	for i := len(src); i > 0; {
		r, size := utf8.DecodeLastRune(src[:i])
		if !session.inputSpace(r) {
			return i
		}
		i -= size
	}
	return 0
}

func lexExpressionCore(session *ExpressionSession, source ExpressionSourceID, src []byte, base, rangeStart int) ([]ExprToken, error) {
	tokens := make([]ExprToken, 0, 8)
	var delimiters []byte
	syntaxBytes := 0
	checkedTokens := 0
	for i := 0; i < len(src); {
		if checkedTokens < len(tokens) {
			syntaxBytes += expressionTokenBytes(tokens[len(tokens)-1])
			checkedTokens = len(tokens)
		}
		r, size := utf8.DecodeRune(src[i:])
		if r == 0 {
			break
		}
		if session.inputSpace(r) {
			i += size
			continue
		}
		if len(tokens) >= MaxExpressionTokens || session.LexTokens >= maxSessionTokens || syntaxBytes >= MaxSyntaxBytes {
			return nil, ShiftExpressionError(session.resource(rangeStart+i, "expression work exceeds the formatter resource limit"), base-rangeStart)
		}
		session.LexTokens++
		if MandatoryInlineMarker(src, i) {
			tokens = append(tokens, ExprToken{Kind: ExprMarker, Text: src[i : i+3], Off: base + i})
			i += 3
			continue
		}
		if r == '"' {
			start := i
			i++
			closed := false
			for i < len(src) {
				switch src[i] {
				case 0:
					return nil, &ExprError{Offset: base + i, Msg: "unterminated string literal"}
				case '\\':
					if i+1 >= len(src) || src[i+1] == 0 {
						return nil, &ExprError{Offset: base + i, Msg: "missing character after escape"}
					}
					i += 2
				case '"':
					i++
					closed = true
				default:
					_, n := utf8.DecodeRune(src[i:])
					i += n
				}
				if closed {
					break
				}
			}
			if !closed {
				return nil, &ExprError{Offset: base + start, Msg: "unterminated string literal"}
			}
			tokens = append(tokens, ExprToken{Kind: exprString, Text: src[start:i], Off: base + start})
			continue
		}
		if r == '@' && i+1 < len(src) && src[i+1] == '"' {
			start := i
			endAbs, err := session.ScanFormAt(source, rangeStart+start, rangeStart+len(src), FormQuoted)
			if err != nil {
				return nil, ShiftExpressionError(err, -rangeStart+base)
			}
			end := endAbs - rangeStart
			fingerprint, err := session.FormFingerprintAt(source, rangeStart+start, endAbs, FormQuoted)
			if err != nil {
				return nil, ShiftExpressionError(err, -rangeStart+base)
			}
			i = end
			tokens = append(tokens, ExprToken{Kind: ExprForm, Text: src[start:i], FormMode: FormQuoted, Fingerprint: fingerprint, Off: base + start})
			continue
		}
		if r == '\\' && i+1 < len(src) && src[i+1] == '@' {
			start := i
			endAbs, err := session.ScanFormAt(source, rangeStart+start, rangeStart+len(src), FormConditional)
			if err != nil {
				return nil, ShiftExpressionError(err, -rangeStart+base)
			}
			end := endAbs - rangeStart
			fingerprint, err := session.FormFingerprintAt(source, rangeStart+start, endAbs, FormConditional)
			if err != nil {
				return nil, ShiftExpressionError(err, -rangeStart+base)
			}
			i = end
			tokens = append(tokens, ExprToken{Kind: ExprForm, Text: src[start:i], FormMode: FormConditional, Fingerprint: fingerprint, Off: base + start})
			continue
		}
		if isASCIIDigit(r) {
			start := i
			end, err := scanNumber(src, i)
			if err != nil {
				err.Offset += base
				return nil, err
			}
			i = end
			tokens = append(tokens, ExprToken{Kind: exprNumber, Text: src[start:i], Off: base + start})
			continue
		}
		if !IsDelimiter(r) {
			start := i
			for i < len(src) {
				run, n := utf8.DecodeRune(src[i:])
				if isDelimiterWithSpace(run, session.AllowFullwidthSpace) {
					break
				}
				i += n
			}
			tokens = append(tokens, ExprToken{Kind: ExprIdentifier, Text: src[start:i], Off: base + start})
			continue
		}
		if isOperator(r) {
			start := i
			i += size
			if i < len(src) && ValidDoubleOperator(src[start], src[i]) {
				i++
			}
			tokens = append(tokens, ExprToken{Kind: ExprOperator, Text: src[start:i], Off: base + start})
			continue
		}
		start := i
		switch r {
		case '(':
			if len(delimiters) >= MaxExpressionDepth {
				return nil, ShiftExpressionError(session.resource(rangeStart+i, "expression nesting exceeds the formatter resource limit"), base-rangeStart)
			}
			delimiters = append(delimiters, ')')
		case ')':
			if len(delimiters) == 0 || delimiters[len(delimiters)-1] != ')' {
				return nil, &ExprError{Offset: base + start, Msg: "unexpected closing parenthesis"}
			}
			delimiters = delimiters[:len(delimiters)-1]
		case '[':
			if len(delimiters) >= MaxExpressionDepth {
				return nil, ShiftExpressionError(session.resource(rangeStart+i, "expression nesting exceeds the formatter resource limit"), base-rangeStart)
			}
			delimiters = append(delimiters, ']')
		case ']':
			if len(delimiters) == 0 || delimiters[len(delimiters)-1] != ']' {
				return nil, &ExprError{Offset: base + start, Msg: "unexpected closing bracket"}
			}
			delimiters = delimiters[:len(delimiters)-1]
		case ',', ':', '.', '{', '}', '$', '\\', '\'', ';', '@':
			if r == '{' || r == '}' || r == '$' || r == '\\' || r == '\'' || r == ';' {
				return nil, &ExprError{Offset: base + start, Msg: fmt.Sprintf("unsupported character %q", r)}
			}
			if r == '@' && !qualifiedVariableAt(session, src, tokens, i) {
				return nil, &ExprError{Offset: base + start, Msg: "@ must separate identifiers"}
			}
		default:
			return nil, &ExprError{Offset: base + start, Msg: fmt.Sprintf("unexpected character %q", r)}
		}
		i += size
		tokens = append(tokens, ExprToken{Kind: exprSymbol, Text: src[start:i], Off: base + start})
	}
	if checkedTokens < len(tokens) {
		syntaxBytes += expressionTokenBytes(tokens[len(tokens)-1])
	}
	if syntaxBytes > MaxSyntaxBytes {
		return nil, ShiftExpressionError(session.resource(rangeStart+len(src), "expression syntax exceeds the formatter resource limit"), base-rangeStart)
	}
	if len(delimiters) != 0 {
		return nil, &ExprError{Offset: base + len(src), Msg: "unbalanced delimiters"}
	}
	if len(tokens) == 0 {
		return nil, &ExprError{Offset: base, Msg: "empty expression"}
	}
	return tokens, nil
}

func expressionTokenBytes(token ExprToken) int {
	if token.Kind == ExprForm {
		return len(token.Fingerprint)
	}
	return len(token.Text)
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func expressionWhitespace(r rune) bool {
	return r == ' ' || r == '\t'
}

func IsDelimiter(r rune) bool {
	if expressionWhitespace(r) {
		return true
	}
	switch r {
	case '.', '+', '-', '*', '/', '%', '=', '!', '<', '>', '|', '&', '^', '~', '?', '#', ')', '}', ']', ',', ':', '(', '{', '[', '$', '\\', '\'', '"', '@', ';':
		return true
	default:
		return false
	}
}

func isDelimiterWithSpace(r rune, allowFullwidthSpace bool) bool {
	if allowFullwidthSpace && r == '\u3000' {
		return true
	}
	return IsDelimiter(r)
}

func isOperator(r rune) bool {
	switch r {
	case '+', '-', '*', '/', '%', '=', '!', '<', '>', '|', '&', '^', '~', '?', '#':
		return true
	default:
		return false
	}
}

func ValidDoubleOperator(first, second byte) bool {
	switch first {
	case '+', '-', '=':
		return second == first || (first == '=' && second == '=')
	case '!':
		return second == '=' || second == '&' || second == '|'
	case '<', '>':
		return second == '=' || second == first
	case '|', '&', '^':
		return second == first
	default:
		return false
	}
}

func scanNumber(src []byte, start int) (int, *ExprError) {
	i := start
	base := 10
	if src[i] == '0' && i+1 < len(src) {
		switch src[i+1] {
		case 'x', 'X':
			base = 16
			i += 2
		case 'b', 'B':
			base = 2
			i += 2
		}
	}
	if base != 10 && i < len(src) && src[i] == '+' {
		i++
	}
	digitStart := i
	for i < len(src) {
		b := src[i]
		if !digitForBase(b, base) {
			r, _ := utf8.DecodeRune(src[i:])
			if r > 127 && r <= 0xffff && unicode.IsDigit(r) {
				return i, &ExprError{Offset: i, Msg: "non-ASCII digit in numeric literal"}
			}
			if base == 2 && b >= '0' && b <= '9' {
				return i, &ExprError{Offset: i, Msg: "invalid binary digit"}
			}
			break
		}
		i++
	}
	if i == digitStart {
		return i, &ExprError{Offset: start, Msg: "number has no digits"}
	}
	mantissaEnd := i
	var exponent []byte
	expBase := baseExponent(src[i:], base)
	if expBase != 0 {
		i++
		exponentStart := i
		if i < len(src) && src[i] == '-' && base != 10 {
			return i, &ExprError{Offset: i, Msg: "non-decimal exponent cannot have a minus sign"}
		}
		if i < len(src) && (src[i] == '+' || src[i] == '-') {
			i++
		}
		expStart := i
		for i < len(src) && digitForBase(src[i], base) {
			if base == 2 && src[i] > '1' {
				return i, &ExprError{Offset: i, Msg: "invalid binary exponent digit"}
			}
			i++
		}
		if base == 2 && i < len(src) && src[i] >= '0' && src[i] <= '9' {
			return i, &ExprError{Offset: i, Msg: "invalid binary exponent digit"}
		}
		if i < len(src) {
			r, _ := utf8.DecodeRune(src[i:])
			if r > 127 && r <= 0xffff && unicode.IsDigit(r) {
				return i, &ExprError{Offset: i, Msg: "non-ASCII digit in numeric exponent"}
			}
		}
		if i == expStart {
			return i, &ExprError{Offset: expStart, Msg: "number exponent has no digits"}
		}
		exponent = src[exponentStart:i]
	}
	if !numberLiteralFits(src[digitStart:mantissaEnd], exponent, base, expBase) {
		return i, &ExprError{Offset: start, Msg: "number literal is outside the signed 64-bit range"}
	}
	return i, nil
}

func baseExponent(src []byte, base int) int {
	if len(src) == 0 {
		return 0
	}
	if (src[0] == 'e' || src[0] == 'E') && base == 10 {
		return 10
	}
	if src[0] == 'p' || src[0] == 'P' {
		return 2
	}
	if (src[0] == 'e' || src[0] == 'E') && base == 2 {
		return 10
	}
	return 0
}

func digitForBase(b byte, base int) bool {
	if b >= '0' && b <= '9' {
		return int(b-'0') < base
	}
	if base == 16 {
		return (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
	}
	return false
}

func qualifiedVariableAt(session *ExpressionSession, src []byte, tokens []ExprToken, at int) bool {
	if len(tokens) == 0 || tokens[len(tokens)-1].Kind != ExprIdentifier {
		return false
	}
	for at++; at < len(src); {
		r, size := utf8.DecodeRune(src[at:])
		if session.inputSpace(r) {
			at += size
			continue
		}
		return !isDelimiterWithSpace(r, session.AllowFullwidthSpace) && !isASCIIDigit(r) && r != utf8.RuneError
	}
	return false
}

func ExpressionWordLike(token ExprToken) bool {
	return token.Kind == ExprIdentifier || token.Kind == exprNumber || token.Kind == exprString || token.Kind == ExprForm
}

func classifyOperators(tokens []ExprToken, caseMode bool) {
	operand := true
	depth := 0
	clauseStart := true
	for i := range tokens {
		t := &tokens[i]
		switch t.Kind {
		case ExprOperator:
			switch string(t.Text) {
			case "+", "-":
				t.Binary = !operand
				operand = true
			case "!", "~":
				operand = true
			case "++", "--":
			default:
				t.Binary = true
				operand = true
			}
			clauseStart = false
		case exprSymbol:
			operand = t.Text[0] != ')' && t.Text[0] != ']'
			if t.Text[0] == '(' || t.Text[0] == '[' {
				depth++
			} else if t.Text[0] == ')' || t.Text[0] == ']' {
				depth--
			}
			clauseStart = t.Text[0] == ',' && depth == 0
		case ExprMarker:
			continue
		default:
			t.CaseKeyword = caseMode && depth == 0 && t.Kind == ExprIdentifier && len(t.Text) == 2 && ((bytes.EqualFold(t.Text, []byte("IS")) && clauseStart) || (bytes.EqualFold(t.Text, []byte("TO")) && !operand))
			operand = t.CaseKeyword
			clauseStart = false
		}
	}
}
