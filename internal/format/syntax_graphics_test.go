package format

import (
	"bytes"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestGraphicsBaseForms(t *testing.T) {
	src := []byte("@F\nPRINT_IMG  \"name\"\nPRINT_IMG IMAGE:0+\"x\"\nPRINT_RECT 1+2,2, 3,4\nPRINT_SPACE ( 1+2 )\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{InstructionIndent, CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing}})
	want := "@F\n    PRINT_IMG \"name\"\n    PRINT_IMG IMAGE:0 + \"x\"\n    PRINT_RECT 1 + 2, 2, 3, 4\n    PRINT_SPACE (1 + 2)\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestGraphicsPixelSpacingOnly(t *testing.T) {
	src := []byte("@F\nPRINT_RECT  10\tpx, (1+2)PX, A[0]pX, 20 px\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{PixelUnitSpacing, CommandSpacing}, Dialect: EMEE})
	want := "@F\nPRINT_RECT 10 px, (1+2) PX, A[0] pX, 20 px\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	if again := FormatWithOptions(r.Output, ERB, Options{Select: []RuleID{PixelUnitSpacing, CommandSpacing}, Dialect: EMEE}); !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
		t.Fatalf("not idempotent: %+v", again)
	}
}

func TestGraphicsPixelExponentBoundaryOwnership(t *testing.T) {
	src := []byte("PRINT_RECT 1p2px, 1e2PX, 3, 4\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{PixelUnitSpacing}, Dialect: EMEE})
	if string(r.Output) != "PRINT_RECT 1p2 px, 1e2 PX, 3, 4\n" || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	r = FormatWithOptions(src, ERB, Options{Select: []RuleID{TokenSpacing}, Dialect: EMEE})
	if !bytes.Equal(r.Output, src) || len(r.Changes) != 0 || len(r.Diagnostics) != 0 {
		t.Fatalf("S009 edited a unit boundary: %+v", r)
	}
}

func TestGraphicsExpressionRulesAndUnits(t *testing.T) {
	src := []byte("@F\nPRINT_RECT F(1+2),2,3,4\nPRINT_SPACE 10\tpx\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{BinarySpacing, DelimiterSpacing, CommaSpacing, PixelUnitSpacing}, Dialect: EMEE})
	want := "@F\nPRINT_RECT F(1 + 2), 2, 3, 4\nPRINT_SPACE 10 px\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestGraphicsRejectsKnownAdjacentOperands(t *testing.T) {
	source := []byte("PRINT_SPACE 00A\nPRINT_SPACE 1@\"x\"\nPRINT_SPACE A 1\nPRINT_SPACE UNKNOWN 1\nPRINT_SPACE F(1,2)\nPRINT_SPACE A:0\n")
	r := FormatWithOptions(source, ERB, Options{Select: []RuleID{BinarySpacing, TokenSpacing}, Dialect: EMEE})
	if !bytes.Equal(r.Output, source) || len(r.Diagnostics) != 3 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Code != "unsupported_syntax" || diagnostic.Severity != Unsupported {
			t.Fatalf("diagnostic=%+v", diagnostic)
		}
	}
}

func TestGraphicsRejectsAdjacentAfterClosedBuiltin(t *testing.T) {
	source := []byte("PRINT_SPACE ABS(1)2\nPRINT_SPACE UNKNOWN(1)2\n")
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		r := FormatWithOptions(source, ERB, Options{Select: []RuleID{BinarySpacing}, Dialect: Dialect(dialect)})
		if !bytes.Equal(r.Output, source) || len(r.Diagnostics) != 1 {
			t.Fatalf("dialect=%d output=%q diagnostics=%+v", dialect, r.Output, r.Diagnostics)
		}
		if r.Diagnostics[0].Code != "unsupported_syntax" || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("dialect=%d diagnostic=%+v", dialect, r.Diagnostics[0])
		}
	}
}

func TestGraphicsRejectsProvenClosedOperatorsAndQualifiers(t *testing.T) {
	source := []byte("PRINT_SPACE RESULT@FUNC\nPRINT_SPACE ABS(1)++\nPRINT_RECT ABS(1)+*1,2,3,4\nPRINT_RECT ABS(1)+?2#3,2,3,4\n")
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		r := FormatWithOptions(source, ERB, Options{Select: []RuleID{BinarySpacing}, Dialect: Dialect(dialect)})
		if !bytes.Equal(r.Output, source) || len(r.Diagnostics) != 4 {
			t.Fatalf("dialect=%d output=%q diagnostics=%+v", dialect, r.Output, r.Diagnostics)
		}
	}
	valid := []byte("PRINT_SPACE ABS(1)+-2\nPRINT_SPACE ABS(1)?2#3\nPRINT_SPACE (RESULT)++\n")
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		r := FormatWithOptions(valid, ERB, Options{Select: []RuleID{}, Dialect: Dialect(dialect)})
		if !bytes.Equal(r.Output, valid) || len(r.Diagnostics) != 0 {
			t.Fatalf("dialect=%d output=%q diagnostics=%+v", dialect, r.Output, r.Diagnostics)
		}
	}
}

