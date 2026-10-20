package format

import (
	"bytes"
	"testing"
)

func TestVariableArgumentAcceptsUnaryPlus(t *testing.T) {
	for _, line := range []string{
		"TIMES +RESULT,1.25",
		"SWAP +RESULT,RESULT",
	} {
		src := []byte("@F\n" + line + "\n")
		r := FormatWithOptions(src, ERB, commandOptions())
		if len(r.Diagnostics) != 0 {
			t.Fatalf("%q diagnostics=%v output=%q", line, r.Diagnostics, r.Output)
		}
		again := FormatWithOptions(r.Output, ERB, commandOptions())
		if !bytes.Equal(again.Output, r.Output) || len(again.Diagnostics) != 0 {
			t.Fatalf("%q is not idempotent: %+v", line, again)
		}
	}
	src := []byte("@F\nPRINTDATA +RESULT\nDATA raw\nENDDATA\n")
	r := FormatWithOptions(src, ERB, commandOptions())
	if len(r.Diagnostics) != 0 {
		t.Fatalf("numeric unary plus output=%q diagnostics=%v", r.Output, r.Diagnostics)
	}
	src = []byte("@F\nSTRDATA +(RESULTS)\nDATA raw\nENDDATA\n")
	r = FormatWithOptions(src, ERB, commandOptions())
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("string unary plus output=%q diagnostics=%v", r.Output, r.Diagnostics)
	}
}

func TestVariableArgumentRejectsUnsupportedIndexShapes(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing}}
	for _, line := range []string{
		"TIMES RESULT[0],1.25",
		"SWAP RESULT:0+1,RESULT",
		"VARSET RESULT:0+1,1",
	} {
		src := []byte("@F\n" + line + "\n")
		r := FormatWithOptions(src, ERB, options)
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q output=%q diagnostics=%v", line, r.Output, r.Diagnostics)
		}
	}
	src := []byte("@F\nPRINTDATA RESULT:0+1\nDATA raw\nENDDATA\n")
	r := FormatWithOptions(src, ERB, options)
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("PRINTDATA output=%q diagnostics=%v", r.Output, r.Diagnostics)
	}
}

func TestSAVEVARValidatesFieldRoles(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing, FormPadding}}
	valid := []string{
		`SAVEVAR "file","message",RESULT,RESULTS`,
		`SAVEVAR "file",RESULTS,RESULT`,
		`SAVEVAR UNKNOWN_FILE,UNKNOWN_MESSAGE,UNKNOWN_VALUE`,
		`SAVEVAR EFAUDIT_FIELDS,RESULT`,
		`SAVEVAR [[FILE]],[[MESSAGE]],RESULT`,
	}
	for _, line := range valid {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, options)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("valid %q diagnostics=%v output=%q", line, r.Diagnostics, r.Output)
		}
	}
	for _, line := range []string{
		`SAVEVAR "file","message",1`,
		`SAVEVAR "file","message",RESULT+1`,
		`SAVEVAR "file",1,RESULT`,
		`SAVEVAR 1,EFAUDIT_FIELDS`,
		`SAVEVAR "file","message",CFLAG`,
		`SAVEVAR "file","message",CSTR`,
	} {
		src := []byte("@F\n" + line + "\n")
		r := FormatWithOptions(src, ERB, options)
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("invalid %q output=%q diagnostics=%v", line, r.Output, r.Diagnostics)
		}
	}
}

func TestSetterRolesFollowKnownDestinationType(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing}}
	for _, line := range []string{
		`VARSET RESULTS,"x"`,
		`VARSET RESULTS,"x",0,1`,
		`CVARSET CSTR,0,"x"`,
		`CVARSET CSTR,0,"x",1`,
		`CVARSET CFLAG,"CSVname",1`,
		`VARSET RESULT@FOO,"x"`,
		`VARSET EFAUDIT_DEST,"x"`,
		`VARSET RESULT:EFAUDIT_FIELDS+1,1`,
		`VARSET EFAUDIT_FIELDS?2#3`,
	} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, options)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("valid %q diagnostics=%v output=%q", line, r.Diagnostics, r.Output)
		}
	}
	for _, line := range []string{
		`VARSET RESULT,"x"`,
		`VARSET RESULTS,1`,
		`CVARSET CSTR,0,1`,
		`CVARSET CFLAG,0,"x"`,
		`VARSET CSTR+1,"x"`,
		`POWER RESULT,"x",2`,
		`VARSET +RESULTS,"x"`,
		`VARSET +(RESULTS:0),"x"`,
	} {
		src := []byte("@F\n" + line + "\n")
		r := FormatWithOptions(src, ERB, options)
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("invalid %q output=%q diagnostics=%v", line, r.Output, r.Diagnostics)
		}
	}
}

func TestSPLITValidatesKnownValueTypes(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing, FormPadding}}
	for _, line := range []string{
		`SPLIT "a","b",RESULTS`,
		`SPLIT "a","b",RESULTS,RESULT`,
		`SPLIT EFAUDIT_FIELDS,1,RESULTS`,
	} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, options)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("valid %q diagnostics=%v output=%q", line, r.Diagnostics, r.Output)
		}
	}
	for _, line := range []string{
		`SPLIT 1,2,RESULTS`,
		`SPLIT "a","b",RESULT`,
		`SPLIT "a","b",RESULTS,RESULTS`,
	} {
		src := []byte("@F\n" + line + "\n")
		r := FormatWithOptions(src, ERB, options)
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("invalid %q output=%q diagnostics=%v", line, r.Output, r.Diagnostics)
		}
	}
}
