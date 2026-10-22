package format

import (
	"bytes"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestRuntimeNumericBounds(t *testing.T) {
	for _, tt := range []struct {
		literal string
		valid   bool
	}{
		{"9223372036854775807", true},
		{"9223372036854775808", false},
		{"-9223372036854775808", false},
		{"0xFFFFFFFFFFFFFFFF", true},
		{"0x8000000000000000", true},
		{"0x10000000000000000", false},
		{"0b" + strings.Repeat("1", 64), true},
		{"0b" + strings.Repeat("1", 65), false},
		{"1p63", true},
		{"1p64", false},
		{"1e19", false},
		{"0e309", false},
		{"1e2147483648", true},
		{"1e-9223372036854775808", true},
		{"1e-9223372036854775809", false},
		{"1e999999999999999999999999", false},
		{"0x1p100000000", true},
		{"0xFFFFFFFFFFFFFFFFpFFFFFFFFFFFFFFFF", true},
		{"0x8000000000000000p1", false},
	} {
		src := []byte("@F\nRETURN " + tt.literal + "+1\n")
		for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
			r := FormatWithOptions(src, ERB, Options{Select: []RuleID{BinarySpacing}, Dialect: Dialect(dialect)})
			want := src
			if tt.valid {
				want = []byte("@F\nRETURN " + tt.literal + " + 1\n")
			}
			if !bytes.Equal(r.Output, want) || (len(r.Diagnostics) == 0) != tt.valid {
				t.Fatalf("%q dialect=%v: output=%q diagnostics=%+v", tt.literal, dialect, r.Output, r.Diagnostics)
			}
		}
	}
}
