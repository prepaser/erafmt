package format

import (
	"strings"
	"testing"
)

func TestContinuationStatementResourceOrigins(t *testing.T) {
	for _, tc := range []struct {
		name, prefix, statement string
	}{
		{"structure", "@F\n" + strings.Repeat("IF 1\n", maxStructureDepth), "IF 1"},
		{"preprocessor", "@F\n" + strings.Repeat("[IF X]\n", maxPreprocessorDepth), "[IF X]"},
		{"skip", "@F\n" + strings.Repeat("[IF X]\n", maxPreprocessorDepth), "[SKIPSTART]"},
		{"state-units", "@F\n" + strings.Repeat("IF 1\n", maxStructureDepth-1) + strings.Repeat("[IF X]\n", maxPreprocessorStates/maxStructureDepth), "[ELSE]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, separator := range []string{"", " "} {
				for _, selection := range [][]RuleID{nil, {}} {
					input := []byte(tc.prefix + "{\n\n  " + tc.statement + "\n}\n")
					result := FormatWithOptions(input, ERB, Options{Select: selection, ContinuationSeparator: &separator})
					assertResourceLimit(t, input, result)
					wantLine := strings.Count(tc.prefix, "\n") + 3
					if len(result.Diagnostics) != 1 || result.Diagnostics[0].Line != wantLine || result.Diagnostics[0].Column != 3 {
						t.Fatalf("separator=%q selection=%v diagnostics=%+v want=%d:3", separator, selection, result.Diagnostics, wantLine)
					}
				}
			}
		})
	}
}

func TestContinuationDelimiterResourceKeepsOrigin(t *testing.T) {
	src := []byte("{\n\n  IF 1\n}\n")
	p := formatter{src: src, lines: linesOf(src), statementOrigins: map[int]sourcePosition{0: {line: 2, offset: 5}}}
	p.resourceProblem(0, 0, "continuation source exceeds the formatter resource limit")
	if len(p.diagnostics) != 1 || p.diagnostics[0].Line != 1 || p.diagnostics[0].Column != 1 {
		t.Fatalf("diagnostics=%+v", p.diagnostics)
	}
}
