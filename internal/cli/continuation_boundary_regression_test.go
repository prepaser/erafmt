package cli

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestContinuationNumericSeparatorBoundaryPreservation(t *testing.T) {
	source := []byte("@F\n{\nPRINTV 0\n 0\nA000\n}\n")
	for _, separator := range []string{"A", "\ufeff"} {
		result := format.FormatWithOptions(source, format.ERB, format.Options{ContinuationSeparator: &separator})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("separator=%q diagnostics=%+v output=%q", separator, result.Diagnostics, result.Output)
		}
		preservation := sourcePreservationOptions{continuationSeparator: &separator}
		if err := checkSourcePreservedOptions(source, result.Output, preservation); err != nil {
			t.Fatalf("separator=%q output=%q: %v", separator, result.Output, err)
		}
		mutated := bytes.Replace(result.Output, []byte("A000"), []byte("B000"), 1)
		if bytes.Equal(mutated, result.Output) {
			t.Fatal("numeric-boundary mutation did not apply")
		}
		if err := checkSourcePreservedOptions(source, mutated, preservation); err == nil {
			t.Fatalf("separator=%q accepted semantic mutation: %q", separator, mutated)
		}
	}
}

func TestContinuationNumericSeparatorBoundaryRetainsProtectedRows(t *testing.T) {
	rows := func(head string) []checkedLine {
		return []checkedLine{{body: []byte(head)}, {body: []byte("A000")}}
	}
	for _, head := range []string{
		";!; PRINTV 0",
		"PRINTV \x00 0",
		"PRINTV ; comment 0",
		`PRINTV "quoted" 0`,
		"PRINTV `raw` 0",
	} {
		if continuationRowHasNumericIdentifierBoundary(trimGrammarSpace([]byte(head), false), rows(head), 0, "A") {
			t.Fatalf("protected row was eligible for numeric boundary normalization: %q", head)
		}
	}

	separator := "A"
	rowsA := checkedLines([]byte("PRINTV 0\n 0\nA000\n"))
	rowsB := checkedLines([]byte("PRINTV 0\n 0\nA000\n"))
	if bytes.Equal(continuationFingerprint(rowsA, separator, format.CommonDialect, false), continuationFingerprint(rowsB, "B", format.CommonDialect, false)) {
		t.Fatal("continuation separator bytes were omitted from the boundary fingerprint")
	}
}

func TestContinuationZeroEndProvenAssignmentAndIncrementRoutes(t *testing.T) {
	separator := ""
	options := format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true, ContinuationSeparator: &separator}
	for _, test := range []struct {
		name, body, from, to string
	}{
		{"assignment", "TEXT_VALUE =“text”", "text", "TEXT"},
		{"prefix-increment", "++ A", "A", "B"},
		{"unicode-prefix-increment", "++ αβ", "αβ", "αγ"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := []byte("@F\n{\n" + test.body + "\n}\n")
			result := format.FormatWithOptions(source, format.ERB, options)
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
			}
			preservation := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true, continuationSeparator: &separator}
			if err := checkSourcePreservedOptions(source, result.Output, preservation); err != nil {
				t.Fatalf("output=%q: %v", result.Output, err)
			}
			mutated := bytes.Replace(result.Output, []byte(test.from), []byte(test.to), 1)
			if bytes.Equal(mutated, result.Output) {
				t.Fatal("semantic mutation did not apply")
			}
			if err := checkSourcePreservedOptions(source, mutated, preservation); err == nil {
				t.Fatalf("accepted semantic mutation: %q", mutated)
			}
		})
	}

	for _, head := range []string{"++ (A)", "+ + A", "++ A+1", "++ UNKNOWN?"} {
		kind, _, exact := continuationLogicalFingerprint([]byte(head), format.EMEE, true)
		if kind != "unknown" || !exact {
			t.Fatalf("malformed increment was trusted: head=%q kind=%q exact=%v", head, kind, exact)
		}
	}
}

