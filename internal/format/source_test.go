package format

import (
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestColumnCheckpointsPreserveUnicodePositions(t *testing.T) {
	src := []byte("@F\n" + strings.Repeat("a€😀́", 1000) + "\r\nshort\n")
	p := formatter{DocumentSession: erabasic.DocumentSession{Src: src, Lines: linesOf(src)}}
	if p.columnAt(0, 2) != 3 || p.Columns != nil {
		t.Fatal("short line allocated a column index")
	}
	l := p.Lines[1]
	offsets := []int{l.Start, l.End, l.Start + 4095, l.Start + 4096, l.Start + 4097}
	for offset := l.Start; offset < l.End; offset += 257 {
		offsets = append(offsets, offset)
	}
	for i := len(offsets) - 1; i >= 0; i-- {
		offset := offsets[i]
		if got, want := p.columnAt(1, offset), erabasic.Column(src, l, offset); got != want {
			t.Fatalf("offset=%d column=%d want=%d", offset, got, want)
		}
	}
	if len(p.Columns[1]) == 0 || len(p.Columns[1]) > (l.End-l.Start)/1024+1 {
		t.Fatalf("unexpected checkpoint count: %d", len(p.Columns[1]))
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
