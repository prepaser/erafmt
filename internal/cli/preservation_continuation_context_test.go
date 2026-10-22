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

func TestContinuationPreservationFormArgumentBoundaries(t *testing.T) {
	cases := []struct {
		name, source, literal, replacement string
		allowFullwidth                     bool
	}{
		{
			name:        "graphics form split across rows",
			source:      "@F\n{\nPRINT_IMG \"image\",@\"name\n{1+2}\"\n}\n",
			literal:     "name",
			replacement: "neme",
		},
		{
			name:        "PRINTV form suffix",
			source:      "@F\n{\nPRINTV @\"value {1+2}\",1+2\n}\n",
			literal:     "value",
			replacement: "valuX",
		},
		{
			name:        "PRINTV raw apostrophe field",
			source:      "@F\n{\nPRINTV @\"value {1+2}\",'raw  text\n}\n",
			literal:     "raw  text",
			replacement: "raw text",
		},
		{
			name:        "PRINTV escaped comma raw field",
			source:      "@F\n{\nPRINTV @\"value {1+2}\",'raw\\,  text\n}\n",
			literal:     "raw\\,  text",
			replacement: "raw\\, text",
		},
		{
			name:        "PRINTV form split with suffix",
			source:      "@F\n{\nPRINTV @\"value\n{1+2}\",1+2\n}\n",
			literal:     "value",
			replacement: "valuX",
		},
		{
			name:           "fullwidth graphics form split",
			source:         "@F\n{\nPRINT_IMG　\"image\",@\"name\n{1+2}\"\n}\n",
			literal:        "name",
			replacement:    "neme",
			allowFullwidth: true,
		},
	}
	for _, separator := range []string{"", ",", " ", "　"} {
		for _, tc := range cases {
			if separator == "　" && !tc.allowFullwidth {
				continue
			}
			t.Run(tc.name+"/"+continuationSeparatorName(separator), func(t *testing.T) {
				options := format.Options{Dialect: format.EMEE, AllowFullwidthSpace: tc.allowFullwidth, ContinuationSeparator: &separator}
				result := format.FormatWithOptions([]byte(tc.source), format.ERB, options)
				for _, diagnostic := range result.Diagnostics {
					if diagnostic.Severity == format.Error {
						t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
					}
				}
				proof := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: tc.allowFullwidth, continuationSeparator: &separator}
				if err := checkSourcePreservedOptions([]byte(tc.source), result.Output, proof); err != nil {
					t.Fatalf("formatted output rejected: %v\noutput=%q", err, result.Output)
				}
				mutated := bytes.Replace(result.Output, []byte(tc.literal), []byte(tc.replacement), 1)
				if bytes.Equal(mutated, result.Output) {
					t.Fatalf("literal %q was absent from %q", tc.literal, result.Output)
				}
				if err := checkSourcePreservedOptions([]byte(tc.source), mutated, proof); err == nil {
					t.Fatalf("accepted literal mutation: %q", mutated)
				}
			})
		}
	}
	separator := " "
	proof := sourcePreservationOptions{dialect: format.EMEE, continuationSeparator: &separator}
	for _, tc := range []struct {
		name, original, redistributed string
	}{
		{
			name:          "PRINTV expression token",
			original:      "@F\n{\nPRINTV @\"literal {1+2}\", 1+\n2\n}\n",
			redistributed: "@F\n{\nPRINTV @\"literal {1+2}\",\n1+ 2\n}\n",
		},
		{
			name:          "graphics expression token",
			original:      "@F\n{\nPRINT_IMG \"image\", @\"label {1+2}\", 1+\n2\n}\n",
			redistributed: "@F\n{\nPRINT_IMG \"image\", @\"label {1+2}\",\n1+ 2\n}\n",
		},
	} {
		t.Run("reject-"+tc.name, func(t *testing.T) {
			result := format.FormatWithOptions([]byte(tc.original), format.ERB, format.Options{Dialect: format.EMEE, ContinuationSeparator: &separator})
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == format.Error {
					t.Fatalf("original diagnostics=%v output=%q", result.Diagnostics, result.Output)
				}
			}
			if err := checkSourcePreservedOptions([]byte(tc.original), result.Output, proof); err != nil {
				t.Fatalf("formatted original rejected: %v output=%q", err, result.Output)
			}
			if err := checkSourcePreservedOptions([]byte(tc.original), []byte(tc.redistributed), proof); err == nil {
				t.Fatalf("accepted expression token redistribution: %q", tc.redistributed)
			}
		})
	}
}