func TestContinuationPrintVQuotedFormBoundary(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"mixed-row", `PRINTV 0,@"literal { 1+2 } tail"`},
		{"split-expression", "PRINTV\n 0&&1/0,@\"literal { 1+2 } tail\""},
		{"split-raw-prefix", "PRINTV\n 0,'raw  text',@\"literal { 1+2 } tail\""},
		{"split-numeric-boundary", "PRINTV\n 0A000,@\"literal { 1+2 } tail\""},
		{"split-nul-identifier", "PRINTV\n A\x00B+1,@\"literal { 1+2 } tail\""},
		{"ordinary-prefix", `PRINTV 0,"plain  text",@"literal { 1+2 } tail"`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			source := []byte("@F\n{\n" + test.body + "\n}\n")
			for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
				for _, separator := range []string{"", " "} {
					result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: dialect, ContinuationSeparator: &separator})
					if len(result.Diagnostics) != 0 {
						t.Fatalf("dialect=%d separator=%q diagnostics=%+v output=%q", dialect, separator, result.Diagnostics, result.Output)
					}
					options := sourcePreservationOptions{dialect: dialect, continuationSeparator: &separator}
					if err := checkSourcePreservedOptions(source, result.Output, options); err != nil {
						t.Fatalf("dialect=%d separator=%q output=%q: %v", dialect, separator, result.Output, err)
					}
				}
			}
		})
	}

	separator := " "
	source := []byte("@F\n{\nPRINTV\n 0&&1/0,@\"literal { 1+2 } tail\"\n}\n")
	result := format.FormatWithOptions(source, format.ERB, format.Options{ContinuationSeparator: &separator})
	mutated := bytes.Replace(result.Output, []byte("1 + 2"), []byte("1 + 3"), 1)
	if bytes.Equal(mutated, result.Output) {
		t.Fatal("quoted FORM mutation did not apply")
	}
	if err := checkSourcePreservedOptions(source, mutated, sourcePreservationOptions{continuationSeparator: &separator}); err == nil {
		t.Fatalf("accepted quoted FORM mutation: %q", mutated)
	}
}

func TestContinuationPrintVQuotedFormBoundaryKeepsExactPrefixes(t *testing.T) {
	separator := " "
	for _, prefix := range []string{
		"9223372036854775808g00,",
		"0x1p-2g00,",
		"A # .,",
		";!; 0,",
		"[[ALIAS]],",
		"A = 1,",
	} {
		source := []byte("@F\n{\nPRINTV\n " + prefix + "@\"literal { 1+2 } tail\"\n}\n")
		result := format.FormatWithOptions(source, format.ERB, format.Options{ContinuationSeparator: &separator})
		var mutated []byte
		for _, from := range []string{"1 + 2", "1+2"} {
			if bytes.Contains(result.Output, []byte(from)) {
				mutated = bytes.Replace(result.Output, []byte(from), []byte("1 + 3"), 1)
				break
			}
		}
		if mutated == nil {
			t.Fatalf("prefix %q produced no mutable FORM expression: %q", prefix, result.Output)
		}
		if err := checkSourcePreservedOptions(source, mutated, sourcePreservationOptions{continuationSeparator: &separator}); err == nil {
			t.Fatalf("prefix %q accepted quoted FORM mutation: %q", prefix, mutated)
		}
	}
}

func TestContinuationPrintVQuotedFormBoundaryRejectsPrefixMutations(t *testing.T) {
	separator := " "
	for _, test := range []struct {
		source, from, to string
	}{
		{"@F\n{\nPRINTV\n 0A000,@\"literal { 1+2 } tail\"\n}\n", "0 A000", "0 A001"},
		{"@F\n{\nPRINTV\n A\x00B+1,@\"literal { 1+2 } tail\"\n}\n", "A\x00B", "A\x00C"},
	} {
		source := []byte(test.source)
		result := format.FormatWithOptions(source, format.ERB, format.Options{ContinuationSeparator: &separator})
		proof := sourcePreservationOptions{continuationSeparator: &separator}
		if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
			t.Fatalf("output=%q: %v", result.Output, err)
		}
		mutated := bytes.Replace(result.Output, []byte(test.from), []byte(test.to), 1)
		if bytes.Equal(mutated, result.Output) {
			t.Fatalf("mutation %q -> %q did not apply to %q", test.from, test.to, result.Output)
		}
		if err := checkSourcePreservedOptions(source, mutated, proof); err == nil {
			t.Fatalf("accepted prefix mutation: %q", mutated)
		}
	}
}

