package format

import (
	"bytes"
	"testing"
)

func TestFormRejectsEmptyFirstField(t *testing.T) {
	rules := allFormRules(t)
	for _, input := range []string{
		`{,}`,
		`{,2}`,
		`{,2,RIGHT}`,
		`{,,RIGHT}`,
		`%,%`,
		`%,2%`,
		`%,2,RIGHT%`,
		`%%`,
	} {
		edits, err := formEdits([]byte(input), rules, formLine)
		if err == nil {
			t.Fatalf("empty FORM operand %q was accepted", input)
		}
		if got := string(applyEdits([]byte(input), edits)); got != input {
			t.Fatalf("invalid FORM operand %q changed to %q", input, got)
		}
	}
}

func TestPrintFormsFormatsOnlySafeLiteralBodies(t *testing.T) {
	options := Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing, FormPadding}, Dialect: EMEE}
	input := []byte("@F\n" +
		"PRINTFORMS \"x { 1+2 }\"\n" +
		"PRINTFORMS CSTR+\"x { 1+2 }\"\n" +
		"PRINTFORMS \"x \\{ 1+2 \\}\"\n" +
		"PRINTFORMS @\"x { 1+2 }\"\n" +
		"PRINTFORMS \"x {,2}\"\n")
	want := []byte("@F\n" +
		"    PRINTFORMS \"x {1 + 2}\"\n" +
		"    PRINTFORMS CSTR + \"x { 1+2 }\"\n" +
		"    PRINTFORMS \"x \\{ 1+2 \\}\"\n" +
		"    PRINTFORMS @\"x {1 + 2}\"\n" +
		"    PRINTFORMS \"x {,2}\"\n")
	r := FormatWithOptions(input, ERB, options)
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 1 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
	if got := FormatWithOptions(r.Output, ERB, options); !bytes.Equal(got.Output, r.Output) {
		t.Fatalf("not idempotent: %q", got.Output)
	}
}

func TestPrintFormsNFormatsValidatedEMEELiteralAndRollsBackOverflow(t *testing.T) {
	options := Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing, FormPadding}, Dialect: EMEE}
	input := []byte("@F\nPRINTFORMSN \"x { 1+2 }\"\nPRINTFORMSN \"x { 1+2 }\", EXTRA\n")
	want := []byte("@F\n    PRINTFORMSN \"x {1 + 2}\"\n    PRINTFORMSN \"x { 1+2 }\", EXTRA\n")
	r := FormatWithOptions(input, ERB, options)
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 1 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
	common := FormatWithOptions([]byte("@F\nPRINTFORMSN \"x { 1+2 }\"\n"), ERB, Options{Select: options.Select, Dialect: CommonDialect})
	if bytes.Contains(common.Output, []byte("{1 + 2}")) || len(common.Diagnostics) != 1 {
		t.Fatalf("common PRINTFORMSN was accepted: output=%q diagnostics=%+v", common.Output, common.Diagnostics)
	}
}

func TestPrintFormsLiteralGuards(t *testing.T) {
	options := Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing, FormPadding}, Dialect: EMEE}
	input := []byte("@F\n" +
		"PRINTFORMS \"x { [[alias]] }\"\n" +
		"PRINTFORMS \"x { A ;!; B }\"\n" +
		"DEBUGPRINTFORM \"x { 1+2 }\"\n")
	want := []byte("@F\n" +
		"    PRINTFORMS \"x { [[alias]] }\"\n" +
		"    PRINTFORMS \"x { A ;!; B }\"\n" +
		"    DEBUGPRINTFORM \"x { 1+2 }\"\n")
	r := FormatWithOptions(input, ERB, options)
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("guard output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
	noExpressionRules := FormatWithOptions([]byte("@F\nPRINTFORMS \"x { 1+2 }\"\n"), ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent}, Dialect: EMEE})
	if !bytes.Equal(noExpressionRules.Output, []byte("@F\n    PRINTFORMS \"x { 1+2 }\"\n")) || len(noExpressionRules.Diagnostics) != 0 {
		t.Fatalf("no-expression-rule output=%q diagnostics=%+v", noExpressionRules.Output, noExpressionRules.Diagnostics)
	}
}
