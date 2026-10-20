package format

import (
	"bytes"
	"fmt"
	"unicode"
	"unicode/utf8"
)

type exprError struct {
	offset   int
	msg      string
	resource bool
}

func (e *exprError) Error() string {
	return fmt.Sprintf("expression at byte %d: %s", e.offset, e.msg)
}

type exprTokenKind uint8

const (
	exprIdentifier exprTokenKind = iota
	exprNumber
	exprString
	exprOperator
	exprSymbol
	exprForm
	exprMarker
)

type exprToken struct {
	kind        exprTokenKind
	text        []byte
	off         int
	binary      bool
	caseKeyword bool
	formMode    formMode
	fingerprint []byte
}

func expressionEditsWithSession(session *expressionSession, source expressionSourceID, src []byte, cacheStart int, caseMode bool, rules ruleSet, verify bool) ([]edit, error) {
	if len(src) == 0 {
		return nil, nil
	}
	if !session.validUTF8(source) {
		return nil, &exprError{msg: "invalid UTF-8"}
	}

	bodyEnd, unsupported, err := expressionBodyEndWithSession(session, source, src, cacheStart)
	if err != nil {
		return nil, err
	}
	if unsupported {
		return nil, nil
	}

	start := leadingTriviaEndWithSession(session, src[:bodyEnd])
	end := trailingTriviaStartWithSession(session, src[:bodyEnd])
	if start >= end {
		return nil, nil
	}
	tokens, err := session.lexRange(source, cacheStart+start, cacheStart+end, start)
	if err != nil {
		return nil, err
	}
	if !session.skipFormTypes {
		if err := provenExpressionsValid(tokens, session.builtinTypes, session.dialect); err != nil {
			return nil, err
		}
	}
	classifyOperators(tokens, caseMode)
	var edits []edit
	for _, token := range tokens {
		if token.kind != exprForm {
			continue
		}
		nested, nestedErr := formEditsWithSession(session, source, token.text, cacheStart+token.off, rules, token.formMode, false)
		if nestedErr != nil {
			return nil, shiftExpressionError(nestedErr, token.off)
		}
		for _, edit := range nested {
			edit.start += token.off
			edit.end += token.off
			edits = append(edits, edit)
		}
	}
	for i := 1; i < len(tokens); i++ {
		if tokens[i-1].kind == exprMarker || tokens[i].kind == exprMarker {
			continue
		}
		rule := expressionGapRule(tokens[i-1], tokens[i])
		if !rules[rule] {
			continue
		}
		text := []byte{}
		if expressionNeedsSpace(tokens, i-1, i, caseMode) {
			text = []byte{' '}
		}
		gapStart := tokens[i-1].off + len(tokens[i-1].text)
		gapEnd := tokens[i].off
		if bytes.Contains(src[gapStart:gapEnd], []byte(";!;")) {
			continue
		}
		if !bytes.Equal(src[gapStart:gapEnd], text) {
			edits = append(edits, edit{gapStart, gapEnd, text, Change{Rule: rule}})
		}
	}
	if len(edits) == 0 {
		return nil, nil
	}
	if !verify {
		return edits, nil
	}
	formatted := applyEdits(src, edits)
	session.setSource(expressionSourceFormatted, formatted)
	formattedEnd := len(formatted) - (len(src) - end)
	check, err := session.lexRange(expressionSourceFormatted, start, formattedEnd, start)
	if err != nil || !sameExpressionTokens(tokens, check) {
		if err == nil {
			err = &exprError{offset: start, msg: "formatting changed expression tokens"}
		}
		return nil, err
	}
	return edits, nil
}

