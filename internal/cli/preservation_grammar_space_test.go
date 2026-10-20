package cli

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestPreservationMixedAssignmentSpace(t *testing.T) {
	separator := "\u3000"
	for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
		for _, tc := range []struct {
			name, source string
			separator    *string
		}{
			{name: "direct-space", source: "@F\nRESULT \u3000+=1+2\n"},
			{name: "direct-tab", source: "@F\nRESULT\t\u3000+=1+2\n"},
			{name: "continued-space", source: "@F\n{\nRESULT \n+=1+2\n}\n", separator: &separator},
			{name: "continued-tab", source: "@F\n{\nRESULT\t\n+=1+2\n}\n", separator: &separator},
		} {
			t.Run(fmt.Sprintf("dialect=%d/%s", dialect, tc.name), func(t *testing.T) {
				source := []byte(tc.source)
				options := format.Options{Dialect: dialect, AllowFullwidthSpace: true, ContinuationSeparator: tc.separator}
				proof := sourcePreservationOptions{dialect: dialect, allowFullwidthSpace: true, continuationSeparator: tc.separator}
				result := format.FormatWithOptions(source, format.ERB, options)
				if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte("1 + 2")) {
					t.Fatalf("output=%q diagnostics=%v", result.Output, result.Diagnostics)
				}
				if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
	literal := []byte("@F\nRESULTS \u3000= raw\u3000value\n")
	changed := []byte("@F\nRESULTS = raw value\n")
	if checkSourcePreservedOptions(literal, changed, sourcePreservationOptions{allowFullwidthSpace: true}) == nil {
		t.Fatal("accepted fullwidth plain assignment payload change")
	}
	withoutOption := []byte("@F\nRESULT \u3000+=1+2\n")
	if checkSourcePreserved(withoutOption, []byte("@F\nRESULT +=1+2\n")) == nil {
		t.Fatal("accepted unconfigured fullwidth assignment change")
	}
}

func TestPreservationContinuedFormCallFullwidthNul(t *testing.T) {
	for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
		for _, separator := range []string{"", " ", "\t"} {
			for _, argument := range []string{"A", "RESULT", "1"} {
				t.Run(fmt.Sprintf("dialect=%d/separator=%q/%s", dialect, separator, argument), func(t *testing.T) {
					source := []byte("@F\n{\nCALLFORM F," + argument + "\u3000\x00 &\n}\n")
					options := format.Options{Dialect: dialect, AllowFullwidthSpace: true, ContinuationSeparator: &separator}
					proof := sourcePreservationOptions{dialect: dialect, allowFullwidthSpace: true, continuationSeparator: &separator}
					result := format.FormatWithOptions(source, format.ERB, options)
					if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte("F, "+argument+"\u3000\x00 &")) {
						t.Fatalf("output=%q diagnostics=%v", result.Output, result.Diagnostics)
					}
					if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
						t.Fatal(err)
					}
					mutated := bytes.Replace(result.Output, []byte("\x00 &"), []byte("\x00  &"), 1)
					if checkSourcePreservedOptions(source, mutated, proof) == nil {
						t.Fatal("accepted ignored suffix change")
					}
				})
			}
		}
	}
}
