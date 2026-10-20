package cli

import (
	"bytes"
	"strings"

	"github.com/prepaser/erafmt/internal/format"
)

type continuationArgumentLiterals struct {
	source    []byte
	dialect   format.Dialect
	fullwidth bool
	spans     []continuationLiteralSpan
}

type continuationLiteralSpan struct{ start, end int }

func continuationArgumentBoundaryFingerprint(rows []checkedLine, separator, kind string, dialect format.Dialect, fullwidth bool) ([]byte, bool) {
	source := continuationLogicalSource(rows, separator)
	head := trimGrammarSpace(source, fullwidth)
	word, end := firstWord(head)
	if end == 0 {
		return nil, false
	}
	command := strings.ToUpper(string(word))
	start := len(source) - len(head) + end
	literals := continuationArgumentLiterals{source: source, dialect: dialect, fullwidth: fullwidth}
	switch {
	case kind == "expression" && isPlainCallCommand(command):
		literals.call(start)
	case kind == "input" && isInputExpressionCommand(command, dialect):
		limit := inputExpressionFieldLimit(command, dialect)
		commas := topLevelCommas(source[start:])
		end := len(source)
		if len(commas)+1 > limit {
			end = start + commas[limit-1]
		}
		literals.expression(start, end, 0)
		literals.raw(end, len(source))
	case kind == "input" && isInputCommand(command, dialect):
		literals.inputForm(start + continuationArgumentSpace(source[start:], fullwidth))
	default:
		return nil, false
	}
	// Logical proof checks tokens; row fields additionally retain literal byte positions.
	out := []byte{'A'}
	offset, index := 0, 0
	for _, row := range rows {
		end := offset + len(row.body)
		var field []byte
		for pos := offset; pos < end; {
			for index < len(literals.spans) && literals.spans[index].end <= pos {
				index++
			}
			if index < len(literals.spans) && literals.spans[index].start <= pos {
				next := min(end, literals.spans[index].end)
				field = append(field, source[pos:next]...)
				pos = next
			} else if size := continuationArgumentSpace(source[pos:end], fullwidth); size != 0 {
				pos += size
			} else {
				field = append(field, source[pos])
				pos++
			}
		}
		out = appendPreservedField(out, field)
		offset = end + len(separator)
	}
	return out, true
}

func continuationArgumentSpace(src []byte, fullwidth bool) int {
	if len(src) == 0 {
		return 0
	}
	if src[0] == ' ' || src[0] == '\t' {
		return 1
	}
	if fullwidth && bytes.HasPrefix(src, []byte("\u3000")) {
		return 3
	}
	return 0
}

func (p *continuationArgumentLiterals) raw(start, end int) {
	if start < end {
		p.spans = append(p.spans, continuationLiteralSpan{start, end})
	}
}

func (p *continuationArgumentLiterals) call(start int) {
	src := p.source
	target := start
	for target < len(src) && continuationArgumentSpace(src[target:], p.fullwidth) != 0 {
		target += continuationArgumentSpace(src[target:], p.fullwidth)
	}
	delimiter := plainCallDelimiter(src, target)
	end := len(src)
	if delimiter >= 0 {
		end = delimiter
	}
	for end > target && (src[end-1] == ' ' || src[end-1] == '\t') && !escapedCallTargetByte(src, end-1) {
		end--
	}
	p.raw(target, end)
	if delimiter < 0 {
		return
	}
	pos := delimiter
	for pos < len(src) && (src[pos] == '(' || src[pos] == '[') {
		close := byte(')')
		if src[pos] == '[' {
			close = ']'
		}
		end := matchingCallDelimiter(src, pos, src[pos], close)
		if end < 0 {
			p.raw(pos, len(src))
			return
		}
		if corpusFormCallArgumentsNeedExact(src[pos+1:end], p.dialect, p.fullwidth) {
			p.raw(pos+1, end)
		} else {
			p.expression(pos+1, end, 0)
		}
		pos = end + 1
		for pos < len(src) && continuationArgumentSpace(src[pos:], p.fullwidth) != 0 {
			pos += continuationArgumentSpace(src[pos:], p.fullwidth)
		}
	}
	if pos < len(src) && src[pos] == ',' {
		if corpusFormCallArgumentsNeedExact(src[pos+1:], p.dialect, p.fullwidth) {
			p.raw(pos+1, len(src))
		} else {
			p.expression(pos+1, len(src), 0)
		}
	} else if pos < len(src) && src[pos] == ';' {
		p.comment(pos, len(src))
	} else {
		p.raw(pos, len(src))
	}
}