func expressionBodyEndWithSession(session *expressionSession, source expressionSourceID, src []byte, cacheStart int) (int, bool, error) {
	if session.hasAlias(source, cacheStart, cacheStart+len(src)) {
		return len(src), true, nil
	}
	quoted := false
	mandatorySeen := false
	for i := 0; i < len(src); {
		b := src[i]
		if quoted {
			if b == 0 {
				return 0, false, &exprError{offset: i, msg: "unterminated string literal"}
			}
			if b == '\\' {
				if i+1 >= len(src) || src[i+1] == 0 {
					return 0, false, &exprError{offset: i, msg: "missing character after escape"}
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
				end, err := session.scanFormAt(source, cacheStart+i, cacheStart+len(src), formQuoted)
				if err != nil {
					return 0, false, shiftExpressionError(err, -cacheStart)
				}
				i = end - cacheStart
				continue
			}
			i++
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				end, err := session.scanFormAt(source, cacheStart+i, cacheStart+len(src), formConditional)
				if err != nil {
					return 0, false, shiftExpressionError(err, -cacheStart)
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
			if mandatoryInlineMarker(src, i) {
				if mandatorySeen {
					return len(src), true, nil
				}
				mandatorySeen = true
				i += 3
				continue
			}
			if executableMarker(src, i) {
				return len(src), true, nil
			}
			return i, false, nil
		default:
			r, size := utf8.DecodeRune(src[i:])
			if r == '\u3000' && !session.allowFullwidthSpace {
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
			if !isDelimiterWithSpace(r, session.allowFullwidthSpace) {
				i = expressionIdentifierEnd(src, i)
				continue
			}
			i += size
		}
	}
	if quoted {
		return 0, false, &exprError{offset: len(src), msg: "unterminated string literal"}
	}
	return len(src), false, nil
}

func expressionExcludedWithSession(session *expressionSession, src []byte) bool {
	excluded, _ := expressionExclusionWithSession(session, src)
	return excluded
}

func expressionExclusionWithSession(session *expressionSession, src []byte) (bool, error) {
	nested := newExpressionSessionWithSpace(src, session != nil && session.allowFullwidthSpace)
	_, excluded, err := expressionBodyEndWithSession(nested, expressionSourceOriginal, src, 0)
	if e, ok := err.(*exprError); ok && executableMarker(src, e.offset) && !mandatoryInlineMarker(src, e.offset) {
		return true, nil
	}
	return excluded, err
}

func expressionIdentifierEnd(src []byte, start int) int {
	for start < len(src) {
		r, size := utf8.DecodeRune(src[start:])
		if isDelimiter(r) || r == '\u3000' {
			break
		}
		start += size
	}
	return start
}

func executableMarker(src []byte, off int) bool {
	if off+3 > len(src) {
		return false
	}
	return (src[off] == ';' && src[off+1] == '!' && src[off+2] == ';') ||
		(src[off] == ';' && src[off+1] == '#' && src[off+2] == ';')
}

func leadingTriviaEndWithSession(session *expressionSession, src []byte) int {
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRune(src[i:])
		if !session.inputSpace(r) {
			return i
		}
		i += size
	}
	return len(src)
}

func trailingTriviaStartWithSession(session *expressionSession, src []byte) int {
	for i := len(src); i > 0; {
		r, size := utf8.DecodeLastRune(src[:i])
		if !session.inputSpace(r) {
			return i
		}
		i -= size
	}
	return 0
}

func lexExpressionCore(session *expressionSession, source expressionSourceID, src []byte, base, rangeStart int) ([]exprToken, error) {
	tokens := make([]exprToken, 0, 8)
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
		if len(tokens) >= maxExpressionTokens || session.lexTokens >= maxSessionTokens || syntaxBytes >= maxSyntaxBytes {
			return nil, shiftExpressionError(session.resource(rangeStart+i, "expression work exceeds the formatter resource limit"), base-rangeStart)
		}
		session.lexTokens++
		if mandatoryInlineMarker(src, i) {
			tokens = append(tokens, exprToken{kind: exprMarker, text: src[i : i+3], off: base + i})
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
					return nil, &exprError{offset: base + i, msg: "unterminated string literal"}
				case '\\':
					if i+1 >= len(src) || src[i+1] == 0 {
						return nil, &exprError{offset: base + i, msg: "missing character after escape"}
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
				return nil, &exprError{offset: base + start, msg: "unterminated string literal"}
			}
			tokens = append(tokens, exprToken{kind: exprString, text: src[start:i], off: base + start})
			continue
		}
		if r == '@' && i+1 < len(src) && src[i+1] == '"' {
			start := i
			endAbs, err := session.scanFormAt(source, rangeStart+start, rangeStart+len(src), formQuoted)
			if err != nil {
				return nil, shiftExpressionError(err, -rangeStart+base)
			}
			end := endAbs - rangeStart
			fingerprint, err := session.formFingerprintAt(source, rangeStart+start, endAbs, formQuoted)
			if err != nil {
				return nil, shiftExpressionError(err, -rangeStart+base)
			}
			i = end
			tokens = append(tokens, exprToken{kind: exprForm, text: src[start:i], formMode: formQuoted, fingerprint: fingerprint, off: base + start})
			continue
		}
		if r == '\\' && i+1 < len(src) && src[i+1] == '@' {
			start := i
			endAbs, err := session.scanFormAt(source, rangeStart+start, rangeStart+len(src), formConditional)
			if err != nil {
				return nil, shiftExpressionError(err, -rangeStart+base)
			}
			end := endAbs - rangeStart
			fingerprint, err := session.formFingerprintAt(source, rangeStart+start, endAbs, formConditional)
			if err != nil {
				return nil, shiftExpressionError(err, -rangeStart+base)
			}
			i = end
			tokens = append(tokens, exprToken{kind: exprForm, text: src[start:i], formMode: formConditional, fingerprint: fingerprint, off: base + start})
			continue
		}
		if isASCIIDigit(r) {
			start := i
			end, err := scanNumber(src, i)
			if err != nil {
				err.offset += base
				return nil, err
			}
			i = end
			tokens = append(tokens, exprToken{kind: exprNumber, text: src[start:i], off: base + start})
			continue
		}
		if !isDelimiter(r) {
			start := i
			for i < len(src) {
				run, n := utf8.DecodeRune(src[i:])
				if isDelimiterWithSpace(run, session.allowFullwidthSpace) {
					break
				}
				i += n
			}
			tokens = append(tokens, exprToken{kind: exprIdentifier, text: src[start:i], off: base + start})
			continue
		}

		if isOperator(r) {
			start := i
			i += size
			if i < len(src) && validDoubleOperator(src[start], src[i]) {
				i++
			}
			tokens = append(tokens, exprToken{kind: exprOperator, text: src[start:i], off: base + start})
			continue
		}

		start := i
		switch r {
		case '(':
			if len(delimiters) >= maxExpressionDepth {
				return nil, shiftExpressionError(session.resource(rangeStart+i, "expression nesting exceeds the formatter resource limit"), base-rangeStart)
			}
			delimiters = append(delimiters, ')')
		case ')':
			if len(delimiters) == 0 || delimiters[len(delimiters)-1] != ')' {
				return nil, &exprError{offset: base + start, msg: "unexpected closing parenthesis"}
			}
			delimiters = delimiters[:len(delimiters)-1]
		case '[':
			if len(delimiters) >= maxExpressionDepth {
				return nil, shiftExpressionError(session.resource(rangeStart+i, "expression nesting exceeds the formatter resource limit"), base-rangeStart)
			}
			delimiters = append(delimiters, ']')
		case ']':
			if len(delimiters) == 0 || delimiters[len(delimiters)-1] != ']' {
				return nil, &exprError{offset: base + start, msg: "unexpected closing bracket"}
			}
			delimiters = delimiters[:len(delimiters)-1]
		case ',', ':', '.', '{', '}', '$', '\\', '\'', ';', '@':
			if r == '{' || r == '}' || r == '$' || r == '\\' || r == '\'' || r == ';' {
				return nil, &exprError{offset: base + start, msg: fmt.Sprintf("unsupported character %q", r)}
			}
			if r == '@' && !qualifiedVariableAt(session, src, tokens, i) {
				return nil, &exprError{offset: base + start, msg: "@ must separate identifiers"}
			}
		default:
			return nil, &exprError{offset: base + start, msg: fmt.Sprintf("unexpected character %q", r)}
		}
		i += size
		tokens = append(tokens, exprToken{kind: exprSymbol, text: src[start:i], off: base + start})
	}
	if checkedTokens < len(tokens) {
		syntaxBytes += expressionTokenBytes(tokens[len(tokens)-1])
	}
	if syntaxBytes > maxSyntaxBytes {
		return nil, shiftExpressionError(session.resource(rangeStart+len(src), "expression syntax exceeds the formatter resource limit"), base-rangeStart)
	}
	if len(delimiters) != 0 {
		return nil, &exprError{offset: base + len(src), msg: "unbalanced delimiters"}
	}
	if len(tokens) == 0 {
		return nil, &exprError{offset: base, msg: "empty expression"}
	}
	return tokens, nil
}

func expressionTokenBytes(token exprToken) int {
	if token.kind == exprForm {
		return len(token.fingerprint)
	}
	return len(token.text)
}

func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }

