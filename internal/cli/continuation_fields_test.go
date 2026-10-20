package cli

import (
	"bytes"
	"github.com/prepaser/erafmt/internal/format"
	"testing"
)

func TestContinuationVerifierPreservesFieldGrammars(t *testing.T) {
	separator := " "
	options := sourcePreservationOptions{dialect: format.EMEE, continuationSeparator: &separator}
	for _, tc := range []struct{ original, changed string }{
		{"SETBGIMAGE one  two", "SETBGIMAGE one two"},
		{"CALLFORM one  two", "CALLFORM one two"},
		{"TIMES RESULT:0, 1e+2", "TIMES RESULT:0, 1e +2"},
	} {
		original := []byte("@F\n{\n" + tc.original + "\n}\n")
		changed := []byte("@F\n{\n" + tc.changed + "\n}\n")
		if err := checkSourcePreservedOptions(original, changed, options); err == nil {
			t.Errorf("accepted field payload mutation %q -> %q", tc.original, tc.changed)
		}
	}
}

func TestCorpusVerifierRecognizesScopedIndexAssignments(t *testing.T) {
	for _, lhs := range []string{"CFLAG:(LOCAL@SELF_KOJO):4900", "LOCAL@SPRING_EXP:(LOCAL@SPRING_DUMMY:0)", "EXP:(LOCAL@SPRING_DUMMY:30):31"} {
		source := []byte("@F\n" + lhs + " = COUNT\n")
		result := format.FormatWithOptions(source, format.ERB, format.Options{})
		if err := checkSourcePreserved(source, result.Output); err != nil {
			t.Fatalf("lhs=%q: %v", lhs, err)
		}
	}
}

func TestContinuationVerifierPreservesHeaderShapes(t *testing.T) {
	separator := " "
	options := sourcePreservationOptions{dialect: format.CommonDialect, continuationSeparator: &separator}
	for _, tc := range []struct {
		name, source, old, replacement string
	}{
		{"function header", "@F\n{\n@ F(ARG : 0)\n}\n", "@F(ARG:0)", "@G(ARG:0)"},
		{"dollar label", "@F\n{\n$ L\n}\n", "$L", "$M"},
		{"preprocessor", "@F\n{\n[IF   FEATURE ]\n}\n[ENDIF]\n", "[IF FEATURE]", "[IF OTHER]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := []byte(tc.source)
			result := format.FormatWithOptions(original, format.ERB, format.Options{ContinuationSeparator: &separator})
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
			}
			if err := checkSourcePreservedOptions(original, result.Output, options); err != nil {
				t.Fatalf("formatted output rejected: %v\noutput=%q", err, result.Output)
			}
			mutated := bytes.Replace(result.Output, []byte(tc.old), []byte(tc.replacement), 1)
			if bytes.Equal(mutated, result.Output) {
				t.Fatalf("mutation target %q was not present in %q", tc.old, result.Output)
			}
			if err := checkSourcePreservedOptions(original, mutated, options); err == nil {
				t.Fatal("header payload mutation was accepted")
			}
		})
	}
}
