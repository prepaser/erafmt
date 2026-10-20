package format

import (
	"bytes"
	"testing"
)

func TestDeclarationAndHeaderRejectBareAssignmentExpressions(t *testing.T) {
	tests := []struct {
		source       string
		code         string
		line, column int
		output       string
	}{
		{"@F(ARG:0=1=2)\n", "unsupported_syntax", 1, 11, "@F(ARG:0=1=2)\n"},
		{"@F\n#DIM VALUE=1=2\n", "unsupported_expression", 2, 13, "@F\n#DIM VALUE=1=2\n"},
		{"@F\n#DIM VALUE,1=1=2\n", "unsupported_expression", 2, 15, "@F\n#DIM VALUE,1=1=2\n"},
	}
	for _, test := range tests {
		assertBareAssignmentDiagnostic(t, []byte(test.source), Options{}, test.code, test.line, test.column, []byte(test.output))
	}
}

func TestDeclarationAndHeaderKeepComparisonExpressions(t *testing.T) {
	for _, source := range []string{
		"@F(ARG:0=1==2)\n",
		"@F\n#DIM VALUE=1==2\n",
		"@F\n#DIM VALUE,1=1==2\n",
		"@F\n#DIMS VALUE,2=\"a\",\"b\"\n",
	} {
		result := FormatWithOptions([]byte(source), ERB, Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
		again := FormatWithOptions(result.Output, ERB, Options{})
		if len(again.Diagnostics) != 0 || !bytes.Equal(again.Output, result.Output) {
			t.Fatalf("source=%q second pass diagnostics=%+v output=%q", source, again.Diagnostics, again.Output)
		}
	}
}

func TestDeclarationAndHeaderAssignmentChecksRespectRuleSelection(t *testing.T) {
	for _, source := range []string{"@F(ARG:0=1=2)\n", "@F\n#DIM VALUE=1=2\n"} {
		result := FormatWithOptions([]byte(source), ERB, Options{Select: []RuleID{}})
		if len(result.Diagnostics) != 0 || !bytes.Equal(result.Output, []byte(source)) {
			t.Fatalf("source=%q result=%+v", source, result)
		}
	}
}

func TestContinuedHeaderRejectsBareAssignmentExpression(t *testing.T) {
	separator := " "
	source := []byte("{\n@F(ARG:0=1=\n2)\n}\nRETURN 0\n")
	want := []byte("{\n@F(ARG:0=1=\n2)\n}\n    RETURN 0\n")
	assertBareAssignmentDiagnostic(t, source, Options{ContinuationSeparator: &separator}, "unsupported_syntax", 2, 11, want)
}

func TestContinuedDeclarationRejectsBareAssignmentExpression(t *testing.T) {
	separator := " "
	source := []byte("@F\n{\n#DIM VALUE=1=\n2\n}\nRETURN 0\n")
	want := []byte("@F\n    {\n#DIM VALUE=1=\n2\n    }\n    RETURN 0\n")
	assertBareAssignmentDiagnostic(t, source, Options{ContinuationSeparator: &separator}, "unsupported_expression", 3, 13, want)
}

func TestDeclarationAndHeaderAssignmentChecksDeferUnknownSyntax(t *testing.T) {
	for _, source := range []string{
		"@F(ARG:0=UNKNOWN=2)\n",
		"@F(ARG:0=UNKNOWN+ABS(1)++)\n",
		"@F(ARG:0=UNKNOWN=2);!;\n",
		"@F\n#DIM VALUE=UNKNOWN=2\n",
		"@F\n#DIM VALUE=1;!;=2\n",
	} {
		result := FormatWithOptions([]byte(source), ERB, Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
		again := FormatWithOptions(result.Output, ERB, Options{})
		if len(again.Diagnostics) != 0 || !bytes.Equal(again.Output, result.Output) {
			t.Fatalf("source=%q second pass diagnostics=%+v output=%q", source, again.Diagnostics, again.Output)
		}
	}
}

func assertBareAssignmentDiagnostic(t *testing.T, source []byte, options Options, code string, line, column int, wantOutput []byte) {
	t.Helper()
	result := FormatWithOptions(source, ERB, options)
	if len(result.Diagnostics) != 1 {
		t.Fatalf("source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
	}
	diagnostic := result.Diagnostics[0]
	if diagnostic.Code != code || diagnostic.Severity != Unsupported || diagnostic.Line != line || diagnostic.Column != column || diagnostic.Message != "assignment operator in expression" || !bytes.Equal(result.Output, wantOutput) {
		t.Fatalf("source=%q diagnostic=%+v output=%q", source, diagnostic, result.Output)
	}
	again := FormatWithOptions(result.Output, ERB, options)
	if !bytes.Equal(again.Output, result.Output) || len(again.Diagnostics) != 1 || again.Diagnostics[0] != diagnostic {
		t.Fatalf("source=%q second pass diagnostic=%+v output=%q", source, again.Diagnostics, again.Output)
	}
}
