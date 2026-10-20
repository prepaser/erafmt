package format

import (
	"strings"
	"testing"
)

func TestColumnCheckpointsPreserveUnicodePositions(t *testing.T) {
	src := []byte("@F\n" + strings.Repeat("a€😀́", 1000) + "\r\nshort\n")
	p := formatter{src: src, lines: linesOf(src)}
	if p.columnAt(0, 2) != 3 || p.columns != nil {
		t.Fatal("short line allocated a column index")
	}
	l := p.lines[1]
	offsets := []int{l.start, l.end, l.start + 4095, l.start + 4096, l.start + 4097}
	for offset := l.start; offset < l.end; offset += 257 {
		offsets = append(offsets, offset)
	}
	for i := len(offsets) - 1; i >= 0; i-- {
		offset := offsets[i]
		if got, want := p.columnAt(1, offset), column(src, l, offset); got != want {
			t.Fatalf("offset=%d column=%d want=%d", offset, got, want)
		}
	}
	if len(p.columns[1]) == 0 || len(p.columns[1]) > (l.end-l.start)/1024+1 {
		t.Fatalf("unexpected checkpoint count: %d", len(p.columns[1]))
	}
}

func TestSortedEditHelpersPreserveInsertionPriority(t *testing.T) {
	edits := []edit{
		{start: 1, end: 1, text: []byte("D"), change: Change{Rule: DeclarationBodySeparation}},
		{start: 1, end: 2, text: []byte("R"), change: Change{Rule: BinarySpacing}},
		{start: 1, end: 1, text: []byte("F"), change: Change{Rule: FunctionSeparation}},
		{start: 1, end: 1, text: []byte("A"), change: Change{Rule: BinarySpacing}},
		{start: 1, end: 1, text: []byte("B"), change: Change{Rule: BinarySpacing}},
	}
	sortEdits(edits)
	if !validSortedEdits(edits, 4) {
		t.Fatal("sorted edits were rejected")
	}
	size, ok := projectedOutputSizeSorted([]byte("abcd"), edits)
	if !ok || size != 8 {
		t.Fatalf("size=%d ok=%v", size, ok)
	}
	if got := string(applySortedEdits([]byte("abcd"), edits, size)); got != "aFDABRcd" {
		t.Fatalf("output=%q", got)
	}
}

func TestSummarizeSortedEditsTracksCrossChunkLineEndings(t *testing.T) {
	tests := []struct {
		name string
		src  string
		edit edit
		last byte
		eol  outputEOL
	}{
		{"source CR plus edit LF", "a\rb", edit{start: 2, end: 2, text: []byte("\n")}, 'b', outputEOLCRLF},
		{"edit CR plus source LF", "a\nb", edit{start: 1, end: 1, text: []byte("\r")}, 'b', outputEOLCRLF},
		{"replacement LF", "a\rb", edit{start: 1, end: 2, text: []byte("\n")}, 'b', outputEOLLF},
		{"deleted output", "a\r\n", edit{start: 1, end: 3, text: nil}, 'a', outputEOLNone},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			summary := summarizeSortedEdits([]byte(test.src), []edit{test.edit})
			if summary.empty || summary.last != test.last || summary.lastEOL != test.eol {
				t.Fatalf("summary=%+v", summary)
			}
		})
	}
}
