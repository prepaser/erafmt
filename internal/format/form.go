package format

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf8"
)

type formMode uint8

const (
	formLine formMode = iota
	formQuoted
	formCall
	formComma
	formConditional
)

type formStop uint8

const (
	formStopEnd formStop = iota
	formStopQuote
	formStopCall
	formStopComma
	formStopSharp
	formStopYenAt
)

type formParser struct {
	src            []byte
	rules          ruleSet
	edits          []edit
	fingerprint    *bytes.Buffer
	firstIslandErr error
	session        *expressionSession
	source         expressionSourceID
	base           int
	verify         bool
}

func (p *formParser) sessionOrNew() *expressionSession {
	if p.session == nil {
		p.session = newExpressionSession(p.src)
		p.source = expressionSourceOriginal
		p.base = 0
	}
	return p.session
}

func formEditsWithSession(session *expressionSession, source expressionSourceID, src []byte, cacheStart int, rules ruleSet, mode formMode, verify bool) ([]edit, error) {
	if !session.validUTF8(source) {
		return nil, &exprError{msg: "invalid UTF-8"}
	}
	if session.skipFormTypes {
		return nil, nil
	}
	if session.hasAlias(source, cacheStart, cacheStart+len(src)) {
		return nil, nil
	}
	p := formParser{src: src, rules: rules, session: session, source: source, base: cacheStart, verify: verify}
	end, err := p.scan(mode)
	if err != nil {
		return nil, err
	}
	if (mode == formQuoted || mode == formConditional) && end != len(src) {
		name := "quoted"
		if mode == formConditional {
			name = "conditional"
		}
		return nil, &exprError{offset: end, msg: "characters after " + name + " FORM"}
	}
	return p.edits, p.firstIslandErr
}

func scanForm(src []byte, mode formMode) (int, error) {
	session := newExpressionSession(src)
	end, err := session.scanFormAt(expressionSourceOriginal, 0, len(src), mode)
	return end, err
}

func (p *formParser) scanNested(start int, mode formMode) (int, error) {
	if p.session == nil {
		end, err := scanForm(p.src[start:], mode)
		return start + end, err
	}
	end, err := p.session.scanFormAt(p.source, p.base+start, p.base+len(p.src), mode)
	if err != nil {
		return 0, shiftExpressionError(err, -p.base)
	}
	return end - p.base, nil
}

func (p *formParser) scan(mode formMode) (end int, err error) {
	if p.session != nil {
		if err := p.session.enterForm(p.base); err != nil {
			return 0, shiftExpressionError(err, -p.base)
		}
		defer func() {
			p.session.leaveForm()
			if err == nil && p.session.exhausted != nil {
				end = 0
				err = shiftExpressionError(p.session.exhausted, -p.base)
			}
		}()
	}
	start := 0
	stop := formStopEnd
	switch mode {
	case formQuoted:
		if len(p.src) < 2 || p.src[0] != '@' || p.src[1] != '"' {
			return 0, &exprError{msg: "quoted FORM must start with @\""}
		}
		p.record(p.src[:2])
		start = 2
		stop = formStopQuote
	case formCall:
		stop = formStopCall
	case formComma:
		stop = formStopComma
	case formLine:
		stop = formStopEnd
	case formConditional:
		if len(p.src) < 2 || p.src[0] != '\\' || p.src[1] != '@' {
			return 0, &exprError{msg: "conditional FORM must start with \\@"}
		}
		return p.conditional(0)
	default:
		return 0, &exprError{msg: "unknown FORM mode"}
	}
	end, err = p.segment(start, stop)
	if err != nil {
		return 0, err
	}
	if stop == formStopQuote {
		if end >= len(p.src) || p.src[end] != '"' {
			return 0, &exprError{offset: end, msg: "unterminated quoted FORM"}
		}
		p.record(p.src[end : end+1])
		return end + 1, nil
	}
	return end, nil
}

