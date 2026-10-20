package format

import (
	"bytes"
	"testing"
)

func TestCommandFieldRolesPreserveValidAndUnresolvedFields(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing, FormPadding}}
	for _, line := range []string{
		`PRINTBUTTON "label","reply"`,
		`SAVEDATA 1,"message"`,
		`SAVECHARA "file","message",1,2`,
		`TINPUT 1,2,0,"timeout"`,
		`TINPUTS 1,"default",0,"timeout"`,
		`ARRAYSHIFT RESULTS,1,"x",0,1`,
		`ARRAYSORT RESULTS,FORWARD,0,1`,
		`ARRAYCOPY "RESULT","RESULT"`,
		`SORTCHARA CSTR`,
		`HTML_TAGSPLIT "html",RESULTS,RESULT`,
		`AWAIT "value"`,
		`SAVECHARA EFAUDIT_FIELDS,1,2`,
		`SETCOLOR UNKNOWN,"unresolved",1`,
	} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, options)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("%q: %+v", line, r.Diagnostics)
		}
		again := FormatWithOptions(r.Output, ERB, options)
		if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 || len(again.Diagnostics) != 0 {
			t.Fatalf("%q is unstable: %+v", line, again)
		}
	}
}

func TestCommandFieldRolesRejectProvenTypeMismatches(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing, FormPadding}}
	for _, line := range []string{
		`BAR  "x",2,3`,
		`SETCOLOR RESULTS`,
		`SAVEDATA 1,2`,
		`SAVECHARA "file",1,2`,
		`TINPUT 1,"default"`,
		`TINPUTS 1,2`,
		`INPUT "default"`,
		`ARRAYREMOVE RESULTS,"offset",1`,
		`ARRAYSHIFT RESULTS,1,2`,
		`ARRAYCOPY 1,2`,
		`SORTCHARA RESULT`,
		`HTML_TAGSPLIT 1`,
		`HTML_TAGSPLIT "html",CSTR`,
		`HTML_TAGSPLIT "html",RESULTS,RESULTS`,
		`SAVEDATA RESULT@PRIVATE,1`,
	} {
		src := []byte("@F\n" + line + "\n")
		r := FormatWithOptions(src, ERB, options)
		if !bytes.Equal(r.Output, src) || len(r.Changes) != 0 || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q: %+v", line, r)
		}
		quiet := FormatWithOptions(src, ERB, Options{Select: []RuleID{}})
		if !bytes.Equal(quiet.Output, src) || len(quiet.Diagnostics) != 0 {
			t.Fatalf("select-none %q: %+v", line, quiet)
		}
	}
	options.Dialect = EMEE
	r := FormatWithOptions([]byte("@F\nINPUTS prompt,\"bad\"\n"), ERB, options)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("INPUTS suffix: %+v", r)
	}
}

func TestCommandCountsPreserveObjectMacros(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing}}
	for _, line := range []string{
		`PRINTBUTTON  EFAUDIT_FIELDS`,
		`SWAPCHARA EFAUDIT_INDEX`,
		`TWAIT EFAUDIT_INDEX`,
		`SETCOLOR EFAUDIT_INDEX,1`,
		`ARRAYREMOVE EFAUDIT_VALUES,1`,
		`ARRAYSHIFT RESULT,EFAUDIT_INDEX`,
		`SPLIT EFAUDIT_FIELDS,RESULTS`,
		`SAVECHARA EFAUDIT_FIELDS,1`,
		`PRINTBUTTON [[FIELDS]]`,
	} {
		src := []byte("@F\n" + line + "\n")
		r := FormatWithOptions(src, ERB, options)
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 0 || len(r.Changes) != 0 {
			t.Fatalf("macro %q: %+v", line, r)
		}
	}
	for _, line := range []string{`SETCOLOR RESULT,1`, `ARRAYREMOVE "x",1`, `PRINTBUTTON @"label"`} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, options)
		if len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("direct count %q: %+v", line, r)
		}
	}
}

func TestColorFieldsPreserveUnpairedTernaryHash(t *testing.T) {
	options := Options{
		Dialect:             EMEE,
		AllowFullwidthSpace: true,
		Select:              []RuleID{CommandSpacing, BinarySpacing, TernarySpacing, CommaSpacing},
	}
	for _, test := range []struct {
		line string
		want string
	}{
		{line: "SETCOLOR #FF00FF", want: "SETCOLOR #FF00FF"},
		{line: "SETBGCOLOR #00FF00", want: "SETBGCOLOR #00FF00"},
		{line: "SETCOLOR　#FF00FF", want: "SETCOLOR　#FF00FF"},
		{line: "SETCOLOR FLAG?1#2", want: "SETCOLOR FLAG ? 1 # 2"},
		{line: "SETCOLOR UNKNOWN?1#2", want: "SETCOLOR UNKNOWN ? 1 # 2"},
		{line: "SETCOLOR EFAUDIT_INDEX,1", want: "SETCOLOR EFAUDIT_INDEX,1"},
	} {
		src := []byte("@F\n" + test.line + "\n")
		want := []byte("@F\n" + test.want + "\n")
		r := FormatWithOptions(src, ERB, options)
		if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
			t.Fatalf("line=%q output=%q diagnostics=%+v want=%q", test.line, r.Output, r.Diagnostics, want)
		}
	}
	separator := ""
	src := []byte("@F\n{\nSETCOLOR #FF00FF\n}\nRETURN 0\n")
	r := FormatWithOptions(src, ERB, Options{
		Dialect:               EMEE,
		AllowFullwidthSpace:   true,
		ContinuationSeparator: &separator,
		Select:                []RuleID{CommandSpacing, BinarySpacing, TernarySpacing, CommaSpacing},
	})
	if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 0 {
		t.Fatalf("continuation output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, src)
	}
}
