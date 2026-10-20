package format

import (
	"bytes"
	"strings"
	"testing"
)

func headerSemanticIssueForTest(src string) (headerArgumentIssue, bool) {
	input := []byte(src)
	session := newExpressionSession(input)
	start, end, ok := headerArgumentRangeWithSession(input, session)
	if !ok {
		return headerArgumentIssue{}, false
	}
	return headerArgumentIssueWithSession(input, start, end, session, CommonDialect)
}

func TestHeaderSemanticErrorKeepsFunctionBoundaries(t *testing.T) {
	src := []byte("  @  BAD(1)\nRETURN 1+2\n@GOOD\nRETURN 3+4\n")
	r := FormatWithOptions(src, ERB, Options{})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_syntax" || r.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("diagnostics=%+v", r.Diagnostics)
	}
	if !bytes.HasPrefix(r.Output, []byte("  @  BAD(1)\n    RETURN 1 + 2\n")) || !bytes.Contains(r.Output, []byte("@GOOD\n    RETURN 3 + 4\n")) {
		t.Fatalf("output=%q", r.Output)
	}
	for _, options := range []Options{{Select: []RuleID{}}, {Select: []RuleID{InstructionIndent}}} {
		r = FormatWithOptions(src, ERB, options)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("disabled semantic checks: %+v", r.Diagnostics)
		}
	}
}

func TestHeaderArgumentSemanticsRejectsProvenInvalidRoles(t *testing.T) {
	for _, test := range []struct {
		source  string
		message string
	}{
		{"@F(1)\n", "assignable variable"},
		{"@F(\"x\")\n", "assignable variable"},
		{"@F(ITEMPRICE:0)\n", "assignable variable"},
		{"@F(GAMEBASE_VERSION)\n", "assignable variable"},
		{"@F(ARG:0=\"x\")\n", "type does not match"},
		{"@F(ARGS:0=1)\n", "type does not match"},
		{"@F(ARG:0=1/0)\n", "valid numeric constant"},
		{"@F(RESULT:0=1)\n", "allowed only"},
		{"@F(ARG:1/0)\n", "valid numeric constant"},
		{"@F(ARG:0:1)\n", "wrong number"},
		{"@F(A+B)\n", "assignable variable"},
		{"@F(RESULT+1)\n", "assignable variable"},
		{"@F((A+B))\n", "assignable variable"},
		{"@F(STRLEN(\"x\"))\n", "assignable variable"},
		{"@F(ABS(1))\n", "assignable variable"},
		{"@F(!RESULT)\n", "assignable variable"},
		{"@F(RESULT++)\n", "assignable variable"},
		{"@F(RESULT:0+1)\n", "assignable variable"},
	} {
		issue, found := headerSemanticIssueForTest(test.source)
		if !found || issue.message == "" || !strings.Contains(issue.message, test.message) {
			t.Fatalf("source=%q issue=%+v found=%v, want message containing %q", test.source, issue, found, test.message)
		}
	}
}