func (p *formParser) segment(start int, stop formStop) (int, error) {
	for i := start; i < len(p.src); {
		if p.atStop(i, stop) {
			return i, nil
		}
		if p.src[i] == '\\' {
			if i+1 >= len(p.src) || p.src[i+1] == 0 {
				return 0, &exprError{offset: i, msg: "missing character after FORM escape"}
			}
			if p.src[i+1] == '@' {
				end, err := p.conditional(i)
				if err != nil {
					return 0, err
				}
				i = end
				continue
			}
			end := escapedEnd(p.src, i)
			p.record(p.src[i:end])
			i = end
			continue
		}
		if tripleFormSymbol(p.src, i) {
			p.record(p.src[i : i+3])
			i += 3
			continue
		}
		switch p.src[i] {
		case '%', '{':
			end, err := p.island(i)
			if err != nil {
				return 0, err
			}
			i = end
		default:
			_, size := utf8.DecodeRune(p.src[i:])
			if size == 0 {
				size = 1
			}
			p.record(p.src[i : i+size])
			i += size
		}
	}
	if stop == formStopQuote {
		return len(p.src), nil
	}
	return len(p.src), nil
}

func (p *formParser) atStop(i int, stop formStop) bool {
	if p.src[i] == 0 {
		return true
	}
	switch stop {
	case formStopQuote:
		return p.src[i] == '"'
	case formStopCall:
		return p.src[i] == '(' || p.src[i] == '[' || p.src[i] == ',' || p.src[i] == ';' || p.src[i] == '\r' || p.src[i] == '\n'
	case formStopComma:
		return p.src[i] == ',' || p.src[i] == '\r' || p.src[i] == '\n'
	case formStopSharp:
		return p.src[i] == '#' || (i+1 < len(p.src) && p.src[i] == '\\' && p.src[i+1] == '@')
	case formStopYenAt:
		return i+1 < len(p.src) && p.src[i] == '\\' && p.src[i+1] == '@'
	default:
		return p.src[i] == '\r' || p.src[i] == '\n'
	}
}

func (p *formParser) island(open int) (int, error) {
	closeByte, tag, valueType := byte('}'), byte('B'), knownValueNumeric
	if p.src[open] == '%' {
		closeByte, tag, valueType = '%', 'P', knownValueString
	}
	close, err := p.expressionEnd(open+1, closeByte)
	if err != nil {
		return 0, err
	}
	p.record(p.src[open : open+1])
	if p.fingerprint != nil {
		if err := p.recordExpressionFingerprint(open+1, close, tag); err != nil {
			return 0, err
		}
	} else if p.rules != nil {
		if err := p.formatIsland(open+1, close, valueType); err != nil && !p.noteIslandError(err) {
			return 0, err
		}
	}
	p.record(p.src[close : close+1])
	return close + 1, nil
}

func (p *formParser) expressionEnd(start int, close byte) (int, error) {
	var delimiters []byte
	for i := start; i < len(p.src); {
		r := p.src[i]
		if r == 0 {
			return 0, &exprError{offset: i, msg: fmt.Sprintf("missing FORM delimiter %q", close)}
		}
		if r == '"' {
			i = quotedEnd(p.src, i)
			if i < 0 {
				return 0, &exprError{offset: start, msg: "unterminated string in FORM expression"}
			}
			continue
		}
		if r == '@' && i+1 < len(p.src) && p.src[i+1] == '"' {
			end, err := p.scanNested(i, formQuoted)
			if err != nil {
				return 0, err
			}
			i = end
			continue
		}
		if r == '\\' && i+1 < len(p.src) && p.src[i+1] == '@' {
			end, err := p.scanNested(i, formConditional)
			if err != nil {
				return 0, err
			}
			i = end
			continue
		}
		if len(delimiters) == 0 && r == close {
			return i, nil
		}
		if r == ';' {
			if mandatoryInlineMarker(p.src, i) {
				i += 3
				continue
			}
			return 0, &exprError{offset: i, msg: "FORM expression reaches a comment before its delimiter"}
		}
		switch r {
		case '(':
			if len(delimiters) >= maxExpressionDepth {
				return 0, shiftExpressionError(p.sessionOrNew().resource(p.base+i, "FORM expression nesting exceeds the formatter resource limit"), -p.base)
			}
			delimiters = append(delimiters, ')')
		case '[':
			if len(delimiters) >= maxExpressionDepth {
				return 0, shiftExpressionError(p.sessionOrNew().resource(p.base+i, "FORM expression nesting exceeds the formatter resource limit"), -p.base)
			}
			delimiters = append(delimiters, ']')
		case ')', ']':
			if len(delimiters) == 0 || delimiters[len(delimiters)-1] != r {
				return 0, &exprError{offset: i, msg: "misnested FORM expression delimiter"}
			}
			delimiters = delimiters[:len(delimiters)-1]
		case '{':
			return 0, &exprError{offset: i, msg: "nested FORM brace in expression"}
		}
		if isASCIIDigit(rune(r)) {
			end, _ := scanNumber(p.src, i)
			i = end
			continue
		}
		run, size := utf8.DecodeRune(p.src[i:])
		if !isDelimiter(run) && run != '\u3000' {
			i = expressionIdentifierEnd(p.src, i)
			continue
		}
		i += size
	}
	return 0, &exprError{offset: start, msg: fmt.Sprintf("missing FORM delimiter %q", close)}
}

