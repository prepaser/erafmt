package format

import (
	"bytes"
	"testing"
)

func extraRoleOptions() Options {
	return Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing}}
}

func TestSpecializedCommandsPreserveMacroPrefixes(t *testing.T) {
	src := []byte("@F\n" +
		"SPLIT  EFAUDIT_FIELDS4 , RESULTS,1\n" +
		"SWAP  EFAUDIT_SWAP2 ,1\n" +
		"ARRAYSORT  EFAUDIT_SORT ,0\n" +
		"SORTCHARA  EFAUDIT_SORTCHARA ,1\n" +
		"HTML_TAGSPLIT  EFAUDIT_HTML3 ,1\n" +
		"ARRAYSORT RESULTS,FORWARD,\"offset\"\n" +
		"ARRAYSORT FOO,FORWARD,1+2\n" +
		"PRINTDATA EFAUDIT_FILE\n" +
		"DATA raw\n" +
		"ENDDATA\n" +
		"STRDATA EFAUDIT_FILE\n" +
		"DATA raw\n" +
		"ENDDATA\n")
	want := []byte("@F\n" +
		"SPLIT  EFAUDIT_FIELDS4 , RESULTS,1\n" +
		"SWAP  EFAUDIT_SWAP2 ,1\n" +
		"ARRAYSORT  EFAUDIT_SORT ,0\n" +
		"SORTCHARA  EFAUDIT_SORTCHARA ,1\n" +
		"HTML_TAGSPLIT  EFAUDIT_HTML3 ,1\n" +
		"ARRAYSORT RESULTS,FORWARD,\"offset\"\n" +
		"ARRAYSORT FOO, FORWARD, 1 + 2\n" +
		"PRINTDATA EFAUDIT_FILE\n" +
		"DATA raw\n" +
		"ENDDATA\n" +
		"STRDATA EFAUDIT_FILE\n" +
		"DATA raw\n" +
		"ENDDATA\n")
	r := FormatWithOptions(src, ERB, extraRoleOptions())
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
	again := FormatWithOptions(r.Output, ERB, extraRoleOptions())
	if !bytes.Equal(again.Output, r.Output) || len(again.Diagnostics) != 0 {
		t.Fatalf("not idempotent: output=%q diagnostics=%+v", again.Output, again.Diagnostics)
	}
}

func TestExtraVariableCommandsValidateKnownTypes(t *testing.T) {
	valid := []string{
		"PRINTDATA RESULT",
		"PRINTDATA CFLAG:0:0",
		"PRINTDATA CFLAG:EFAUDIT_INDEX",
		"STRDATA RESULTS",
		"STRDATA CSTR:0:0",
		"STRDATA CSTR:EFAUDIT_INDEX",
		"SWAP RESULT,RESULT",
		"SWAP RESULTS,RESULTS",
		"SWAP EFAUDIT_SWAP2,1",
	}
	for _, line := range valid {
		src := extraRoleSource(line)
		r := FormatWithOptions(src, ERB, extraRoleOptions())
		if len(r.Diagnostics) != 0 {
			t.Fatalf("valid %q diagnostics=%+v output=%q", line, r.Diagnostics, r.Output)
		}
	}
	invalid := []string{
		"PRINTDATA RESULTS",
		"PRINTDATA CSTR:0:0",
		"STRDATA RESULT",
		"STRDATA CFLAG:0:0",
		"SWAP RESULTS,RESULT",
		"SWAP CSTR:0:0,RESULT",
		`SWAP @"literal",RESULT`,
		`SPLIT "text",",",@"literal"`,
		`ARRAYSORT @"literal",FORWARD`,
		`HTML_TAGSPLIT @"html",1`,
	}
	for _, line := range invalid {
		src := extraRoleSource(line)
		r := FormatWithOptions(src, ERB, extraRoleOptions())
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("invalid %q output=%q diagnostics=%+v", line, r.Output, r.Diagnostics)
		}
	}
}

func extraRoleSource(line string) []byte {
	if len(line) >= 9 && line[:9] == "PRINTDATA" || len(line) >= 7 && line[:7] == "STRDATA" {
		return []byte("@F\n" + line + "\nDATA raw\nENDDATA\n")
	}
	return []byte("@F\n" + line + "\n")
}