func (p *continuationArgumentLiterals) comment(start, end int) {
	src := p.source
	if corpusProtectedComment(src[start:end]) && !ordinaryInlineDoubleSemicolon(src[start:end]) {
		p.raw(start, end)
		return
	}
	left, right := start+1, end
	for left < right && (src[left] == ' ' || src[left] == '\t') {
		left++
	}
	for right > left && (src[right-1] == ' ' || src[right-1] == '\t') {
		right--
	}
	p.raw(left, right)
}

func (p *continuationArgumentLiterals) inputForm(start int) {
	first := inputFormComma(p.source[start:])
	if p.dialect == format.CommonDialect {
		if first >= 0 {
			p.raw(start, len(p.source))
		} else {
			p.form(start, len(p.source), 0)
		}
		return
	}
	if p.dialect != format.EMEE {
		p.form(start, len(p.source), 0)
		return
	}
	if first < 0 {
		p.form(start, len(p.source), 0)
		return
	}
	first += start
	p.form(start, first, 0)
	suffixStart := first + 1
	comment := incrementCommentStart(p.source[suffixStart:])
	if comment < 0 || comment > len(p.source)-suffixStart {
		comment = len(p.source) - suffixStart
	}
	end := suffixStart + comment
	code := p.source[suffixStart:end]
	commas := topLevelCommas(code)
	if inputFormSuffixOverflow(code, commas, p.fullwidth) {
		p.raw(start, len(p.source))
		return
	}
	field := suffixStart
	for _, comma := range append(commas, end-suffixStart) {
		limit := suffixStart + comma
		p.expression(field, limit, 0)
		field = limit + 1
	}
	if end < len(p.source) {
		p.comment(end, len(p.source))
	}
}

func (p *continuationArgumentLiterals) expression(start, end, depth int) {
	if depth >= 128 {
		p.raw(start, end)
		return
	}
	src := p.source[:end]
	for i := start; i < end; {
		switch {
		case src[i] == '@' && i+1 < end && src[i+1] == '"':
			next, complete := quotedFormEndComplete(src, i)
			if !complete {
				p.raw(i, end)
				return
			}
			p.form(i+2, next-1, depth+1)
			i = next
		case src[i] == '\\' && i+1 < end && src[i+1] == '@':
			next, ok := p.conditional(i, end, depth+1)
			if !ok {
				p.raw(i, end)
				return
			}
			i = next
		case src[i] == '"' || src[i] == '\'':
			next := printVQuotedEnd(src, i)
			if next <= i {
				p.raw(i, end)
				return
			}
			p.raw(i, next)
			i = next
		case src[i] == ';':
			p.comment(i, end)
			return
		default:
			i++
		}
	}
}

func (p *continuationArgumentLiterals) conditional(start, end, depth int) (int, bool) {
	question, hash, close, ok := formConditional(p.source[:end], start)
	if !ok {
		return end, false
	}
	if formConditionalNeedsExactDialect(p.source[start:close+2], p.dialect) {
		p.raw(start+2, question)
	} else {
		p.expression(start+2, question, depth)
	}
	if hash >= 0 {
		p.form(question+1, hash, depth)
		p.form(hash+1, close, depth)
	} else {
		p.form(question+1, close, depth)
	}
	return close + 2, true
}

func (p *continuationArgumentLiterals) form(start, end, depth int) {
	if depth >= 128 {
		p.raw(start, end)
		return
	}
	src := p.source[:end]
	literal := start
	for i := start; i < end; {
		if src[i] == '\\' && i+1 < end {
			if src[i+1] == '@' {
				p.raw(literal, i)
				next, ok := p.conditional(i, end, depth+1)
				if !ok {
					p.raw(i, end)
					return
				}
				i, literal = next, next
			} else {
				i += 2
			}
			continue
		}
		if src[i] == '{' || src[i] == '%' {
			close := formIslandEnd(src, i, src[i])
			if close >= 0 {
				p.raw(literal, i)
				if formIslandNeedsExactDialect(src[i+1:close], src[i], p.fullwidth, p.dialect) {
					p.raw(i+1, close)
				} else {
					p.expression(i+1, close, depth+1)
				}
				i, literal = close+1, close+1
				continue
			}
		}
		i++
	}
	p.raw(literal, end)
}
