package cli

import (
	"strconv"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestCorpusDiagnosticGroupsCaptureLateCategories(t *testing.T) {
	var groups corpusDiagnosticAccumulator
	for i := 0; i < 20; i++ {
		groups.add("first.ERB", format.Diagnostic{Line: 2, Column: 1, Severity: format.Unsupported, Code: "unsupported_expression", Message: "unknown expression"})
	}
	groups.add("late.ERB", format.Diagnostic{Line: 7, Column: 4, Severity: format.Warning, Code: "branch_after_final", Message: "branch follows final branch"})

	got, omitted := groups.snapshot()
	if omitted != 0 || len(got) != 2 {
		t.Fatalf("groups=%+v omitted=%d", got, omitted)
	}
	late := got[1]
	if late.Severity != format.Warning || late.Code != "branch_after_final" || late.Count != 1 || late.Representative.Path != "late.ERB" || late.Representative.Line != 7 || late.Representative.Column != 4 || len(late.Reasons) != 1 || late.Reasons[0].Reason != "branch follows final branch" {
		t.Fatalf("late group=%+v", late)
	}
}

func TestCorpusDiagnosticGroupsNormalizeAndSelectRepresentative(t *testing.T) {
	var groups corpusDiagnosticAccumulator
	groups.add("z.ERB", format.Diagnostic{Line: 9, Column: 2, Severity: format.Error, Code: "invalid_structure", Message: "  unclosed   IF before line 42  "})
	groups.add("a.ERB", format.Diagnostic{Line: 3, Column: 1, Severity: format.Error, Code: "invalid_structure", Message: "unclosed IF before line 99"})

	got, omitted := groups.snapshot()
	if omitted != 0 || len(got) != 1 || got[0].Count != 2 || len(got[0].Reasons) != 1 || got[0].Reasons[0].Count != 2 || got[0].Reasons[0].Reason != "unclosed IF before line <line>" {
		t.Fatalf("groups=%+v omitted=%d", got, omitted)
	}
	if got[0].Representative.Path != "a.ERB" || got[0].Representative.Line != 3 || got[0].Representative.Column != 1 || got[0].Reasons[0].Representative.Path != "a.ERB" {
		t.Fatalf("representative=%+v", got[0].Representative)
	}
}

func TestCorpusDiagnosticReasonKeepsUnsupportedCommand(t *testing.T) {
	var groups corpusDiagnosticAccumulator
	commands := map[string]bool{"FUTURE": true, "FOO\u00a0BAR": true, "FOO\u00a0BAZ": true}
	for command := range commands {
		groups.add("command.ERB", format.Diagnostic{Severity: format.Unsupported, Code: "unsupported_command", Message: "unsupported command " + command + " (runtime configuration)"})
	}

	got, omitted := groups.snapshot()
	if omitted != 0 || len(got) != 1 || len(got[0].Reasons) != len(commands) {
		t.Fatalf("categories=%+v omitted=%d", got, omitted)
	}
	for _, reason := range got[0].Reasons {
		if reason.Reason != "unsupported command (runtime configuration)" || !commands[reason.Command] || reason.Count != 1 {
			t.Fatalf("reason=%+v", reason)
		}
	}
}

func TestCorpusDiagnosticGroupsBounded(t *testing.T) {
	var groups corpusDiagnosticAccumulator
	for i := 0; i < corpusMaxDiagnosticReasons+8; i++ {
		groups.add("many.ERB", format.Diagnostic{Line: i + 1, Column: 1, Severity: format.Unsupported, Code: "unsupported_command", Message: "command-" + strconv.Itoa(i)})
	}
	for i := 0; i < 7; i++ {
		groups.add("many.ERB", format.Diagnostic{Line: 100 + i, Column: 1, Severity: format.Unsupported, Code: "unsupported_command", Message: "command-" + strconv.Itoa(corpusMaxDiagnosticReasons+1)})
	}
	groups.add("late.ERB", format.Diagnostic{Line: 1, Column: 1, Severity: format.Error, Code: "resource_limit", Message: "resource limit"})

	got, omitted := groups.snapshot()
	if len(got) != 2 || omitted != 0 {
		t.Fatalf("categories=%+v omitted=%d", got, omitted)
	}
	var unsupported, late corpusDiagnosticCategory
	for _, category := range got {
		switch category.Code {
		case "unsupported_command":
			unsupported = category
		case "resource_limit":
			late = category
		}
	}
	if len(unsupported.Reasons) != corpusMaxDiagnosticReasons || unsupported.ReasonOccurrencesOmitted != 15 {
		t.Fatalf("unsupported category=%+v", unsupported)
	}
	if late.Code != "resource_limit" || late.Representative.Path != "late.ERB" || len(late.Reasons) != 1 {
		t.Fatalf("late category=%+v", late)
	}
	for i := 0; i < corpusMaxDiagnosticCategories; i++ {
		groups.add("categories.ERB", format.Diagnostic{Severity: format.Unsupported, Code: "category-" + strconv.Itoa(i), Message: "reason"})
	}
	got, omitted = groups.snapshot()
	if len(got) != corpusMaxDiagnosticCategories || omitted != 2 {
		t.Fatalf("category limit: len=%d omitted=%d", len(got), omitted)
	}
}

func TestCorpusDiagnosticReasonHashDistinguishesLongMessages(t *testing.T) {
	var groups corpusDiagnosticAccumulator
	prefix := strings.Repeat("a", corpusMaxDiagnosticReason)
	groups.add("a.ERB", format.Diagnostic{Severity: format.Unsupported, Code: "unsupported_expression", Message: prefix + "first"})
	groups.add("b.ERB", format.Diagnostic{Severity: format.Unsupported, Code: "unsupported_expression", Message: prefix + "second"})

	got, omitted := groups.snapshot()
	if omitted != 0 || len(got) != 1 || len(got[0].Reasons) != 2 || got[0].Reasons[0].Reason == got[0].Reasons[1].Reason {
		t.Fatalf("categories=%+v omitted=%d", got, omitted)
	}
	for _, reason := range got[0].Reasons {
		if !strings.Contains(reason.Reason, "sha256=") {
			t.Fatalf("long reason lacks digest: %+v", reason)
		}
	}
}
