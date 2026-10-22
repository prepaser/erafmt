package format

import (
	"bytes"

	"testing"
)

func continuationProofDiagnostic(result Result, code string) (Diagnostic, bool) {
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == code {
			return diagnostic, true
		}
	}
	return Diagnostic{}, false
}

func TestContinuationProofMapsStructureWarnings(t *testing.T) {
	separator := " "
	options := Options{ContinuationSeparator: &separator}
	t.Run("nested repeat and count", func(t *testing.T) {
		src := []byte("@F\nREPEAT 1\n{\nREPEAT 0\n}\nREND\nREND\n")
		result := FormatWithOptions(src, ERB, options)
		if len(result.Diagnostics) != 2 {
			t.Fatalf("diagnostics=%+v", result.Diagnostics)
		}
		if nested := result.Diagnostics[0]; nested.Code != "nested_repeat" || nested.Line != 4 || nested.Column != 1 {
			t.Fatalf("nested diagnostic=%+v", nested)
		}
		if count := result.Diagnostics[1]; count.Code != "nonpositive_repeat" || count.Line != 4 || count.Column != 8 {
			t.Fatalf("count diagnostic=%+v", count)
		}
	})
	t.Run("final branch keeps physical column", func(t *testing.T) {
		src := []byte("@F\nIF 1\nELSE\n{\n  ELSEIF 2\n}\nENDIF\n")
		result := FormatWithOptions(src, ERB, options)
		diagnostic, ok := continuationProofDiagnostic(result, "branch_after_final")
		if !ok || diagnostic.Line != 5 || diagnostic.Column != 3 {
			t.Fatalf("diagnostics=%+v", result.Diagnostics)
		}
	})
}

func TestContinuationProofMergesDataMetadataConservatively(t *testing.T) {
	separator := " "
	options := Options{ContinuationSeparator: &separator}
	tests := []struct {
		name string
		src  string
		want bool
		line int
	}{
		{name: "agreeing branches", src: "@F\n[IF X]\n{\nPRINTDATA\n}\n[ELSE]\n{\nPRINTDATA\n}\n[ENDIF]\nENDDATA\n", want: true, line: 11},
		{name: "alias branch", src: "@F\n[IF X]\n{\nPRINTDATA\n}\n[ELSE]\n{\nPRINTDATA [[ALIAS]]\n}\n[ENDIF]\nENDDATA\n"},
		{name: "different branch frames", src: "@F\n[IF X]\n{\nPRINTDATA\n}\n[ELSE]\n{\nREPEAT 1\n}\n[ENDIF]\nENDDATA\n"},
		{name: "empty data list", src: "@F\nPRINTDATA\n{\nDATALIST\n}\nENDLIST\nENDDATA\n", want: true, line: 6},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := FormatWithOptions([]byte(test.src), ERB, options)
			diagnostic, ok := continuationProofDiagnostic(result, "empty_data")
			if ok != test.want || test.want && diagnostic.Line != test.line {
				t.Fatalf("diagnostics=%+v", result.Diagnostics)
			}
			again := FormatWithOptions(result.Output, ERB, options)
			if !bytes.Equal(again.Output, result.Output) || len(again.Changes) != 0 {
				t.Fatalf("not idempotent: output=%q again=%q changes=%+v", result.Output, again.Output, again.Changes)
			}
		})
	}
}

func TestContinuationProofDefersUncertainRows(t *testing.T) {
	separator := " "
	tests := []struct {
		name    string
		src     string
		options Options
	}{
		{name: "alias", src: "@F\nREPEAT 1\n{\nREPEAT 2 [[ALIAS]]\n}\nREND\nREND\n", options: Options{ContinuationSeparator: &separator}},
		{name: "leading marker", src: "@F\nREPEAT 1\n{\n;!;REPEAT 2\n}\nREND\nREND\n", options: Options{ContinuationSeparator: &separator}},
		{name: "missing separator", src: "@F\nREPEAT 1\n{\nREPEAT 2\n}\nREND\nREND\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := FormatWithOptions([]byte(test.src), ERB, test.options)
			for _, code := range []string{"nested_repeat", "nonpositive_repeat", "empty_data", "branch_after_final"} {
				if _, ok := continuationProofDiagnostic(result, code); ok {
					t.Fatalf("unexpected %s diagnostic: %+v", code, result.Diagnostics)
				}
			}
		})
	}
}

func TestContinuationProofRespectsSelectionAndIdempotence(t *testing.T) {
	separator := " "
	src := []byte("@F\nREPEAT 1\n{\nREPEAT 2\n}\nREND\nREND\n")
	quiet := FormatWithOptions(src, ERB, Options{Select: []RuleID{}, ContinuationSeparator: &separator})
	if !bytes.Equal(quiet.Output, src) || len(quiet.Changes) != 0 {
		t.Fatalf("zero-rule output=%q changes=%+v", quiet.Output, quiet.Changes)
	}
	if _, ok := continuationProofDiagnostic(quiet, "nested_repeat"); !ok {
		t.Fatalf("zero-rule diagnostics=%+v", quiet.Diagnostics)
	}
	indented := FormatWithOptions(src, ERB, Options{Select: []RuleID{InstructionIndent}, ContinuationSeparator: &separator})
	want := []byte("@F\n    REPEAT 1\n{\nREPEAT 2\n}\n        REND\n    REND\n")
	if !bytes.Equal(indented.Output, want) || len(indented.Changes) == 0 {
		t.Fatalf("selected output=%q changes=%+v", indented.Output, indented.Changes)
	}
	if _, ok := continuationProofDiagnostic(indented, "nested_repeat"); !ok {
		t.Fatalf("selected diagnostics=%+v", indented.Diagnostics)
	}
	again := FormatWithOptions(indented.Output, ERB, Options{Select: []RuleID{InstructionIndent}, ContinuationSeparator: &separator})
	if !bytes.Equal(again.Output, indented.Output) || len(again.Changes) != 0 {
		t.Fatalf("not idempotent: output=%q again=%q changes=%+v", indented.Output, again.Output, again.Changes)
	}
}
