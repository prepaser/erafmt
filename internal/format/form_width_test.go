package format

import (
	"bytes"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestFormWidthAdvisoryUsesRuntimeIntCast(t *testing.T) {
	rules, err := resolveRules(Options{Select: []RuleID{FormPadding}})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		source string
		count  int
	}{{"{1,-1}", 1}, {"{1,2147483648}", 1}, {"{1,4294967296}", 0}, {"{1+RESULT,-1}", 1}, {"{USR,-1}", 0}, {"{ABS(1),-1}", 0}, {"{1,USR}", 0}, {"%RESULTS,-1%", 0}} {
		session := erabasic.NewExpressionSession([]byte(test.source))
		_, err := formEditsWithSession(session, erabasic.ExpressionSourceOriginal, []byte(test.source), 0, rules, erabasic.FormLine, true)
		if err != nil {
			t.Fatalf("source=%q: unexpected hard error: %v", test.source, err)
		}
		advisories := session.FormAdvisoriesIn(erabasic.ExpressionSourceOriginal, 0, len(test.source))
		if len(advisories) != test.count {
			t.Fatalf("source=%q: advisories=%+v, want %d", test.source, advisories, test.count)
		}
	}
}

func TestFormWidthAdvisoryDoesNotReplaceHardError(t *testing.T) {
	rules, err := resolveRules(Options{Select: []RuleID{FormPadding}})
	if err != nil {
		t.Fatal(err)
	}
	source := []byte("{1,-1}{,2}")
	session := erabasic.NewExpressionSession(source)
	_, err = formEditsWithSession(session, erabasic.ExpressionSourceOriginal, source, 0, rules, erabasic.FormLine, true)
	if err == nil || err.Error() != "expression at byte 7: empty FORM expression" {
		t.Fatalf("hard error=%v, want empty second FORM expression", err)
	}
	advisories := session.FormAdvisoriesIn(erabasic.ExpressionSourceOriginal, 0, len(source))
	if len(advisories) != 1 || advisories[0].Code != "suspicious_form_width" {
		t.Fatalf("advisories=%+v, want one width advisory", advisories)
	}
}

func TestFormWidthAdvisoryReachesFormatResult(t *testing.T) {
	source := []byte("@F\n    PRINTFORM {1,-1}{ 2 + 3 }\n")
	result := FormatWithOptions(source, ERB, Options{Dialect: CommonDialect})
	want := []byte("@F\n    PRINTFORM {1,-1}{2 + 3}\n")
	if !bytes.Equal(result.Output, want) {
		t.Fatalf("output=%q want=%q diagnostics=%+v", result.Output, want, result.Diagnostics)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Severity != Warning || result.Diagnostics[0].Code != "suspicious_form_width" {
		t.Fatalf("diagnostics=%+v, want one width warning", result.Diagnostics)
	}
	if result.Diagnostics[0].Line != 2 || result.Diagnostics[0].Column <= 1 {
		t.Fatalf("diagnostic position=%+v, want physical line 2", result.Diagnostics[0])
	}
}

func TestFormWidthWarningSurvivesHardError(t *testing.T) {
	for _, source := range [][]byte{[]byte("@F\n    PRINTFORM {1,-1}{,2}\n"), []byte("@F\n    PRINTFORM {,2}{1,-1}\n"), []byte("@F\n    PRINTFORM {1,-1,RIGHT+1}\n")} {
		result := FormatWithOptions(source, ERB, Options{Dialect: CommonDialect})
		if !bytes.Equal(result.Output, source) {
			t.Fatalf("hard-error line changed: output=%q source=%q diagnostics=%+v", result.Output, source, result.Diagnostics)
		}
		var warning, hard bool
		for _, diagnostic := range result.Diagnostics {
			warning = warning || diagnostic.Code == "suspicious_form_width" && diagnostic.Severity == Warning
			hard = hard || diagnostic.Code == "unsupported_expression"
		}
		if !warning || !hard {
			t.Fatalf("source=%q diagnostics=%+v, want width warning and hard expression diagnostic", source, result.Diagnostics)
		}
	}
}

func TestFormWidthWarningReachesNestedForms(t *testing.T) {
	separator := " "
	for _, test := range []struct {
		name   string
		source string
	}{
		{"returnform", "@F\n    RETURNFORM \"x {1,-1}\"\n"},
		{"expression", "@F\n    PRINTBUTTON @\"x {1,-1}\", \"reply\"\n"},
		{"printv", "@F\n    PRINTV @\"x {1,-1}\"\n"},
		{"continuation", "@F\n{\n    PRINTFORM {1,-1}\n}\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := Options{Dialect: CommonDialect}
			if test.name == "continuation" {
				options.ContinuationSeparator = &separator
			}
			result := FormatWithOptions([]byte(test.source), ERB, options)
			count := 0
			line := 0
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Code == "suspicious_form_width" && diagnostic.Severity == Warning {
					count++
					line = diagnostic.Line
				}
			}
			if count != 1 {
				t.Fatalf("output=%q diagnostics=%+v, want one width warning", result.Output, result.Diagnostics)
			}
			wantLine := 2
			if test.name == "continuation" {
				wantLine = 3
			}
			if line != wantLine {
				t.Fatalf("diagnostics=%+v, width warning line=%d want=%d", result.Diagnostics, line, wantLine)
			}
			again := FormatWithOptions(result.Output, ERB, options)
			if !bytes.Equal(again.Output, result.Output) || len(again.Changes) != 0 || len(again.Diagnostics) != len(result.Diagnostics) {
				t.Fatalf("second pass diagnostics=%+v, first=%+v", again.Diagnostics, result.Diagnostics)
			}
		})
	}
}

