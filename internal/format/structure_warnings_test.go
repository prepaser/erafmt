package format

import (
	"bytes"
	"testing"
)

func TestTrustedStructureWarnings(t *testing.T) {
	for _, tt := range []struct{ body, code string }{
		{"$LOOP\n$LOOP\n", "duplicate_label"},
		{"REPEAT 1-1\nREND\n", "nonpositive_repeat"},
		{"REPEAT\nREND\n", "nonpositive_repeat"},
	} {
		src := []byte("@F\n" + tt.body)
		r := FormatWithOptions(src, ERB, Options{})
		if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != tt.code || r.Diagnostics[0].Severity != Warning {
			t.Fatalf("%q: %+v", tt.body, r.Diagnostics)
		}
		again := FormatWithOptions(r.Output, ERB, Options{})
		if !bytes.Equal(r.Output, again.Output) || len(again.Changes) != 0 {
			t.Fatalf("unstable: %q", again.Output)
		}
		if r := FormatWithOptions(src, ERB, Options{Select: []RuleID{InstructionIndent}}); len(r.Diagnostics) != 0 {
			t.Fatalf("selection: %+v", r.Diagnostics)
		}
	}
}

func TestStructureWarningsDeferUncertainPaths(t *testing.T) {
	for _, body := range []string{
		"$LOOP\n@G\n$LOOP\n",
		"$LOOP\n$loop\n",
		"$LOOP\n[IF USR_MACRO]\n$OTHER\n[ENDIF]\n$LOOP\n",
		"REPEAT USR_MACRO\nREND\n",
		";#;REPEAT 0\n;#;REND\n",
	} {
		r := FormatWithOptions([]byte("@F\n"+body), ERB, Options{})
		for _, diagnostic := range r.Diagnostics {
			if diagnostic.Code == "duplicate_label" || diagnostic.Code == "nonpositive_repeat" {
				t.Fatalf("%q: %+v", body, r.Diagnostics)
			}
		}
	}
}
