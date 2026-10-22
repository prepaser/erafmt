package erabasic

import "bytes"

var fullwidthSpace = []byte{0xe3, 0x80, 0x80}

func InputSpaceSize(src []byte, pos int, allowFullwidth bool) int {
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

func InputSpaceAt(src []byte, pos int, allowFullwidth bool) bool {
	return InputSpaceSize(src, pos, allowFullwidth) != 0
}

func TrimInputLeft(src []byte, start, end int, allowFullwidth bool) int {
	for start < end {
		size := InputSpaceSize(src, start, allowFullwidth)
		if size == 0 {
			break
		}
		start += size
	}
	return start
}

func TrimInputRight(src []byte, start, end int, allowFullwidth bool) int {
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

func TrimInput(src []byte, allowFullwidth bool) []byte {
	start := TrimInputLeft(src, 0, len(src), allowFullwidth)
	end := TrimInputRight(src, start, len(src), allowFullwidth)
	return src[start:end]
}

func MakeLinesWithSpace(src []byte, allowFullwidth bool, count int) []Line {
	lines := make([]Line, 0, count)
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
		indentEnd := TrimInputLeft(src, start, bodyEnd, allowFullwidth)
		lines = append(lines, Line{start, bodyEnd, end, indentEnd})
		start = end
	}
	return lines
}

func headOfWithSpace(b []byte, allowFullwidth bool) (head []byte, prefix int, optional bool, marked bool) {
	return HeadOfWithSpaceDialect(b, allowFullwidth, CommonDialect)
}

func HeadOfWithSpaceDialect(b []byte, allowFullwidth bool, dialect Dialect) (head []byte, prefix int, optional bool, marked bool) {
	for {
		prefix = TrimInputLeft(b, prefix, len(b), allowFullwidth)
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

func CommandOfWithSpace(b []byte, allowFullwidth bool) (string, int) {
	i := 0
	for i < len(b) && ((b[i] >= 'a' && b[i] <= 'z') || (b[i] >= 'A' && b[i] <= 'Z') || b[i] == '_' || i > 0 && b[i] >= '0' && b[i] <= '9') {
		i++
	}
	if i == 0 || (i < len(b) && !InputSpaceAt(b, i, allowFullwidth) && b[i] != ';') {
		return "", 0
	}
	return string(bytes.ToUpper(b[:i])), i
}
