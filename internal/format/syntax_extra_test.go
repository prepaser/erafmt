package format

import (
	"bytes"
	"testing"
)

func TestSyntaxExtraCommands(t *testing.T) {
	input := []byte("@TIMES\nTIMES   VALUE : IDX , +01.2500e-02 ; keep  \n@DATA\nPRINTDATAK   VAR : 0\nDATA raw\nENDDATA\n@STR\nSTRDATA   TEXT : 0\nDATA raw\nENDDATA\n@SPLIT\nSPLIT \"a\"+\"b\",\"c\", OUT : 0, COUNT : 1\n@SWAP\nSWAP A : 0,B : 1\n")
	want := []byte("@TIMES\n    TIMES VALUE:IDX, +01.2500e-02 ; keep  \n@DATA\n    PRINTDATAK VAR:0\n        DATA raw\n    ENDDATA\n@STR\n    STRDATA TEXT:0\n        DATA raw\n    ENDDATA\n@SPLIT\n    SPLIT \"a\" + \"b\", \"c\", OUT:0, COUNT:1\n@SWAP\n    SWAP A:0, B:1\n")
	options := Options{Select: []RuleID{
		FunctionIndent, InstructionIndent, CommandSpacing, BinarySpacing,
		CommaSpacing, IndexSpacing, InlineCommentSpacing, CodeTrailingWhitespace,
	}}
	r := FormatWithOptions(input, ERB, options)
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
}

func TestSyntaxExtraTimesPreservesCoefficientAndRejectsMalformed(t *testing.T) {
	valid := []byte("@F\nTIMES VALUE, 0001.2500e-02\n")
	r := FormatWithOptions(valid, ERB, Options{Select: []RuleID{CommandSpacing, CommaSpacing}})
	if !bytes.Equal(r.Output, []byte("@F\nTIMES VALUE, 0001.2500e-02\n")) || len(r.Diagnostics) != 0 {
		t.Fatalf("valid output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	for _, coefficient := range []string{"1e+2", "1..2", "1.2junk", "10px"} {
		input := []byte("@F\nTIMES VALUE, " + coefficient + "\n")
		r := FormatWithOptions(input, ERB, Options{Select: []RuleID{CommandSpacing, CommaSpacing, BinarySpacing}})
		if !bytes.Equal(r.Output, input) || len(r.Diagnostics) != 1 {
			t.Fatalf("coefficient=%q output=%q diagnostics=%+v", coefficient, r.Output, r.Diagnostics)
		}
	}
}

func TestSyntaxExtraMalformedSelectionDoesNotDiagnose(t *testing.T) {
	input := []byte("@F\nTIMES VALUE, 1e+2\nSPLIT A,B\nSWAP A,B+1\nPRINTDATA A+B\nENDDATA\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{CommandSpacing}})
	want := []byte("@F\nTIMES VALUE, 1e+2\nSPLIT A,B\nSWAP A,B+1\nPRINTDATA A+B\nENDDATA\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestSyntaxExtraBoundariesAndEmptyFields(t *testing.T) {
	input := []byte("@F\nTIMES;tail\nSPLIT;tail\nSWAP A,\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{CommandSpacing, CommaSpacing}})
	if !bytes.Equal(r.Output, input) || len(r.Diagnostics) != 1 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}
