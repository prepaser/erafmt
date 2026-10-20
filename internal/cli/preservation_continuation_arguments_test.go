package cli

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestContinuationArgumentPreservation(t *testing.T) {
	tests := []struct {
		name, body, want, from, to string
		emptyOnly                  bool
	}{
		{"escaped target", "\nTRYCALL F\\;tail(1+2)", "1 + 2", "F\\;tail", "G\\;tail", false},
		{"split target", "TRYCALL F\\;ta\nil(1+2)", "1 + 2", ";ta", ";tA", false},
		{"split target escape", "TRYCALL F\\\n;tail(1+2)", "1 + 2", "tail", "tAil", true},
		{"split call string", "TRYCALL F\\;tail(\"a\\\n\"b\",1+2)", "1 + 2", "a\\", "c\\", true},
		{"split input string", "TINPUTS 1,\"d\nefault\",0,\"timeout\"", ", 0,", "efault", "efaUlt", false},
		{"input literal padding", "TINPUTS 1,\"\n  default  \",0,\"timeout\"", ", 0,", "default  ", "default ", false},
		{"split input escape", "TINPUTS 1,\"a\\\n\"b\",0,\"timeout\"", ", 0,", "a\\", "c\\", true},
		{"call FORM argument", "TRYCALL F\\;tail(@\"literal {1+2}\ntext\",3+4)", "1 + 2", "literal ", "literal  ", false},
		{"input FORM argument", "TINPUTS 1,@\"d {1+2}\nefault\",0,\"timeout\"", "1 + 2", "efault", "efaUlt", false},
		{"raw input FORM", "INPUTS \"before {1+2}\nafter\"", "1 + 2", "before ", "before  ", false},
		{"call NUL suffix", "TRYCALL F\\;tail(1+2)\x00ignored  ", "1 + 2", "ignored  ", "ignored ", false},
	}
	for _, tc := range tests {
		for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
			for _, wide := range []bool{false, true} {
				for _, separator := range []string{"", " ", "\t", "\u3000"} {
					if tc.emptyOnly && separator != "" || !wide && separator == "\u3000" {
						continue
					}
					t.Run(fmt.Sprintf("%s/%d/wide=%t/separator=%q", tc.name, dialect, wide, separator), func(t *testing.T) {
						source := []byte("@F\n{\n" + tc.body + "\n}\n")
						options := format.Options{Dialect: dialect, AllowFullwidthSpace: wide, ContinuationSeparator: &separator}
						proof := sourcePreservationOptions{dialect: dialect, allowFullwidthSpace: wide, continuationSeparator: &separator}
						result := format.FormatWithOptions(source, format.ERB, options)
						if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte(tc.want)) {
							t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
						}
						if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
							t.Fatalf("formatted output rejected: %v output=%q", err, result.Output)
						}
						mutated := bytes.Replace(result.Output, []byte(tc.from), []byte(tc.to), 1)
						if bytes.Equal(mutated, result.Output) {
							t.Fatalf("mutation target %q missing", tc.from)
						}
						if err := checkSourcePreservedOptions(source, mutated, proof); err == nil {
							t.Fatalf("accepted payload mutation %q", mutated)
						}
						from, to := "1 + 2", "1 + 3"
						if !bytes.Contains(result.Output, []byte(from)) {
							from, to = ", 0,", ", 1,"
						}
						mutated = bytes.Replace(result.Output, []byte(from), []byte(to), 1)
						if bytes.Equal(mutated, result.Output) || checkSourcePreservedOptions(source, mutated, proof) == nil {
							t.Fatalf("argument mutation was absent or accepted: %q", mutated)
						}
					})
				}
			}
		}
	}
}

func TestContinuationArgumentPhysicalPayloads(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"call target", "TRYCALL F\\;ta\nil(1+2)", "TRYCALL F\\;t\nail(1+2)"},
		{"call string", "TRYCALL F(\"ab\ncd\",1)", "TRYCALL F(\"a\nbcd\",1)"},
		{"input string", "TINPUTS 1,\"ab\ncd\",0,\"timeout\"", "TINPUTS 1,\"a\nbcd\",0,\"timeout\""},
		{"FORM literal", "TINPUTS 1,@\"ab\ncd {1+2}\",0,\"timeout\"", "TINPUTS 1,@\"a\nbcd {1+2}\",0,\"timeout\""},
		{"raw input literal", "INPUTS \"ab\ncd {1+2}\"", "INPUTS \"a\nbcd {1+2}\""},
		{"ignored suffix", "TRYCALL F(1)\x00ab\ncd", "TRYCALL F(1)\x00a\nbcd"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			separator := ""
			before, after := []byte("@F\n{\n"+tc.before+"\n}\n"), []byte("@F\n{\n"+tc.after+"\n}\n")
			if !bytes.Equal(bytes.ReplaceAll(before, []byte{'\n'}, nil), bytes.ReplaceAll(after, []byte{'\n'}, nil)) {
				t.Fatal("control changed logical source")
			}
			if err := checkSourcePreservedOptions(before, after, sourcePreservationOptions{continuationSeparator: &separator}); err == nil {
				t.Fatal("accepted physical payload redistribution")
			}
		})
	}
}
