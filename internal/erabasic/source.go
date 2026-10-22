package erabasic

import "unicode/utf8"

type Line struct{ Start, BodyEnd, End, IndentEnd int }
type SourcePosition struct{ Line, Offset int }

func LBody(src []byte, l Line) []byte {
	return src[l.Start:l.BodyEnd]
}

func ILineStart(lines []Line, i int) int {
	if i < 0 || i >= len(lines) {
		return 0
	}
	return lines[i].Start
}

func HalfSpace(b byte) bool {
	return b == ' ' || b == '\t'
}

func Column(src []byte, l Line, offset int) int {
	offset = min(l.BodyEnd, max(l.Start, offset))
	return utf8.RuneCount(src[l.Start:offset]) + 1
}

type columnPoint struct{ Offset, Column int }
