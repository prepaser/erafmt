package format

import (
	"bytes"
	"testing"
)

func TestFullwidthSpaceHelpers(t *testing.T) {
	src := []byte("\u3000\tvalue\u3000")
	if got := trimInput(src, false); !bytes.Equal(got, src) {
		t.Fatalf("trimInput(default) = %q, want unchanged", got)
	}
	if got := trimInput(src, true); string(got) != "value" {
		t.Fatalf("trimInput(allow) = %q, want value", got)
	}
	got, _, _, _ := headOfWithSpace([]byte("\u3000;#;\u3000RETURN 1"), true)
	if string(got) != "RETURN 1" {
		t.Fatalf("headOfWithSpace = %q, want RETURN 1", got)
	}
	if command, end := commandOfWithSpace([]byte("RETURN\u30001"), true); command != "RETURN" || end != len("RETURN") {
		t.Fatalf("commandOfWithSpace = %q, %d", command, end)
	}
}

func TestFullwidthHeaderSpaceOption(t *testing.T) {
	valid := []string{
		"@\u3000F",
		"@F\u3000(ARG:0)",
		"@F(ARG:0\u3000+ 1)",
		"@F(1)\u3000; note",
	}
	for _, source := range valid {
		src := []byte(source)
		if got := validateHeader(src); got.status != headerUnsupported {
			t.Errorf("validateHeader(%q) = %v %q, want unsupported", source, got.status, got.message)
		}
		session := newExpressionSessionWithSpace(src, true)
		if got := validateHeaderWithSession(src, session); got.status != headerValid {
			t.Errorf("validateHeaderWithSession(%q) = %v %q, want valid", source, got.status, got.message)
		}
	}
}

func TestFullwidthFormPayloadIsNotWhitespace(t *testing.T) {
	src := []byte(`@"x {A　+1}　　literal"`)
	rules, err := resolveRules(Options{})
	if err != nil {
		t.Fatal(err)
	}
	session := newExpressionSessionWithSpace(src, true)
	edits, err := formEditsWithSession(session, expressionSourceOriginal, src, 0, rules, formQuoted, true)
	if err != nil {
		t.Fatal(err)
	}
	got := applyEdits(src, edits)
	want := []byte(`@"x {A + 1}　　literal"`)
	if !bytes.Equal(got, want) {
		t.Fatalf("formatted FORM = %q, want %q", got, want)
	}
}

func TestFullwidthOptionEndToEnd(t *testing.T) {
	src := []byte("@F\u3000(ARG:0)\nRETURN\u30001\n")
	want := []byte("@F(ARG:0)\n    RETURN 1\n")
	for _, allow := range []bool{false, true} {
		result := FormatWithOptions(src, ERB, Options{AllowFullwidthSpace: allow})
		if allow {
			if len(result.Diagnostics) != 0 || !bytes.Equal(result.Output, want) {
				t.Fatalf("allow-fullwidth output=%q diagnostics=%v, want %q", result.Output, result.Diagnostics, want)
			}
			assertFullwidthIdempotent(t, result.Output, Options{AllowFullwidthSpace: true})
		} else {
			if !bytes.Equal(result.Output, src) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Severity != Unsupported {
				t.Fatalf("default output=%q diagnostics=%v, want unchanged unsupported input", result.Output, result.Diagnostics)
			}
		}
	}
}

func TestFullwidthOptionProtectsOpaquePayloads(t *testing.T) {
	src := []byte("@F\u3000(ARG:0)\nPRINTFORMW\u3000\u3000literal\nFOO\u3000=\u3000Y\u3000+\u30001\n; comment\u3000\u3000payload\n")
	result := FormatWithOptions(src, ERB, Options{AllowFullwidthSpace: true})
	want := []byte("@F(ARG:0)\n    PRINTFORMW\u3000\u3000literal\n    FOO = \u3000Y\u3000+\u30001\n    ; comment\u3000\u3000payload\n")
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", result.Diagnostics)
	}
	if !bytes.Equal(result.Output, want) {
		t.Fatalf("output = %q, want %q", result.Output, want)
	}
	if !bytes.Contains(result.Output, []byte("PRINTFORMW\u3000\u3000literal")) {
		t.Fatal("raw FORM command lost its second fullwidth delimiter")
	}
	for _, payload := range [][]byte{[]byte("literal\n"), []byte("Y\u3000+\u30001\n"), []byte("comment\u3000\u3000payload\n")} {
		if !bytes.Contains(result.Output, payload) {
			t.Fatalf("output %q lost payload %q", result.Output, payload)
		}
	}
	assertFullwidthIdempotent(t, result.Output, Options{AllowFullwidthSpace: true})
}