func TestFormAdvisoryMappingSearchAndColumnFallback(t *testing.T) {
	for _, test := range []struct {
		name        string
		existing    []Diagnostic
		mapped      []int
		wantColumns []int
	}{
		{name: "same warning is suppressed", existing: []Diagnostic{{Line: 1, Column: 1, Severity: Warning, Code: "suspicious_form_width"}}, mapped: []int{0}, wantColumns: []int{1}},
		{name: "different code remains visible", existing: []Diagnostic{{Line: 1, Column: 1, Severity: Warning, Code: "other"}}, mapped: []int{0}, wantColumns: []int{1, 1}},
		{
			name:        "old warnings remain searchable after skipping new warnings",
			existing:    []Diagnostic{{Line: 1, Column: 4, Severity: Warning, Code: "suspicious_form_width"}},
			mapped:      []int{0, 1, 3},
			wantColumns: []int{4, 1, 2},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			src := []byte("12345")
			p := formatter{DocumentSession: erabasic.DocumentSession{Src: src, Lines: linesOf(src)}, diagnostics: append([]Diagnostic(nil), test.existing...), statementDiagnostic: 0}
			session := erabasic.NewExpressionSession(src)
			for offset := range test.mapped {
				session.AddFormAdvisory(erabasic.FormAdvisory{Source: erabasic.ExpressionSourceOriginal, Offset: offset, Code: "suspicious_form_width", Message: "width"})
			}
			p.addFormAdvisoriesMapped(0, session, 0, len(src), func(offset int) int {
				return test.mapped[offset]
			})
			if len(p.diagnostics) != len(test.wantColumns) {
				t.Fatalf("diagnostics=%+v, want columns %v", p.diagnostics, test.wantColumns)
			}
			for i, column := range test.wantColumns {
				if p.diagnostics[i].Column != column {
					t.Fatalf("diagnostics=%+v, want columns %v", p.diagnostics, test.wantColumns)
				}
			}
		})
	}
	src := []byte(strings.Repeat("x", 4095) + "α€🙂tail")
	mapped := []int{4095, 4096, 4097, 4099, 4100, 4101, 4102, 4103, 4104}
	p := formatter{DocumentSession: erabasic.DocumentSession{Src: src, Lines: linesOf(src)}}
	session := erabasic.NewExpressionSession(src)
	for offset := range mapped {
		session.AddFormAdvisory(erabasic.FormAdvisory{Source: erabasic.ExpressionSourceOriginal, Offset: offset, Code: "suspicious_form_width", Message: "width"})
	}
	wantColumns := make([]int, 0, len(mapped))
	seen := make(map[int]bool)
	for _, offset := range mapped {
		column := erabasic.Column(src, p.Lines[0], offset)
		if !seen[column] {
			seen[column] = true
			wantColumns = append(wantColumns, column)
		}
	}
	p.addFormAdvisoriesMapped(0, session, 0, len(src), func(offset int) int {
		return mapped[offset]
	})
	if len(p.diagnostics) != len(wantColumns) {
		t.Fatalf("partial UTF-8 diagnostics=%+v, want %d unique columns", p.diagnostics, len(wantColumns))
	}
	for i, want := range wantColumns {
		if p.diagnostics[i].Column != want {
			t.Fatalf("partial UTF-8 diagnostics=%+v, want column %d at index %d", p.diagnostics, want, i)
		}
	}
}

func TestFormAdvisoryMappingPreservesMetadataAbort(t *testing.T) {
	src := []byte(strings.Repeat("x", 4096))
	p := formatter{DocumentSession: erabasic.DocumentSession{Src: src, Lines: linesOf(src), Metadata: erabasic.NewMetadataBudget(erabasic.MaxMetadataBytes - 1)}}
	session := erabasic.NewExpressionSession(src)
	session.AddFormAdvisory(erabasic.FormAdvisory{Source: erabasic.ExpressionSourceOriginal, Offset: 0, Code: "suspicious_form_width", Message: "width"})
	p.addFormAdvisoriesMapped(0, session, 0, len(src), func(int) int {
		return 4096
	})
	if !p.Aborted || len(p.diagnostics) != 1 || p.diagnostics[0].Code != "resource_limit" || p.diagnostics[0].Severity != Error {
		t.Fatalf("aborted=%t metadata=%d diagnostics=%+v", p.Aborted, p.Metadata.Used(), p.diagnostics)
	}
}
