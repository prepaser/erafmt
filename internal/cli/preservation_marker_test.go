package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestSourcePreservationMandatoryMarker(t *testing.T) {
	for _, statement := range []string{
		"RETURN ;!; 1+2", "RETURN A+B;!;+C", "RETURN A  ;!;\t+B+C",
		"RETURN A\u3000;!;\u3000+B+C", "RETURN ;!;  ;!; 1+2",
		"PRINTV ;!; A+B", "CALL F( ;!; 1+2 )", "PRINT_RECT 1;!; ,2,3,4",
		"PRINT_RECT 1\u3000;!;\t,2,3,4", "VARSET A;!;,1+2", "ARRAYSHIFT A;!;,1,2,3",
		"PRINT_RECT 1 \u3000;!; \u3000,2,3,4",
		"SAVEVAR \"f\",\"m\",;!;RESULT", "SAVEVAR \"f\",\"m\",\t;!; RESULT",
	} {
		t.Run(statement, func(t *testing.T) {
			src := []byte("@F\n" + statement + "\n")
			result := format.FormatWithOptions(src, format.ERB, format.Options{AllowFullwidthSpace: true, Dialect: format.EMEE})
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
			}
			if err := checkSourcePreservedOptions(src, result.Output, sourcePreservationOptions{allowFullwidthSpace: true, dialect: format.EMEE}); err != nil {
				t.Fatalf("%v: output=%q", err, result.Output)
			}
		})
	}
}

func TestSourcePreservationRejectsMarkerTriviaMutation(t *testing.T) {
	for _, test := range []struct{ source, from, to string }{
		{"RETURN A ;!; +B", "A ;!;", "A;!;"},
		{"RETURN A  ;!; +B", "A  ;!;", "A ;!;"},
		{"RETURN A ;!;  ", ";!;  ", ";!;"},
		{"RETURN  ;!; A", "RETURN  ;!;", "RETURN ;!;"},
		{"PRINTV  ;!; A", "PRINTV  ;!;", "PRINTV ;!;"},
		{"RETURN A\u3000;!; +B", "\u3000", " "},
		{"RETURN A ;!;\u3000+B", "\u3000", " "},
		{"PRINT_RECT 1\u3000;!; ,2,3,4", "\u3000", " "},
		{"SAVEVAR \"f\",\"m\", \u3000;!;RESULT", "\u3000", " "},
		{"PRINTV 'raw ;!;\u3000'", "\u3000", " "},
		{"PRINTFORM raw ;!;\u3000", "\u3000", " "},
		{"BEGIN raw ;!;\u3000payload", "\u3000", " "},
		{"RETURN \"raw ;!;\u3000\"", "\u3000", " "},
	} {
		t.Run(test.source, func(t *testing.T) {
			src := []byte("@F\n" + test.source + "\n")
			changed := bytes.Replace(src, []byte(test.from), []byte(test.to), 1)
			if err := checkSourcePreservedOptions(src, changed, sourcePreservationOptions{allowFullwidthSpace: true, dialect: format.EMEE}); err == nil {
				t.Fatalf("accepted mutation: %q -> %q", src, changed)
			}
		})
	}
}

func TestSourcePreservationKeepsUnselectedFullwidthTokenBoundary(t *testing.T) {
	for _, statement := range []string{"RETURN A \u3000B", "RETURN A\u3000 B"} {
		src := []byte("@F\n" + statement + "\n")
		changed := bytes.Replace(src, []byte(statement), []byte("RETURN A\u3000B"), 1)
		if err := checkSourcePreserved(src, changed); err == nil {
			t.Fatalf("accepted token boundary mutation: %q -> %q", src, changed)
		}
	}
}

func TestSourcePreservationCommandGapWithoutExpressionProof(t *testing.T) {
	for _, statement := range []string{"SETFONT  1+2", "HTML_PRINT  1+2", "PRINTCPERLINE  RESULTS", "AWAIT  \"x\"", "SETFONT\u3000 1+2", "SETFONT \u3000\t\u3000 1+2", "SAVEVAR  1+2", "SETFONT  1==2", "HTML_PRINT  !ARG:1", "SAVEVAR  ARGS:1==ARGS:2", "PRINT_SPACE  1,2", "PRINT_IMG  1,2"} {
		t.Run(statement, func(t *testing.T) {
			src := []byte("@F\n" + statement + "\n")
			result := format.FormatWithOptions(src, format.ERB, format.Options{Select: []format.RuleID{format.CommandSpacing}, AllowFullwidthSpace: true, Dialect: format.EMEE})
			if len(result.Diagnostics) != 0 || bytes.Equal(src, result.Output) {
				t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
			}
			options := sourcePreservationOptions{allowFullwidthSpace: true, dialect: format.EMEE}
			if err := checkSourcePreservedOptions(src, result.Output, options); err != nil {
				t.Fatalf("%v: output=%q", err, result.Output)
			}
			body := trimLeadingGrammarSpace([]byte(statement[strings.IndexAny(statement, " \u3000"):]))
			changed := bytes.Replace(result.Output, body, append(append([]byte(nil), body...), '9'), 1)
			if bytes.Equal(changed, result.Output) {
				t.Fatal("body mutation was not applied")
			}
			if err := checkSourcePreservedOptions(src, changed, options); err == nil {
				t.Fatalf("accepted body mutation: %q", changed)
			}
		})
	}
}
