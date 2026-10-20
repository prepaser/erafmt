package format

import "bytes"

var fullwidthSpace = []byte{0xe3, 0x80, 0x80}

func inputSpaceSize(src []byte, pos int, allowFullwidth bool) int {
	if pos < 0 || pos >= len(src) {
		return 0
	}
	if src[pos] == ' ' || src[pos] == '\t' {
		return 1
	}
	if allowFullwidth && pos+len(fullwidthSpace) <= len(src) && bytes.Equal(src[pos:pos+len(fullwidthSpace)], fullwidthSpace) {
		return len(fullwidthSpace)
	}
	return 0
}

func inputSpaceAt(src []byte, pos int, allowFullwidth bool) bool {
	return inputSpaceSize(src, pos, allowFullwidth) != 0
}

func trimInputLeft(src []byte, start, end int, allowFullwidth bool) int {
	for start < end {
		size := inputSpaceSize(src, start, allowFullwidth)
		if size == 0 {
			break
		}
		start += size
	}
	return start
}

func trimInputRight(src []byte, start, end int, allowFullwidth bool) int {
	for end > start {
		if src[end-1] == ' ' || src[end-1] == '\t' {
			end--
			continue
		}
		if allowFullwidth && end-start >= len(fullwidthSpace) && bytes.Equal(src[end-len(fullwidthSpace):end], fullwidthSpace) {
			end -= len(fullwidthSpace)
			continue
		}
		break
	}
	return end
}

func trimInput(src []byte, allowFullwidth bool) []byte {
	start := trimInputLeft(src, 0, len(src), allowFullwidth)
	end := trimInputRight(src, start, len(src), allowFullwidth)
	return src[start:end]
}

func makeLinesWithSpace(src []byte, allowFullwidth bool, count int) []line {
	lines := make([]line, 0, count)
	for start := 0; start < len(src); {
		bodyEnd := start
		for bodyEnd < len(src) && src[bodyEnd] != '\r' && src[bodyEnd] != '\n' {
			bodyEnd++
		}
		end := bodyEnd
		if end < len(src) {
			end++
			if src[bodyEnd] == '\r' && end < len(src) && src[end] == '\n' {
				end++
			}
		}
		indentEnd := trimInputLeft(src, start, bodyEnd, allowFullwidth)
		lines = append(lines, line{start, bodyEnd, end, indentEnd})
		start = end
	}
	return lines
}

func headOfWithSpace(b []byte, allowFullwidth bool) (head []byte, prefix int, optional bool, marked bool) {
	return headOfWithSpaceDialect(b, allowFullwidth, CommonDialect)
}

func headOfWithSpaceDialect(b []byte, allowFullwidth bool, dialect Dialect) (head []byte, prefix int, optional bool, marked bool) {
	for {
		prefix = trimInputLeft(b, prefix, len(b), allowFullwidth)
		if prefix+3 > len(b) {
			break
		}
		marker := string(b[prefix : prefix+3])
		markerOptional, ok := lineMarker(marker, dialect)
		if !ok {
			break
		}
		marked = true
		optional = optional || markerOptional
		prefix += 3
	}
	return b[prefix:], prefix, optional, marked
}

func lineMarker(marker string, dialect Dialect) (optional, ok bool) {
	switch marker {
	case ";!;":
		return false, true
	case ";#;":
		return true, true
	case ";^;":
		return false, dialect == EMEE
	default:
		return false, false
	}
}

func commandOfWithSpace(b []byte, allowFullwidth bool) (string, int) {
	i := 0
	for i < len(b) && ((b[i] >= 'a' && b[i] <= 'z') || (b[i] >= 'A' && b[i] <= 'Z') || b[i] == '_' || i > 0 && b[i] >= '0' && b[i] <= '9') {
		i++
	}
	if i == 0 || (i < len(b) && !inputSpaceAt(b, i, allowFullwidth) && b[i] != ';') {
		return "", 0
	}
	return string(bytes.ToUpper(b[:i])), i
}

func (p *formatter) inputSpaceIn(src []byte, pos int) bool {
	return inputSpaceAt(src, pos, p.allowFullwidthSpace)
}

func (p *formatter) trimInputIn(src []byte) []byte {
	return trimInput(src, p.allowFullwidthSpace)
}

func (p *formatter) trimInputLeft(start, end int) int {
	return trimInputLeft(p.src, start, end, p.allowFullwidthSpace)
}

func (p *formatter) trimInputRight(start, end int) int {
	return trimInputRight(p.src, start, end, p.allowFullwidthSpace)
}

func (p *formatter) headOf(body []byte) (head []byte, prefix int, optional bool, marked bool) {
	return headOfWithSpaceDialect(body, p.allowFullwidthSpace, p.dialect)
}

func (p *formatter) commandOf(head []byte) (string, int) {
	return commandOfWithSpace(head, p.allowFullwidthSpace)
}

func (p *formatter) newExpressionSession(src []byte) *expressionSession {
	session := newExpressionSessionWithSpace(src, p.allowFullwidthSpace)
	session.builtinTypes = true
	session.dialect = p.dialect
	session.onResource = p.syntaxResource
	return session
}