func (p *formParser) formatIsland(start, end int, valueType knownValueType) error {
	innerStart, innerEnd := start, end
	editBase := innerStart
	session := p.sessionOrNew()
	if expressionExcludedWithSession(session, p.src[innerStart:innerEnd]) {
		return nil
	}
	if p.formExpressionHasMandatoryMarker(innerStart, innerEnd) {
		return nil
	}
	fields := p.splitTopLevel(innerStart, innerEnd)
	suspiciousWidth := p.noteIslandWidth(valueType, fields)
	if err := p.validateIslandShape(fields); err != nil {
		return &formIslandError{err: err}
	}
	if tokens, err := session.lexRange(p.source, p.base+innerStart, p.base+innerEnd, innerStart); err != nil {
		return &formIslandError{err: err}
	} else if err := formExpressionPrefix(tokens); err != nil {
		return &formIslandError{err: err}
	} else if err := provenExpressionShape(tokens, session.dialect, false); err != nil {
		return &formIslandError{err: err}
	}
	if err := p.validateIslandTypes(valueType, fields); err != nil {
		return &formIslandError{err: err}
	}
	if err := p.validateIslandAlignment(fields); err != nil {
		return &formIslandError{err: err}
	}
	cacheStart := p.base + innerStart
	edits, err := expressionEditsWithSession(session, p.source, p.src[innerStart:innerEnd], cacheStart, false, p.rules, p.verify)
	if err != nil {
		return &formIslandError{err: shiftExpressionError(err, innerStart)}
	}
	if leadingTriviaEndWithSession(session, p.src[innerStart:innerEnd]) >= trailingTriviaStartWithSession(session, p.src[innerStart:innerEnd]) {
		return &formIslandError{err: &exprError{offset: start, msg: "empty FORM expression"}}
	}
	if suspiciousWidth {
		return nil
	}
	var local []edit
	if p.rules[FormPadding] {
		innerStart = trimInputLeftWithSession(p.src, innerStart, innerEnd, session)
		innerEnd = trimInputRight(p.src, innerStart, innerEnd, session.allowFullwidthSpace)
		if innerStart != start {
			local = append(local, edit{start, innerStart, nil, Change{Rule: FormPadding}})
		}
		if innerEnd != end {
			local = append(local, edit{innerEnd, end, nil, Change{Rule: FormPadding}})
		}
	}
	for _, e := range edits {
		e.start += editBase
		e.end += editBase
		local = append(local, e)
	}
	p.edits = append(p.edits, local...)
	return nil
}