func TestHeaderArgumentSemanticsKeepsProvenValidAndUnknownRoles(t *testing.T) {
	for _, source := range []string{
		"@F(ARG)\n",
		"@F(ARGS)\n",
		"@F(COUNT)\n",
		"@F(RESULT)\n",
		"@F(FLAG)\n",
		"@F(LOCAL)\n",
		"@F(LOCALS)\n",
		"@F(ARG=1)\n",
		"@F(ARGS=\"x\")\n",
		"@F(ARG:(0+1)=2)\n",
		"@F(ARG:(\"a\"==\"b\"))\n",
		"@F(ARG:(\"a\"==\"b\")=(\"x\"==\"y\"))\n",
		"@F(ARG:0)\n",
		"@F(RESULT:0)\n",
		"@F((ARG:0))\n",
		"@F(+RESULT)\n",
		"@F((RESULT))\n",
		"@F(LOCAL@OTHER:0)\n",
		"@F(LOCAL@RESULT:0)\n",
		"@F(RESULT@LOCAL:0)\n",
		"@F(PRIVATE_VALUE=1)\n",
		"@F(ARG:UNKNOWN, 1)\n",
		"@F(CFLAG:UNKNOWN, 1)\n",
		"@F(CDFLAG:UNKNOWN, 1)\n",
		"@F(DA:UNKNOWN, 1)\n",
		"@F(TA:UNKNOWN, 1)\n",
		"@F(ARG:0=UNKNOWN, 1)\n",
		"@F(UNKNOWN_VALUE, 1)\n",
		"@F(USER_FUNCTION(1), 1)\n",
		"@F(FLAG:\"name\")\n",
		"@F(CFLAG:0:\"name\")\n",
		"@F(ARG:\"name\")\n",
	} {
		if issue, found := headerSemanticIssueForTest(source); found {
			t.Fatalf("source=%q unexpectedly reported %+v", source, issue)
		}
	}
}

func TestHeaderArgumentSemanticsRequiresKnownBuiltinIndexes(t *testing.T) {
	for _, source := range []string{
		"@F(DA)\n",
		"@F(DA:0)\n",
		"@F(DB)\n",
		"@F(DB:0)\n",
		"@F(DC)\n",
		"@F(DC:0)\n",
		"@F(DD)\n",
		"@F(DD:0)\n",
		"@F(DE)\n",
		"@F(DE:0)\n",
		"@F(DITEMTYPE)\n",
		"@F(DITEMTYPE:0)\n",
		"@F(TA)\n",
		"@F(TA:0)\n",
		"@F(TA:0:0)\n",
		"@F(TB)\n",
		"@F(TB:0)\n",
		"@F(TB:0:0)\n",
		"@F(CDFLAG)\n",
		"@F(CDFLAG:0)\n",
		"@F(CDFLAG:0:0)\n",
		"@F(ABL)\n",
		"@F(ABL:0)\n",
		"@F(CFLAG)\n",
		"@F(CFLAG:0)\n",
		"@F(CSTR)\n",
		"@F(CSTR:0)\n",
		"@F(NAME)\n",
	} {
		issue, found := headerSemanticIssueForTest(source)
		if !found || !strings.Contains(issue.message, "wrong number of indexes") {
			t.Fatalf("source=%q issue=%+v found=%v", source, issue, found)
		}
	}

	for _, source := range []string{
		"@F(DA:0:0)\n",
		"@F(DB:0:0)\n",
		"@F(DC:0:0)\n",
		"@F(DD:0:0)\n",
		"@F(DE:0:0)\n",
		"@F(DITEMTYPE:0:0)\n",
		"@F(TA:0:0:0)\n",
		"@F(TB:0:0:0)\n",
		"@F(CDFLAG:0:0:0)\n",
		"@F(ABL:0:0)\n",
		"@F(CFLAG:0:0)\n",
		"@F(CSTR:0:0)\n",
		"@F(NAME:0)\n",
	} {
		if issue, found := headerSemanticIssueForTest(source); found {
			t.Fatalf("source=%q unexpectedly reported %+v", source, issue)
		}
	}
}

func TestHeaderArgumentSemanticsStopsAfterUnknownRole(t *testing.T) {
	for _, source := range []string{
		"@F(UNKNOWN, 1, ITEMPRICE:0)\n",
		"@F(ARG:UNKNOWN, 1, RESULT)\n",
		"@F(USER_FUNCTION(), \"x\")\n",
	} {
		if issue, found := headerSemanticIssueForTest(source); found {
			t.Fatalf("source=%q checked past unknown role: %+v", source, issue)
		}
	}
}