func TestFullwidthOptionFormatsKnownSyntax(t *testing.T) {
	src := []byte("@F\u3000(ARG:0)\n#DIM\u3000STATIC\u3000USR_X\u3000,\u30001\nPRINTV\u3000A\u3000+\u30001\nPRINT_RECT\u30001\u3000,\u30002\u3000,\u30003\u3000,\u30004\n")
	result := FormatWithOptions(src, ERB, Options{AllowFullwidthSpace: true})
	want := []byte("@F(ARG:0)\n    #DIM STATIC USR_X, 1\n\n    PRINTV A + 1\n    PRINT_RECT 1, 2, 3, 4\n")
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", result.Diagnostics)
	}
	if !bytes.Equal(result.Output, want) {
		t.Fatalf("output = %q, want %q", result.Output, want)
	}
	assertFullwidthIdempotent(t, result.Output, Options{AllowFullwidthSpace: true})
}

func TestFullwidthOptionFormatsRefDeclaration(t *testing.T) {
	src := []byte("@F\u3000(ARG:0)\n#DIM\u3000REF\u3000USR_X\u3000,\u3000,\nRETURN\u30000\n")
	result := FormatWithOptions(src, ERB, Options{AllowFullwidthSpace: true})
	want := []byte("@F(ARG:0)\n    #DIM REF USR_X,,\n\n    RETURN 0\n")
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", result.Diagnostics)
	}
	if !bytes.Equal(result.Output, want) {
		t.Fatalf("output = %q, want %q", result.Output, want)
	}
	assertFullwidthIdempotent(t, result.Output, Options{AllowFullwidthSpace: true})
}

func TestFullwidthOptionFormatsPreprocessorFields(t *testing.T) {
	src := []byte("@F\u3000(ARG:0)\n[IF\u3000USR_X]\nRETURN\u30001\n[ENDIF]\n")
	result := FormatWithOptions(src, ERB, Options{AllowFullwidthSpace: true})
	want := []byte("@F(ARG:0)\n[IF USR_X]\n    RETURN 1\n[ENDIF]\n")
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", result.Diagnostics)
	}
	if !bytes.Equal(result.Output, want) {
		t.Fatalf("output = %q, want %q", result.Output, want)
	}
	assertFullwidthIdempotent(t, result.Output, Options{AllowFullwidthSpace: true})
}

func TestFullwidthOptionSelectNonePreservesInput(t *testing.T) {
	src := []byte("@F\u3000(ARG:0)\nRETURN\u30001\n")
	result := FormatWithOptions(src, ERB, Options{AllowFullwidthSpace: true, Select: []RuleID{}})
	if len(result.Diagnostics) != 0 || !bytes.Equal(result.Output, src) || len(result.Changes) != 0 {
		t.Fatalf("select-none result=%q diagnostics=%v changes=%v", result.Output, result.Diagnostics, result.Changes)
	}
}

