package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestPreservationOracleDeferredIncrementExpressions(t *testing.T) {
	for _, body := range []string{
		"A0! ++0",
		"A0++ 0",
		"UNKNOWN(1)++ 0",
		"UNKNOWN++ --0",
		"UNKNOWN + 1++",
		`UNKNOWN + "x"++`,
		"UNKNOWN + ARGS++",
		"UNKNOWN + ITEMPRICE++",
		"ARGS++ + UNKNOWN",
		"ITEMPRICE++ + UNKNOWN",
	} {
		source := []byte("@F\nRETURN " + body + "\n")
		result := format.FormatWithOptions(source, format.ERB, format.Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("body=%q diagnostics=%v output=%q", body, result.Diagnostics, result.Output)
		}
		if err := checkSourcePreserved(source, result.Output); err != nil {
			t.Fatalf("body=%q output=%q: %v", body, result.Output, err)
		}
	}
}

func TestPreservationOracleKnownIncrementFailuresStayExact(t *testing.T) {
	for _, body := range []string{
		"ABS(1)++",
		"++ABS(1)",
		"++1",
		`"x"++`,
		"UNKNOWN + (1)++",
		"UNKNOWN + ABS(1)++",
	} {
		source := []byte("@F\nRETURN " + body + "\n")
		result := format.FormatWithOptions(source, format.ERB, format.Options{})
		if len(result.Diagnostics) == 0 {
			t.Fatalf("body=%q expected a formatter diagnostic: output=%q", body, result.Output)
		}
		if !bytes.Contains(result.Output, []byte(body)) {
			t.Fatalf("body=%q was not preserved in output=%q", body, result.Output)
		}
		changedBody := incrementGapMutation(body)
		changed := bytes.Replace(result.Output, []byte(body), []byte(changedBody), 1)
		if bytes.Equal(changed, result.Output) {
			t.Fatalf("body=%q mutation was not applied: output=%q", body, result.Output)
		}
		if err := checkSourcePreserved(source, changed); err == nil {
			t.Fatalf("body=%q accepted mutation=%q", body, changed)
		}
	}
}

func incrementGapMutation(body string) string {
	if i := strings.Index(body, "++"); i >= 0 {
		if i == 0 {
			return "++ " + body[2:]
		}
		return body[:i] + " ++" + body[i+2:]
	}
	return body
}