func (p *formParser) noteIslandWidth(valueType knownValueType, fields []syntaxSpan) bool {
	if valueType != knownValueNumeric || p.session == nil || p.session.skipFormTypes {
		return false
	}
	if len(fields) < 2 {
		return false
	}
	valueSpan := fields[0]
	valueStart := trimInputLeftWithSession(p.src, valueSpan.start, valueSpan.end, p.session)
	valueEnd := trimInputRight(p.src, valueStart, valueSpan.end, p.session.allowFullwidthSpace)
	if valueStart >= valueEnd {
		return false
	}
	valueTokens, err := p.session.lexRange(p.source, p.base+valueStart, p.base+valueEnd, valueStart)
	if err != nil || commandMacroPossibleForDialect(valueTokens, p.session.dialect) || knownExpressionTypeForDialect(valueTokens, p.session.dialect) != knownValueNumeric {
		return false
	}
	width := fields[1]
	widthStart := trimInputLeftWithSession(p.src, width.start, width.end, p.session)
	widthEnd := trimInputRight(p.src, widthStart, width.end, p.session.allowFullwidthSpace)
	if widthStart >= widthEnd {
		return false
	}
	widthTokens, err := p.session.lexRange(p.source, p.base+widthStart, p.base+widthEnd, widthStart)
	if err != nil || commandMacroPossibleForDialect(widthTokens, p.session.dialect) {
		return false
	}
	widthValue, known := constantIntegerValue(widthTokens)
	if !known || int32(widthValue) >= 0 {
		return false
	}
	p.session.addFormAdvisory(formAdvisory{
		source:  p.source,
		offset:  p.base + widthStart,
		code:    "suspicious_form_width",
		message: "FORM width is negative after signed 32-bit conversion",
	})
	return true
}

func (p *formParser) formFieldType(start, end int) (knownValueType, bool) {
	session := p.sessionOrNew()
	tokens, err := session.lexRange(p.source, p.base+start, p.base+end, start)
	if err != nil || commandMacroPossibleForDialect(tokens, session.dialect) {
		return knownValueUnknown, true
	}
	return knownExpressionTypeForDialect(tokens, session.dialect), false
}

func (p *formParser) scalarIndexesMissing(start, end int) bool {
	session := p.sessionOrNew()
	tokens, err := session.lexRange(p.source, p.base+start, p.base+end, start)
	return err == nil && builtinScalarIndexesMissing(tokens, session.dialect)
}

func (p *formParser) validateIslandTypes(valueType knownValueType, fields []syntaxSpan) error {
	session := p.sessionOrNew()
	if session.skipFormTypes {
		return nil
	}
	for index, field := range fields[:min(2, len(fields))] {
		left := trimInputLeftWithSession(p.src, field.start, field.end, session)
		right := trimInputRight(p.src, left, field.end, session.allowFullwidthSpace)
		if left == right {
			return nil
		}
		if p.scalarIndexesMissing(left, right) {
			return &exprError{offset: left, msg: "scalar variable requires indexes"}
		}
		actual, macro := p.formFieldType(left, right)
		if macro {
			return nil
		}
		expected, message := valueType, "FORM placeholder expression must be numeric"
		if valueType == knownValueString {
			message = "FORM percent expression must be a string"
		}
		if index > 0 {
			expected, message = knownValueNumeric, "FORM width must be numeric"
		}
		if !commandTypeMatches(expected, actual) {
			return &exprError{offset: left, msg: message}
		}
	}
	return nil
}

func (p *formParser) validateIslandShape(fields []syntaxSpan) error {
	first := fields[0]
	firstLeft := trimInputLeftWithSession(p.src, first.start, first.end, p.session)
	firstRight := trimInputRight(p.src, firstLeft, first.end, p.session.allowFullwidthSpace)
	if firstLeft == firstRight {
		return &exprError{offset: first.start, msg: "empty FORM expression"}
	}
	if len(fields) > 3 {
		return &exprError{offset: fields[3].start, msg: "too many FORM placeholder fields"}
	}
	if len(fields) == 3 {
		field := fields[1]
		left := trimInputLeftWithSession(p.src, field.start, field.end, p.session)
		right := trimInputRight(p.src, left, field.end, p.session.allowFullwidthSpace)
		if left == right {
			return &exprError{offset: field.start, msg: "empty FORM width before alignment"}
		}
		alignment := fields[2]
		left = trimInputLeftWithSession(p.src, alignment.start, alignment.end, p.session)
		right = trimInputRight(p.src, left, alignment.end, p.session.allowFullwidthSpace)
		if left < right {
			tokens, err := p.session.lexRange(p.source, p.base+left, p.base+right, left)
			if err != nil {
				return err
			}
			if len(tokens) == 0 {
				return &exprError{offset: left, msg: "FORM alignment must be an identifier"}
			}
			if tokens[0].kind != exprIdentifier {
				return &exprError{offset: left, msg: "FORM alignment must be an identifier"}
			}
			if bytes.EqualFold(tokens[0].text, []byte("LEFT")) || bytes.EqualFold(tokens[0].text, []byte("RIGHT")) {
				for _, token := range tokens[1:] {
					if token.kind != exprIdentifier {
						return &exprError{offset: token.off, msg: "FORM alignment has extra syntax"}
					}
				}
			}
		}
	}
	return nil
}

