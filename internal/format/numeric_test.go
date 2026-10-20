package format

import (
	"bytes"
	"strings"
	"testing"
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
		for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
			r := FormatWithOptions(src, ERB, Options{Select: []RuleID{BinarySpacing}, Dialect: dialect})
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

func TestDirectIntegerLiteralKeepsRuntimeValues(t *testing.T) {
	for _, tt := range []struct {
		literal string
		value   int64
	}{
		{"9007199254740993p0", 9007199254740993},
		{"9223372036854775807p0", 9223372036854775807},
		{"1p63", -9223372036854775808},
		{"-1p63", -9223372036854775808},
		{"0xFFFFFFFFFFFFFFFFpFFFFFFFFFFFFFFFF", 0},
		{"1e-1", 0},
		{"0x+0", 0},
		{"0b+1p+0", 1},
	} {
		tokens, err := lexExpression([]byte(tt.literal), 0)
		if err != nil {
			t.Fatalf("%q lex error: %v", tt.literal, err)
		}
		value, kind := directIntegerLiteral(tokens)
		if kind != knownValueNumeric || value != tt.value {
			t.Fatalf("%q: value=%d kind=%d", tt.literal, value, kind)
		}
	}
}
