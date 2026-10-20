package format

import (
	"bytes"
	"cmp"
	"slices"
	"sort"
	"unicode/utf8"
)

type line struct {
	start, bodyEnd, end, indentEnd int
}

type sourcePosition struct {
	line, offset int
}

type edit struct {
	start, end int
	text       []byte
	change     Change
}

func halfSpace(b byte) bool { return b == ' ' || b == '\t' }

func allSpaces(b []byte) bool {
	for _, c := range b {
		if c != ' ' {
			return false
		}
	}
	return true
}

func applyEdits(src []byte, edits []edit) []byte {
	if len(edits) == 0 {
		return src
	}
	sortEdits(edits)
	var out bytes.Buffer
	pos := 0
	for _, e := range edits {
		out.Write(src[pos:e.start])
		out.Write(e.text)
		pos = e.end
	}
	out.Write(src[pos:])
	return out.Bytes()
}

func projectedOutputSizeSorted(src []byte, edits []edit) (int, bool) {
	var size uint64
	pos := 0
	for _, e := range edits {
		if e.start < pos || e.start < 0 || e.end < e.start || e.end > len(src) {
			return 0, false
		}
		size += uint64(e.start - pos)
		size += uint64(len(e.text))
		pos = e.end
	}
	size += uint64(len(src) - pos)
	if size > uint64(^uint(0)>>1) {
		return 0, false
	}
	return int(size), true
}

func sortEdits(edits []edit) {
	slices.SortStableFunc(edits, func(a, b edit) int {
		if n := cmp.Compare(a.start, b.start); n != 0 {
			return n
		}
		if n := cmp.Compare(a.end, b.end); n != 0 {
			return n
		}
		return cmp.Compare(insertionPriority(a.change.Rule), insertionPriority(b.change.Rule))
	})
}

func insertionPriority(rule RuleID) int {
	switch rule {
	case FunctionSeparation:
		return 0
	case DeclarationBodySeparation:
		return 1
	default:
		return 2
	}
}

func validEdits(edits []edit, size int) bool {
	sortEdits(edits)
	return validSortedEdits(edits, size)
}

func validSortedEdits(edits []edit, size int) bool {
	end := 0
	for _, e := range edits {
		if e.start < end || e.start < 0 || e.end < e.start || e.end > size {
			return false
		}
		end = e.end
	}
	return true
}

func applySortedEdits(src []byte, edits []edit, size int) []byte {
	if len(edits) == 0 {
		return src
	}
	if size == 0 {
		return nil
	}
	out := make([]byte, 0, size)
	pos := 0
	for _, e := range edits {
		out = append(out, src[pos:e.start]...)
		out = append(out, e.text...)
		pos = e.end
	}
	return append(out, src[pos:]...)
}

type outputEOL uint8

const (
	outputEOLNone outputEOL = iota
	outputEOLCR
	outputEOLLF
	outputEOLCRLF
)

type outputSummary struct {
	empty   bool
	last    byte
	lastEOL outputEOL
}

func summarizeSortedEdits(src []byte, edits []edit) outputSummary {
	summary := outputSummary{empty: true}
	consume := func(chunk []byte) {
		if len(chunk) == 0 {
			return
		}
		if end := bytes.LastIndexAny(chunk, "\r\n"); end >= 0 {
			if chunk[end] == '\r' {
				summary.lastEOL = outputEOLCR
			} else if end > 0 && chunk[end-1] == '\r' || end == 0 && !summary.empty && summary.last == '\r' {
				summary.lastEOL = outputEOLCRLF
			} else {
				summary.lastEOL = outputEOLLF
			}
		}
		summary.last = chunk[len(chunk)-1]
		summary.empty = false
	}

	pos := 0
	for _, e := range edits {
		consume(src[pos:e.start])
		consume(e.text)
		pos = e.end
	}
	consume(src[pos:])
	return summary
}

func column(src []byte, l line, offset int) int {
	offset = min(l.bodyEnd, max(l.start, offset))
	return utf8.RuneCount(src[l.start:offset]) + 1
}

type columnPoint struct {
	offset, column int
}

func (p *formatter) columnAt(row, offset int) int {
	l := p.lines[row]
	offset = min(l.bodyEnd, max(l.start, offset))
	if offset-l.start < 4096 {
		return column(p.src, l, offset)
	}
	points, ok := p.columns[row]
	if !ok {
		if p.columns == nil {
			p.columns = make(map[int][]columnPoint)
		}
		count := (l.end-l.start)/1024 + 1
		if !p.reserveMetadata(row, offset, count*32) {
			return column(p.src, l, offset)
		}
		points = make([]columnPoint, 1, count)
		points[0] = columnPoint{l.start, 1}
		col, last := 1, l.start
		for pos := l.start; pos < l.end; pos++ {
			if !utf8.RuneStart(p.src[pos]) {
				continue
			}
			if pos-last >= 1024 {
				points = append(points, columnPoint{pos, col})
				last = pos
			}
			col++
		}
		p.columns[row] = points
	}
	i := sort.Search(len(points), func(i int) bool { return points[i].offset > offset }) - 1
	return points[i].column + utf8.RuneCount(p.src[points[i].offset:offset])
}
