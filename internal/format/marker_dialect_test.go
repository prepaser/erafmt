package format

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestCaretMarkerIsOrdinaryCommentOutsideEMEE(t *testing.T) {
	input := []byte("@F\n;^; IF A  \nRETURN 0\n;^; ENDIF\t\n")
	want := []byte("@F\n    ;^; IF A  \n    RETURN 0\n    ;^; ENDIF\t\n")
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824} {
		r := FormatWithOptions(input, ERB, Options{Dialect: Dialect(dialect), Select: []RuleID{FunctionIndent, InstructionIndent, CommentIndent, CommentPrefixSpacing, CommentTrailingWhitespace}})
		if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
			t.Fatalf("dialect=%v output=%q diagnostics=%+v, want %q", dialect, r.Output, r.Diagnostics, want)
		}
	}
}

func TestCaretMarkerIsStructuralInEMEE(t *testing.T) {
	input := []byte("@F\n;^; IF A\nRETURN 0\n;^; ENDIF\n")
	want := []byte("@F\n    ;^; IF A\n        RETURN 0\n    ;^; ENDIF\n")
	r := FormatWithOptions(input, ERB, Options{Dialect: EMEE, Select: []RuleID{FunctionIndent, InstructionIndent}})
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v, want %q", r.Output, r.Diagnostics, want)
	}
	again := FormatWithOptions(r.Output, ERB, Options{Dialect: EMEE, Select: []RuleID{FunctionIndent, InstructionIndent}})
	if !bytes.Equal(again.Output, r.Output) || len(again.Diagnostics) != 0 || len(again.Changes) != 0 {
		t.Fatalf("not idempotent: %+v", again)
	}
}

func TestCaretMarkerSIFPairFollowsDialect(t *testing.T) {
	input := []byte("@F\n;^; SIF A\n;^; PRINTL x\nRETURN 0\n")
	common := FormatWithOptions(input, ERB, Options{Dialect: CommonDialect, Select: []RuleID{FunctionIndent, InstructionIndent, CommentIndent}})
	commonWant := []byte("@F\n    ;^; SIF A\n    ;^; PRINTL x\n    RETURN 0\n")
	if !bytes.Equal(common.Output, commonWant) || len(common.Diagnostics) != 0 {
		t.Fatalf("common output=%q diagnostics=%+v, want %q", common.Output, common.Diagnostics, commonWant)
	}
	emee := FormatWithOptions(input, ERB, Options{Dialect: EMEE, Select: []RuleID{FunctionIndent, InstructionIndent, SIFIndent}})
	emeeWant := []byte("@F\n    ;^; SIF A\n        ;^; PRINTL x\n    RETURN 0\n")
	if !bytes.Equal(emee.Output, emeeWant) || len(emee.Diagnostics) != 0 {
		t.Fatalf("emee output=%q diagnostics=%+v, want %q", emee.Output, emee.Diagnostics, emeeWant)
	}
}
