package format

import (
	"fmt"
	"testing"
)

func TestSelectCaseProvenTypes(t *testing.T) {
	base := "@F\nSELECTCASE %s\nCASE %s\nRETURN 0\nENDSELECT\nRETURN 0\n"
	for _, tc := range []struct {
		name  string
		value string
		casev string
		bad   bool
	}{
		{name: "matching numeric", value: "1", casev: "2"},
		{name: "matching string", value: `"x"`, casev: `"y"`},
		{name: "selector ignores extra fields", value: `"x",1`, casev: `"y"`},
		{name: "mismatched", value: `"x"`, casev: "1", bad: true},
		{name: "mixed case fields", value: "1", casev: `1,"x"`, bad: true},
		{name: "trailing case comma", value: `"x"`, casev: `1,`, bad: true},
		{name: "string range", value: `"x"`, casev: `"a" TO "z"`},
		{name: "numeric range mismatch", value: "1", casev: `"a" TO "z"`, bad: true},
		{name: "numeric is", value: "1", casev: "IS > 0"},
		{name: "variable-name call", value: `"x"`, casev: "RESULT()"},
		{name: "variable-name call range", value: `"x"`, casev: `RESULT() TO "z"`},
		{name: "variable-name call comparison", value: `"x"`, casev: "IS == RESULT()"},
		{name: "numeric variable mismatch", value: `"x"`, casev: "RESULT", bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := FormatWithOptions([]byte(fmt.Sprintf(base, tc.value, tc.casev)), ERB, Options{Select: []RuleID{InstructionIndent, BinarySpacing, CaseSpacing}})
			if tc.bad {
				if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_syntax" {
					t.Fatalf("expected mismatch diagnostic: %+v", r.Diagnostics)
				}
				return
			}
			if len(r.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %+v", r.Diagnostics)
			}
		})
	}
}

func TestSelectCaseUnknownTypeDefers(t *testing.T) {
	for _, selector := range []string{"VALUE"} {
		src := fmt.Sprintf("@F\nSELECTCASE %s\nCASE 1\nRETURN 0\nENDSELECT\n", selector)
		r := FormatWithOptions([]byte(src), ERB, Options{Select: []RuleID{InstructionIndent, BinarySpacing, CaseSpacing}})
		if len(r.Diagnostics) != 0 {
			t.Fatalf("selector=%q diagnostics=%+v", selector, r.Diagnostics)
		}
	}
}

func TestSelectCaseTypeMergesPreprocessorPaths(t *testing.T) {
	src := "@F\n[IF X]\nSELECTCASE 1\n[ELSE]\nSELECTCASE \"x\"\n[ENDIF]\nCASE 1\nRETURN 0\nENDSELECT\n"
	r := FormatWithOptions([]byte(src), ERB, Options{Select: []RuleID{InstructionIndent, BinarySpacing, CaseSpacing}})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", r.Diagnostics)
	}
}

func TestEmptyDataWarnings(t *testing.T) {
	for _, tc := range []struct {
		name  string
		src   string
		lines []int
	}{
		{name: "data", src: "@F\nPRINTDATA\nENDDATA\n", lines: []int{3}},
		{name: "list", src: "@F\nPRINTDATA\nDATALIST\nENDLIST\nENDDATA\n", lines: []int{4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := FormatWithOptions([]byte(tc.src), ERB, Options{Select: []RuleID{InstructionIndent}})
			if len(r.Diagnostics) != len(tc.lines) {
				t.Fatalf("unexpected diagnostics: %+v", r.Diagnostics)
			}
			for i, line := range tc.lines {
				if r.Diagnostics[i].Code != "empty_data" || r.Diagnostics[i].Line != line {
					t.Fatalf("unexpected diagnostics: %+v", r.Diagnostics)
				}
			}
		})
	}
	for _, src := range []string{
		"@F\nPRINTDATA\nDATA 1\nENDDATA\n",
		"@F\nPRINTDATA\nDATALIST\nDATA 1\nENDLIST\nENDDATA\n",
		"@F\n[IF X]\nPRINTDATA\nENDDATA\n[ENDIF]\n",
	} {
		r := FormatWithOptions([]byte(src), ERB, Options{Select: []RuleID{InstructionIndent}})
		if len(r.Diagnostics) != 0 {
			t.Fatalf("unexpected diagnostics for %q: %+v", src, r.Diagnostics)
		}
	}
}
