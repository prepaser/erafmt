package format

import (
	"bytes"

	"testing"
)

func TestBitCommandLiteralAndOmissionGuards(t *testing.T) {
	bad := []string{
		"SETBIT",
		"SETBIT ,1",
		"SETBIT RESULT,,1",
		"CLEARBIT RESULT, ,1",
		"SETBIT 1,2",
		"SETBIT (-1),2",
		"SETBIT \"RESULT\",1",
		"SETBIT RESULT,\"1\"",
		"SETBIT RESULT,@\"1\"",
		"INVERTBIT RESULT,64",
		"SETBIT RESULT,-1",
		"SETBIT RESULT,0x40",
		"SETBIT RESULT,0b1000000",
		"SETBIT RESULT,1e2",
		"SETBIT RESULT,1p6",
		"SETBIT RESULT,0xFFFFFFFFFFFFFFFF",
	}
	for _, line := range bad {
		input := []byte("@F\n" + line + "\n")
		r := FormatWithOptions(input, ERB, Options{Select: []RuleID{BinarySpacing, CommaSpacing}})
		if !bytes.Equal(r.Output, input) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q output=%q diagnostics=%v", line, r.Output, r.Diagnostics)
		}
		quiet := FormatWithOptions(input, ERB, Options{Select: []RuleID{}})
		if !bytes.Equal(quiet.Output, input) || len(quiet.Diagnostics) != 0 || len(quiet.Changes) != 0 {
			t.Fatalf("select-none %q: %+v", line, quiet)
		}
	}
}

func TestBitCommandsPreserveMacroAndDynamicFields(t *testing.T) {
	for _, line := range []string{
		"SETBIT EFAUDIT_BITS",
		"SETBIT EFAUDIT_BITS,",
		"SETBIT RESULT",
		"SETBIT RESULT,",
		"SETBIT RESULT,(INDEX+1)",
		"SETBIT (RESULT),0x3f,",
		"SETBIT RESULT,0b111111",
		"SETBIT RESULT,0x+3F",
		"SETBIT RESULT,1e-100",
		"SETBIT RESULT,10p-2",
		"SETBIT RESULT,1e4294967296",
		"SETBIT RESULT,1e4294967295",
		"SETBIT RESULT,-0xFFFFFFFFFFFFFFFF",
		"SETBIT RESULT,(-1e-100)",
	} {
		input := []byte("@F\n" + line + "\n")
		opts := Options{Select: []RuleID{BinarySpacing, CommaSpacing, DelimiterSpacing}}
		r := FormatWithOptions(input, ERB, opts)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("%q diagnostics=%v", line, r.Diagnostics)
		}
		again := FormatWithOptions(r.Output, ERB, opts)
		if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 || len(again.Diagnostics) != 0 {
			t.Fatalf("%q is not idempotent: %+v", line, again)
		}
	}
}

func TestBitCommandRuleAndFullwidthOwnership(t *testing.T) {
	input := []byte("@F\nSETBIT　RESULT　,　INDEX+1\n")
	r := FormatWithOptions(input, ERB, Options{AllowFullwidthSpace: true, Select: []RuleID{BinarySpacing}})
	if string(r.Output) != "@F\nSETBIT　RESULT　,　INDEX + 1\n" || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%v", r.Output, r.Diagnostics)
	}
	for _, change := range r.Changes {
		if change.Rule != BinarySpacing {
			t.Fatalf("unexpected rule %v", change)
		}
	}
	input = []byte("@F\nSETBIT  RESULT,64\n")
	r = FormatWithOptions(input, ERB, Options{Select: []RuleID{CommandSpacing}})
	if string(r.Output) != "@F\nSETBIT RESULT,64\n" || len(r.Diagnostics) != 0 {
		t.Fatalf("S001 output=%q diagnostics=%v", r.Output, r.Diagnostics)
	}
}

func TestBitCommandExpressionErrorColumn(t *testing.T) {
	r := FormatWithOptions([]byte("@F\nRETURN 0\nSETBIT αβ,1+(\n"), ERB, Options{Select: []RuleID{BinarySpacing}})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Line != 3 || r.Diagnostics[0].Column != 14 {
		t.Fatalf("diagnostics=%v", r.Diagnostics)
	}
}

func TestBitCommandDestinationShapeGuardsKnownExpressions(t *testing.T) {
	options := Options{Select: []RuleID{BinarySpacing, CommaSpacing, DelimiterSpacing}}
	for _, line := range []string{"SETBIT RESULT+1,1", "CLEARBIT RESULT*1,1", "INVERTBIT STRLENS(\"x\"),1"} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, options)
		if string(r.Output) != "@F\n"+line+"\n" || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q output=%q diagnostics=%v", line, r.Output, r.Diagnostics)
		}
	}
	for _, line := range []string{"SETBIT +RESULT,1", "SETBIT (RESULT),1", "SETBIT RESULT:1,1", "SETBIT F(1),1"} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, options)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("%q diagnostics=%v", line, r.Diagnostics)
		}
	}
}

func TestBitCommandsRejectReadonlyDestinations(t *testing.T) {
	options := Options{Select: []RuleID{BinarySpacing, CommaSpacing, DelimiterSpacing}}
	for _, line := range []string{"SETBIT ITEMPRICE,1", "CLEARBIT ABLNAME:0,1", "INVERTBIT ;!; ITEMPRICE,1"} {
		src := []byte("@F\n" + line + "\n")
		r := FormatWithOptions(src, ERB, options)
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q output=%q diagnostics=%+v", line, r.Output, r.Diagnostics)
		}
	}
}

func TestReuseLastLineUsesNullableFormGrammar(t *testing.T) {
	input := []byte("@F\nREUSELASTLINE\nREUSELASTLINE  text { A+1 } ; literal  \nREUSELASTLINE;{B+2}\n")
	opts := Options{Select: []RuleID{BinarySpacing, FormPadding, CommentPrefixSpacing, CodeTrailingWhitespace}}
	r := FormatWithOptions(input, ERB, opts)
	want := "@F\nREUSELASTLINE\nREUSELASTLINE  text {A + 1} ; literal  \nREUSELASTLINE;{B + 2}\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%v", r.Output, r.Diagnostics)
	}
}
