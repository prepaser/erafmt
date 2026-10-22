package format

import (
	"testing"
)

func TestScalarIgnoredSurplusPreservesBody(t *testing.T) {
	for _, source := range []string{"REDRAW  1+2,!!\n", "CLEARLINE  1+2,\"x\"\n", "CUPCHECK  1+2,3\n", "CALLTRAIN  1+2,3\n", "DELDATA  1+2,3\n", "PRINT_ABL  1+2,3\n", "PRINTS  \"x\"+\"y\",1\n", "PRINTSL  \"x\",!!\n"} {
		r := FormatWithOptions([]byte(source), ERB, Options{Select: []RuleID{BinarySpacing, CommandSpacing, CommaSpacing}})
		if string(r.Output) != source || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "ignored_arguments" || r.Diagnostics[0].Severity != Warning {
			t.Fatalf("source=%q output=%q diagnostics=%+v", source, r.Output, r.Diagnostics)
		}
	}
	for _, source := range []string{"REDRAW EFA_MACRO,!!\n", "REDRAW 1,\n", "RETURN 1,2\n"} {
		r := FormatWithOptions([]byte(source), ERB, Options{Select: []RuleID{BinarySpacing}})
		if len(r.Diagnostics) != 0 || string(r.Output) != source {
			t.Fatalf("source=%q output=%q diagnostics=%+v", source, r.Output, r.Diagnostics)
		}
	}
	for _, source := range []string{"REDRAW \"x\",1\n", "REDRAW !!1,2\n", "PRINTS 1,\"x\"\n"} {
		r := FormatWithOptions([]byte(source), ERB, Options{Select: []RuleID{BinarySpacing}})
		if len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported || string(r.Output) != source {
			t.Fatalf("source=%q output=%q diagnostics=%+v", source, r.Output, r.Diagnostics)
		}
	}
}