func expressionWhitespace(r rune) bool { return r == ' ' || r == '\t' }

func isDelimiter(r rune) bool {
	if expressionWhitespace(r) {
		return true
	}
	switch r {
	case '.', '+', '-', '*', '/', '%', '=', '!', '<', '>', '|', '&', '^', '~', '?', '#',
		')', '}', ']', ',', ':', '(', '{', '[', '$', '\\', '\'', '"', '@', ';':
		return true
	default:
		return false
	}
}

func isDelimiterWithSpace(r rune, allowFullwidthSpace bool) bool {
	if allowFullwidthSpace && r == '\u3000' {
		return true
	}
	return isDelimiter(r)
}

func isOperator(r rune) bool {
	switch r {
	case '+', '-', '*', '/', '%', '=', '!', '<', '>', '|', '&', '^', '~', '?', '#':
		return true
	default:
		return false
	}
}

func validDoubleOperator(first, second byte) bool {
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

func scanNumber(src []byte, start int) (int, *exprError) {
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
				return i, &exprError{offset: i, msg: "non-ASCII digit in numeric literal"}
			}
			if base == 2 && b >= '0' && b <= '9' {
				return i, &exprError{offset: i, msg: "invalid binary digit"}
			}
			break
		}
		i++
	}
	if i == digitStart {
		return i, &exprError{offset: start, msg: "number has no digits"}
	}
	mantissaEnd := i
	var exponent []byte
	expBase := baseExponent(src[i:], base)
	if expBase != 0 {
		i++
		exponentStart := i
		if i < len(src) && src[i] == '-' && base != 10 {
			return i, &exprError{offset: i, msg: "non-decimal exponent cannot have a minus sign"}
		}
		if i < len(src) && (src[i] == '+' || src[i] == '-') {
			i++
		}
		expStart := i
		for i < len(src) && digitForBase(src[i], base) {
			if base == 2 && src[i] > '1' {
				return i, &exprError{offset: i, msg: "invalid binary exponent digit"}
			}
			i++
		}
		if base == 2 && i < len(src) && src[i] >= '0' && src[i] <= '9' {
			return i, &exprError{offset: i, msg: "invalid binary exponent digit"}
		}
		if i < len(src) {
			r, _ := utf8.DecodeRune(src[i:])
			if r > 127 && r <= 0xffff && unicode.IsDigit(r) {
				return i, &exprError{offset: i, msg: "non-ASCII digit in numeric exponent"}
			}
		}
		if i == expStart {
			return i, &exprError{offset: expStart, msg: "number exponent has no digits"}
		}
		exponent = src[exponentStart:i]
	}
	if !numberLiteralFits(src[digitStart:mantissaEnd], exponent, base, expBase) {
		return i, &exprError{offset: start, msg: "number literal is outside the signed 64-bit range"}
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

func sameExpressionTokens(a, b []exprToken) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].kind != b[i].kind {
			return false
		}
		if a[i].kind == exprForm {
			if a[i].formMode != b[i].formMode {
				return false
			}
			if !bytes.Equal(a[i].fingerprint, b[i].fingerprint) {
				return false
			}
			continue
		}
		if !bytes.Equal(a[i].text, b[i].text) {
			return false
		}
	}
	return true
}

