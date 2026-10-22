package format

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestCallArgumentTails(t *testing.T) {
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		for _, command := range []string{"CALL", "CALLF", "TRYCALL", "JUMP", "GOTO", "CALLFORM", "TRYCALLFORM", "CALLSHARP"} {
			if erabasic.CommandUnavailable(command, dialect) {
				continue
			}
			for _, tail := range []string{",2", "(2)", " 2", ")", "[2]", " +", ` "text"`, " RESULT", "　RESULT"} {
				t.Run(fmt.Sprintf("%d/%s/%q", dialect, command, tail), func(t *testing.T) {
					target := "TARGET"
					if strings.Contains(command, "FORM") {
						target += "{1+2}"
					}
					prefix := "    " + command + " " + target + "(1+2)"
					source := []byte("@F\n" + prefix + tail + "\n")
					options := Options{Dialect: Dialect(dialect), AllowFullwidthSpace: true}
					result := FormatWithOptions(source, ERB, options)
					if !bytes.Equal(result.Output, source) || len(result.Changes) != 0 || len(result.Diagnostics) != 1 {
						t.Fatalf("output=%q changes=%v diagnostics=%v", result.Output, result.Changes, result.Diagnostics)
					}
					d := result.Diagnostics[0]
					column := utf8.RuneCountInString(prefix+tail[:len(tail)-len(strings.TrimLeft(tail, " 　"))]) + 1
					if d.Code != "unsupported_syntax" || d.Severity != Unsupported || d.Line != 2 || d.Column != column {
						t.Fatalf("diagnostic=%+v want=2:%d unsupported_syntax", d, column)
					}
					again := FormatWithOptions(result.Output, ERB, options)
					if !bytes.Equal(again.Output, result.Output) || len(again.Changes) != 0 || len(again.Diagnostics) != 1 || again.Diagnostics[0] != d {
						t.Fatalf("unstable result: %+v", again)
					}
				})
			}
		}
	}
}

func TestCallArgumentTailBoundaries(t *testing.T) {
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		for _, body := range []string{
			`CALL TARGET(1+2)`,
			`CALL TARGET(1+2);note`,
			`CALL TARGET[1](1+2)`,
			`CALL TARGET(1+2) MACRO`,
			`CALL TARGET(MACRO),2`,
			`CALL TARGET[MACRO](1+2),2`,
			`CALL TARGET(@"{MACRO}") 2`,
			`CALL TARGET(1+2) ;!; +2`,
			`CALL TARGET(1+2) ;#; +2`,
			`CALL TARGET(1+2) ;^; +2`,
			`CALL TARGET(;!; 1+2),2`,
			`CALL TARGET(1+2) RESULT　MACRO`,
			"CALL TARGET(1+2) RESULT\x00ignored",
			"CALL TARGET(1+2)\x00ignored(2)",
			"CALLFORM TARGET(1+2)\x00ignored(2)",
		} {
			t.Run(fmt.Sprintf("%d/%q", dialect, body), func(t *testing.T) {
				source := []byte("@F\n" + body + "\n")
				options := Options{Dialect: Dialect(dialect)}
				result := FormatWithOptions(source, ERB, options)
				if len(result.Diagnostics) != 0 {
					t.Fatalf("output=%q diagnostics=%v", result.Output, result.Diagnostics)
				}
				again := FormatWithOptions(result.Output, ERB, options)
				if !bytes.Equal(result.Output, again.Output) || len(again.Changes) != 0 || len(again.Diagnostics) != 0 {
					t.Fatalf("unstable result: %+v", again)
				}
			})
		}
	}
	source := []byte("@F\nCALL TARGET(1+2),2\n")
	result := FormatWithOptions(source, ERB, Options{Select: []RuleID{}})
	if !bytes.Equal(result.Output, source) || len(result.Diagnostics) != 0 || len(result.Changes) != 0 {
		t.Fatalf("disabled parsing: %+v", result)
	}
}

func TestCallArgumentTailContinuation(t *testing.T) {
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		for _, separator := range []string{"", " ", "\t"} {
			source := []byte("@F\n{\nCALL TARGET(1+\n2)\n ,2\n}\n")
			result := FormatWithOptions(source, ERB, Options{Dialect: Dialect(dialect), ContinuationSeparator: &separator})
			if len(result.Diagnostics) != 1 || !bytes.Contains(result.Output, []byte("CALL TARGET(1+\n2)\n ,2\n")) {
				t.Fatalf("dialect=%d separator=%q output=%q diagnostics=%v", dialect, separator, result.Output, result.Diagnostics)
			}
			d := result.Diagnostics[0]
			if d.Code != "unsupported_syntax" || d.Severity != Unsupported || d.Line != 5 || d.Column != 2 {
				t.Fatalf("dialect=%d separator=%q diagnostic=%+v want=5:2 unsupported_syntax", dialect, separator, d)
			}
		}
	}
}
