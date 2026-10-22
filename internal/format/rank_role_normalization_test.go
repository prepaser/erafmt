package format

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func rankRoleOptions(dialect erabasic.Dialect) Options {
	return Options{Dialect: Dialect(dialect), Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing, FormPadding}}
}

func TestRankRoleNormalizationRejectsProvenInvalidBackingRoles(t *testing.T) {
	cases := []string{
		"ARRAYREMOVE DA,1,2",
		"ARRAYREMOVE CDFLAG:0:0:0,1,2",
		"ARRAYSHIFT DA:0:0,1,0",
		"ARRAYSORT (DA),FORWARD",
		"CVARSET CDFLAG:0:0:0,0,1",
		"CVARSET (CDFLAG),0,1",
		"FOR ABL:0,0,1\nNEXT",
		`FOR ABL:("key"),0,1` + "\nNEXT",
		"FOR (ABL),0,1\nNEXT",
		"SORTCHARA DA:0:0",
		"SORTCHARA (DA)",
	}
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		for _, body := range cases {
			src := []byte("@F\n" + body + "\n")
			r := FormatWithOptions(src, ERB, rankRoleOptions(dialect))
			if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
				t.Fatalf("dialect=%v body=%q output=%q diagnostics=%+v", dialect, body, r.Output, r.Diagnostics)
			}
		}
	}
}

func TestRankRoleNormalizationPreservesValidIndexedBackingRoles(t *testing.T) {
	cases := []string{"ARRAYREMOVE A:0,1,2", "ARRAYSHIFT A:0,1,0", "ARRAYSORT (A),FORWARD", "CVARSET CFLAG:0,0,1", "FOR A:0,0,1\nNEXT", "SORTCHARA CFLAG:0", `SORTCHARA CFLAG:"key"`, `SORTCHARA CFLAG:("key"+"suffix")`}
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		for _, body := range cases {
			r := FormatWithOptions([]byte("@F\n"+body+"\n"), ERB, rankRoleOptions(dialect))
			if len(r.Diagnostics) != 0 {
				t.Fatalf("dialect=%v body=%q output=%q diagnostics=%+v", dialect, body, r.Output, r.Diagnostics)
			}
		}
	}
}
