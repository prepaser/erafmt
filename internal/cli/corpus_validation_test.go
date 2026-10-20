package cli

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestCorpusValidationAcceptsEmptyNonfatalOutput(t *testing.T) {
	tests := []struct {
		name string
		src  []byte
		want []byte
	}{
		{name: "empty"},
		{name: "all blank", src: []byte(" \t\r\n\n")},
		{name: "nonempty", src: []byte("@F\nRETURN 0"), want: []byte("@F\n    RETURN 0\n")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := format.FormatWithOptions(tt.src, format.ERB, format.Options{})
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == format.Error {
					t.Fatalf("unexpected fatal diagnostic: %+v", diagnostic)
				}
			}
			if err := validateCorpusOutput(result.Output); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(result.Output, tt.want) {
				t.Fatalf("output = %q, want %q", result.Output, tt.want)
			}
		})
	}
}

func TestCorpusValidationPreservesBlankRawPrintAndDataPayload(t *testing.T) {
	src := []byte("@F\nPRINTL  \nPRINTDATA\nDATA  \nENDDATA\n")
	result := format.FormatWithOptions(src, format.ERB, format.Options{})
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity == format.Error {
			t.Fatalf("unexpected fatal diagnostic: %+v", diagnostic)
		}
	}
	if err := validateCorpusOutput(result.Output); err != nil {
		t.Fatal(err)
	}
	if err := checkSourcePreserved(src, result.Output); err != nil {
		t.Fatalf("payload preservation: %v\noutput=%q", err, result.Output)
	}
	if !bytes.Contains(result.Output, []byte("PRINTL  \n")) || !bytes.Contains(result.Output, []byte("DATA  \n")) {
		t.Fatalf("blank payload changed: %q", result.Output)
	}
}

func TestCorpusReentryValidation(t *testing.T) {
	output := []byte("\ufeffX = 1\n")
	first := format.Result{Output: output}
	second := format.FormatWithOptions(output, format.ERB, format.Options{})
	if err := validateCorpusReentry(first, second); err == nil {
		t.Fatal("accepted fatal reentry with unchanged output")
	}
	warning := format.Diagnostic{Severity: format.Warning, Code: "invalid_loop_control", Line: 2, Column: 5}
	first.Diagnostics = []format.Diagnostic{warning}
	warning.Line, warning.Column = 1, 1
	second = format.Result{Output: output, Diagnostics: []format.Diagnostic{warning}}
	if err := validateCorpusReentry(first, second); err != nil {
		t.Fatalf("rejected shifted diagnostic: %v", err)
	}
	second.Diagnostics = append(second.Diagnostics, warning)
	if err := validateCorpusReentry(first, second); err == nil {
		t.Fatal("accepted changed diagnostic count")
	}
	second.Diagnostics = first.Diagnostics
	second.Changes = []format.Change{{Rule: format.InstructionIndent}}
	if err := validateCorpusReentry(first, second); err == nil {
		t.Fatal("accepted second-pass changes")
	}
	second.Changes = nil
	second.Output = []byte("X = 1\n")
	if err := validateCorpusReentry(first, second); err == nil {
		t.Fatal("accepted changed second-pass output")
	}
}
