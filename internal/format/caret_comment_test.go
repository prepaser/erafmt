package format

import (
	"bytes"
	"testing"
)

func TestCaretHeaderAndExpressionComments(t *testing.T) {
	src := []byte("@F;^; IF A\nRETURN 1+2;^; tail  \nPRINTV 1+2;^; tail\nRETURNF @\"x;^; y\"\n")
	want := []byte("@F;^; IF A\n    RETURN 1 + 2;^; tail  \n    PRINTV 1 + 2;^; tail\n    RETURNF @\"x;^; y\"\n")
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		r := FormatWithOptions(src, ERB, Options{Dialect: dialect})
		if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
			t.Fatalf("dialect=%v: output=%q diagnostics=%+v", dialect, r.Output, r.Diagnostics)
		}
		again := FormatWithOptions(r.Output, ERB, Options{Dialect: dialect})
		if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
			t.Fatalf("unstable caret comments: %+v", again)
		}
	}
}

func TestCaretCommentTerminatesFormIslands(t *testing.T) {
	for _, body := range []string{`RETURNF @"{RESULT;^;B}"`, `RETURNF \@1;^;x?yes#no\@`} {
		src := []byte("@F\n" + body + "\n")
		r := FormatWithOptions(src, ERB, Options{Select: []RuleID{BinarySpacing, FormPadding}})
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_expression" {
			t.Fatalf("%q: %+v", body, r)
		}
	}
}
