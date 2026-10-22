package erabasic

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf8"
)

type FormMode uint8

const (
	FormLine FormMode = iota
	FormQuoted
	FormCall
	FormComma
	FormConditional
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

type FormAnalyzer struct {
	Src            []byte
	Fingerprint    *bytes.Buffer
	FirstIslandErr error
	Visitor        formEventVisitor
	Validate       bool
	Session        *ExpressionSession
	Source         ExpressionSourceID
	Base           int
}

func (p *FormAnalyzer) SessionOrNew() *ExpressionSession {
	if p.Session == nil {
		p.Session = NewExpressionSession(p.Src)
		p.Source = ExpressionSourceOriginal
		p.Base = 0
	}
	return p.Session
}

func ScanForm(src []byte, mode FormMode) (int, error) {
	session := NewExpressionSession(src)
	end, err := session.ScanFormAt(ExpressionSourceOriginal, 0, len(src), mode)
	return end, err
}

func (p *FormAnalyzer) scanNested(start int, mode FormMode) (int, error) {
	if p.Session == nil {
		end, err := ScanForm(p.Src[start:], mode)
		return start + end, err
	}
	end, err := p.Session.ScanFormAt(p.Source, p.Base+start, p.Base+len(p.Src), mode)
	if err != nil {
		return 0, ShiftExpressionError(err, -p.Base)
	}
	return end - p.Base, nil
}

func (p *FormAnalyzer) Scan(mode FormMode) (end int, err error) {
	if p.Session != nil {
		if err := p.Session.enterForm(p.Base); err != nil {
			return 0, ShiftExpressionError(err, -p.Base)
		}
		defer func() {
			p.Session.leaveForm()
			if err == nil && p.Session.Exhausted != nil {
				end = 0
				err = ShiftExpressionError(p.Session.Exhausted, -p.Base)
			}
		}()
	}
	start := 0
	stop := formStopEnd
	switch mode {
	case FormQuoted:
		if len(p.Src) < 2 || p.Src[0] != '@' || p.Src[1] != '"' {
			return 0, &ExprError{Msg: "quoted FORM must start with @\""}
		}
		p.record(p.Src[:2])
		start = 2
		stop = formStopQuote
	case FormCall:
		stop = formStopCall
	case FormComma:
		stop = formStopComma
	case FormLine:
		stop = formStopEnd
	case FormConditional:
		if len(p.Src) < 2 || p.Src[0] != '\\' || p.Src[1] != '@' {
			return 0, &ExprError{Msg: "conditional FORM must start with \\@"}
		}
		return p.conditional(0)
	default:
		return 0, &ExprError{Msg: "unknown FORM mode"}
	}
	end, err = p.segment(start, stop)
	if err != nil {
		return 0, err
	}
	if stop == formStopQuote {
		if end >= len(p.Src) || p.Src[end] != '"' {
			return 0, &ExprError{Offset: end, Msg: "unterminated quoted FORM"}
		}
		p.record(p.Src[end : end+1])
		return end + 1, nil
	}
	return end, nil
}

func (p *FormAnalyzer) segment(start int, stop formStop) (int, error) {
	for i := start; i < len(p.Src); {
		if p.atStop(i, stop) {
			return i, nil
		}
		if p.Src[i] == '\\' {
			if i+1 >= len(p.Src) || p.Src[i+1] == 0 {
				return 0, &ExprError{Offset: i, Msg: "missing character after FORM escape"}
			}
			if p.Src[i+1] == '@' {
				end, err := p.conditional(i)
				if err != nil {
					return 0, err
				}
				i = end
				continue
			}
			end := escapedEnd(p.Src, i)
			p.record(p.Src[i:end])
			i = end
			continue
		}
		if tripleFormSymbol(p.Src, i) {
			p.record(p.Src[i : i+3])
			i += 3
			continue
		}
		switch p.Src[i] {
		case '%', '{':
			end, err := p.island(i)
			if err != nil {
				return 0, err
			}
			i = end
		default:
			_, size := utf8.DecodeRune(p.Src[i:])
			if size == 0 {
				size = 1
			}
			p.record(p.Src[i : i+size])
			i += size
		}
	}
	if stop == formStopQuote {
		return len(p.Src), nil
	}
	return len(p.Src), nil
}

func (p *FormAnalyzer) atStop(i int, stop formStop) bool {
	if p.Src[i] == 0 {
		return true
	}
	switch stop {
	case formStopQuote:
		return p.Src[i] == '"'
	case formStopCall:
		return p.Src[i] == '(' || p.Src[i] == '[' || p.Src[i] == ',' || p.Src[i] == ';' || p.Src[i] == '\r' || p.Src[i] == '\n'
	case formStopComma:
		return p.Src[i] == ',' || p.Src[i] == '\r' || p.Src[i] == '\n'
	case formStopSharp:
		return p.Src[i] == '#' || (i+1 < len(p.Src) && p.Src[i] == '\\' && p.Src[i+1] == '@')
	case formStopYenAt:
		return i+1 < len(p.Src) && p.Src[i] == '\\' && p.Src[i+1] == '@'
	default:
		return p.Src[i] == '\r' || p.Src[i] == '\n'
	}
}

func (p *FormAnalyzer) island(open int) (int, error) {
	closeByte, tag, valueType := byte('}'), byte('B'), KnownValueNumeric
	if p.Src[open] == '%' {
		closeByte, tag, valueType = '%', 'P', KnownValueString
	}
	close, err := p.expressionEnd(open+1, closeByte)
	if err != nil {
		return 0, err
	}
	p.record(p.Src[open : open+1])
	if p.Fingerprint != nil {
		if err := p.recordExpressionFingerprint(open+1, close, tag); err != nil {
			return 0, err
		}
	} else if p.Validate {
		event, err := p.analyzeIsland(open+1, close, valueType)
		if err != nil {
			if !p.noteIslandError(err) {
				return 0, err
			}
		} else if p.Visitor != nil {
			if err := p.Visitor(event); err != nil && !p.noteIslandError(err) {
				return 0, err
			}
		}
	}
	p.record(p.Src[close : close+1])
	return close + 1, nil
}

func (p *FormAnalyzer) expressionEnd(start int, close byte) (int, error) {
	var delimiters []byte
	for i := start; i < len(p.Src); {
		r := p.Src[i]
		if r == 0 {
			return 0, &ExprError{Offset: i, Msg: fmt.Sprintf("missing FORM delimiter %q", close)}
		}
		if r == '"' {
			i = QuotedEnd(p.Src, i)
			if i < 0 {
				return 0, &ExprError{Offset: start, Msg: "unterminated string in FORM expression"}
			}
			continue
		}
		if r == '@' && i+1 < len(p.Src) && p.Src[i+1] == '"' {
			end, err := p.scanNested(i, FormQuoted)
			if err != nil {
				return 0, err
			}
			i = end
			continue
		}
		if r == '\\' && i+1 < len(p.Src) && p.Src[i+1] == '@' {
			end, err := p.scanNested(i, FormConditional)
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
			if MandatoryInlineMarker(p.Src, i) {
				i += 3
				continue
			}
			return 0, &ExprError{Offset: i, Msg: "FORM expression reaches a comment before its delimiter"}
		}
		switch r {
		case '(':
			if len(delimiters) >= MaxExpressionDepth {
				return 0, ShiftExpressionError(p.SessionOrNew().resource(p.Base+i, "FORM expression nesting exceeds the formatter resource limit"), -p.Base)
			}
			delimiters = append(delimiters, ')')
		case '[':
			if len(delimiters) >= MaxExpressionDepth {
				return 0, ShiftExpressionError(p.SessionOrNew().resource(p.Base+i, "FORM expression nesting exceeds the formatter resource limit"), -p.Base)
			}
			delimiters = append(delimiters, ']')
		case ')', ']':
			if len(delimiters) == 0 || delimiters[len(delimiters)-1] != r {
				return 0, &ExprError{Offset: i, Msg: "misnested FORM expression delimiter"}
			}
			delimiters = delimiters[:len(delimiters)-1]
		case '{':
			return 0, &ExprError{Offset: i, Msg: "nested FORM brace in expression"}
		}
		if isASCIIDigit(rune(r)) {
			end, _ := scanNumber(p.Src, i)
			i = end
			continue
		}
		run, size := utf8.DecodeRune(p.Src[i:])
		if !IsDelimiter(run) && run != '\u3000' {
			i = expressionIdentifierEnd(p.Src, i)
			continue
		}
		i += size
	}
	return 0, &ExprError{Offset: start, Msg: fmt.Sprintf("missing FORM delimiter %q", close)}
}

func (p *FormAnalyzer) noteIslandWidth(valueType KnownValueType, fields []SyntaxSpan) bool {
	if valueType != KnownValueNumeric || p.Session == nil || p.Session.SkipFormTypes {
		return false
	}
	if len(fields) < 2 {
		return false
	}
	valueSpan := fields[0]
	valueStart := TrimInputLeftWithSession(p.Src, valueSpan.Start, valueSpan.End, p.Session)
	valueEnd := TrimInputRight(p.Src, valueStart, valueSpan.End, p.Session.AllowFullwidthSpace)
	if valueStart >= valueEnd {
		return false
	}
	valueTokens, err := p.Session.LexRange(p.Source, p.Base+valueStart, p.Base+valueEnd, valueStart)
	if err != nil || CommandMacroPossibleForDialect(valueTokens, p.Session.Dialect) || KnownExpressionTypeForDialect(valueTokens, p.Session.Dialect) != KnownValueNumeric {
		return false
	}
	width := fields[1]
	widthStart := TrimInputLeftWithSession(p.Src, width.Start, width.End, p.Session)
	widthEnd := TrimInputRight(p.Src, widthStart, width.End, p.Session.AllowFullwidthSpace)
	if widthStart >= widthEnd {
		return false
	}
	widthTokens, err := p.Session.LexRange(p.Source, p.Base+widthStart, p.Base+widthEnd, widthStart)
	if err != nil || CommandMacroPossibleForDialect(widthTokens, p.Session.Dialect) {
		return false
	}
	widthValue, known := constantIntegerValue(widthTokens)
	if !known || int32(widthValue) >= 0 {
		return false
	}
	p.Session.AddFormAdvisory(FormAdvisory{Source: p.Source, Offset: p.Base + widthStart, Code: "suspicious_form_width", Message: "FORM width is negative after signed 32-bit conversion"})
	return true
}

func (p *FormAnalyzer) formFieldType(start, end int) (KnownValueType, bool) {
	session := p.SessionOrNew()
	tokens, err := session.LexRange(p.Source, p.Base+start, p.Base+end, start)
	if err != nil || CommandMacroPossibleForDialect(tokens, session.Dialect) {
		return KnownValueUnknown, true
	}
	return KnownExpressionTypeForDialect(tokens, session.Dialect), false
}

func (p *FormAnalyzer) scalarIndexesMissing(start, end int) bool {
	session := p.SessionOrNew()
	tokens, err := session.LexRange(p.Source, p.Base+start, p.Base+end, start)
	return err == nil && BuiltinScalarIndexesMissing(tokens, session.Dialect)
}

func (p *FormAnalyzer) validateIslandTypes(valueType KnownValueType, fields []SyntaxSpan) error {
	session := p.SessionOrNew()
	if session.SkipFormTypes {
		return nil
	}
	for index, field := range fields[:min(2, len(fields))] {
		left := TrimInputLeftWithSession(p.Src, field.Start, field.End, session)
		right := TrimInputRight(p.Src, left, field.End, session.AllowFullwidthSpace)
		if left == right {
			return nil
		}
		if p.scalarIndexesMissing(left, right) {
			return &ExprError{Offset: left, Msg: "scalar variable requires indexes"}
		}
		actual, macro := p.formFieldType(left, right)
		if macro {
			return nil
		}
		expected, message := valueType, "FORM placeholder expression must be numeric"
		if valueType == KnownValueString {
			message = "FORM percent expression must be a string"
		}
		if index > 0 {
			expected, message = KnownValueNumeric, "FORM width must be numeric"
		}
		if !CommandTypeMatches(expected, actual) {
			return &ExprError{Offset: left, Msg: message}
		}
	}
	return nil
}

func (p *FormAnalyzer) validateIslandShape(fields []SyntaxSpan) error {
	first := fields[0]
	firstLeft := TrimInputLeftWithSession(p.Src, first.Start, first.End, p.Session)
	firstRight := TrimInputRight(p.Src, firstLeft, first.End, p.Session.AllowFullwidthSpace)
	if firstLeft == firstRight {
		return &ExprError{Offset: first.Start, Msg: "empty FORM expression"}
	}
	if len(fields) > 3 {
		return &ExprError{Offset: fields[3].Start, Msg: "too many FORM placeholder fields"}
	}
	if len(fields) == 3 {
		field := fields[1]
		left := TrimInputLeftWithSession(p.Src, field.Start, field.End, p.Session)
		right := TrimInputRight(p.Src, left, field.End, p.Session.AllowFullwidthSpace)
		if left == right {
			return &ExprError{Offset: field.Start, Msg: "empty FORM width before alignment"}
		}
		alignment := fields[2]
		left = TrimInputLeftWithSession(p.Src, alignment.Start, alignment.End, p.Session)
		right = TrimInputRight(p.Src, left, alignment.End, p.Session.AllowFullwidthSpace)
		if left < right {
			tokens, err := p.Session.LexRange(p.Source, p.Base+left, p.Base+right, left)
			if err != nil {
				return err
			}
			if len(tokens) == 0 {
				return &ExprError{Offset: left, Msg: "FORM alignment must be an identifier"}
			}
			if tokens[0].Kind != ExprIdentifier {
				return &ExprError{Offset: left, Msg: "FORM alignment must be an identifier"}
			}
			if bytes.EqualFold(tokens[0].Text, []byte("LEFT")) || bytes.EqualFold(tokens[0].Text, []byte("RIGHT")) {
				for _, token := range tokens[1:] {
					if token.Kind != ExprIdentifier {
						return &ExprError{Offset: token.Off, Msg: "FORM alignment has extra syntax"}
					}
				}
			}
		}
	}
	return nil
}

func (p *FormAnalyzer) validateIslandAlignment(fields []SyntaxSpan) error {
	session := p.SessionOrNew()
	if len(fields) != 3 || !p.formAlignmentRolesProven(fields[0], fields[1]) {
		return nil
	}
	field := fields[2]
	left := TrimInputLeftWithSession(p.Src, field.Start, field.End, session)
	right := TrimInputRight(p.Src, left, field.End, session.AllowFullwidthSpace)
	if left >= right {
		return nil
	}
	tokens, err := session.LexRange(p.Source, p.Base+left, p.Base+right, left)
	if err != nil {
		return err
	}
	if len(tokens) == 1 && tokens[0].Kind == ExprIdentifier && setterKnownNameForDialect(tokens[0].Text, session.Dialect) {
		return &ExprError{Offset: left, Msg: "FORM alignment cannot be a system variable"}
	}
	return nil
}

func (p *FormAnalyzer) formAlignmentRolesProven(value, width SyntaxSpan) bool {
	if p.Session == nil || p.Session.SkipFormTypes {
		return false
	}
	for _, field := range []SyntaxSpan{value, width} {
		left := TrimInputLeftWithSession(p.Src, field.Start, field.End, p.Session)
		right := TrimInputRight(p.Src, left, field.End, p.Session.AllowFullwidthSpace)
		if left >= right {
			return false
		}
		kind, macro := p.formFieldType(left, right)
		if macro || kind == KnownValueUnknown {
			return false
		}
	}
	return true
}

func (p *FormAnalyzer) conditional(open int) (int, error) {
	question, err := p.expressionEnd(open+2, '?')
	if err != nil {
		return 0, err
	}
	p.record(p.Src[open : open+2])
	if p.Fingerprint != nil {
		if err := p.recordExpressionFingerprint(open+2, question, 'Q'); err != nil {
			return 0, err
		}
	} else if p.Validate {
		event, conditionErr := p.analyzeCondition(open+2, question)
		if conditionErr != nil {
			if !p.noteIslandError(conditionErr) {
				return 0, conditionErr
			}
		} else if p.Visitor != nil {
			if err := p.Visitor(event); err != nil && !p.noteIslandError(err) {
				return 0, err
			}
		}
	}
	p.record(p.Src[question : question+1])
	hash, err := p.segment(question+1, formStopSharp)
	if err != nil {
		return 0, err
	}
	if hash < len(p.Src) && p.Src[hash] == '\\' && hash+1 < len(p.Src) && p.Src[hash+1] == '@' {
		p.record(p.Src[hash : hash+2])
		return hash + 2, nil
	}
	if hash >= len(p.Src) || p.Src[hash] != '#' {
		return 0, &ExprError{Offset: hash, Msg: "missing FORM conditional #"}
	}
	p.record(p.Src[hash : hash+1])
	end, err := p.segment(hash+1, formStopYenAt)
	if err != nil {
		return 0, err
	}
	if end+1 >= len(p.Src) || p.Src[end] != '\\' || p.Src[end+1] != '@' {
		return 0, &ExprError{Offset: end, Msg: "missing closing FORM conditional \\@"}
	}
	p.record(p.Src[end : end+2])
	return end + 2, nil
}

func formExpressionPrefix(tokens []ExprToken) error {
	if len(tokens) == 0 {
		return nil
	}
	first := tokens[0]
	if first.Kind == exprSymbol && string(first.Text) == "," {
		return expressionShapeError(first, "empty FORM expression")
	}
	if first.Kind == ExprOperator {
		switch string(first.Text) {
		case "+", "-", "!", "~", "++", "--":
		default:
			return expressionShapeError(first, "FORM expression starts with a binary operator")
		}
	}
	return nil
}

func (p *FormAnalyzer) recordExpressionFingerprint(start, end int, tag byte) error {
	var length [binary.MaxVarintLen64]byte
	if p.FormExpressionHasMandatoryMarker(start, end) {
		n := binary.PutUvarint(length[:], uint64(end-start))
		if !p.reserveFingerprint(3 + n + end - start) {
			return ShiftExpressionError(p.Session.Exhausted, -p.Base)
		}
		p.Fingerprint.WriteByte(tag)
		p.Fingerprint.WriteByte('R')
		p.Fingerprint.Write(length[:n])
		p.Fingerprint.Write(p.Src[start:end])
		p.Fingerprint.WriteByte(0)
		return nil
	}
	tokens, err := p.SessionOrNew().LexRange(p.Source, p.Base+start, p.Base+end, start)
	if err != nil {
		return err
	}
	size := 2
	for _, token := range tokens {
		text := token.Text
		if token.Kind == ExprForm {
			text = token.Fingerprint
		}
		n := binary.PutUvarint(length[:], uint64(len(text)))
		if !appendWithinLimit(size, 1+n+len(text), maxFormFingerprintBytes) {
			return ShiftExpressionError(p.Session.resource(p.Base+start, "FORM fingerprint work exceeds the formatter resource limit"), -p.Base)
		}
		size += 1 + n + len(text)
	}
	if !p.reserveFingerprint(size) {
		return ShiftExpressionError(p.Session.Exhausted, -p.Base)
	}
	p.Fingerprint.WriteByte(tag)
	for _, token := range tokens {
		p.Fingerprint.WriteByte(byte(token.Kind))
		text := token.Text
		if token.Kind == ExprForm {
			text = token.Fingerprint
		}
		n := binary.PutUvarint(length[:], uint64(len(text)))
		p.Fingerprint.Write(length[:n])
		p.Fingerprint.Write(text)
	}
	p.Fingerprint.WriteByte(0)
	return nil
}

func MandatoryInlineMarker(src []byte, off int) bool {
	return off >= 0 && off+3 <= len(src) && src[off] == ';' && src[off+1] == '!' && src[off+2] == ';'
}

func (p *FormAnalyzer) FormExpressionHasMandatoryMarker(start, end int) bool {
	src := p.Src[start:end]
	for i := 0; i < len(src); {
		switch src[i] {
		case '"':
			end := QuotedEnd(src, i)
			if end < 0 {
				return false
			}
			i = end
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				end, err := p.scanNested(start+i, FormQuoted)
				if err != nil {
					return false
				}
				i = end - start
				continue
			}
			i++
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				end, err := p.scanNested(start+i, FormConditional)
				if err != nil {
					return false
				}
				i = end - start
				continue
			}
			i++
		default:
			if MandatoryInlineMarker(src, i) {
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

func (p *FormAnalyzer) splitTopLevel(start, end int) []SyntaxSpan {
	parent := p.SessionOrNew()
	session := NewExpressionSessionWithSpace(p.Src, parent.AllowFullwidthSpace)
	session.Dialect = parent.Dialect
	session.OnResource = func(offset int, message string) {
		parent.resource(p.Base+offset, message)
	}
	return SplitTopLevel(p.Src, start, end, ',', session)
}

func (p *FormAnalyzer) record(raw []byte) {
	if p.Fingerprint == nil {
		return
	}
	var length [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(length[:], uint64(len(raw)))
	if !p.reserveFingerprint(1 + n + len(raw)) {
		return
	}
	p.Fingerprint.WriteByte('L')
	p.Fingerprint.Write(length[:n])
	p.Fingerprint.Write(raw)
}

func (p *FormAnalyzer) reserveFingerprint(addition int) bool {
	if p.Session == nil || p.Session.Exhausted != nil {
		return p.Session == nil
	}
	if err := p.Session.checkFormFingerprint(p.Base, p.Fingerprint.Len(), addition); err != nil {
		if p.Session.Exhausted == nil {
			p.Session.Exhausted = err
		}
		return false
	}
	return true
}

func (p *FormAnalyzer) noteIslandError(err error) bool {
	island, ok := err.(*FormIslandError)
	if !ok {
		return false
	}
	if p.FirstIslandErr == nil || formResourceError(island.Err) && !formResourceError(p.FirstIslandErr) {
		p.FirstIslandErr = island.Err
	}
	return true
}

func formResourceError(err error) bool {
	var expressionErr *ExprError
	return errors.As(err, &expressionErr) && expressionErr.Resource
}

type FormIslandError struct{ Err error }

func (e *FormIslandError) Error() string {
	return e.Err.Error()
}

func (e *FormIslandError) Unwrap() error {
	return e.Err
}

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

func QuotedEnd(src []byte, start int) int {
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
