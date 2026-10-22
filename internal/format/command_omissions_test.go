package format

import (
	"bytes"

	"testing"
)

func TestCommandOmittedFields(t *testing.T) {
	for _, test := range []struct {
		source string
		bad    bool
	}{
		{"@F\nIF\nENDIF\n", false},
		{"@F\nIF ,1\nENDIF\n", true},
		{"@F\nIF 1,,2\nENDIF\n", false},
		{"@F\nRETURN 1,,2\n", true},
		{"@F\nRETURN 1,\n", false},
		{"@F\nRETURNF 1,,2\n", false},
		{"@F\nPRINTS\n", true},
		{"@F\nPRINTFORMS ; note\n", true},
		{"@F\nSELECTCASE\nCASE 1\nENDSELECT\n", true},
		{"@F\nSELECTCASE 1\nCASE\nENDSELECT\n", true},
		{"@F\nSELECTCASE 1\nCASE 1,,2\nENDSELECT\n", true},
		{"@F\nSELECTCASE 1\nCASE IS >0,,2\nENDSELECT\n", true},
		{"@F\nSELECTCASE 1\nCASE 0 TO 1,,2\nENDSELECT\n", true},
		{"@F\nRETURN MACRO,,2\n", false},
		{"@F\nRETURN ,1;!;\n", false},
	} {
		for _, options := range []Options{{}, {Select: []RuleID{CodeTrailingWhitespace}}, {Select: []RuleID{}}} {
			result := FormatWithOptions([]byte(test.source), ERB, options)
			want := test.bad && (options.Select == nil || len(options.Select) != 0)
			if (len(result.Diagnostics) != 0) != want {
				t.Fatalf("source=%q options=%+v diagnostics=%+v", test.source, options, result.Diagnostics)
			}
			again := FormatWithOptions(result.Output, ERB, options)
			if !bytes.Equal(result.Output, again.Output) || len(again.Changes) != 0 {
				t.Fatalf("source=%q not idempotent: %q -> %q", test.source, result.Output, again.Output)
			}
		}
	}
}
