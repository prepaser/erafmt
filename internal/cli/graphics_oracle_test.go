package cli

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestGraphicsOracleEmbeddedNULIdentifiers(t *testing.T) {
	for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
		for _, wide := range []bool{false, true} {
			for _, body := range []string{
				"PRINT_SPACE A\x00B+1",
				"PRINT_RECT A\x00B+1,2+3,4+5,6+7",
				"PRINT_SPACE A\x00B+1\x00ignored  ",
			} {
				t.Run(fmt.Sprintf("%s/%d/wide=%t", body[:bytes.IndexByte([]byte(body), ' ')], dialect, wide), func(t *testing.T) {
					source := []byte("@F\n" + body + "\n")
					result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: dialect, AllowFullwidthSpace: wide})
					if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte("A\x00B + 1")) {
						t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
					}
					proof := sourcePreservationOptions{dialect: dialect, allowFullwidthSpace: wide}
					if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
						t.Fatalf("formatted output rejected: %v", err)
					}
					for _, change := range []struct{ from, to string }{
						{"A\x00B", "A\x00C"},
						{"A\x00B", "AB"},
						{"+ 1", "+ 2"},
					} {
						mutated := bytes.Replace(result.Output, []byte(change.from), []byte(change.to), 1)
						if err := checkSourcePreservedOptions(source, mutated, proof); err == nil {
							t.Fatalf("accepted mutation: %q", mutated)
						}
					}
					if bytes.Contains(result.Output, []byte("ignored  ")) {
						mutated := bytes.Replace(result.Output, []byte("ignored  "), []byte("ignored "), 1)
						if err := checkSourcePreservedOptions(source, mutated, proof); err == nil {
							t.Fatalf("accepted NUL suffix mutation: %q", mutated)
						}
					}
				})
			}
		}
	}
}

func TestGraphicsOracleAcceptsDeferredAndKnownFields(t *testing.T) {
	for _, test := range []struct {
		name, source string
		dialect      format.Dialect
	}{
		{"deferred quoted form", "@F\nPRINT_SPACE  @\"a {1+2}\"\n", format.EMEE},
		{"unicode identifier", "@F\nPRINT_SPACE  1\u00a0+2\n", format.CommonDialect},
		{"known string then numeric", "@F\nPRINT_IMG  \"x\",RESULTS,RESULT\n", format.EMEE},
		{"numeric image coercion", "@F\nPRINT_IMG  1+2,3,4\n", format.EMEE},
		{"string numeric coercion", "@F\nPRINT_SPACE  \"a\"+\"b\"\n", format.EMEE},
		{"string numeric unit", "@F\nPRINT_SPACE  \"abc\"    px\n", format.EMEE},
		{"string name concatenation", "@F\nPRINT_IMG  \"x\",\"a\"+\"b\"\n", format.EMEE},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := format.FormatWithOptions([]byte(test.source), format.ERB, format.Options{Dialect: test.dialect})
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
			}
			if bytes.Equal(result.Output, []byte(test.source)) {
				t.Fatalf("fixture did not exercise formatting: %q", result.Output)
			}
			if err := checkSourcePreservedDialect([]byte(test.source), result.Output, test.dialect); err != nil {
				t.Fatalf("formatted output rejected: %v\noutput=%q", err, result.Output)
			}
		})
	}
}

func TestGraphicsOracleKeepsKnownAndProtectedFieldsStrict(t *testing.T) {
	for _, test := range []struct {
		name, source, changed string
		dialect               format.Dialect
	}{
		{"known numeric first field", "@F\nPRINT_IMG RESULT\n", "@F\nPRINT_IMG RESULT + 1\n", format.CommonDialect},
		{"known string numeric command", "@F\nPRINT_SPACE RESULTS\n", "@F\nPRINT_SPACE RESULTS + 1\n", format.CommonDialect},
		{"unresolved image field", "@F\nPRINT_IMG \"x\", NAME+1\n", "@F\nPRINT_IMG \"x\", NAME + 2\n", format.EMEE},
		{"unresolved image trivia", "@F\nPRINT_IMG \"x\", USR_X+1\n", "@F\nPRINT_IMG \"x\", USR_X + 1\n", format.EMEE},
		{"NUL image field", "@F\nPRINT_IMG \"x\", A\x00B+1\n", "@F\nPRINT_IMG \"x\", A\x00B + 1\n", format.EMEE},
		{"quoted payload", "@F\nPRINT_IMG \"x;!;  y\"\n", "@F\nPRINT_IMG \"x;!; y\"\n", format.EMEE},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := checkSourcePreservedDialect([]byte(test.source), []byte(test.changed), test.dialect); err == nil {
				t.Fatal("accepted protected or invalid graphics mutation")
			}
		})
	}
}
