package format

import "testing"

func TestFormAlignmentRejectsKnownSystemNamesAfterProvenFields(t *testing.T) {
	rules := allFormRules(t)
	for _, name := range []string{"RESULT", "RESULTS", "CFLAG", "CSTR"} {
		for _, source := range []string{"{1,2," + name + "}", "%\"text\",2," + name + "%"} {
			edits, err := formEdits([]byte(source), rules, formLine)
			if err == nil {
				t.Fatalf("alignment %q was accepted", source)
			}
			if got := string(applyEdits([]byte(source), edits)); got != source {
				t.Fatalf("invalid alignment %q changed to %q", source, got)
			}
		}
	}
}

func TestFormAlignmentPreservesDeferredAndCompatibleShapes(t *testing.T) {
	rules, err := resolveRules(Options{Select: []RuleID{FormPadding}})
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{
		"{EFAUDIT_VALUE,2,CSTR}",
		"{1,EFAUDIT_WIDTH,CSTR}",
		"{1,2,ALIGN(X)}",
		"{1,2,CSTR;!;}",
	} {
		edits, err := formEdits([]byte(source), rules, formLine)
		if err != nil {
			t.Fatalf("deferred alignment %q: %v", source, err)
		}
		if got := string(applyEdits([]byte(source), edits)); got != source {
			t.Fatalf("deferred alignment %q changed to %q", source, got)
		}
	}
}

func TestFormAlignmentRejectsKnownRolePunctuationTail(t *testing.T) {
	rules, err := resolveRules(Options{Select: []RuleID{FormPadding}})
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"{1,2,RIGHT+1}", "{1,2,RIGHT[0]}", "{1,2,RIGHT(1)}"} {
		edits, err := formEdits([]byte(source), rules, formLine)
		if err == nil {
			t.Fatalf("punctuation tail %q was accepted", source)
		}
		if got := string(applyEdits([]byte(source), edits)); got != source {
			t.Fatalf("invalid alignment %q changed to %q", source, got)
		}
	}
}

func TestFormAlignmentKeepsNeighboringValidIsland(t *testing.T) {
	rules := allFormRules(t)
	source := []byte("x {1,2,CSTR} y { A+B }")
	edits, err := formEdits(source, rules, formLine)
	if err == nil {
		t.Fatal("invalid alignment was accepted")
	}
	if got, want := string(applyEdits(source, edits)), "x {1,2,CSTR} y {A + B}"; got != want {
		t.Fatalf("output=%q want=%q", got, want)
	}
}
