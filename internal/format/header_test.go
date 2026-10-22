package format

import (
	"bytes"

	"testing"
)

func TestBracketFunctionHeaderPreservesWholeFile(t *testing.T) {
	source := []byte("@GOOD\nRETURN 1+2\n@F[1+2]\nRETURN 3+4\n")
	for _, options := range []Options{{}, {Select: []RuleID{}}} {
		result := FormatWithOptions(source, ERB, options)
		if !bytes.Equal(result.Output, source) {
			t.Fatalf("options=%+v output=%q, want unchanged source", options, result.Output)
		}
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Severity != Error || result.Diagnostics[0].Code != "invalid_header" {
			t.Fatalf("options=%+v diagnostics=%+v, want one invalid_header error", options, result.Diagnostics)
		}
	}
}
