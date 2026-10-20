package format

import (
	"bytes"
	"testing"
)

func TestGenericCommandRolesRejectProvenMismatches(t *testing.T) {
	for _, test := range []struct {
		line string
		body string
	}{
		{`IF "x"`, "IF \"x\"\nENDIF\n"},
		{`RETURN "x"`, "RETURN \"x\"\n"},
		{`RETURN 1,"x"`, "RETURN 1,\"x\"\n"},
		{`PRINTS 1`, "PRINTS 1\n"},
		{`REDRAW "x"`, "REDRAW \"x\"\n"},
		{`FOR "x",0,1`, "FOR \"x\",0,1\nNEXT\n"},
		{`FOR CFLAG,0,1`, "FOR CFLAG,0,1\nNEXT\n"},
		{`FOR RESULT,0,,2`, "FOR RESULT,0,,2\nNEXT\n"},
		{`FOR RESULT,0,1,"x"`, "FOR RESULT,0,1,\"x\"\nNEXT\n"},
	} {
		r := FormatWithOptions([]byte("@F\n"+test.body), ERB, commandOptions())
		if len(r.Diagnostics) != 1 || !bytes.Contains(r.Output, []byte("    "+test.line+"\n")) {
			t.Fatalf("%q output=%q diagnostics=%+v", test.line, r.Output, r.Diagnostics)
		}
	}
}

func TestFORRequiresArguments(t *testing.T) {
	r := FormatWithOptions([]byte("@F\nFOR\nNEXT\n"), ERB, commandOptions())
	if len(r.Diagnostics) != 1 || !bytes.Equal(r.Output, []byte("@F\n    FOR\n    NEXT\n")) {
		t.Fatalf("result=%+v", r)
	}
}

func TestGenericCommandRolesPreserveUnknownMacros(t *testing.T) {
	src := []byte("@F\nIF EFAUDIT_INDEX\nRETURN EFAUDIT_VALUE\nENDIF\nFOR EFAUDIT_VALUES,2\nNEXT\nRETURN 0\n")
	r := FormatWithOptions(src, ERB, commandOptions())
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", r.Diagnostics, r.Output)
	}
	again := FormatWithOptions(r.Output, ERB, commandOptions())
	if !bytes.Equal(again.Output, r.Output) || len(again.Diagnostics) != 0 {
		t.Fatalf("not idempotent: first=%q second=%q diagnostics=%+v", r.Output, again.Output, again.Diagnostics)
	}
}

func TestFORRolesPreserveInlineMarkers(t *testing.T) {
	for _, line := range []string{"FOR ;!; RESULT,0,1", "FOR RESULT;!;,0,1", "FOR RESULT,;!;0,1"} {
		r := FormatWithOptions([]byte("@F\n"+line+"\nNEXT\n"), ERB, commandOptions())
		if len(r.Diagnostics) != 0 || !bytes.Contains(r.Output, []byte(";!;")) {
			t.Fatalf("%q: %+v", line, r)
		}
	}
}

func TestModernOnlyMethodNamesRemainVariablesInLegacyDialect(t *testing.T) {
	src := []byte("@F\nXML_GET = 1\nXML_GET += 1+2\nXML_GET++\n++XML_GET\nIF 1\n;#;SIF 1\n;#;XML_GET += 2\nENDIF\nRETURN 0\n")
	r := FormatWithOptions(src, ERB, Options{Dialect: Emuera1824, Select: commandOptions().Select})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", r.Diagnostics, r.Output)
	}
	if !bytes.Contains(r.Output, []byte("XML_GET += 1 + 2")) || !bytes.Contains(r.Output, []byte("XML_GET++")) {
		t.Fatalf("output=%q", r.Output)
	}
}

func TestCommandRolesRespectDialectSpecificFields(t *testing.T) {
	legacy := FormatWithOptions([]byte("@F\nHTML_PRINT 1\nHTML_PRINT RESULTS\n"), ERB, Options{Dialect: Emuera1824, Select: commandOptions().Select})
	if len(legacy.Diagnostics) != 1 || !bytes.Contains(legacy.Output, []byte("HTML_PRINT 1")) {
		t.Fatalf("legacy=%+v", legacy)
	}
	modern := FormatWithOptions([]byte("@F\nHTML_PRINT RESULTS,1+2\nHTML_PRINT_ISLAND 1+2\nPLAYSOUND 1+2\n"), ERB, Options{Dialect: EMEE, Select: commandOptions().Select})
	if len(modern.Diagnostics) != 2 {
		t.Fatalf("modern=%+v", modern)
	}
	for _, line := range []string{"HTML_PRINT_ISLAND 1+2", "PLAYSOUND 1+2"} {
		if !bytes.Contains(modern.Output, []byte(line)) {
			t.Fatalf("modern output=%q lost %q", modern.Output, line)
		}
	}
}
