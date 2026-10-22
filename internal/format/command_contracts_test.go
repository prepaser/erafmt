package format

import (
	"bytes"

	"testing"
)

func TestSAVECHARAProvesLiteralIDs(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing}}
	for _, body := range []string{`SAVECHARA "file","message",-1`, `SAVECHARA "file","message",2147483648`, `SAVECHARA "file","message",1,1`} {
		src := []byte("@F\n" + body + "\n")
		r := FormatWithOptions(src, ERB, options)
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q: output=%q diagnostics=%+v", body, r.Output, r.Diagnostics)
		}
	}
	for _, body := range []string{`SAVECHARA "file","message",1+2`, `SAVECHARA "file","message",1+2,1+2`, `SAVECHARA "file","message",EFAUDIT_ID`, `SAVECHARA EFAUDIT_FIELDS,1,-1`, `SAVECHARA "file","message",1,`} {
		r := FormatWithOptions([]byte("@F\n"+body+"\n"), ERB, options)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("%q: diagnostics=%+v output=%q", body, r.Diagnostics, r.Output)
		}
	}
}

func TestSAVEVARRejectsKnownImmutableAndDuplicateVariables(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing}}
	for _, body := range []string{`SAVEVAR "file","message",ITEMPRICE`, `SAVEVAR "file","message",RESULT,RESULT`, `SAVEVAR "file","message",RESULT:0,RESULT:1`} {
		src := []byte("@F\n" + body + "\n")
		r := FormatWithOptions(src, ERB, options)
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q: output=%q diagnostics=%+v", body, r.Output, r.Diagnostics)
		}
	}
}

func TestMutableCommandRolesRejectKnownImmutableVariables(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing}}
	for _, body := range []string{`TIMES ITEMPRICE,1`, `SWAP ITEMPRICE,RESULT`, `ARRAYSORT ITEMPRICE,FORWARD`, `SORTCHARA ITEMPRICE`, `PRINTDATA ITEMPRICE`, `STRDATA ITEMNAME`} {
		src := []byte("@F\n" + body + "\n")
		if bytes.HasPrefix([]byte(body), []byte("PRINTDATA")) {
			src = []byte("@F\n" + body + "\nDATA raw\nENDDATA\n")
		}
		if bytes.HasPrefix([]byte(body), []byte("STRDATA")) {
			src = []byte("@F\n" + body + "\nDATA raw\nENDDATA\n")
		}
		r := FormatWithOptions(src, ERB, options)
		if len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q: output=%q diagnostics=%+v", body, r.Output, r.Diagnostics)
		}
	}
}

func TestTIMESRuntimeNumericFallbackIsPreserved(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing}}
	for _, coefficient := range []string{"1e+2", "+1e+2", "1E+02", "1e", "1+2"} {
		src := []byte("@F\nTIMES RESULT," + coefficient + "\n")
		r := FormatWithOptions(src, ERB, options)
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Warning {
			t.Fatalf("%q: output=%q diagnostics=%+v", coefficient, r.Output, r.Diagnostics)
		}
	}
	for _, coefficient := range []string{"1e-2", ".5", "1."} {
		r := FormatWithOptions([]byte("@F\nTIMES RESULT,"+coefficient+"\n"), ERB, options)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("%q: diagnostics=%+v output=%q", coefficient, r.Diagnostics, r.Output)
		}
	}
}
