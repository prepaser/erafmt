package cli

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestContinuationPreservationBoundaryContexts(t *testing.T) {
	tests := []struct {
		name, body, separator, want, from, to, payloadFrom, payloadTo string
		dialect                                                       format.Dialect
		allowFullwidth                                                bool
	}{
		{
			name:        "leading blank PRINTV raw atom",
			body:        "\nPRINTV 'x,1+2,\x00;note",
			separator:   " ",
			want:        "1 + 2",
			from:        "\x00;note",
			to:          "\x00;notes",
			payloadFrom: "'x",
			payloadTo:   "'y",
			dialect:     format.EMEE,
		},
		{
			name:        "leading blank PRINTV raw atom without NUL",
			body:        "\nPRINTV 'x,1+2,;note",
			separator:   " ",
			want:        "1 + 2",
			from:        "1 + 2",
			to:          "1 + 3",
			payloadFrom: "'x",
			payloadTo:   "'y",
			dialect:     format.EMEE,
		},
		{
			name:        "CALL target virtual quote",
			body:        "TRYCALL \nF(1+2)\x00ignored",
			separator:   "\"",
			want:        "1 + 2",
			from:        "\x00ignored",
			to:          "\x00ignorex",
			payloadFrom: "\nF(",
			payloadTo:   "\nG(",
			dialect:     format.EMEE,
		},
		{
			name:        "FORM conditional empty join",
			body:        "PRINTFORM \\\n@1?{1+2}#x\\@{3+4}",
			separator:   "",
			want:        "@ 1 ?{1 + 2}",
			from:        "{3 + 4}",
			to:          "{3 + 5}",
			payloadFrom: "#x",
			payloadTo:   "#y",
			dialect:     format.EMEE,
		},
		{
			name:        "PRINTFORMS virtual at",
			body:        "PRINTFORMS \n\"x {1+2}\"\x00ignored",
			separator:   "@",
			want:        "{1 + 2}",
			from:        "\x00ignored",
			to:          "\x00ignorex",
			payloadFrom: "\"x ",
			payloadTo:   "\"y ",
			dialect:     format.EMEE,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte("@F\n{\n" + tc.body + "\n}\n")
			options := format.Options{Dialect: tc.dialect, AllowFullwidthSpace: tc.allowFullwidth, ContinuationSeparator: &tc.separator}
			result := format.FormatWithOptions(source, format.ERB, options)
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
			}
			if !bytes.Contains(result.Output, []byte(tc.want)) {
				t.Fatalf("safe formatting missing: want %q in %q", tc.want, result.Output)
			}
			proof := sourcePreservationOptions{dialect: tc.dialect, allowFullwidthSpace: tc.allowFullwidth, continuationSeparator: &tc.separator}
			if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
				t.Fatalf("formatted output rejected: %v\noutput=%q", err, result.Output)
			}
			mutated := bytes.Replace(result.Output, []byte(tc.from), []byte(tc.to), 1)
			if bytes.Equal(mutated, result.Output) {
				t.Fatalf("mutation target %q was absent from %q", tc.from, result.Output)
			}
			if err := checkSourcePreservedOptions(source, mutated, proof); err == nil {
				t.Fatalf("accepted payload mutation: %q", mutated)
			}
			mutated = bytes.Replace(result.Output, []byte(tc.payloadFrom), []byte(tc.payloadTo), 1)
			if bytes.Equal(mutated, result.Output) {
				t.Fatalf("literal mutation target %q was absent from %q", tc.payloadFrom, result.Output)
			}
			if err := checkSourcePreservedOptions(source, mutated, proof); err == nil {
				t.Fatalf("accepted literal payload mutation: %q", mutated)
			}
		})
	}
}

func TestContinuationPreservationRejectsContextRowRedistribution(t *testing.T) {
	empty := ""
	proof := sourcePreservationOptions{dialect: format.EMEE, continuationSeparator: &empty}
	original := []byte("@F\n{\n\nPRINTV 'x,1+2,\x00ignored\nAB\n}\n")
	redistributed := []byte("@F\n{\n\nPRINTV 'x,1+2,\x00ignoredA\nB\n}\n")
	if err := checkSourcePreservedOptions(original, redistributed, proof); err == nil {
		t.Fatalf("accepted ignored suffix redistribution: %q", redistributed)
	}

	formOriginal := []byte("@F\n{\nPRINTFORM \\\n@1?{1+2}#x\\@A\nBC\n}\n")
	formRedistributed := []byte("@F\n{\nPRINTFORM \\\n@1?{1+2}#x\\@AB\nC\n}\n")
	if err := checkSourcePreservedOptions(formOriginal, formRedistributed, proof); err == nil {
		t.Fatalf("accepted FORM row redistribution: %q", formRedistributed)
	}
}
