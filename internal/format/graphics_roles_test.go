package format

import "testing"

func TestGraphicsConstantCompletenessWithSelectedRules(t *testing.T) {
	for _, test := range []struct {
		source string
		rule   RuleID
	}{
		{"PRINT_IMG !!1\n", CodeTrailingWhitespace},
		{"PRINT_SPACE !!1\n", CodeTrailingWhitespace},
		{"PRINT_SPACE !!1  px\n", PixelUnitSpacing},
	} {
		r := FormatWithOptions([]byte(test.source), ERB, Options{Dialect: EMEE, Select: []RuleID{test.rule}})
		if len(r.Diagnostics) != 1 || string(r.Output) != test.source {
			t.Fatalf("source=%q output=%q diagnostics=%+v", test.source, r.Output, r.Diagnostics)
		}
	}
}

func TestBaseGraphicsRolesRejectKnownOppositeTypes(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824} {
		for _, source := range []string{
			"PRINT_IMG 1\n",
			"PRINT_IMG RESULT\n",
			"PRINT_SPACE RESULTS\n",
			"PRINT_RECT CSTR,1,2,3\n",
		} {
			r := FormatWithOptions([]byte(source), ERB, Options{Dialect: dialect, Select: []RuleID{BinarySpacing}})
			if len(r.Diagnostics) == 0 {
				t.Fatalf("dialect=%d accepted %q", dialect, source)
			}
			if string(r.Output) != source {
				t.Fatalf("dialect=%d changed invalid %q to %q", dialect, source, r.Output)
			}
		}
	}
}

func TestBaseGraphicsRolesAcceptKnownTypesAndDeferUnknowns(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824} {
		source := "PRINT_IMG CSTR\nPRINT_SPACE RESULT\nPRINT_RECT CFLAG,1,2,3\nPRINT_IMG NAME\nPRINT_SPACE NAME:0\n"
		r := FormatWithOptions([]byte(source), ERB, Options{Dialect: dialect, Select: []RuleID{BinarySpacing}})
		if len(r.Diagnostics) != 0 || string(r.Output) != source {
			t.Fatalf("dialect=%d output=%q diagnostics=%+v", dialect, r.Output, r.Diagnostics)
		}
	}
}

func TestEMEEGraphicsKeepsGetterCoercions(t *testing.T) {
	source := []byte("PRINT_IMG 1\nPRINT_SPACE RESULTS\nPRINT_RECT CSTR,1,2,3\n")
	r := FormatWithOptions(source, ERB, Options{Dialect: EMEE, Select: []RuleID{BinarySpacing}})
	if len(r.Diagnostics) != 0 || string(r.Output) != string(source) {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestBaseGraphicsRolesDeferIndexedMacroExpressions(t *testing.T) {
	source := []byte("PRINT_IMG RESULT:EFAUDIT_INDEX\nPRINT_SPACE CSTR:EFAUDIT_INDEX\nPRINT_RECT RESULT:EFAUDIT_INDEX,1,2,3\n")
	for _, dialect := range []Dialect{CommonDialect, Emuera1824} {
		r := FormatWithOptions(source, ERB, Options{Dialect: dialect, Select: []RuleID{BinarySpacing}})
		if len(r.Diagnostics) != 0 || string(r.Output) != string(source) {
			t.Fatalf("dialect=%d output=%q diagnostics=%+v", dialect, r.Output, r.Diagnostics)
		}
	}
}

func TestGraphicsCallsCanOverrideBuiltinReturnTypes(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		for _, source := range []string{
			"PRINT_IMG ABS(\"x\")\n",
			"PRINT_SPACE TOUPPER(1)\n",
			"PRINT_IMG RESULT(\"x\")\n",
			"PRINT_RECT RESULTS(1),1,2,3\n",
		} {
			r := FormatWithOptions([]byte(source), ERB, Options{Dialect: dialect, Select: []RuleID{BinarySpacing}})
			if len(r.Diagnostics) != 0 || string(r.Output) != source {
				t.Fatalf("dialect=%d source=%q output=%q diagnostics=%+v", dialect, source, r.Output, r.Diagnostics)
			}
		}
	}
}

func TestGraphicsMarkersDoNotJoinMacroNames(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		source := "PRINT_IMG RE;!;SULT\nPRINT_SPACE RES;!;ULTS\nPRINT_RECT RES;!;ULTS,1,2,3\nPRINT_IMG D;!;AY\n"
		r := FormatWithOptions([]byte(source), ERB, Options{Dialect: dialect, Select: []RuleID{BinarySpacing}})
		if len(r.Diagnostics) != 0 || string(r.Output) != source {
			t.Fatalf("dialect=%d output=%q diagnostics=%+v", dialect, r.Output, r.Diagnostics)
		}
	}
}

func TestBaseGraphicsLiteralRangeAdvisory(t *testing.T) {
	source := "PRINT_SPACE -1\nPRINT_RECT 0,1,2,2147483648\nPRINT_SPACE 2147483647\n"
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		r := FormatWithOptions([]byte(source), ERB, Options{Dialect: dialect, Select: []RuleID{BinarySpacing}})
		want := 2
		if dialect == EMEE {
			want = 0
		}
		if string(r.Output) != source || len(r.Diagnostics) != want {
			t.Fatalf("dialect=%d output=%q diagnostics=%+v", dialect, r.Output, r.Diagnostics)
		}
		for _, d := range r.Diagnostics {
			if d.Severity != Warning || d.Code != "suspicious_graphics_range" {
				t.Fatalf("unexpected diagnostic: %+v", d)
			}
		}
	}
}
