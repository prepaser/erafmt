package format

import (
	"bytes"

	"testing"
)

func TestFormatExpressionSpacing(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"binary operators", "  A+B*C  ; keep this", "  A + B * C  ; keep this"},
		{"FORM atom", `A+@"x { B+1 }"`, `A + @"x {B + 1}"`},
		{"call and indexing", "F( A[0 : 2],B,, C, )", "F(A[0:2], B,, C,)"},
		{"comma and unary arguments", "F(A,-1,!B,~B,++B,(B))", "F(A, -1, !B, ~B, ++B, (B))"},
		{"ternary", "FLAG?YES#NO", "FLAG ? YES # NO"},
		{"unary collision", "A*- -B", "A * - -B"},
		{"unary chain", "! ~ !A + ++ +B", "!~!A + +++B"},
		{"postfix", "VALUE ++ + 1", "VALUE++ + 1"},
		{"numeric spelling", "0x0f+0b101+12p-2", "0x0f + 0b101 + 12p-2"},
		{"signed prefixed numerals", "0x+FF+0b+10", "0x+FF + 0b+10"},
		{"numeric range spelling", "-9223372036854775807+0xFFFFFFFFFFFFFFFF", "-9223372036854775807 + 0xFFFFFFFFFFFFFFFF"},
		{"case keywords", "1 TO 3,IS(-1)", "1 TO 3, IS (-1)"},
		{"case named index", "FLAG:IS-1,F(IS+2),3 TO - 4,IS>=5", "FLAG:IS - 1, F(IS + 2), 3 TO -4, IS >= 5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatExpression([]byte(tt.in), tt.name == "case keywords" || tt.name == "case named index")
			if err != nil {
				t.Fatalf("formatExpression() error = %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("formatExpression() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatExpressionQualifiedVariablesAndConditionalForm(t *testing.T) {
	input := `LOCAL @ FUNC:0 + ARG @ OTHER`
	want := `LOCAL@FUNC:0 + ARG@OTHER`
	got, err := formatExpression([]byte(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("qualified variable expression = %q, want %q", got, want)
	}
	input = `A+\@ A+1?left,  raw#right; raw\@+B`
	want = `A + \@ A + 1 ?left,  raw#right; raw\@ + B`
	got, err = formatExpression([]byte(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("conditional FORM expression = %q, want %q", got, want)
	}
}

func TestFormatExpressionOpaqueRegions(t *testing.T) {
	tests := []string{`A+@"FORM + text"`, `A+[[runtime alias]]`, `A+B;!; conditional`, `A+B;#; conditional`, `A+B;^; conditional`}
	for _, input := range tests {
		got, err := formatExpression([]byte(input), false)
		if err != nil {
			t.Fatalf("formatExpression(%q) error = %v", input, err)
		}
		want := input
		if input == `A+@"FORM + text"` {
			want = `A + @"FORM + text"`
		} else if input == `A+B;!; conditional` {
			want = `A + B;!; conditional`
		} else if input == `A+B;^; conditional` {
			want = `A + B;^; conditional`
		}
		if !bytes.Equal(got, []byte(want)) {
			t.Fatalf("opaque input changed: got %q, want %q", got, want)
		}
	}
	input := `A+"quoted + text; text"`
	want := `A + "quoted + text; text"`
	got, err := formatExpression([]byte(input), false)
	if err != nil || string(got) != want {
		t.Fatalf("quoted string changed: got %q, err %v, want %q", got, err, want)
	}
}

func TestFormatExpressionPreservesOuterTrivia(t *testing.T) {
	input := "\t\u00a0A+B \t ; comment with  spaces  "
	want := "\t\u00a0A + B \t ; comment with  spaces  "
	got, err := formatExpression([]byte(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatExpressionMandatoryInlineMarker(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{{`(1+2);!;+(3+4)`, `(1 + 2);!;+ (3 + 4)`}, {`;!;  A+B`, `;!;  A + B`}, {`A+B  ;!;`, `A + B  ;!;`}, {`A+B ;!; + C`, `A + B ;!; + C`}, {`A+@"x {  B ;!; C  }"+D`, `A + @"x {  B ;!; C  }" + D`}}
	for _, tt := range tests {
		got, err := formatExpression([]byte(tt.input), false)
		if err != nil {
			t.Fatalf("formatExpression(%q): %v", tt.input, err)
		}
		if string(got) != tt.want {
			t.Fatalf("formatExpression(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatMandatoryInlineMarkerInCommand(t *testing.T) {
	r := FormatWithOptions([]byte("@F\nRETURN (1+2);!;+(3+4)\n"), ERB, Options{})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", r.Diagnostics)
	}
	if got, want := string(r.Output), "@F\n    RETURN (1 + 2);!;+ (3 + 4)\n"; got != want {
		t.Fatalf("command output = %q, want %q", got, want)
	}
}

func TestMandatoryMarkerCommentAndTrailingBoundaries(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"RETURN ;!;  A+B", "RETURN ;!;  A + B"},
		{"RETURN A+B  ;!;  ", "RETURN A + B  ;!;  "},
		{"RETURN A+B;!;+C   ;tail  ", "RETURN A + B;!;+ C ; tail"},
		{"RETURN A+B;!;  ;tail  ", "RETURN A + B;!;  ;tail  "},
		{"PRINTV A+B;!;+C   ;tail  ", "PRINTV A + B;!;+ C ; tail"},
		{"PRINTV A+B  ;!;  ", "PRINTV A + B  ;!;  "},
		{"PRINTV 'raw ;!;  ", "PRINTV 'raw ;!;  "},
		{"PRINTV A+B;!;;#;+C  ", "PRINTV A+B;!;;#;+C  "},
		{"PRINT_RECT 1;!; , 2,3,4", "PRINT_RECT 1;!; , 2, 3, 4"},
		{"PRINT_RECT 1, ;!; 2,3,4", "PRINT_RECT 1, ;!; 2, 3, 4"},
		{"CALL F( ;!; 1+2 )", "CALL F( ;!; 1 + 2)"},
		{"CALL F(1+2 ;!;  )", "CALL F(1 + 2 ;!;  )"},
	} {
		opts := Options{Select: []RuleID{BinarySpacing, CommaSpacing, DelimiterSpacing, InlineCommentSpacing, CommentPrefixSpacing, CommentTrailingWhitespace, CodeTrailingWhitespace}}
		r := FormatWithOptions([]byte("@F\n"+tt.input+"\n"), ERB, opts)
		if string(r.Output) != "@F\n"+tt.want+"\n" || len(r.Diagnostics) != 0 {
			t.Fatalf("%q output=%q diagnostics=%v", tt.input, r.Output, r.Diagnostics)
		}
		again := FormatWithOptions(r.Output, ERB, opts)
		if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
			t.Fatalf("%q is not idempotent: %+v", tt.input, again)
		}
	}
}

func TestFormatExpressionMandatoryMarkerBoundariesStayOpaque(t *testing.T) {
	for _, input := range []string{`A+B;!;;!;+C`, `A+B;!;;#;+C`, `A+B;#;;!;+C`, `A+B;!;[[alias]]+C`, "A+\u3000B;!;+C"} {
		got, err := formatExpression([]byte(input), false)
		if err != nil {
			t.Fatalf("formatExpression(%q): %v", input, err)
		}
		if string(got) != input {
			t.Fatalf("opaque marker boundary changed %q to %q", input, got)
		}
	}
}

func TestFormatExpressionIdempotentAndTokenPreserving(t *testing.T) {
	inputs := []string{`- -A + !(B == "x+y")`, `F(0x12p+2,0b101, A[1:3])`, `A ? "left # side" # B`}
	for _, input := range inputs {
		first, err := formatExpression([]byte(input), false)
		if err != nil {
			t.Fatalf("first format %q: %v", input, err)
		}
		second, err := formatExpression(first, false)
		if err != nil {
			t.Fatalf("second format %q: %v", first, err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("not idempotent: first %q, second %q", first, second)
		}
	}
}

func TestFormatExpressionErrorsDoNotMutate(t *testing.T) {
	for _, input := range []string{`A+(B`, `A+([)]`, `A+0b102`, `A+0b1e102`, `A+0x-FF`, `A+0x1p-2`, "A+1２", "A+1e２", `A+"unterminated`} {
		got, err := formatExpression([]byte(input), false)
		if err == nil {
			t.Fatalf("formatExpression(%q) returned nil error", input)
		}
		if string(got) != input {
			t.Fatalf("error changed input: got %q, want %q", got, input)
		}
	}
}

func FuzzFormatExpression(f *testing.F) {
	for _, seed := range []string{"A+B", `F(1,"x")`, `- -A`, `A?B#C`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		first, err := formatExpression([]byte(input), false)
		if err != nil {
			return
		}
		second, err := formatExpression(first, false)
		if err != nil {
			t.Fatalf("formatted output cannot be read: %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("format is not idempotent: %q then %q", first, second)
		}
	})
}