func (p *formParser) validateIslandAlignment(fields []syntaxSpan) error {
	session := p.sessionOrNew()
	if len(fields) != 3 || !p.formAlignmentRolesProven(fields[0], fields[1]) {
		return nil
	}
	field := fields[2]
	left := trimInputLeftWithSession(p.src, field.start, field.end, session)
	right := trimInputRight(p.src, left, field.end, session.allowFullwidthSpace)
	if left >= right {
		return nil
	}
	tokens, err := session.lexRange(p.source, p.base+left, p.base+right, left)
	if err != nil {
		return err
	}
	if len(tokens) == 1 && tokens[0].kind == exprIdentifier && setterKnownNameForDialect(tokens[0].text, session.dialect) {
		return &exprError{offset: left, msg: "FORM alignment cannot be a system variable"}
	}
	return nil
}

func (p *formParser) formAlignmentRolesProven(value, width syntaxSpan) bool {
	if p.session == nil || p.session.skipFormTypes {
		return false
	}
	for _, field := range []syntaxSpan{value, width} {
		left := trimInputLeftWithSession(p.src, field.start, field.end, p.session)
		right := trimInputRight(p.src, left, field.end, p.session.allowFullwidthSpace)
		if left >= right {
			return false
		}
		kind, macro := p.formFieldType(left, right)
		if macro || kind == knownValueUnknown {
			return false
		}
	}
	return true
}

func (p *formParser) conditional(open int) (int, error) {
	question, err := p.expressionEnd(open+2, '?')
	if err != nil {
		return 0, err
	}
	p.record(p.src[open : open+2])
	if p.fingerprint != nil {
		if err := p.recordExpressionFingerprint(open+2, question, 'Q'); err != nil {
			return 0, err
		}
	} else if p.rules != nil {
		conditionErr := p.formatCondition(open+2, question)
		if conditionErr != nil && !p.noteIslandError(conditionErr) {
			return 0, conditionErr
		}
		if conditionErr == nil && p.rules[FormConditionSpacing] && !p.formExpressionHasMandatoryMarker(open+2, question) && conditionalSpacingAllowedWithSession(p.sessionOrNew(), p.src, open+2, question) {
			p.formatConditionalSpacing(open+2, question)
		}
	}
	p.record(p.src[question : question+1])
	hash, err := p.segment(question+1, formStopSharp)
	if err != nil {
		return 0, err
	}
	if hash < len(p.src) && p.src[hash] == '\\' && hash+1 < len(p.src) && p.src[hash+1] == '@' {
		p.record(p.src[hash : hash+2])
		return hash + 2, nil
	}
	if hash >= len(p.src) || p.src[hash] != '#' {
		return 0, &exprError{offset: hash, msg: "missing FORM conditional #"}
	}
	p.record(p.src[hash : hash+1])
	end, err := p.segment(hash+1, formStopYenAt)
	if err != nil {
		return 0, err
	}
	if end+1 >= len(p.src) || p.src[end] != '\\' || p.src[end+1] != '@' {
		return 0, &exprError{offset: end, msg: "missing closing FORM conditional \\@"}
	}
	p.record(p.src[end : end+2])
	return end + 2, nil
}

func conditionalSpacingAllowedWithSession(session *expressionSession, src []byte, start, end int) bool {
	if start >= end || leadingTriviaEndWithSession(session, src[start:end]) >= trailingTriviaStartWithSession(session, src[start:end]) {
		return false
	}
	return !expressionExcludedWithSession(session, src[start:end])
}