func TestHeaderArgumentSemanticsDefersUnsupportedConstantEvaluation(t *testing.T) {
	long := strings.Repeat("1+", 2048) + "1"
	for _, source := range []string{
		"@F(ARG:" + long + ", ITEMPRICE:0)\n",
		"@F(ARG:0=" + long + ", ITEMPRICE:0)\n",
	} {
		if issue, found := headerSemanticIssueForTest(source); found {
			t.Fatalf("source with unsupported constant evaluation reported %+v", issue)
		}
	}
}

func TestHeaderArgumentSemanticsRejectsRuntimeBuiltinDefaults(t *testing.T) {
	for _, test := range []struct {
		source  string
		message string
	}{
		{"@F(ARG:0=RESULT)\n", "constant expression"},
		{"@F(ARGS:0=RESULTS)\n", "constant expression"},
		{"@F(ARG:0=LOCAL:0)\n", "constant expression"},
		{"@F(ARG:0=CFLAG:0:0)\n", "constant expression"},
		{"@F(ARG:RESULT)\n", "numeric constants"},
		{"@F(ARG:0=CHARANUM)\n", "constant expression"},
		{"@F(ARG:0=LASTLOAD_VERSION)\n", "constant expression"},
		{"@F(ARG:0=ISTIMEOUT)\n", "constant expression"},
		{"@F(ARGS:0=LASTLOAD_TEXT)\n", "constant expression"},
	} {
		issue, found := headerSemanticIssueForTest(test.source)
		if !found || !strings.Contains(issue.message, test.message) {
			t.Fatalf("source=%q issue=%+v found=%v, want message containing %q", test.source, issue, found, test.message)
		}
	}
}

func TestHeaderArgumentSemanticsAcceptsKnownConstantBuiltins(t *testing.T) {
	for _, source := range []string{
		"@F(ARG:0=ITEMPRICE:0)\n",
		"@F(ARGS:0=ITEMNAME:0)\n",
		"@F(ARG:0=GAMEBASE_VERSION+1)\n",
		"@F(ARG:0=GAMEBASE_VERSION/0)\n",
		"@F(ARG:0=1/(GAMEBASE_VERSION-1))\n",
		"@F(ARGS:0=MONEYLABEL)\n",
		"@F(ARG:GAMEBASE_VERSION)\n",
		"@F(ARG:0=PRIVATE_CONST)\n",
	} {
		if issue, found := headerSemanticIssueForTest(source); found {
			t.Fatalf("source=%q unexpectedly reported %+v", source, issue)
		}
	}
}

func TestHeaderArgumentSemanticsWarnsOnDuplicateBuiltinParameters(t *testing.T) {
	source := []byte("@F(ARG,ARG:(1-1),LOCAL:0,LOCAL:0)\n")
	result := FormatWithOptions(source, ERB, Options{})
	if len(result.Diagnostics) != 2 || result.Diagnostics[0].Code != "duplicate_parameter" || result.Diagnostics[1].Code != "duplicate_parameter" {
		t.Fatalf("formatted diagnostics=%+v, want duplicate warnings", result.Diagnostics)
	}
	session := newExpressionSession(source)
	start, end, ok := headerArgumentRangeWithSession(source, session)
	if !ok {
		t.Fatal("header argument range was not recognized")
	}
	diagnostics := headerArgumentDiagnosticsWithSession(source, start, end, session, CommonDialect)
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics=%+v, want two duplicate warnings", diagnostics)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.severity != Warning || diagnostic.code != "duplicate_parameter" {
			t.Fatalf("diagnostic=%+v, want duplicate warning", diagnostic)
		}
	}
}

func TestHeaderArgumentSemanticsStopsDuplicateProofAfterUnknown(t *testing.T) {
	source := []byte("@F(ARG,UNKNOWN,ARG:0)\n")
	session := newExpressionSession(source)
	start, end, ok := headerArgumentRangeWithSession(source, session)
	if !ok {
		t.Fatal("header argument range was not recognized")
	}
	if diagnostics := headerArgumentDiagnosticsWithSession(source, start, end, session, CommonDialect); len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v, want unknown parameter to stop proof", diagnostics)
	}
}
