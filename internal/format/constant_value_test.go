package format

import "testing"

func TestConstantIntegerValue(t *testing.T) {
	for _, test := range []struct {
		source string
		want   int64
	}{
		{"1+2*3", 7},
		{"(1+2)*3", 9},
		{"-3/2", -1},
		{"-3%2", -1},
		{"9223372036854775807+1", -1 << 63},
		{"0x8000000000000000-1", 1<<63 - 1},
		{"0x8000000000000000*2", 0},
		{"1<<64", 1},
		{"1<<-1", -1 << 63},
		{"-8>>65", -4},
		{"1|2&4", 0},
		{"5^3", 6},
		{"!0+~0", 0},
		{"1<2==1", 1},
		{"1&&2||0", 1},
		{"1^^2", 0},
		{"0!&1", 1},
		{"0!|0", 1},
		{"1?2+3#4", 5},
		{"0?2#3*4", 12},
	} {
		tokens, err := lexExpression([]byte(test.source), 0)
		value, ok := constantIntegerValue(tokens)
		if err != nil || !ok || value != test.want {
			t.Fatalf("%q: got %d, known=%v, error=%v; want %d", test.source, value, ok, err, test.want)
		}
	}
	for _, source := range []string{"1/0", "1%0", "0x8000000000000000/-1", "0x8000000000000000%-1", "RESULT+1", "ABS(1)", "1;!;+2", `"x"=="x"`} {
		tokens, _ := lexExpression([]byte(source), 0)
		if value, ok := constantIntegerValue(tokens); ok {
			t.Fatalf("%q: unexpectedly evaluated to %d", source, value)
		}
	}
}
