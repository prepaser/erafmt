package format

import (
	"bytes"
	"testing"
)

func TestKnownDestinationRoles(t *testing.T) {
	options := destinationRoleOptions()
	for _, line := range []string{
		"TIMES CSTR,1.5",
		"TIMES RESULTS:0,1.5",
		"SETBIT CSTR,1",
		"CLEARBIT RESULTS:0,1",
		"INVERTBIT CSTR:0,1",
		"CVARSET RESULT,0,1",
		"CVARSET RESULTS,0,\"x\"",
		"CVARSET CFLAG,0,\"x\"",
		"CVARSET CSTR,0,1",
	} {
		input := []byte("@F\n" + line + "\n")
		r := FormatWithOptions(input, ERB, options)
		if !bytes.Equal(r.Output, input) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q output=%q diagnostics=%+v", line, r.Output, r.Diagnostics)
		}
	}

	for _, line := range []string{
		"TIMES RESULT,1.5",
		"SETBIT RESULT,1",
		"CVARSET CFLAG,0,1",
		"CVARSET CSTR,0,\"x\"",
	} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, options)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("valid %q output=%q diagnostics=%+v", line, r.Output, r.Diagnostics)
		}
	}
}

func TestDestinationRolesDeferUnknownAndMarkedFields(t *testing.T) {
	options := destinationRoleOptions()
	for _, line := range []string{
		"TIMES EFAUDIT_DEST,1.5",
		"TIMES CSTR:EFAUDIT_INDEX,1.5",
		"SETBIT EFAUDIT_DEST,1",
		"SETBIT CSTR:EFAUDIT_INDEX,1",
		"CVARSET EFAUDIT_DEST,0,1",
		"CVARSET RESULT:EFAUDIT_INDEX,0,1",
		"SETBIT CSTR,1;!;",
		"CVARSET RESULT,0,1;!;",
	} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, options)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("deferred %q output=%q diagnostics=%+v", line, r.Output, r.Diagnostics)
		}
		again := FormatWithOptions(r.Output, ERB, options)
		if !bytes.Equal(again.Output, r.Output) || len(again.Diagnostics) != 0 {
			t.Fatalf("not idempotent %q: first=%+v again=%+v", line, r, again)
		}
	}
}

func destinationRoleOptions() Options {
	return Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing}}
}