func (p *formParser) formatConditionalSpacing(start, end int) {
	session := p.sessionOrNew()
	left := start + leadingTriviaEndWithSession(session, p.src[start:end])
	trailing := trailingTriviaStartWithSession(session, p.src[start:end])
	right := start + trailing
	if left > start {
		if !bytes.Equal(p.src[start:left], []byte{' '}) {
			p.edits = append(p.edits, edit{start, left, []byte{' '}, Change{Rule: FormConditionSpacing}})
		}
	} else {
		p.edits = append(p.edits, edit{start, start, []byte{' '}, Change{Rule: FormConditionSpacing}})
	}
	if right < end {
		if !bytes.Equal(p.src[right:end], []byte{' '}) {
			p.edits = append(p.edits, edit{right, end, []byte{' '}, Change{Rule: FormConditionSpacing}})
		}
	} else {
		p.edits = append(p.edits, edit{end, end, []byte{' '}, Change{Rule: FormConditionSpacing}})
	}
}

func (p *formParser) formatCondition(start, end int) error {
	session := p.sessionOrNew()
	if p.formExpressionHasMandatoryMarker(start, end) {
		return nil
	}
	if start == end || leadingTriviaEndWithSession(session, p.src[start:end]) >= trailingTriviaStartWithSession(session, p.src[start:end]) {
		return &formIslandError{err: &exprError{offset: start, msg: "empty FORM conditional expression"}}
	}
	if !session.skipFormTypes {
		if p.scalarIndexesMissing(start, end) {
			return &formIslandError{err: &exprError{offset: start, msg: "scalar variable requires indexes"}}
		}
		actual, _ := p.formFieldType(start, end)
		if actual == knownValueString {
			return &formIslandError{err: &exprError{offset: start, msg: "FORM conditional requires a numeric expression"}}
		}
	}
	if tokens, err := session.lexRange(p.source, p.base+start, p.base+end, start); err != nil {
		return &formIslandError{err: err}
	} else if err := formExpressionPrefix(tokens); err != nil {
		return &formIslandError{err: err}
	} else if !expressionNeedsDeferral(tokens, session.dialect) && len(p.splitTopLevel(start, end)) != 1 {
		return &formIslandError{err: &exprError{offset: start, msg: "FORM conditional requires one expression"}}
	} else if err := provenExpressionShape(tokens, session.dialect, false); err != nil {
		return &formIslandError{err: err}
	}
	edits, err := expressionEditsWithSession(session, p.source, p.src[start:end], p.base+start, false, p.rules, p.verify)
	if err != nil {
		return &formIslandError{err: shiftExpressionError(err, start)}
	}
	for _, e := range edits {
		e.start += start
		e.end += start
		p.edits = append(p.edits, e)
	}
	return nil
}

func formExpressionPrefix(tokens []exprToken) error {
	if len(tokens) == 0 {
		return nil
	}
	first := tokens[0]
	if first.kind == exprSymbol && string(first.text) == "," {
		return expressionShapeError(first, "empty FORM expression")
	}
	if first.kind == exprOperator {
		switch string(first.text) {
		case "+", "-", "!", "~", "++", "--":
		default:
			return expressionShapeError(first, "FORM expression starts with a binary operator")
		}
	}
	return nil
}

func (p *formParser) recordExpressionFingerprint(start, end int, tag byte) error {
	var length [binary.MaxVarintLen64]byte
	if p.formExpressionHasMandatoryMarker(start, end) {
		n := binary.PutUvarint(length[:], uint64(end-start))
		if !p.reserveFingerprint(3 + n + end - start) {
			return shiftExpressionError(p.session.exhausted, -p.base)
		}
		p.fingerprint.WriteByte(tag)
		p.fingerprint.WriteByte('R')
		p.fingerprint.Write(length[:n])
		p.fingerprint.Write(p.src[start:end])
		p.fingerprint.WriteByte(0)
		return nil
	}
	tokens, err := p.sessionOrNew().lexRange(p.source, p.base+start, p.base+end, start)
	if err != nil {
		return err
	}
	size := 2
	for _, token := range tokens {
		text := token.text
		if token.kind == exprForm {
			text = token.fingerprint
		}
		n := binary.PutUvarint(length[:], uint64(len(text)))
		if !appendWithinLimit(size, 1+n+len(text), maxFormFingerprintBytes) {
			return shiftExpressionError(p.session.resource(p.base+start, "FORM fingerprint work exceeds the formatter resource limit"), -p.base)
		}
		size += 1 + n + len(text)
	}
	if !p.reserveFingerprint(size) {
		return shiftExpressionError(p.session.exhausted, -p.base)
	}
	p.fingerprint.WriteByte(tag)
	for _, token := range tokens {
		p.fingerprint.WriteByte(byte(token.kind))
		text := token.text
		if token.kind == exprForm {
			text = token.fingerprint
		}
		n := binary.PutUvarint(length[:], uint64(len(text)))
		p.fingerprint.Write(length[:n])
		p.fingerprint.Write(text)
	}
	p.fingerprint.WriteByte(0)
	return nil
}