func TestFullwidthOptionSingleRuleAndUnicodeColumn(t *testing.T) {
	src := []byte("@F(ARG:0)\nRETURN A\u3000+\u30001\n")
	result := FormatWithOptions(src, ERB, Options{AllowFullwidthSpace: true, Select: []RuleID{BinarySpacing}})
	want := []byte("@F(ARG:0)\nRETURN A + 1\n")
	if len(result.Diagnostics) != 0 || !bytes.Equal(result.Output, want) {
		t.Fatalf("result=%q diagnostics=%v, want %q", result.Output, result.Diagnostics, want)
	}
	if len(result.Changes) != 2 || result.Changes[0] != (Change{Rule: BinarySpacing, Line: 2, Column: 9}) || result.Changes[1] != (Change{Rule: BinarySpacing, Line: 2, Column: 11}) {
		t.Fatalf("changes=%v, want S002 at Unicode columns 9 and 11", result.Changes)
	}
	assertFullwidthIdempotent(t, result.Output, Options{AllowFullwidthSpace: true, Select: []RuleID{BinarySpacing}})
}

func TestFullwidthOptionCommandSpacingLeavesExpressionGapDisabled(t *testing.T) {
	src := []byte("@F\u3000(ARG:0)\nRETURN\u3000A\u3000+\u30001\n")
	result := FormatWithOptions(src, ERB, Options{AllowFullwidthSpace: true, Select: []RuleID{CommandSpacing}})
	want := []byte("@F\u3000(ARG:0)\nRETURN A\u3000+\u30001\n")
	if len(result.Diagnostics) != 0 || !bytes.Equal(result.Output, want) {
		t.Fatalf("result=%q diagnostics=%v, want %q", result.Output, result.Diagnostics, want)
	}
}

func TestFullwidthReturnFormSeparatorRequiresPayload(t *testing.T) {
	options := Options{AllowFullwidthSpace: true, Select: []RuleID{BinarySpacing}}
	missing := []byte("@F\nRETURNFORM\u3000\n")
	result := FormatWithOptions(missing, ERB, options)
	if !bytes.Equal(result.Output, missing) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_syntax" || result.Diagnostics[0].Message != "missing FORM argument" {
		t.Fatalf("single fullwidth separator result=%q diagnostics=%v", result.Output, result.Diagnostics)
	}
	raw := []byte("@F\nRETURNFORM\u3000\u3000\n")
	result = FormatWithOptions(raw, ERB, options)
	if !bytes.Equal(result.Output, raw) || len(result.Diagnostics) != 0 {
		t.Fatalf("double fullwidth separator result=%q diagnostics=%v", result.Output, result.Diagnostics)
	}
}

func assertFullwidthIdempotent(t *testing.T, src []byte, options Options) {
	t.Helper()
	result := FormatWithOptions(src, ERB, options)
	if len(result.Diagnostics) != 0 || !bytes.Equal(result.Output, src) || len(result.Changes) != 0 {
		t.Fatalf("second pass result=%q diagnostics=%v changes=%v", result.Output, result.Diagnostics, result.Changes)
	}
}

func TestFullwidthUnavailableCommandAssignment(t *testing.T) {
	src := []byte("@F\nFORCE_QUIT　=　Y　+　1\n")
	r := FormatWithOptions(src, ERB, Options{AllowFullwidthSpace: true, Dialect: Emuera1824, Select: []RuleID{AssignmentLeftSpacing, AssignmentRightSpacing}})
	if string(r.Output) != "@F\nFORCE_QUIT = 　Y　+　1\n" || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%v", r.Output, r.Diagnostics)
	}
}

func TestFullwidthContinuationHeaderMarker(t *testing.T) {
	separator := ""
	r := FormatWithOptions([]byte("{\n@　F\n}\nRETURN 0\n"), ERB, Options{AllowFullwidthSpace: true, ContinuationSeparator: &separator, Select: []RuleID{LabelMarkerSpacing}})
	if string(r.Output) != "{\n@F\n}\nRETURN 0\n" || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%v", r.Output, r.Diagnostics)
	}
}

func TestRegisteredRawSemicolonConsumesOneByte(t *testing.T) {
	r := FormatWithOptions([]byte("@F\nINPUTS;hello {A+1}\n"), ERB, Options{Select: []RuleID{BinarySpacing}})
	if string(r.Output) != "@F\nINPUTS;hello {A + 1}\n" || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%v", r.Output, r.Diagnostics)
	}
}
