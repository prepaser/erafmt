package format

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

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

func TestHeaderArgumentSemanticsWarnsOnDuplicateBuiltinParameters(t *testing.T) {
	source := []byte("@F(ARG,ARG:(1-1),LOCAL:0,LOCAL:0)\n")
	result := FormatWithOptions(source, ERB, Options{})
	if len(result.Diagnostics) != 2 || result.Diagnostics[0].Code != "duplicate_parameter" || result.Diagnostics[1].Code != "duplicate_parameter" {
		t.Fatalf("formatted diagnostics=%+v, want duplicate warnings", result.Diagnostics)
	}
	session := erabasic.NewExpressionSession(source)
	start, end, ok := erabasic.HeaderArgumentRangeWithSession(source, session)
	if !ok {
		t.Fatal("header argument range was not recognized")
	}
	diagnostics := erabasic.HeaderArgumentDiagnosticsWithSession(source, start, end, session, erabasic.CommonDialect)
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics=%+v, want two duplicate warnings", diagnostics)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity != erabasic.Warning || diagnostic.Code != "duplicate_parameter" {
			t.Fatalf("diagnostic=%+v, want duplicate warning", diagnostic)
		}
	}
}
