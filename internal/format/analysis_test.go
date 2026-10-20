package format

import (
	"bytes"
	"testing"
)

func TestMalformedHeaderPreservesWholeFile(t *testing.T) {
	for _, header := range []string{"@", "@123", "@F extra", "@F(1) extra", "@F(ARG=)", "@F[1", "@F(ARG==1)"} {
		input := []byte("@GOOD\nRETURN 1+2\n" + header + "\nRETURN 3+4\n")
		for _, options := range []Options{{}, {Select: []RuleID{}}} {
			r := FormatWithOptions(input, ERB, options)
			if !bytes.Equal(r.Output, input) || len(r.Changes) != 0 || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "invalid_header" || r.Diagnostics[0].Severity != Error {
				t.Fatalf("header=%q result=%+v", header, r)
			}
		}
	}
}

func TestDeclarationDefaultKeepsComparisonTokens(t *testing.T) {
	input := []byte("@F(ARG=1==2)\n#DIM USR_X,1==1=1==2\n")
	r := FormatWithOptions(input, ERB, Options{})
	want := "@F(ARG = 1 == 2)\n    #DIM USR_X, 1 == 1 = 1 == 2\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	input = []byte("@F\n#DIM USR_X==1\nRETURN 0\n")
	r = FormatWithOptions(input, ERB, Options{})
	if !bytes.Contains(r.Output, []byte("\n#DIM USR_X==1\n")) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("invalid initializer result=%+v", r)
	}
}

func TestMisplacedERBDeclarationPreservesItsRegion(t *testing.T) {
	for _, region := range []string{
		"#DIM  CONST USR_X,1+2\nRETURN 0\n",
		"@BAD\nRETURN 0\n#DIM  CONST USR_X,1+2\n",
		"@BAD\n$LABEL\n#DIM  USR_X\n",
	} {
		input := []byte(region + "@GOOD\nRETURN 1+2\n")
		r := FormatWithOptions(input, ERB, Options{})
		want := region + "@GOOD\n    RETURN 1 + 2\n"
		if string(r.Output) != want || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_declaration" {
			t.Fatalf("output=%q want=%q diagnostics=%+v", r.Output, want, r.Diagnostics)
		}
	}
	r := FormatWithOptions([]byte("#DIM  USR_X,1+2\n"), ERH, Options{})
	if string(r.Output) != "#DIM USR_X, 1 + 2\n" || len(r.Diagnostics) != 0 {
		t.Fatalf("ERH result=%+v", r)
	}
}

func TestDeclarationContextAcrossPreprocessorBranches(t *testing.T) {
	for _, src := range []string{
		"@F\n[IF USR_X]\nRETURN 0\n[ENDIF]\n#DIM  USR_X\n",
		"@F\n[IF USR_X]\n#DIM USR_X\n[ELSE]\nRETURN 0\n[ENDIF]\n#DIM  USR_Y\n",
	} {
		r := FormatWithOptions([]byte(src), ERB, Options{})
		if string(r.Output) != src || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_declaration" {
			t.Fatalf("result=%+v", r)
		}
	}
	src := []byte("@F\n[IF USR_X]\nRETURN 0\n[ENDIF]\nRETURN 1\n")
	r := FormatWithOptions(src, ERB, Options{})
	if len(r.Diagnostics) != 0 || !bytes.Contains(r.Output, []byte("    RETURN 1\n")) {
		t.Fatalf("valid conditional body result=%+v", r)
	}
}

func TestPreprocessorSuffixPreservesUntrustedFile(t *testing.T) {
	for _, suffix := range []string{" trailing", ";!; marker", " ; [[alias]]"} {
		input := []byte("@F\n[IF   USR_X]" + suffix + "\nRETURN 1+2\n[ENDIF]\n")
		r := FormatWithOptions(input, ERB, Options{})
		if !bytes.Equal(r.Output, input) || len(r.Changes) != 0 || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("suffix=%q result=%+v", suffix, r)
		}
	}
	input := []byte("@F\n[IF   USR_X] ; ordinary comment\nRETURN 0\n[ENDIF]\n")
	r := FormatWithOptions(input, ERB, Options{})
	if len(r.Diagnostics) != 0 || !bytes.Contains(r.Output, []byte("[IF USR_X] ; ordinary comment")) {
		t.Fatalf("ordinary comment result=%+v", r)
	}
}

func TestConditionalBranchesKeepCommonStructure(t *testing.T) {
	cases := []struct{ source, want string }{
		{"@F\nIF A\nRETURN 0\n[IF_DEBUG]\nELSEIF B\nRETURN 1\n[ENDIF]\nELSEIF C\nRETURN 2\nENDIF\n", "@F\n    IF A\n        RETURN 0\n[IF_DEBUG]\n    ELSEIF B\n        RETURN 1\n[ENDIF]\n    ELSEIF C\n        RETURN 2\n    ENDIF\n"},
		{"@F\nSELECTCASE A\nCASE 1\nRETURN 0\n[IF_DEBUG]\nCASEELSE\nRETURN 1\n[ENDIF]\nENDSELECT\nRETURN 2\n", "@F\n    SELECTCASE A\n        CASE 1\n            RETURN 0\n[IF_DEBUG]\n        CASEELSE\n            RETURN 1\n[ENDIF]\n    ENDSELECT\n    RETURN 2\n"},
	}
	for _, tc := range cases {
		for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
			r := FormatWithOptions([]byte(tc.source), ERB, Options{Select: []RuleID{InstructionIndent}, Dialect: dialect})
			if string(r.Output) != tc.want || len(r.Diagnostics) != 0 {
				t.Fatalf("dialect=%v output=%q diagnostics=%+v", dialect, r.Output, r.Diagnostics)
			}
			again := FormatWithOptions(r.Output, ERB, Options{Select: []RuleID{InstructionIndent}, Dialect: dialect})
			if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
				t.Fatalf("not idempotent: %+v", again)
			}
		}
	}
}

func TestConditionalFinalBranchRequiresCommonCloser(t *testing.T) {
	for _, tail := range []string{"CASE 2\nRETURN 2\nENDSELECT\n", ";#;ENDSELECT\n", "RETURN 2\nENDSELECT\n", "[IF USR_X]\nENDSELECT\n[ENDIF]\n"} {
		input := []byte("@F\nSELECTCASE A\nCASE 1\nRETURN 0\n[IF_DEBUG]\nCASEELSE\nRETURN 1\n[ENDIF]\n" + tail)
		r := FormatWithOptions(input, ERB, Options{})
		if !bytes.Equal(r.Output, input) || len(r.Changes) != 0 || len(r.Diagnostics) == 0 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("tail=%q result=%+v", tail, r)
		}
	}
}
