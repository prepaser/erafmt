package format

import (
	"bytes"
	"testing"
)

func TestMalformedNestedFormDiagnostics(t *testing.T) {
	for _, body := range []string{
		`RETURNF @"text{1+2}`,
		`RETURNF \@1?text#other`,
		`RETURNF @"text{1+2"`,
	} {
		src := []byte("@F\n" + body + "\n")
		r := FormatWithOptions(src, ERB, Options{Select: []RuleID{BinarySpacing, FormPadding}})
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_expression" || r.Diagnostics[0].Line != 2 {
			t.Fatalf("%q: %+v", body, r)
		}
		ignored := FormatWithOptions(src, ERB, Options{Select: []RuleID{InstructionIndent}})
		if len(ignored.Diagnostics) != 0 {
			t.Fatalf("syntax diagnostics without expression rules: %+v", ignored.Diagnostics)
		}
	}
}

func TestNulBoundaries(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  string
	}{
		{"RETURN 1+2\x00 +3 ; ignored \t", "RETURN 1 + 2\x00 +3 ; ignored \t"},
		{"VARSET RESULT,1+2\x00,,ARG:0+3  ", "VARSET RESULT, 1 + 2\x00,,ARG:0+3  "},
		{"PRINTFORM {1+2}x\x00{1,,RIGHT}", "PRINTFORM {1 + 2}x\x00{1,,RIGHT}"},
		{"PRINTFORM x\x00{A", "PRINTFORM x\x00{A"},
		{"RETURNFORM x\x00,{3+4}", "RETURNFORM x\x00,{3+4}"},
		{"PRINTV 'x\x00,2+3  ", "PRINTV 'x\x00,2+3  "},
		{"PRINTV 1+2\x00,3+4  ", "PRINTV 1 + 2\x00,3+4  "},
		{"RETURN A\x00B+1", "RETURN A\x00B + 1"},
		{"PRINTV A\x00B+1", "PRINTV A\x00B + 1"},
		{"PRINTFORM {A\x00B+1}", "PRINTFORM {A\x00B + 1}"},
	} {
		src := []byte("@F\n" + tt.input + "\n")
		r := FormatWithOptions(src, ERB, Options{})
		want := []byte("@F\n    " + tt.want + "\n")
		if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
			t.Fatalf("%q: output=%q diagnostics=%+v, want %q", tt.input, r.Output, r.Diagnostics, want)
		}
		again := FormatWithOptions(r.Output, ERB, Options{})
		if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 || len(again.Diagnostics) != 0 {
			t.Fatalf("unstable NUL boundary %q: %+v", tt.input, again)
		}
	}
}

func TestUnresolvedNestedFormMarkersRemainOpaque(t *testing.T) {
	for _, body := range []string{
		`RETURNF @"{A;#;B}"`,
		`RETURNF \@A;#;B?x#y\@`,
		`RETURNF @"{[[alias]]}"`,
	} {
		src := []byte("@F\n" + body + "\n")
		r := FormatWithOptions(src, ERB, Options{Select: []RuleID{BinarySpacing, FormPadding}})
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 0 || len(r.Changes) != 0 {
			t.Fatalf("opaque FORM %q: %+v", body, r)
		}
	}
}

func TestNulUnterminatedLiterals(t *testing.T) {
	for _, body := range []string{
		"RETURNF \"x\x00y\"",
		"RETURNF \"x\\\x00y\"",
		"RETURNF @\"x\x00{1+2}\"",
		"PRINTFORM x\\\x00y",
		"PRINTFORM {1+\x00}",
		"PRINTV 'x\\\x00y",
	} {
		src := []byte("@F\n" + body + "\n")
		r := FormatWithOptions(src, ERB, Options{Select: []RuleID{BinarySpacing, FormPadding}})
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 {
			t.Fatalf("%q: output=%q diagnostics=%+v", body, r.Output, r.Diagnostics)
		}
	}
}
