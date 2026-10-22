package format

import (
	"bytes"

	"testing"
)

func TestSpecializedCommandSurplusIsPreservedAsWarning(t *testing.T) {
	src := []byte("@F\n" + "ARRAYSORT RESULTS,FORWARD,1,2,3\n" + "SORTCHARA CSTR,FORWARD,1\n" + "SAVECHARA \"file\",\"message\",1,\n" + "HTML_TAGSPLIT \"a\",RESULTS,RESULT,1\n" + "TIMES RESULT,1,2\n" + "SWAP RESULT,RESULT,1\n" + "SPLIT \"a\",\",\",RESULTS,RESULT,1\n" + "PRINTDATA RESULT,\nDATA raw\nENDDATA\n" + "STRDATA RESULTS,\nDATA raw\nENDDATA\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, CommandSpacing, BinarySpacing, CommaSpacing}})
	for _, body := range []string{
		"ARRAYSORT RESULTS,FORWARD,1,2,3",
		"SORTCHARA CSTR,FORWARD,1",
		"SAVECHARA \"file\", \"message\", 1,",
		"HTML_TAGSPLIT \"a\",RESULTS,RESULT,1",
		"TIMES RESULT,1,2",
		"SWAP RESULT,RESULT,1",
		"SPLIT \"a\",\",\",RESULTS,RESULT,1",
	} {
		if !bytes.Contains(r.Output, []byte(body)) {
			t.Fatalf("body %q changed: output=%q diagnostics=%+v", body, r.Output, r.Diagnostics)
		}
	}
	if !bytes.Contains(r.Output, []byte("PRINTDATA RESULT,")) || !bytes.Contains(r.Output, []byte("STRDATA RESULTS,")) {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	unsupported := 0
	warnings := 0
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Severity == Unsupported {
			unsupported++
		}
		if diagnostic.Code == "ignored_arguments" {
			warnings++
		}
	}
	if unsupported != 0 || warnings != 6 {
		t.Fatalf("diagnostics=%+v", r.Diagnostics)
	}
}

func TestSpecializedCommandsAcceptOneTrailingComma(t *testing.T) {
	src := []byte("@F\n" + "TIMES RESULT,1,\n" + "SWAP RESULT,RESULT,\n" + "SPLIT \"a\",\",\",RESULTS,\n" + "ARRAYSORT RESULTS,\n" + "SORTCHARA CSTR,\n" + "HTML_TAGSPLIT \"a\",RESULTS,\n" + "PRINTDATA RESULT,\nDATA raw\nENDDATA\n" + "STRDATA RESULTS,\nDATA raw\nENDDATA\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, CommandSpacing, BinarySpacing, CommaSpacing}})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", r.Diagnostics, r.Output)
	}
	for _, body := range []string{"TIMES RESULT, 1,", "SWAP RESULT, RESULT,", "SPLIT \"a\", \",\", RESULTS,", "ARRAYSORT RESULTS,", "SORTCHARA CSTR,", "HTML_TAGSPLIT \"a\", RESULTS,", "PRINTDATA RESULT,", "STRDATA RESULTS,"} {
		if !bytes.Contains(r.Output, []byte(body)) {
			t.Fatalf("body %q missing: output=%q", body, r.Output)
		}
	}
}

func TestSplitAndHTMLTagSplitAcceptNumericScalarCount(t *testing.T) {
	src := []byte("@F\nSPLIT \"a\",\",\",RESULTS,NO\nHTML_TAGSPLIT \"a\",RESULTS,NO\n")
	r := FormatWithOptions(src, ERB, commandOptions())
	if len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}
