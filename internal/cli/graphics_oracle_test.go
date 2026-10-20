package cli

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

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