func TestGraphicsCallArgumentsMayContainStrings(t *testing.T) {
	src := []byte("@F\nPRINT_SPACE STRLENS(\"abc\")\nPRINT_RECT STRLENS(@\"x {1+2}\")\tpx, 1, 2, 3\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{PixelUnitSpacing}, Dialect: EMEE})
	want := "@F\nPRINT_SPACE STRLENS(\"abc\")\nPRINT_RECT STRLENS(@\"x {1+2}\") px, 1, 2, 3\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824} {
		r := FormatWithOptions([]byte("PRINT_SPACE STRLENS(\"abc\")\n"), ERB, Options{Select: []RuleID{CommandSpacing}, Dialect: Dialect(dialect)})
		if string(r.Output) != "PRINT_SPACE STRLENS(\"abc\")\n" || len(r.Diagnostics) != 0 {
			t.Fatalf("dialect=%d output=%q diagnostics=%+v", dialect, r.Output, r.Diagnostics)
		}
	}
}

func TestGraphicsImageCallArgumentsDeferReturnType(t *testing.T) {
	src := []byte("PRINT_IMG \"name\", 1, STRLENS(\"abc\")\nPRINT_IMG \"name\", 1, STRLENS(@\"x {1+2}\")\tpx\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{PixelUnitSpacing}, Dialect: EMEE})
	want := string(src)
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestGraphicsPixelUnitPreservesAdjacentMarkerGap(t *testing.T) {
	for _, gap := range []string{"", " ", "    ", "\t", " \t "} {
		for _, source := range []string{"PRINT_SPACE 1;!;" + gap + "px\n", "PRINT_RECT 1;!;" + gap + "px,2\tpx,3 px,4 px\n", "PRINT_IMG \"x\",1;!;" + gap + "px\n"} {
			r := FormatWithOptions([]byte(source), ERB, Options{Dialect: EMEE, Select: []RuleID{PixelUnitSpacing}})
			want := strings.ReplaceAll(source, "2\tpx", "2 px")
			if len(r.Diagnostics) != 0 || string(r.Output) != want {
				t.Fatalf("source=%q output=%q diagnostics=%+v", source, r.Output, r.Diagnostics)
			}
			if again := FormatWithOptions(r.Output, ERB, Options{Dialect: EMEE, Select: []RuleID{PixelUnitSpacing}}); !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
				t.Fatalf("not idempotent: %q", r.Output)
			}
		}
	}
	control := FormatWithOptions([]byte("PRINT_SPACE ;!; 1\tpx\n"), ERB, Options{Dialect: EMEE, Select: []RuleID{PixelUnitSpacing}})
	if string(control.Output) != "PRINT_SPACE ;!; 1 px\n" || len(control.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", control.Output, control.Diagnostics)
	}
}

func TestGraphicsTopLevelStringsRemainInvalid(t *testing.T) {
	src := []byte("PRINT_SPACE \"abc\"\nPRINT_SPACE (\"abc\")\nPRINT_SPACE F((\"abc\"))\n")
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824} {
		r := FormatWithOptions(src, ERB, Options{Select: []RuleID{PixelUnitSpacing}, Dialect: Dialect(dialect)})
		if string(r.Output) != string(src) || len(r.Diagnostics) != 0 {
			t.Fatalf("dialect=%d output=%q diagnostics=%+v", dialect, r.Output, r.Diagnostics)
		}
	}
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{PixelUnitSpacing}, Dialect: EMEE})
	if string(r.Output) != string(src) || len(r.Diagnostics) != 0 {
		t.Fatalf("emee output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestGraphicsDialectAndUnitBoundaries(t *testing.T) {
	src := []byte("@F\nPRINT_RECT  10\tpx\nPRINT_RECT  10px\n")
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824} {
		r := FormatWithOptions(src, ERB, Options{Select: []RuleID{PixelUnitSpacing, CommandSpacing}, Dialect: Dialect(dialect)})
		want := "@F\nPRINT_RECT 10\tpx\nPRINT_RECT 10px\n"
		if string(r.Output) != want || len(r.Diagnostics) != 0 {
			t.Fatalf("dialect=%d output=%q diagnostics=%+v", dialect, r.Output, r.Diagnostics)
		}
	}
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{PixelUnitSpacing, CommandSpacing}, Dialect: EMEE})
	want := "@F\nPRINT_RECT 10 px\nPRINT_RECT 10px\n"
	if string(r.Output) != want || len(r.Diagnostics) != 1 {
		t.Fatalf("EMEE output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestGraphicsImageTypesAreConservative(t *testing.T) {
	src := []byte("@F\nPRINT_IMG \"name\", 1, 2\nPRINT_IMG \"name\", 10\tpx\nPRINT_IMG NAME, 1\nPRINT_IMG \"name\", UNKNOWN, 1\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{PixelUnitSpacing, CommandSpacing, CommaSpacing}, Dialect: EMEE})
	want := "@F\nPRINT_IMG \"name\", 1, 2\nPRINT_IMG \"name\", 10 px\nPRINT_IMG NAME, 1\nPRINT_IMG \"name\", UNKNOWN, 1\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestGraphicsFormStringAndComments(t *testing.T) {
	src := []byte("@F\nPRINT_IMG @\"x { A+1 }\" ; keep, comma\nPRINT_SPACE 1 ; keep  \n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{InstructionIndent, BinarySpacing, FormPadding, InlineCommentSpacing, CommentTrailingWhitespace}})
	want := "@F\n    PRINT_IMG @\"x {A + 1}\" ; keep, comma\n    PRINT_SPACE 1 ; keep\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestGraphicsFormWidthAdvisoryReachesResult(t *testing.T) {
	for _, test := range []struct {
		name    string
		source  string
		dialect erabasic.Dialect
	}{{"image", "PRINT_IMG @\"image {1,-1}\"\n", erabasic.CommonDialect}, {"space", "PRINT_SPACE STRLENS(@\"x {1,-1}\")\n", erabasic.EMEE}} {
		t.Run(test.name, func(t *testing.T) {
			r := FormatWithOptions([]byte(test.source), ERB, Options{Dialect: Dialect(test.dialect), Select: []RuleID{FormPadding}})
			if !bytes.Equal(r.Output, []byte(test.source)) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Warning || r.Diagnostics[0].Code != "suspicious_form_width" {
				t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
			}
		})
	}
}

func TestGraphicsPixelUnitRejectsIncompleteExpression(t *testing.T) {
	src := []byte("PRINT_RECT 1+px\nPRINT_RECT A*px\nPRINT_RECT 1?px\nPRINT_RECT A!px\nPRINT_RECT A=px\nPRINT_RECT F(+,A)px\nPRINT_RECT F(+)px\nPRINT_RECT (,A)px\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{PixelUnitSpacing}, Dialect: EMEE})
	if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 8 {
		t.Fatalf("incomplete pixel expressions output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestGraphicsPixelUnitAcceptsPostfixAndUnaryExpressions(t *testing.T) {
	src := []byte("PRINT_RECT A++px\nPRINT_RECT --A px\nPRINT_RECT (1+2)px\nPRINT_RECT F(A?B#C)px\nPRINT_RECT A:0 px\nPRINT_RECT LOCAL@FUNC:0 px\nPRINT_RECT F()px\nPRINT_RECT CLIENTWIDTH()px\nPRINT_RECT F(,A)px\nPRINT_RECT F(A,,B)px\nPRINT_RECT F(A,)px\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{PixelUnitSpacing}, Dialect: EMEE})
	want := []byte("PRINT_RECT A++ px\nPRINT_RECT --A px\nPRINT_RECT (1+2) px\nPRINT_RECT F(A?B#C) px\nPRINT_RECT A:0 px\nPRINT_RECT LOCAL@FUNC:0 px\nPRINT_RECT F() px\nPRINT_RECT CLIENTWIDTH() px\nPRINT_RECT F(,A) px\nPRINT_RECT F(A,,B) px\nPRINT_RECT F(A,) px\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("valid pixel expressions output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestGraphicsConstantExpressionValidation(t *testing.T) {
	src := []byte("PRINT_SPACE 1+\nPRINT_SPACE 1?(2#3)\nPRINT_SPACE (1?2)#3\nPRINT_SPACE 1?(2?3#4)#5\nPRINT_SPACE A++\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{BinarySpacing, TernarySpacing}, Dialect: EMEE})
	want := []byte("PRINT_SPACE 1+\nPRINT_SPACE 1?(2#3)\nPRINT_SPACE (1?2)#3\nPRINT_SPACE 1 ? (2 ? 3 # 4) # 5\nPRINT_SPACE A++\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 3 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestGraphicsMacroExpressionsRemainOpaque(t *testing.T) {
	src := []byte("PRINT_SPACE 1 EFAUDIT_PLUS 2\nPRINT_SPACE 1?EFAUDIT_A?(EFAUDIT_B#5)#6\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{BinarySpacing, TernarySpacing}, Dialect: EMEE})
	want := []byte("PRINT_SPACE 1 EFAUDIT_PLUS 2\nPRINT_SPACE 1 ? EFAUDIT_A ? (EFAUDIT_B # 5) # 6\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestGraphicsInvalidArgumentsKeepCommandGap(t *testing.T) {
	src := []byte("PRINT_SPACE\t1+\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{CommandSpacing}, Dialect: EMEE})
	if string(r.Output) != "PRINT_SPACE 1+\n" || len(r.Diagnostics) != 0 || len(r.Changes) != 1 || r.Changes[0].Rule != CommandSpacing {
		t.Fatalf("output=%q diagnostics=%+v changes=%+v", r.Output, r.Diagnostics, r.Changes)
	}
}
