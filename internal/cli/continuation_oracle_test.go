package cli

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestContinuationPreservationEmptyJoinPrintVTrailingCode(t *testing.T) {
	separator := ""
	original := []byte("@F\n{\nPRINTV \n\n}\n")
	result := format.FormatWithOptions(original, format.ERB, format.Options{ContinuationSeparator: &separator})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if !bytes.Contains(result.Output, []byte("PRINTV\n\n")) {
		t.Fatalf("trailing command gap was not removed: %q", result.Output)
	}
	if err := checkSourcePreservedOptions(original, result.Output, sourcePreservationOptions{continuationSeparator: &separator}); err != nil {
		t.Fatalf("formatted output rejected: %v\noutput=%q", err, result.Output)
	}

	fullwidth := []byte("@F\n{\nPRINTV　\n\n}\n")
	fullwidthResult := format.FormatWithOptions(fullwidth, format.ERB, format.Options{AllowFullwidthSpace: true, ContinuationSeparator: &separator})
	if len(fullwidthResult.Diagnostics) != 0 {
		t.Fatalf("fullwidth diagnostics=%+v output=%q", fullwidthResult.Diagnostics, fullwidthResult.Output)
	}
	if err := checkSourcePreservedOptions(fullwidth, fullwidthResult.Output, sourcePreservationOptions{allowFullwidthSpace: true, continuationSeparator: &separator}); err != nil {
		t.Fatalf("fullwidth output rejected: %v\noutput=%q", err, fullwidthResult.Output)
	}
}

func TestContinuationPreservationEmptyJoinKeepsPayload(t *testing.T) {
	separator := ""
	for _, test := range []struct{ source, from, to string }{
		{"PRINTV \nA\n", "PRINTV ", "PRINTV"},
		{"PRINTV 'raw' \n\n", "'raw' ", "'raw'"},
		{"PRINTV 'raw\ntext \n", "text ", "text"},
		{"PRINTV ;!; \n\n", ";!; ", ";!;"},
		{"PRINTFORM raw \n\n", "raw ", "raw"},
	} {
		src := []byte("@F\n{\n" + test.source + "}\n")
		changed := bytes.Replace(src, []byte(test.from), []byte(test.to), 1)
		if err := checkSourcePreservedOptions(src, changed, sourcePreservationOptions{continuationSeparator: &separator}); err == nil {
			t.Fatalf("accepted payload mutation: %q -> %q", src, changed)
		}
	}
	plain := []byte("@F\n{\nPRINTV raw\ntext \n}\n")
	result := format.FormatWithOptions(plain, format.ERB, format.Options{ContinuationSeparator: &separator})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("plain PRINTV diagnostics=%v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreservedOptions(plain, result.Output, sourcePreservationOptions{continuationSeparator: &separator}); err != nil {
		t.Fatalf("plain PRINTV grammar trim rejected: %v\noutput=%q", err, result.Output)
	}
}

func TestContinuationPreservationEmptyJoinNoArgCommand(t *testing.T) {
	separator := ""
	for _, src := range []string{
		"@F\n{\nDO \n\n}\nLOOP 1\n",
		"@F\nIF 1\n{\nELSE \n\n}\nENDIF\n",
	} {
		result := format.FormatWithOptions([]byte(src), format.ERB, format.Options{ContinuationSeparator: &separator})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
		}
		if err := checkSourcePreservedOptions([]byte(src), result.Output, sourcePreservationOptions{continuationSeparator: &separator}); err != nil {
			t.Fatalf("%v: output=%q", err, result.Output)
		}
	}
}

func TestContinuationPreservationInputGrammar(t *testing.T) {
	for _, separator := range []string{"", " "} {
		for _, body := range []string{"INPUT \n1+2", "ONEINPUT \nF( 1+2 )", "TINPUT \n1,2,3", "TINPUTS \n1,\"text\",2"} {
			src := []byte("@F\n{\n" + body + "\n}\n")
			result := format.FormatWithOptions(src, format.ERB, format.Options{Dialect: format.EMEE, ContinuationSeparator: &separator})
			options := sourcePreservationOptions{dialect: format.EMEE, continuationSeparator: &separator}
			if err := checkSourcePreservedOptions(src, result.Output, options); err != nil {
				t.Fatalf("separator=%q body=%q: %v output=%q", separator, body, err, result.Output)
			}
			changed := bytes.Replace(result.Output, []byte("1"), []byte("9"), 1)
			if err := checkSourcePreservedOptions(src, changed, options); err == nil {
				t.Fatalf("accepted input value mutation: %q", changed)
			}
		}
	}
}
