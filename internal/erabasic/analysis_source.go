package erabasic

import (
	"sort"
	"unicode/utf8"
)

func (s *DocumentSession) SourceColumn(row, offset int, reserve func(int, int) bool) int {
	l := s.Lines[row]
	offset = min(l.BodyEnd, max(l.Start, offset))
	if offset-l.Start < 4096 {
		return Column(s.Src, l, offset)
	}
	points, ok := s.Columns[row]
	if !ok {
		if s.Columns == nil {
			s.Columns = make(map[int][]columnPoint)
		}
		count := (l.End-l.Start)/1024 + 1
		if !reserve(offset, count*32) {
			return Column(s.Src, l, offset)
		}
		points = make([]columnPoint, 1, count)
		points[0] = columnPoint{l.Start, 1}
		col, last := 1, l.Start
		for pos := l.Start; pos < l.End; pos++ {
			if !utf8.RuneStart(s.Src[pos]) {
				continue
			}
			if pos-last >= 1024 {
				points = append(points, columnPoint{pos, col})
				last = pos
			}
			col++
		}
		s.Columns[row] = points
	}
	i := sort.Search(len(points), func(i int) bool {
		return points[i].Offset > offset
	}) - 1
	return points[i].Column + utf8.RuneCount(s.Src[points[i].Offset:offset])
}

func (s *DocumentSession) StatementSourceOrigin(line, offset int) (int, int) {
	if origin, ok := s.StatementOrigins[line]; ok {
		return origin.Line, origin.Offset
	}
	return line, offset
}

type physicalLocation struct{ Line, Column int }

func (s *DocumentSession) ResourceLocation(line, offset int) physicalLocation {
	if len(s.Lines) == 0 {
		return physicalLocation{1, 1}
	}
	if line < 0 || line >= len(s.Lines) {
		line = 0
	}
	if offset < s.Lines[line].Start || offset > s.Lines[line].End {
		offset = s.Lines[line].Start
	}
	return physicalLocation{line + 1, utf8.RuneCount(s.Src[s.Lines[line].Start:offset]) + 1}
}
