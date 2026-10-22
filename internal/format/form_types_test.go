package format

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestFormTypesPreserveInvalidIslands(t *testing.T) {
	rules := allFormRules(t)
	for _, island := range []string{`{ "x" }`, `{CSTR}`, `{1,"bad"}`, `% 1 %`, `%"x","bad"%`, `\@"x"?left#right\@`, `{=a}`, `{ * UNKNOWN }`, `% = UNKNOWN %`, `\@,?\@`, `\@1,2?yes#no\@`} {
		input := []byte(island + ` { 1+2 }`)
		edits, err := formEdits(input, rules, erabasic.FormLine)
		want := []byte(island + ` {1 + 2}`)
		if err == nil || !bytes.Equal(applyEdits(input, edits), want) {
			t.Fatalf("island=%q output=%q error=%v", island, applyEdits(input, edits), err)
		}
	}
}

func TestFormTypesKeepUnknownMacroRoles(t *testing.T) {
	rules := allFormRules(t)
	for _, input := range []string{
		`{1,2,RIGHT}`,
		`%"x",1%`,
		`{RESULT}`,
		`%CSTR%`,
		`\@1?yes#no\@`,
		`{CSTR:EFAUDIT_COMPARE}`,
		`%RESULT:EFAUDIT_TERNARY%`,
		`{1,CSTR:EFAUDIT_COMPARE}`,
		`%"x",CSTR:EFAUDIT_COMPARE%`,
		`\@CSTR:EFAUDIT_COMPARE?yes#no\@`,
		`{UNKNOWN,"unresolved"}`,
		`%UNKNOWN,"unresolved"%`,
		`{F(),"unresolved"}`,
		`%F(),"unresolved"%`,
		`{+UNKNOWN}`,
		`{-UNKNOWN}`,
		`{!UNKNOWN}`,
		`{~UNKNOWN}`,
		`{++UNKNOWN}`,
		`{UNKNOWN=1}`,
	} {
		edits, err := formEdits([]byte(input), rules, erabasic.FormLine)
		if err != nil {
			t.Fatalf("%q: %v", input, err)
		}
		output := applyEdits([]byte(input), edits)
		again, err := formEdits(output, rules, erabasic.FormLine)
		if err != nil || len(again) != 0 {
			t.Fatalf("unstable %q: edits=%+v error=%v", output, again, err)
		}
	}
}

func TestDebugFormTypesDoNotRejectInactivePayloads(t *testing.T) {
	for _, command := range []string{"DEBUGPRINTFORM", "DEBUGPRINTFORML"} {
		r := FormatWithOptions([]byte("@F\n"+command+` { "x" } % 1 % \@"x"?left#right\@`+"\n"), ERB, Options{})
		if len(r.Diagnostics) != 0 {
			t.Fatalf("%s: %+v", command, r.Diagnostics)
		}
	}
	r := FormatWithOptions([]byte("@F\nPRINTFORM { \"x\" }\n"), ERB, Options{Select: []RuleID{}})
	if len(r.Diagnostics) != 0 || len(r.Changes) != 0 {
		t.Fatalf("select-none: %+v", r)
	}
}
