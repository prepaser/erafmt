package format

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestContinuationVirtualDiagnosticIgnoresBackingStorage(t *testing.T) {
	text := "@F\n{\nP\n}\n"
	separator := "RINTBUTTONLC　"
	var want Result
	for n, tail := range []string{"", strings.Repeat("x", 64), strings.Repeat("€😀", 16)} {
		storage := []byte(text + tail)
		src := storage[:len(text)]
		if n == 0 {
			src = src[:len(src):len(src)]
		}
		r := FormatWithOptions(src, ERB, Options{ContinuationSeparator: &separator})
		if !bytes.Equal(r.Output, src) || len(r.Changes) != 0 || len(r.Diagnostics) != 1 {
			t.Fatalf("tail=%q result=%+v", tail, r)
		}
		d := r.Diagnostics[0]
		if d.Line != 3 || d.Column != 2 || d.Severity != Unsupported || d.Code != "unsupported_structure" {
			t.Fatalf("tail=%q diagnostic=%+v", tail, d)
		}
		if n == 0 {
			want = r
		} else if !reflect.DeepEqual(r, want) {
			t.Fatalf("result depends on backing storage: got=%+v want=%+v", r, want)
		}
	}
}

func TestContinuationStatementDiagnosticsUsePhysicalOrigin(t *testing.T) {
	for _, tc := range []struct {
		name, src, separator, code string
		line, column               int
	}{
		{"else", "@F\n{\nELSE\n}\n", " ", "invalid_structure", 3, 1},
		{"blank-rows", "@F\n{\n\n\nELSE\n}\n", " ", "invalid_structure", 5, 1},
		{"split-command", "@F\n{\n  EL\nSE\n}\n", "", "invalid_structure", 3, 3},
		{"unclosed-if", "@F\n{\n  I\nF 1\n}\n", "", "invalid_structure", 3, 3},
		{"sif", "@F\n{\n  SIF 1\n}\n", " ", "invalid_structure", 3, 3},
		{"preprocessor", "@F\n{\n[IF X Y]\n}\n", " ", "unsupported_structure", 3, 1},
		{"unclosed-preprocessor", "@F\n{\n\n  [IF X]\n}\n", " ", "invalid_preprocessor", 4, 3},
		{"header", "@A\n{\n@F(ARG=);comment\n}\n", " ", "invalid_header", 3, 1},
		{"declaration", "@F\nRETURN 0\n{\n#DIM X\n}\n", " ", "unsupported_declaration", 4, 1},
		{"break", "@F\n{\n  BREAK\n}\n", " ", "invalid_loop_control", 3, 3},
		{"continue", "@F\n{\n  CONTINUE\n}\n", " ", "invalid_loop_control", 3, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := FormatWithOptions([]byte(tc.src), ERB, Options{ContinuationSeparator: &tc.separator})
			for _, d := range r.Diagnostics {
				if d.Code == tc.code {
					if d.Line != tc.line || d.Column != tc.column {
						t.Fatalf("diagnostic=%+v want=%d:%d", d, tc.line, tc.column)
					}
					return
				}
			}
			t.Fatalf("missing %s: %+v", tc.code, r.Diagnostics)
		})
	}
}

func TestContinuationUnprovenDiagnosticsKeepDelimiterOrigin(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		lines     []int
	}{
		{"unterminated", "@F\n{\nRETURN 0\n", []int{2}},
		{"nested", "@F\n{\n{\n}\n", []int{3, 3}},
		{"closing-tail", "@F\n{\n} RETURN\n", []int{3, 2}},
		{"unknown-command", "@F\n{\nRETURN\nX\n}\n", []int{2}},
		{"unknown-header", "@F\n{\n@G\n(ARG)\n}\n", []int{2}},
		{"unknown-dispatch", "@F\n{\n\nRETURN 0\n}\n", []int{2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := FormatWithOptions([]byte(tc.src), ERB, Options{})
			if len(r.Diagnostics) != len(tc.lines) {
				t.Fatalf("diagnostics=%+v want lines=%v", r.Diagnostics, tc.lines)
			}
			for i, d := range r.Diagnostics {
				if d.Line != tc.lines[i] || d.Column != 1 {
					t.Fatalf("diagnostic=%+v want=%d:1", d, tc.lines[i])
				}
			}
		})
	}
}

func TestDiagnosticColumnsClampToPhysicalLine(t *testing.T) {
	for _, text := range []string{"€😀", strings.Repeat("€😀", 1024)} {
		src := []byte("before\n" + text + "\r\nafter")
		l := line{start: len("before\n"), bodyEnd: len("before\n" + text), end: len("before\n" + text + "\r\n")}
		p := formatter{src: src, lines: []line{l}}
		for _, tc := range []struct{ offset, want int }{{-1, 1}, {len(src) + 100, utf8.RuneCountInString(text) + 1}, {l.start, 1}, {l.bodyEnd, utf8.RuneCountInString(text) + 1}} {
			if got := column(src, l, tc.offset); got != tc.want {
				t.Fatalf("column(%d)=%d want=%d", tc.offset, got, tc.want)
			}
			if got := p.columnAt(0, tc.offset); got != tc.want {
				t.Fatalf("columnAt(%d)=%d want=%d", tc.offset, got, tc.want)
			}
		}
	}
}

