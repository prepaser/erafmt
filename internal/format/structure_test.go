package format

import (
	"bytes"
	"testing"
)

func TestERHRejectsRuntimeUnsupportedStructures(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		code string
	}{
		{name: "preprocessor", src: "#DIM USR_X,1+2\n[IF USR_X]\n#DIM USR_Y,2+3\n[ENDIF]\n", code: "unsupported_structure"},
		{name: "local size", src: "#LOCALSIZE 1+2\n", code: "unsupported_declaration"},
		{name: "local string size", src: "#LOCALSSIZE 1+2\n", code: "unsupported_declaration"},
		{name: "single", src: "#SINGLE\n", code: "unsupported_declaration"},
		{name: "later", src: "#LATER\n", code: "unsupported_declaration"},
		{name: "priority", src: "#PRI\n", code: "unsupported_declaration"},
		{name: "only", src: "#ONLY\n", code: "unsupported_declaration"},
		{name: "function", src: "#FUNCTION\n", code: "unsupported_declaration"},
		{name: "string function", src: "#FUNCTIONS\n", code: "unsupported_declaration"},
		{name: "unknown", src: "#UNKNOWN\n", code: "unsupported_declaration"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := FormatWithOptions([]byte(tc.src), ERH, Options{Select: []RuleID{BinarySpacing, DeclarationIndent}})
			if !bytes.Equal(r.Output, []byte(tc.src)) || len(r.Changes) != 0 || len(r.Diagnostics) != 1 {
				t.Fatalf("result=%+v", r)
			}
			if r.Diagnostics[0].Severity != Unsupported || r.Diagnostics[0].Code != tc.code {
				t.Fatalf("diagnostic=%+v", r.Diagnostics[0])
			}
		})
	}
}

func TestERHReferencePreservationWithFullwidthWhitespace(t *testing.T) {
	src := []byte("#DIM\u3000REF\u3000UserVar\n#DIM Other,1+2\n")
	r := FormatWithOptions(src, ERH, Options{Select: []RuleID{}, AllowFullwidthSpace: true})
	if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_declaration" {
		t.Fatalf("result=%+v", r)
	}
}

func TestERHRetainsSupportedHeaderForms(t *testing.T) {
	src := []byte("#DEFINE USR_MACRO  1+2\n#DIM USR_NUM,1+2\n#DIMS USR_STR,2+3\n")
	r := FormatWithOptions(src, ERH, Options{Select: []RuleID{BinarySpacing, CommaSpacing}})
	if !bytes.Equal(r.Output, []byte("#DEFINE USR_MACRO  1+2\n#DIM USR_NUM, 1 + 2\n#DIMS USR_STR, 2 + 3\n")) || len(r.Diagnostics) != 0 {
		t.Fatalf("result=%+v", r)
	}
}

func TestUnsupportedERHPreservesSyntaxWithFileRules(t *testing.T) {
	src := []byte("#DIM USR_NUM,1+2\r\n#LOCALSIZE 2+3")
	r := FormatWithOptions(src, ERH, Options{})
	if !bytes.Equal(r.Output, []byte("#DIM USR_NUM,1+2\n#LOCALSIZE 2+3\n")) || len(r.Diagnostics) != 1 {
		t.Fatalf("result=%+v", r)
	}
	for _, change := range r.Changes {
		if change.Rule != FinalNewline && change.Rule != LineEndingNormalization {
			t.Fatalf("unexpected syntax change: %+v", change)
		}
	}
}

func TestLoopControlOutsideLoopIsWarningOnly(t *testing.T) {
	src := []byte("@F\nBREAK\nCONTINUE\nFOR I,0,1\nBREAK\nCONTINUE\nNEXT\nRETURN 0\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{InstructionIndent, CommaSpacing}})
	want := []byte("@F\n    BREAK\n    CONTINUE\n    FOR I, 0, 1\n        BREAK\n        CONTINUE\n    NEXT\n    RETURN 0\n")
	if !bytes.Equal(r.Output, want) || len(r.Changes) == 0 || len(r.Diagnostics) != 2 {
		t.Fatalf("output=%q changes=%+v diagnostics=%+v", r.Output, r.Changes, r.Diagnostics)
	}
	for _, d := range r.Diagnostics {
		if d.Severity != Warning || d.Code != "invalid_loop_control" {
			t.Fatalf("diagnostic=%+v", r.Diagnostics)
		}
	}
}

func TestConditionalLoopControlDoesNotWarn(t *testing.T) {
	src := []byte("@F\n[IF USR_X]\nBREAK\n[ENDIF]\nRETURN 0\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{InstructionIndent}})
	if !bytes.Equal(r.Output, []byte("@F\n[IF USR_X]\n    BREAK\n[ENDIF]\n    RETURN 0\n")) || len(r.Diagnostics) != 0 {
		t.Fatalf("result=%+v", r)
	}
}

func TestFinalBranchWarningsKeepFormatting(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "if",
			src:  "@F\nIF 1\nRETURN 1\nELSE\nRETURN 2\nELSEIF 3\nRETURN 3\nENDIF\nRETURN 0\n",
			want: "@F\n    IF 1\n        RETURN 1\n    ELSE\n        RETURN 2\n    ELSEIF 3\n        RETURN 3\n    ENDIF\n    RETURN 0\n",
		},
		{
			name: "select",
			src:  "@F\nSELECTCASE 1\nCASEELSE\nRETURN 1\nCASE 2\nRETURN 2\nENDSELECT\nRETURN 0\n",
			want: "@F\n    SELECTCASE 1\n        CASEELSE\n            RETURN 1\n        CASE 2\n            RETURN 2\n    ENDSELECT\n    RETURN 0\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := FormatWithOptions([]byte(tc.src), ERB, Options{Select: []RuleID{InstructionIndent}})
			if string(r.Output) != tc.want || len(r.Diagnostics) != 1 {
				t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
			}
			d := r.Diagnostics[0]
			if d.Severity != Warning || d.Code != "branch_after_final" {
				t.Fatalf("diagnostic=%+v", d)
			}
		})
	}
}

func TestNestedRepeatWarningIsAdvisory(t *testing.T) {
	src := []byte("@F\nREPEAT\nREPEAT\nREND\nREND\nRETURN 0\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{InstructionIndent}})
	want := "@F\n    REPEAT\n        REPEAT\n        REND\n    REND\n    RETURN 0\n"
	if string(r.Output) != want || len(r.Diagnostics) != 1 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	if d := r.Diagnostics[0]; d.Severity != Warning || d.Code != "nested_repeat" {
		t.Fatalf("diagnostic=%+v", d)
	}
}

func TestConditionalRepeatDoesNotWarn(t *testing.T) {
	src := []byte("@F\n[IF USR_X]\nREPEAT\nREPEAT\nREND\nREND\n[ENDIF]\nRETURN 0\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{InstructionIndent}})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}
