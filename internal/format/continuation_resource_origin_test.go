package format

import (
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestContinuationStatementResourceOrigins(t *testing.T) {
	for _, tc := range []struct{ name, prefix, statement string }{
		{"structure", "@F\n" + strings.Repeat("IF 1\n", erabasic.MaxStructureDepth), "IF 1"},
		{"preprocessor", "@F\n" + strings.Repeat("[IF X]\n", erabasic.MaxPreprocessorDepth), "[IF X]"},
		{"skip", "@F\n" + strings.Repeat("[IF X]\n", erabasic.MaxPreprocessorDepth), "[SKIPSTART]"},
		{"state-units", "@F\n" + strings.Repeat("IF 1\n", erabasic.MaxStructureDepth-1) + strings.Repeat("[IF X]\n", erabasic.MaxPreprocessorStates/erabasic.MaxStructureDepth), "[ELSE]"},
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
	p := formatter{DocumentSession: erabasic.DocumentSession{Src: src, Lines: linesOf(src), StatementOrigins: map[int]erabasic.SourcePosition{0: {Line: 2, Offset: 5}}}}
	p.resourceProblem(0, 0, "continuation source exceeds the formatter resource limit")
	if len(p.diagnostics) != 1 || p.diagnostics[0].Line != 1 || p.diagnostics[0].Column != 1 {
		t.Fatalf("diagnostics=%+v", p.diagnostics)
	}
}

func TestVariableArgumentResourceKeepsFieldOrigin(t *testing.T) {
	field := "A:(" + strings.Repeat("1+", 40000) + "1)"
	for _, tc := range []struct {
		name, gap string
		fullwidth bool
		indent    string
		want      int
	}{
		{"ascii", " ", false, "", 65543},
		{"ascii-indent", " ", false, "  ", 65545},
		{"ascii-gap", "    ", false, "", 65546},
		{"fullwidth", "　", true, "", 65543},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := []byte("@F\n" + tc.indent + "TIMES" + tc.gap + field + ",1\n")
			result := FormatWithOptions(input, ERB, Options{AllowFullwidthSpace: tc.fullwidth})
			assertFieldResourceOrigin(t, result, 2, tc.want)
		})
	}
}

func TestContinuationVariableArgumentResourceKeepsFieldOrigin(t *testing.T) {
	field := "A:(" + strings.Repeat("1+", 40000) + "1)"
	logical := "TIMES " + field + ",1"
	cut := 65539
	separator := ""
	input := []byte("@F\n{\n" + logical[:cut] + "\n" + logical[cut:] + "\n}\n")
	result := FormatWithOptions(input, ERB, Options{ContinuationSeparator: &separator})
	assertFieldResourceOrigin(t, result, 4, 4)
}

func assertFieldResourceOrigin(t *testing.T, result Result, line, column int) {
	t.Helper()
	if len(result.Diagnostics) != 1 {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
	diagnostic := result.Diagnostics[0]
	if diagnostic.Code != "resource_limit" || diagnostic.Line != line || diagnostic.Column != column {
		t.Fatalf("diagnostic=%+v want=%d:%d", diagnostic, line, column)
	}
}