func qualifiedVariableAt(session *expressionSession, src []byte, tokens []exprToken, at int) bool {
	if len(tokens) == 0 || tokens[len(tokens)-1].kind != exprIdentifier {
		return false
	}
	for at++; at < len(src); {
		r, size := utf8.DecodeRune(src[at:])
		if session.inputSpace(r) {
			at += size
			continue
		}
		return !isDelimiterWithSpace(r, session.allowFullwidthSpace) && !isASCIIDigit(r) && r != utf8.RuneError
	}
	return false
}

func expressionGapRule(prev, cur exprToken) RuleID {
	if prev.kind == exprMarker || cur.kind == exprMarker {
		return ""
	}
	if cur.text[0] == ',' {
		return CommaSpacing
	}
	if cur.text[0] == ')' || cur.text[0] == ']' {
		return DelimiterSpacing
	}
	if prev.text[0] == ',' {
		return CommaSpacing
	}
	if prev.text[0] == ':' || cur.text[0] == ':' {
		return IndexSpacing
	}
	if prev.text[0] == '(' || prev.text[0] == '[' {
		return DelimiterSpacing
	}
	if prev.caseKeyword || cur.caseKeyword {
		return CaseSpacing
	}
	if prev.kind == exprOperator || cur.kind == exprOperator {
		if prev.binary || cur.binary {
			if (prev.binary && (prev.text[0] == '?' || prev.text[0] == '#')) ||
				(cur.binary && (cur.text[0] == '?' || cur.text[0] == '#')) {
				return TernarySpacing
			}
			return BinarySpacing
		}
		return UnarySpacing
	}
	if cur.text[0] == '(' || cur.text[0] == '[' {
		return DelimiterSpacing
	}
	return TokenSpacing
}

