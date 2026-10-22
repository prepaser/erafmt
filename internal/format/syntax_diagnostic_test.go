package format

import (
	"bytes"

	"testing"
)

func TestUnsupportedCommandKeepsItsIndentAndContinues(t *testing.T) {
	input := []byte("@F\n\tFUTURECOMMAND A+B\nRETURN 1+2\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent, BinarySpacing}})
	want := []byte("@F\n\tFUTURECOMMAND A+B\n    RETURN 1 + 2\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 1 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
	if r.Diagnostics[0].Code != "unsupported_command" || r.Diagnostics[0].Line != 2 {
		t.Fatalf("diagnostic=%+v", r.Diagnostics)
	}
}

func TestUnsupportedCommandRollbackRetainsPreviousEdits(t *testing.T) {
	input := []byte("@F\nRETURN 1+2\n@G\nSIF 1\n\tFUTURECOMMAND A+B\nRETURN 3+4\n")
	options := Options{Select: []RuleID{InstructionIndent, SIFIndent, BinarySpacing}}
	r := FormatWithOptions(input, ERB, options)
	want := []byte("@F\n    RETURN 1 + 2\n@G\n    SIF 1\n\tFUTURECOMMAND A+B\n    RETURN 3 + 4\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_command" {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestUnsupportedCommandDiagnosticRequiresSelectedRule(t *testing.T) {
	input := []byte("@F\n\tFUTURECOMMAND A+B\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{}})
	if !bytes.Equal(r.Output, input) || len(r.Diagnostics) != 0 || len(r.Changes) != 0 {
		t.Fatalf("result=%+v", r)
	}
}

func TestWrongDialectCommandIsReported(t *testing.T) {
	input := []byte("@F\n\tPRINTVN A+B\nRETURN 0\n")
	common := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent, BinarySpacing}, Dialect: CommonDialect})
	commonWant := []byte("@F\n\tPRINTVN A+B\n    RETURN 0\n")
	if !bytes.Equal(common.Output, commonWant) || len(common.Diagnostics) != 1 || common.Diagnostics[0].Code != "unsupported_command" {
		t.Fatalf("common=%+v", common)
	}
	emee := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent, BinarySpacing}, Dialect: EMEE})
	want := []byte("@F\n    PRINTVN A + B\n    RETURN 0\n")
	if !bytes.Equal(emee.Output, want) || len(emee.Diagnostics) != 0 {
		t.Fatalf("emee=%+v want=%q", emee, want)
	}
}

func TestOpaquePrintAndDataPayloadsHaveNoCoverageDiagnostic(t *testing.T) {
	input := []byte("@F\nPRINTDATA\nDATA  raw=payload++\nENDDATA\nPRINTLC  [0] text\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent, BinarySpacing}})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", r.Diagnostics)
	}
	if !bytes.Contains(r.Output, []byte("DATA  raw=payload++")) || !bytes.Contains(r.Output, []byte("PRINTLC  [0] text")) {
		t.Fatalf("opaque payload changed: %q", r.Output)
	}
}

func TestReturnFormRequiredArgumentBoundaries(t *testing.T) {
	tests := []struct {
		name string
		tail string
		bad  bool
	}{
		{name: "bare", tail: "", bad: true},
		{name: "space", tail: " ", bad: true},
		{name: "tab", tail: "\t", bad: true},
		{name: "two spaces", tail: "  "},
		{name: "two tabs", tail: "\t\t"},
		{name: "empty semicolon", tail: ";", bad: true},
		{name: "semicolon raw", tail: ";raw"},
		{name: "space semicolon", tail: " ;"},
		{name: "space semicolon raw", tail: " ; literal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []byte("@F\nRETURNFORM" + tt.tail + "\n")
			r := FormatWithOptions(input, ERB, Options{Select: []RuleID{BinarySpacing}})
			if !bytes.Equal(r.Output, input) {
				t.Fatalf("output=%q want unchanged=%q", r.Output, input)
			}
			if tt.bad {
				if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_syntax" || r.Diagnostics[0].Message != "missing FORM argument" {
					t.Fatalf("diagnostics=%+v", r.Diagnostics)
				}
				return
			}
			if len(r.Diagnostics) != 0 {
				t.Fatalf("diagnostics=%+v", r.Diagnostics)
			}
		})
	}
}

func TestReturnFormMissingArgumentDiagnosticFollowsExpressionSelection(t *testing.T) {
	input := []byte("@F\nRETURNFORM\n")
	for _, test := range []struct {
		options Options
		want    string
	}{{options: Options{Select: []RuleID{}}, want: string(input)}, {options: Options{Select: []RuleID{InstructionIndent}}, want: "@F\n    RETURNFORM\n"}} {
		r := FormatWithOptions(input, ERB, test.options)
		if !bytes.Equal(r.Output, []byte(test.want)) || len(r.Diagnostics) != 0 {
			t.Fatalf("options=%+v output=%q diagnostics=%+v", test.options, r.Output, r.Diagnostics)
		}
	}
}