func mandatoryInlineMarker(src []byte, off int) bool {
	return off >= 0 && off+3 <= len(src) && src[off] == ';' && src[off+1] == '!' && src[off+2] == ';'
}

func (p *formParser) formExpressionHasMandatoryMarker(start, end int) bool {
	src := p.src[start:end]
	for i := 0; i < len(src); {
		switch src[i] {
		case '"':
			end := quotedEnd(src, i)
			if end < 0 {
				return false
			}
			i = end
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				end, err := p.scanNested(start+i, formQuoted)
				if err != nil {
					return false
				}
				i = end - start
				continue
			}
			i++
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				end, err := p.scanNested(start+i, formConditional)
				if err != nil {
					return false
				}
				i = end - start
				continue
			}
			i++
		default:
			if mandatoryInlineMarker(src, i) {
				return true
			}
			_, size := utf8.DecodeRune(src[i:])
			if size == 0 {
				size = 1
			}
			i += size
		}
	}
	return false
}

func (p *formParser) splitTopLevel(start, end int) []syntaxSpan {
	parent := p.sessionOrNew()
	session := newExpressionSessionWithSpace(p.src, parent.allowFullwidthSpace)
	session.dialect = parent.dialect
	session.onResource = func(offset int, message string) { parent.resource(p.base+offset, message) }
	return splitTopLevel(p.src, start, end, ',', session)
}

func (p *formParser) record(raw []byte) {
	if p.fingerprint == nil {
		return
	}
	var length [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(length[:], uint64(len(raw)))
	if !p.reserveFingerprint(1 + n + len(raw)) {
		return
	}
	p.fingerprint.WriteByte('L')
	p.fingerprint.Write(length[:n])
	p.fingerprint.Write(raw)
}

func (p *formParser) reserveFingerprint(addition int) bool {
	if p.session == nil || p.session.exhausted != nil {
		return p.session == nil
	}
	if err := p.session.checkFormFingerprint(p.base, p.fingerprint.Len(), addition); err != nil {
		if p.session.exhausted == nil {
			p.session.exhausted = err
		}
		return false
	}
	return true
}

func (p *formParser) noteIslandError(err error) bool {
	island, ok := err.(*formIslandError)
	if !ok {
		return false
	}
	if p.firstIslandErr == nil || formResourceError(island.err) && !formResourceError(p.firstIslandErr) {
		p.firstIslandErr = island.err
	}
	return true
}

func formResourceError(err error) bool {
	var expressionErr *exprError
	return errors.As(err, &expressionErr) && expressionErr.resource
}

type formIslandError struct {
	err error
}

func (e *formIslandError) Error() string { return e.err.Error() }

func (e *formIslandError) Unwrap() error { return e.err }

func escapedEnd(src []byte, start int) int {
	if start+1 >= len(src) {
		return len(src)
	}
	_, size := utf8.DecodeRune(src[start+1:])
	if size == 0 {
		size = 1
	}
	return start + 1 + size
}

func quotedEnd(src []byte, start int) int {
	for i := start + 1; i < len(src); {
		switch src[i] {
		case 0:
			return -1
		case '\\':
			if i+1 >= len(src) || src[i+1] == 0 {
				return -1
			}
			i = escapedEnd(src, i)
		case '"':
			return i + 1
		default:
			_, size := utf8.DecodeRune(src[i:])
			i += size
		}
	}
	return -1
}

func tripleFormSymbol(src []byte, start int) bool {
	if start+3 > len(src) || src[start] != src[start+1] || src[start] != src[start+2] {
		return false
	}
	switch src[start] {
	case '*', '+', '=', '/', '$':
		return true
	default:
		return false
	}
}
