package format

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func methodOptions(dialect erabasic.Dialect) Options {
	return Options{Dialect: Dialect(dialect), Select: []RuleID{FunctionIndent, InstructionIndent, CommandSpacing, BinarySpacing, CommaSpacing, TokenSpacing}}
}

func TestBuiltinMethodExpressionFormatting(t *testing.T) {
	src := []byte("@F\nSUBSTRING  \"raw { A+1 }\",1+2,3+4\nLOG10  1+2\nUNKNOWN10  A+1\n")
	want := []byte("@F\n    SUBSTRING \"raw { A+1 }\", 1 + 2, 3 + 4\n    LOG10 1 + 2\nUNKNOWN10  A+1\n")
	r := FormatWithOptions(src, ERB, methodOptions(erabasic.CommonDialect))
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_command" {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
	again := FormatWithOptions(r.Output, ERB, methodOptions(erabasic.CommonDialect))
	if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
		t.Fatalf("not idempotent: %+v", again)
	}
}

func TestBuiltinMethodDialectAndUnknownBoundaries(t *testing.T) {
	src := []byte("@F\nSETVAR  \"LOCAL:0\",1+2\nSUBSTRING  \"x\",1+2\nUNKNOWNMETHOD  A+1\n")
	emee := FormatWithOptions(src, ERB, methodOptions(erabasic.EMEE))
	wantEMEE := []byte("@F\n    SETVAR \"LOCAL:0\", 1 + 2\n    SUBSTRING \"x\", 1 + 2\nUNKNOWNMETHOD  A+1\n")
	if !bytes.Equal(emee.Output, wantEMEE) || len(emee.Diagnostics) != 1 || emee.Diagnostics[0].Code != "unsupported_command" {
		t.Fatalf("emee output=%q diagnostics=%+v want=%q", emee.Output, emee.Diagnostics, wantEMEE)
	}
	base := FormatWithOptions(src, ERB, methodOptions(erabasic.Emuera1824))
	wantBase := []byte("@F\nSETVAR  \"LOCAL:0\",1+2\n    SUBSTRING \"x\", 1 + 2\nUNKNOWNMETHOD  A+1\n")
	if !bytes.Equal(base.Output, wantBase) || len(base.Diagnostics) != 2 {
		t.Fatalf("base output=%q diagnostics=%+v want=%q", base.Output, base.Diagnostics, wantBase)
	}
}

func TestBuiltinMethodConsumedSemicolonAndExplicitShadow(t *testing.T) {
	src := []byte("@F\nSUBSTRING;\"x\",1+2,3+4\nPOWER RESULT,2+3,4+5\n")
	r := FormatWithOptions(src, ERB, methodOptions(erabasic.CommonDialect))
	want := []byte("@F\n    SUBSTRING;\"x\",1+2,3+4\n    POWER RESULT, 2 + 3, 4 + 5\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
	if !erabasic.ReservedBuiltinMethod("POWER") {
		t.Fatal("POWER must remain in the method identity set")
	}
}

func TestDirectSTRFORMLiteralFormatting(t *testing.T) {
	src := []byte("@F\nSTRFORM  \"x { 1+2 }\"\nSTRFORM @\"x { 1+2 }\"\nSTRFORM  CSTR\nSTRFORM  \"x { 1+2 }\" + CSTR\nSTRFORM  \"x \\\\{ 1+2 \\\\}\"\n")
	r := FormatWithOptions(src, ERB, methodOptions(erabasic.CommonDialect))
	want := []byte("@F\n    STRFORM \"x { 1 + 2 }\"\n    STRFORM @\"x { 1 + 2 }\"\n    STRFORM CSTR\n    STRFORM \"x { 1+2 }\" + CSTR\n    STRFORM \"x \\\\{ 1+2 \\\\}\"\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
}

func TestDirectSTRFORMValidationRollsBackPayload(t *testing.T) {
	for _, src := range []string{"@F\nSTRFORM \"x { 1+2 }\", 1\n", "@F\nSTRFORM 1\n"} {
		r := FormatWithOptions([]byte(src), ERB, methodOptions(erabasic.CommonDialect))
		if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_syntax" || bytes.Contains(r.Output, []byte("{1 + 2}")) {
			t.Fatalf("source=%q output=%q diagnostics=%+v", src, r.Output, r.Diagnostics)
		}
	}
}

func TestDirectSTRFORMDoesNotChangeNestedOverride(t *testing.T) {
	src := []byte("@STRFORM()\n#FUNCTIONS\nRETURNF \"override\"\nSTRFORM \"x { 1+2 }\"\n@F\nRETURN STRFORM(\"x { 1+2 }\")\n")
	r := FormatWithOptions(src, ERB, methodOptions(erabasic.CommonDialect))
	want := []byte("@STRFORM()\n#FUNCTIONS\n    RETURNF \"override\"\n    STRFORM \"x { 1 + 2 }\"\n@F\n    RETURN STRFORM(\"x { 1+2 }\")\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
}

func TestBuiltinMethodDialectInstructionOverride(t *testing.T) {
	src := []byte("OUTPUTLOG @\"log{ A+1 }\",1+2\n")
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824} {
		r := FormatWithOptions(src, ERB, Options{Dialect: Dialect(dialect)})
		if !bytes.Equal(r.Output, src) || len(r.Changes) != 0 {
			t.Fatalf("dialect %d changed an opaque instruction: %+v", dialect, r)
		}
	}
	r := FormatWithOptions(src, ERB, Options{Dialect: EMEE})
	if string(r.Output) != "OUTPUTLOG @\"log{A + 1}\", 1 + 2\n" || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}
