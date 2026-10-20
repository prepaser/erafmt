package format

import (
	"bytes"
	"testing"
)

func TestDialectDoesNotEnableRules(t *testing.T) {
	src := []byte("\r\n@ F\r\nINPUTS prompt,1+2\r\nPRINTFORMN { A+1 }\r\nRETURNF \\@ A?yes#no\\@\r\n")
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		r := FormatWithOptions(src, ERB, Options{Select: []RuleID{}, Dialect: dialect})
		if !bytes.Equal(r.Output, src) || len(r.Changes) != 0 || len(r.Diagnostics) != 0 {
			t.Fatalf("dialect %d enabled formatting: %+v", dialect, r)
		}
	}
}

func TestQualifiedVariableSyntax(t *testing.T) {
	src := []byte("@F\nLOCAL @ F : 0+=1+2\n++ LOCAL @ F : 1\nLOCAL @ F : 2=  raw  ; tail\n")
	opts := Options{Select: []RuleID{TokenSpacing, IndexSpacing, UnarySpacing, BinarySpacing, AssignmentLeftSpacing, AssignmentRightSpacing}}
	r := FormatWithOptions(src, ERB, opts)
	want := "@F\nLOCAL@F:0 += 1 + 2\n++LOCAL@F:1\nLOCAL@F:2 = raw  ; tail\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output %q, diagnostics %+v", r.Output, r.Diagnostics)
	}
	again := FormatWithOptions(r.Output, ERB, opts)
	if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
		t.Fatalf("not idempotent: %+v", again)
	}
}

func TestBareFormAtomBoundaries(t *testing.T) {
	src := []byte("@F\nCALL F(\\@A?x,;(#y)\\@, 1+2) ; keep  \nS'=\\@B?x=;( #y\\@+\"z\"\n")
	opts := Options{Select: []RuleID{BinarySpacing, DelimiterSpacing, CommaSpacing, FormConditionSpacing}}
	r := FormatWithOptions(src, ERB, opts)
	want := "@F\nCALL F(\\@ A ?x,;(#y)\\@, 1 + 2) ; keep  \nS'=\\@ B ?x=;( #y\\@ + \"z\"\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output %q, diagnostics %+v", r.Output, r.Diagnostics)
	}
}

func TestEMEEOnlyInstructionsAreDialectGated(t *testing.T) {
	src := []byte("@F\nCALLSHARP F\nFORCE_QUIT\nFORCE_QUIT_AND_RESTART\nQUIT_AND_RESTART\nSKIPLOG 1+2\nTRYCALLF F\nTRYCALLFORMF F\n")
	options := Options{Select: []RuleID{FunctionIndent, InstructionIndent, CommandSpacing, BinarySpacing}, Dialect: CommonDialect}
	for _, dialect := range []Dialect{CommonDialect, Emuera1824} {
		options.Dialect = dialect
		r := FormatWithOptions(src, ERB, options)
		if string(r.Output) != string(src) || len(r.Diagnostics) != 7 || len(r.Changes) != 0 {
			t.Fatalf("dialect=%v output=%q diagnostics=%+v changes=%+v", dialect, r.Output, r.Diagnostics, r.Changes)
		}
	}
	options.Dialect = EMEE
	r := FormatWithOptions(src, ERB, options)
	want := "@F\n    CALLSHARP F\n    FORCE_QUIT\n    FORCE_QUIT_AND_RESTART\n    QUIT_AND_RESTART\n    SKIPLOG 1 + 2\n    TRYCALLF F\n    TRYCALLFORMF F\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("EMEE output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestUnavailableInstructionNamesRemainAssignable(t *testing.T) {
	src := []byte("@F\nCALLSHARP = 1+2\nSKIPLOG = 3+4\nTRYCALLF = 5+6\ncallsharp =  7+8\n")
	options := Options{Select: []RuleID{FunctionIndent, InstructionIndent, CommandSpacing, BinarySpacing, AssignmentLeftSpacing, AssignmentRightSpacing}}
	for _, dialect := range []Dialect{CommonDialect, Emuera1824} {
		options.Dialect = dialect
		r := FormatWithOptions(src, ERB, options)
		want := "@F\n    CALLSHARP = 1+2\n    SKIPLOG = 3+4\n    TRYCALLF = 5+6\n    callsharp = 7+8\n"
		if string(r.Output) != want || len(r.Diagnostics) != 0 {
			t.Fatalf("dialect=%v output=%q diagnostics=%+v", dialect, r.Output, r.Diagnostics)
		}
	}
	options.Dialect = EMEE
	r := FormatWithOptions(src, ERB, options)
	want := "@F\nCALLSHARP = 1+2\nSKIPLOG = 3+4\nTRYCALLF = 5+6\ncallsharp =  7+8\n"
	if string(r.Output) != want || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_structure" {
		t.Fatalf("EMEE output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestOutputLogIsNoArgOutsideEMEE(t *testing.T) {
	src := []byte("@F\nOUTPUTLOG\nOUTPUTLOG \"file\",1+2\n")
	for _, dialect := range []Dialect{CommonDialect, Emuera1824} {
		r := FormatWithOptions(src, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing}, Dialect: dialect})
		want := "@F\n    OUTPUTLOG\n    OUTPUTLOG \"file\",1+2\n"
		if string(r.Output) != want || len(r.Diagnostics) != 0 {
			t.Fatalf("dialect=%v output=%q diagnostics=%+v", dialect, r.Output, r.Diagnostics)
		}
	}
	emee := FormatWithOptions(src, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing, CommaSpacing}, Dialect: EMEE})
	want := "@F\n    OUTPUTLOG\n    OUTPUTLOG \"file\", 1 + 2\n"
	if string(emee.Output) != want || len(emee.Diagnostics) != 0 {
		t.Fatalf("EMEE output=%q diagnostics=%+v", emee.Output, emee.Diagnostics)
	}
}
