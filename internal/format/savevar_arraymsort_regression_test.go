package format

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestSAVEVARRejectsLocalAndCalculatedBuiltinTargets(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing}}
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		for _, target := range []string{"ARG", "ARGS", "LOCAL", "LOCALS", "ARG:0", "ARGS:0", "LOCAL:0", "LOCALS:0", "LOCAL:(1+1)", "WINDOW_TITLE"} {
			src := []byte("@F\nSAVEVAR \"file\",\"message\"," + target + "\n")
			r := FormatWithOptions(src, ERB, Options{Dialect: Dialect(dialect), Select: options.Select})
			if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
				t.Fatalf("dialect=%d target=%q output=%q diagnostics=%+v", dialect, target, r.Output, r.Diagnostics)
			}
		}
	}
}

func TestSAVEVARKeepsDeferredAndOrdinaryTargets(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing}}
	for _, target := range []string{"RESULT", "RESULTS", "GLOBAL", "RESULT:0", "LOCAL@FUNC", "LOCAL@FUNC:0", "LOCAL:INDEX"} {
		src := []byte("@F\nSAVEVAR \"file\",\"message\"," + target + "\n")
		r := FormatWithOptions(src, ERB, options)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("target=%q output=%q diagnostics=%+v", target, r.Output, r.Diagnostics)
		}
	}
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		r := FormatWithOptions([]byte("@F\nVARSET WINDOW_TITLE,\"title\"\n"), ERB, Options{Dialect: Dialect(dialect)})
		if len(r.Diagnostics) != 0 {
			t.Fatalf("dialect=%d WINDOW_TITLE assignment diagnostics=%+v output=%q", dialect, r.Diagnostics, r.Output)
		}
	}
}

func TestARRAYMSORTSecondaryRankThreeIsBaseOnlyInvalid(t *testing.T) {
	line := "@F\nARRAYMSORT RESULT,TA\n"
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.EMEE} {
		r := FormatWithOptions([]byte(line), ERB, methodOptions(dialect))
		if len(r.Diagnostics) != 0 {
			t.Fatalf("dialect=%d unexpectedly rejected rank-three secondary: %+v output=%q", dialect, r.Diagnostics, r.Output)
		}
	}
	for _, target := range []string{"TA", "TA:0", "TA:0:0:0"} {
		src := []byte("@F\nARRAYMSORT RESULT," + target + "\n")
		r := FormatWithOptions(src, ERB, methodOptions(erabasic.Emuera1824))
		if !bytes.Contains(r.Output, src[3:]) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("base dialect accepted rank-three secondary: target=%q output=%q diagnostics=%+v", target, r.Output, r.Diagnostics)
		}
	}
	for _, target := range []string{"RESULT", "DA", "DA:0:0", "SECONDARY"} {
		r := FormatWithOptions([]byte("@F\nARRAYMSORT RESULT,"+target+"\n"), ERB, methodOptions(erabasic.Emuera1824))
		if len(r.Diagnostics) != 0 {
			t.Fatalf("base dialect target=%q diagnostics=%+v", target, r.Diagnostics)
		}
	}
}
