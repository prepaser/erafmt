package format

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestGraphicsTrailingCommaUsesEffectiveShape(t *testing.T) {
	source := []byte("PRINT_SPACE 1,\nPRINT_RECT 1,\nPRINT_RECT 1,2,3,4,\nPRINT_IMG \"x\",\n")
	r := FormatWithOptions(source, ERB, Options{Dialect: CommonDialect, Select: []RuleID{BinarySpacing, CommaSpacing}})
	want := []byte("PRINT_SPACE 1,\nPRINT_RECT 1,\nPRINT_RECT 1, 2, 3, 4,\nPRINT_IMG \"x\",\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	again := FormatWithOptions(r.Output, ERB, Options{Dialect: CommonDialect, Select: []RuleID{BinarySpacing, CommaSpacing}})
	if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
		t.Fatalf("not idempotent: output=%q changes=%+v", again.Output, again.Changes)
	}
	invalid := FormatWithOptions([]byte("PRINT_RECT 1,2,3,\n"), ERB, Options{Dialect: CommonDialect, Select: []RuleID{CommaSpacing}})
	if !bytes.Equal(invalid.Output, []byte("PRINT_RECT 1,2,3,\n")) || len(invalid.Diagnostics) != 1 {
		t.Fatalf("three-field output=%q diagnostics=%+v", invalid.Output, invalid.Diagnostics)
	}
	invalid = FormatWithOptions([]byte("PRINT_IMG \"x\",1+\"y\"\n"), ERB, Options{Dialect: EMEE, Select: []RuleID{BinarySpacing}})
	if !bytes.Equal(invalid.Output, []byte("PRINT_IMG \"x\",1+\"y\"\n")) || len(invalid.Diagnostics) != 1 {
		t.Fatalf("mixed image field output=%q diagnostics=%+v", invalid.Output, invalid.Diagnostics)
	}
}

func TestGraphicsNumericFieldsAcceptProvenComparisons(t *testing.T) {
	source := []byte("PRINT_SPACE \"a\"==\"b\"\nPRINT_RECT \"a\"==\"b\",1,2,3\nPRINT_SPACE RESULTS==\"a\"\nPRINT_RECT CSTR==\"a\",1,2,3\nPRINT_SPACE UNKNOWN==\"a\"\nPRINT_RECT UNKNOWN==\"a\",1,2,3\n")
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824} {
		r := FormatWithOptions(source, ERB, Options{Dialect: Dialect(dialect), Select: []RuleID{BinarySpacing, CommaSpacing}})
		want := []byte("PRINT_SPACE \"a\" == \"b\"\nPRINT_RECT \"a\" == \"b\", 1, 2, 3\nPRINT_SPACE RESULTS == \"a\"\nPRINT_RECT CSTR == \"a\", 1, 2, 3\nPRINT_SPACE UNKNOWN == \"a\"\nPRINT_RECT UNKNOWN == \"a\", 1, 2, 3\n")
		if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
			t.Fatalf("dialect=%d output=%q diagnostics=%+v", dialect, r.Output, r.Diagnostics)
		}
	}
}

func TestEMEEImageTailDefersAmbiguousFields(t *testing.T) {
	source := []byte("PRINT_IMG \"x\",\"a\"+\"b\"\nPRINT_IMG \"x\",RESULTS\nPRINT_IMG \"x\",UNKNOWN,1\nPRINT_IMG \"x\",1,UNKNOWN\nPRINT_IMG \"x\",1,CSTR\n")
	r := FormatWithOptions(source, ERB, Options{Dialect: EMEE, Select: []RuleID{BinarySpacing, CommaSpacing}})
	want := []byte("PRINT_IMG \"x\", \"a\" + \"b\"\nPRINT_IMG \"x\", RESULTS\nPRINT_IMG \"x\",UNKNOWN,1\nPRINT_IMG \"x\",1,UNKNOWN\nPRINT_IMG \"x\",1,CSTR\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 1 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	if again := FormatWithOptions(r.Output, ERB, Options{Dialect: EMEE, Select: []RuleID{BinarySpacing, CommaSpacing}}); !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
		t.Fatalf("not idempotent: output=%q changes=%+v", again.Output, again.Changes)
	}
}

func TestGraphicsMarkersStayFieldScoped(t *testing.T) {
	source := []byte("PRINT_IMG \"x\",;!;RESULT\nPRINT_SPACE ;#; RESULT\nPRINT_IMG \"x\",1,;!;CSTR\n")
	r := FormatWithOptions(source, ERB, Options{Dialect: EMEE, Select: []RuleID{BinarySpacing, CommaSpacing}})
	if !bytes.Equal(r.Output, source) || len(r.Diagnostics) != 1 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}
