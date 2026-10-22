package erabasic

import "testing"

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
		value, kind := DirectIntegerLiteral(tokens)
		if kind != KnownValueNumeric || value != tt.value {
			t.Fatalf("%q: value=%d kind=%d", tt.literal, value, kind)
		}
	}
}
