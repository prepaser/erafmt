package cli

import (
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestSourcePreservationDeferredArguments(t *testing.T) {
	for _, statement := range []string{
		"SETFONT A\x00B+1", "HTML_PRINT A\x00B+1", "LOADVAR A\x00B+1",
		"SAVEVAR F( 1+2 )", "SAVEVAR @\"a {1+2}\"", "SAVEVAR \"f\",F( 1+2 )",
	} {
		src := []byte("@F\n" + statement + "\n")
		result := format.FormatWithOptions(src, format.ERB, format.Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("statement=%q diagnostics=%v output=%q", statement, result.Diagnostics, result.Output)
		}
		if err := checkSourcePreserved(src, result.Output); err != nil {
			t.Fatalf("statement=%q: %v output=%q", statement, err, result.Output)
		}
	}
}

func TestSourcePreservationKeepsKnownInvalidSaveVarShape(t *testing.T) {
	for _, test := range []struct{ source, changed string }{
		{"SAVEVAR \"file\" ", "SAVEVAR \"file\""},
		{"SAVEVAR \"f\"+\"m\"", "SAVEVAR \"f\" + \"m\""},
		{"SAVEVAR 1,F(1+2)", "SAVEVAR 1,F(1 + 2)"},
		{"SAVEVAR \"f\x00m\" ", "SAVEVAR \"f\x00m\""},
		{"SAVEVAR \"f\"+1", "SAVEVAR \"f\" + 1"},
		{"SAVEVAR \"f\x00m\"+1", "SAVEVAR \"f\x00m\" + 1"},
	} {
		src := []byte("@F\n" + test.source + "\n")
		changed := []byte("@F\n" + test.changed + "\n")
		if err := checkSourcePreserved(src, changed); err == nil {
			t.Fatalf("accepted invalid argument mutation: %q -> %q", src, changed)
		}
	}
}
