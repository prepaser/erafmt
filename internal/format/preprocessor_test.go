package format

import (
	"bytes"
	"testing"
)

func TestPreprocessorQueriesDoNotAllocate(t *testing.T) {
	p := formatter{}
	var conditional, skipped bool
	allocs := testing.AllocsPerRun(1000, func() {
		conditional = p.conditional()
		skipped = p.skipped()
	})
	if allocs != 0 || conditional || skipped {
		t.Fatalf("allocs=%v conditional=%v skipped=%v", allocs, conditional, skipped)
	}
}

func TestPreprocessorDirectiveScanningKeepsSuffixAndStateRules(t *testing.T) {
	cases := []struct {
		name string
		body string
		code string
	}{
		{"comment suffix", "@F\n[IF X] ; comment\nRETURN 1+2\n[ENDIF]\n", ""},
		{"third field", "@F\n[IF X Y] ; comment\nRETURN 1+2\n[ENDIF]\n", "unsupported_structure"},
		{"non-comment suffix", "@F\n[IF X] RETURN 1\n[ENDIF]\n", "unsupported_structure"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := []byte(tc.body)
			result := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
			if tc.code == "" {
				if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte("    RETURN 1+2")) {
					t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
				}
				return
			}
			if !bytes.Equal(result.Output, input) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != tc.code {
				t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
			}
		})
	}
}
