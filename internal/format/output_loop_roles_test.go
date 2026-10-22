package format

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestSplitReadonlyDestinations(t *testing.T) {
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		options := destinationRoleOptions()
		options.Dialect = Dialect(dialect)
		for _, line := range []string{`SPLIT "a,b",",",ITEMNAME`, `SPLIT "a,b",",",RESULTS,ITEMPRICE:0`, `HTML_TAGSPLIT "<b>",ITEMNAME`, `HTML_TAGSPLIT "<b>",RESULTS,ITEMPRICE:0`} {
			src := []byte("@F\n" + line + "\n")
			r := FormatWithOptions(src, ERB, options)
			if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
				t.Fatalf("dialect=%v line=%q result=%+v", dialect, line, r)
			}
		}
		for _, line := range []string{
			`SPLIT ITEMNAME:0,",",RESULTS,RESULT`,
			`SPLIT MACRO_FIELDS,",",ITEMNAME`,
			`SPLIT "a,b",",",UNKNOWN,ITEMPRICE:0`,
			`SPLIT "a,b",",",ITEMNAME:UNKNOWN`,
			`HTML_TAGSPLIT MACRO_FIELDS,ITEMNAME`,
			`HTML_TAGSPLIT "<b>",RESULTS,RESULT`,
		} {
			r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, options)
			if len(r.Diagnostics) != 0 {
				t.Fatalf("dialect=%v line=%q result=%+v", dialect, line, r)
			}
		}
	}
}

func TestFOROmissionAndSurplus(t *testing.T) {
	for _, test := range []struct {
		line string
		code string
	}{
		{"FOR RESULT,0,2,", ""},
		{"FOR RESULT,,2", ""},
		{"FOR RESULT,0,2,,", "unsupported_syntax"},
		{"FOR RESULT,0,2,,1", "unsupported_syntax"},
		{"FOR RESULT,0,2,1,4", "ignored_arguments"},
		{"FOR RESULT,0,2,1,!!", "ignored_arguments"},
		{"FOR RESULT,0,2,1,", ""},
		{"FOR RESULT,0,2,\"x\",4", "unsupported_syntax"},
		{"FOR RESULT,0,2,1,MACRO", ""},
		{"FOR RESULT,MACRO,2,,", ""},
		{"FOR MACRO,0,2,1,4", ""},
	} {
		src := []byte("@F\n" + test.line + "\nNEXT\n")
		options := destinationRoleOptions()
		r := FormatWithOptions(src, ERB, options)
		if test.code == "" {
			if len(r.Diagnostics) != 0 {
				t.Fatalf("line=%q result=%+v", test.line, r)
			}
		} else if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != test.code || !bytes.Equal(r.Output, src) {
			t.Fatalf("line=%q result=%+v, want %s and preserved source", test.line, r, test.code)
		}
		again := FormatWithOptions(r.Output, ERB, options)
		if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
			t.Fatalf("line=%q is not idempotent: %+v", test.line, again)
		}
		options.Select = []RuleID{}
		if r = FormatWithOptions(src, ERB, options); !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 0 {
			t.Fatalf("line=%q disabled rules result=%+v", test.line, r)
		}
	}
}