func expressionNeedsSpace(tokens []exprToken, prevIndex, index int, caseMode bool) bool {
	prev, cur := tokens[prevIndex], tokens[index]
	if prev.kind == exprOperator && cur.kind == exprOperator {
		return prev.binary || cur.binary || (len(prev.text) == 1 && validDoubleOperator(prev.text[0], cur.text[0]))
	}
	if cur.text[0] == ')' || cur.text[0] == ']' || cur.text[0] == ',' {
		return false
	}
	if prev.text[0] == ',' {
		return true
	}
	if prev.text[0] == '(' || prev.text[0] == '[' || prev.text[0] == ':' || cur.text[0] == ':' {
		return false
	}
	if caseMode {
		if prev.caseKeyword {
			return cur.text[0] != ')' && cur.text[0] != ']' && cur.text[0] != ','
		}
		if cur.caseKeyword {
			return prev.text[0] != '(' && prev.text[0] != '['
		}
	}
	if prev.kind == exprOperator {
		return prev.binary
	}
	if cur.kind == exprOperator {
		return cur.binary
	}
	if cur.text[0] == '(' || cur.text[0] == '[' {
		return false
	}
	return expressionWordLike(prev) && expressionWordLike(cur)
}

func expressionWordLike(token exprToken) bool {
	return token.kind == exprIdentifier || token.kind == exprNumber || token.kind == exprString || token.kind == exprForm
}

func classifyOperators(tokens []exprToken, caseMode bool) {
	operand := true
	depth := 0
	clauseStart := true
	for i := range tokens {
		t := &tokens[i]
		switch t.kind {
		case exprOperator:
			switch string(t.text) {
			case "+", "-":
				t.binary = !operand
				operand = true
			case "!", "~":
				operand = true
			case "++", "--":
			default:
				t.binary = true
				operand = true
			}
			clauseStart = false
		case exprSymbol:
			operand = t.text[0] != ')' && t.text[0] != ']'
			if t.text[0] == '(' || t.text[0] == '[' {
				depth++
			} else if t.text[0] == ')' || t.text[0] == ']' {
				depth--
			}
			clauseStart = t.text[0] == ',' && depth == 0
		case exprMarker:
			continue
		default:
			t.caseKeyword = caseMode && depth == 0 && t.kind == exprIdentifier && len(t.text) == 2 &&
				((bytes.EqualFold(t.text, []byte("IS")) && clauseStart) ||
					(bytes.EqualFold(t.text, []byte("TO")) && !operand))
			operand = t.caseKeyword
			clauseStart = false
		}
	}
}