func TestContinuationDiagnosticSegmentBoundaries(t *testing.T) {
	segments := []continuationSegment{
		{logicalStart: 0, logicalEnd: 2, sourceStart: 10, sourceEnd: 12},
		{logicalStart: 2, logicalEnd: 4, sourceStart: 20, sourceEnd: 22},
		{logicalStart: 6, logicalEnd: 6, sourceStart: 30, sourceEnd: 30},
		{logicalStart: 8, logicalEnd: 10, sourceStart: 40, sourceEnd: 42},
	}
	for _, tc := range []struct {
		offset, segment, local int
	}{
		{offset: -1, segment: 0, local: -1},
		{offset: 0, segment: 0, local: 0},
		{offset: 1, segment: 0, local: 1},
		{offset: 2, segment: 1, local: 0},
		{offset: 4, segment: 1, local: 2},
		{offset: 5, segment: 1, local: 2},
		{offset: 6, segment: 2, local: 0},
		{offset: 7, segment: 2, local: 0},
		{offset: 8, segment: 3, local: 0},
		{offset: 10, segment: 3, local: 2},
		{offset: 99, segment: 3, local: 2},
	} {
		segment, local := continuationDiagnosticSegment(tc.offset, segments)
		if segment != tc.segment || local != tc.local {
			t.Errorf("offset=%d got=(%d,%d), want=(%d,%d)", tc.offset, segment, local, tc.segment, tc.local)
		}
	}
	if segment, local := continuationDiagnosticSegment(0, nil); segment != -1 || local != 0 {
		t.Fatalf("empty segments got=(%d,%d), want=(-1,0)", segment, local)
	}
	duplicateEnds := []continuationSegment{
		{logicalStart: 0, logicalEnd: 0, sourceStart: 10, sourceEnd: 10},
		{logicalStart: 0, logicalEnd: 0, sourceStart: 20, sourceEnd: 20},
		{logicalStart: 0, logicalEnd: 1, sourceStart: 30, sourceEnd: 31},
	}
	for _, tc := range []struct {
		offset, segment, local int
	}{
		{offset: 0, segment: 2, local: 0},
		{offset: 1, segment: 2, local: 1},
		{offset: 2, segment: 2, local: 1},
	} {
		segment, local := continuationDiagnosticSegment(tc.offset, duplicateEnds)
		if segment != tc.segment || local != tc.local {
			t.Errorf("duplicate ends offset=%d got=(%d,%d), want=(%d,%d)", tc.offset, segment, local, tc.segment, tc.local)
		}
	}
	allEmpty := duplicateEnds[:2]
	if segment, local := continuationDiagnosticSegment(0, allEmpty); segment != 0 || local != 0 {
		t.Fatalf("all-empty start got=(%d,%d), want=(0,0)", segment, local)
	}
	if segment, local := continuationDiagnosticSegment(1, allEmpty); segment != 1 || local != 0 {
		t.Fatalf("all-empty tail got=(%d,%d), want=(1,0)", segment, local)
	}
}

func TestContinuationDiagnosticCursorPreservesNonMonotonicColumns(t *testing.T) {
	physical := []byte("A€\n😀B\n")
	logical := []byte("A€|😀B")
	segments := []continuationSegment{
		{logicalStart: 0, logicalEnd: 4, line: 0, sourceStart: 0, sourceEnd: 4},
		{logicalStart: 5, logicalEnd: 10, line: 1, sourceStart: 5, sourceEnd: 10},
	}

	p := formatter{src: physical, lines: linesOf(physical)}
	continuationAppendDiagnostics(&p, []Diagnostic{
		{Column: 5, Severity: Error, Code: "cursor_error", Message: "error"},
		{Column: 2, Severity: Warning, Code: "cursor_warning", Message: "warning"},
		{Column: -10, Severity: Warning, Code: "cursor_negative", Message: "negative"},
		{Column: 100, Severity: Warning, Code: "cursor_oversized", Message: "oversized"},
		{Column: 1, Severity: Warning, Code: "cursor_reset", Message: "reset"},
	}, segments, logical)
	if len(p.diagnostics) != 5 {
		t.Fatalf("diagnostics=%+v, want five diagnostics", p.diagnostics)
	}
	for i, want := range []struct {
		line, column int
		severity     Severity
		code         string
	}{
		{line: 2, column: 2, severity: Error, code: "cursor_error"},
		{line: 1, column: 2, severity: Warning, code: "cursor_warning"},
		{line: 1, column: 1, severity: Warning, code: "cursor_negative"},
		{line: 2, column: 3, severity: Warning, code: "cursor_oversized"},
		{line: 1, column: 1, severity: Warning, code: "cursor_reset"},
	} {
		got := p.diagnostics[i]
		if got.Line != want.line || got.Column != want.column || got.Severity != want.severity || got.Code != want.code {
			t.Errorf("diagnostic[%d]=%+v, want %d:%d %s/%s", i, got, want.line, want.column, want.severity, want.code)
		}
	}

	p = formatter{src: physical, lines: linesOf(physical)}
	continuationAppendDiagnostics(&p, []Diagnostic{
		{Column: 5, Severity: Warning, Code: "before_limit", Message: "before"},
		{Column: 2, Severity: Error, Code: "resource_limit", Message: "limit"},
		{Column: 1, Severity: Warning, Code: "after_limit", Message: "after"},
	}, segments, logical)
	if len(p.diagnostics) != 2 || !p.aborted {
		t.Fatalf("resource diagnostics=%+v aborted=%t, want two diagnostics and abort", p.diagnostics, p.aborted)
	}
	if got := p.diagnostics[1]; got.Code != "resource_limit" || got.Line != 1 || got.Column != 2 {
		t.Fatalf("resource diagnostic=%+v, want line 1 column 2", got)
	}
}