func TestContinuationPrintVQuotedFormBoundaryFullwidth(t *testing.T) {
	separator := " "
	source := []byte("@F\n{\nPRINTV　0,　@\"literal　{　1+2　}　tail\"\n}\n")
	options := format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true, ContinuationSeparator: &separator}
	result := format.FormatWithOptions(source, format.ERB, options)
	proof := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true, continuationSeparator: &separator}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
		t.Fatalf("fullwidth output rejected: %v output=%q", err, result.Output)
	}
	mutated := bytes.Replace(result.Output, []byte("tail"), []byte("TAIL"), 1)
	if err := checkSourcePreservedOptions(source, mutated, proof); err == nil {
		t.Fatalf("accepted fullwidth quoted FORM mutation: %q", mutated)
	}
}

func TestContinuationPrintVQuotedFormBoundarySeparators(t *testing.T) {
	source := []byte("@F\n{\nPRINTV 0,@\"literal { 1+2 } tail\"\n}\n")
	for _, separator := range []string{"", " ", "+", "A", "\ufeff"} {
		result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: format.EMEE, ContinuationSeparator: &separator})
		if separator == "A" {
			if len(result.Diagnostics) == 0 {
				t.Fatalf("separator=%q unexpectedly accepted adjacent operand: output=%q", separator, result.Output)
			}
			continue
		}
		if len(result.Diagnostics) != 0 {
			t.Fatalf("separator=%q diagnostics=%+v output=%q", separator, result.Diagnostics, result.Output)
		}
		proof := sourcePreservationOptions{dialect: format.EMEE, continuationSeparator: &separator}
		if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
			t.Fatalf("separator=%q output=%q: %v", separator, result.Output, err)
		}
		second := format.FormatWithOptions(result.Output, format.ERB, format.Options{Dialect: format.EMEE, ContinuationSeparator: &separator})
		if !bytes.Equal(result.Output, second.Output) || len(second.Changes) != 0 {
			t.Fatalf("separator=%q unstable output: first=%q second=%q changes=%d", separator, result.Output, second.Output, len(second.Changes))
		}
		mutated := bytes.Replace(result.Output, []byte("tail"), []byte("TAIL"), 1)
		if err := checkSourcePreservedOptions(source, mutated, proof); err == nil {
			t.Fatalf("separator=%q accepted quoted FORM mutation: %q", separator, mutated)
		}
	}
	for _, test := range []struct {
		body, from string
	}{
		{`PRINTV 0,'raw  text',@"literal { 1+2 } tail"`, "raw  text"},
		{`PRINTV 0,"plain  text",@"literal { 1+2 } tail"`, "plain  text"},
	} {
		for _, separator := range []string{"+", "A", "\ufeff"} {
			source := []byte("@F\n{\n" + test.body + "\n}\n")
			result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: format.EMEE, ContinuationSeparator: &separator})
			if separator == "A" {
				if len(result.Diagnostics) == 0 {
					t.Fatalf("body=%q separator=%q unexpectedly accepted adjacent operand: output=%q", test.body, separator, result.Output)
				}
				continue
			}
			if len(result.Diagnostics) != 0 {
				t.Fatalf("body=%q separator=%q diagnostics=%+v output=%q", test.body, separator, result.Diagnostics, result.Output)
			}
			proof := sourcePreservationOptions{dialect: format.EMEE, continuationSeparator: &separator}
			if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
				t.Fatalf("body=%q separator=%q output=%q: %v", test.body, separator, result.Output, err)
			}
			mutated := bytes.Replace(result.Output, []byte(test.from), []byte("changed"), 1)
			if bytes.Equal(mutated, result.Output) {
				t.Fatalf("body=%q separator=%q raw payload mutation did not apply", test.body, separator)
			}
			if err := checkSourcePreservedOptions(source, mutated, proof); err == nil {
				t.Fatalf("body=%q separator=%q accepted raw payload mutation: %q", test.body, separator, mutated)
			}
		}
	}
}
