package format

import (
	"bytes"
	"strings"
	"testing"
)

func TestPreprocessorFields(t *testing.T) {
	cases := []struct {
		name      string
		head      string
		allowWide bool
		first     string
		second    string
		count     int
	}{
		{"empty", "[]", false, "", "", 0},
		{"single", "[SKIPSTART]", false, "SKIPSTART", "", 1},
		{"two", "[IF\tX]", false, "IF", "X", 2},
		{"three", "[IF X Y]", false, "IF", "X", 3},
		{"fullwidth", "[IF\u3000X]", true, "IF", "X", 2},
		{"fullwidth-in-token", "[IF\u3000X]", false, "IF\u3000X", "", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			head := []byte(tc.head)
			end := bytes.IndexByte(head, ']')
			first, second, count := preprocessorFields(head, end, tc.allowWide)
			if string(first) != tc.first || string(second) != tc.second || count != tc.count {
				t.Fatalf("got (%q, %q, %d), want (%q, %q, %d)", first, second, count, tc.first, tc.second, tc.count)
			}
		})
	}
}

func TestPreprocessorFieldsStopAfterThird(t *testing.T) {
	short := []byte("[IF X Y]")
	long := []byte("[IF X " + strings.Repeat("Y ", 1<<16) + "]")
	allocs := func(head []byte) float64 {
		return testing.AllocsPerRun(100, func() {
			first, second, count := preprocessorFields(head, bytes.IndexByte(head, ']'), false)
			if len(first)+len(second)+count == 0 {
				t.Fatal("scanner returned no fields")
			}
		})
	}
	if got := allocs(short); got != 0 {
		t.Fatalf("short directive scanner allocated %v times", got)
	}
	if got := allocs(long); got != 0 {
		t.Fatalf("long directive scanner allocated %v times", got)
	}
}

func TestPreprocessorDirectiveScanningKeepsSuffixAndStateRules(t *testing.T) {
	cases := []struct {
		name string
		body string
		code string
	}{
		{"comment suffix", "@F\n[IF X] ; comment\nRETURN 1+2\n[ENDIF]\n", ""},
		{"third field", "@F\n[IF X Y] ; comment\nRETURN 1+2\n[ENDIF]\n", "unsupported_structure"},
		{"non-comment suffix", "@F\n[IF X] RETURN 1\n[ENDIF]\n", "unsupported_structure"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := []byte(tc.body)
			result := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
			if tc.code == "" {
				if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte("    RETURN 1+2")) {
					t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
				}
				return
			}
			if !bytes.Equal(result.Output, input) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != tc.code {
				t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
			}
		})
	}
}
