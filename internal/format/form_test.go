package format

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func allFormRules(t *testing.T) ruleSet {
	t.Helper()
	rules, err := resolveRules(Options{})
	if err != nil {
		t.Fatal(err)
	}
	rules[FormPadding] = true
	return rules
}

func applyForm(t *testing.T, input string, mode formMode) string {
	t.Helper()
	rules := allFormRules(t)
	edits, err := formEdits([]byte(input), rules, mode)
	if err != nil {
		t.Fatalf("formEdits(%q): %v", input, err)
	}
	return string(applyEdits([]byte(input), edits))
}

func TestFormEditsLineAndQuoted(t *testing.T) {
	if got, want := applyForm(t, `text { A+B, 6, RIGHT } % F( A , -1 ) %`, formLine), `text {A + B, 6, RIGHT} %F(A, -1)%`; got != want {
		t.Fatalf("line FORM = %q, want %q", got, want)
	}
	if got, want := applyForm(t, `@"text { A+B }"`, formQuoted), `@"text {A + B}"`; got != want {
		t.Fatalf("quoted FORM = %q, want %q", got, want)
	}
}

func TestFormPlaceholderFieldShape(t *testing.T) {
	rules := allFormRules(t)
	valid := map[string]string{
		`{1,}`:                `{1,}`,
		`{1,2,}`:              `{1, 2,}`,
		`{1,2,EFAUDIT_ALIGN}`: `{1, 2, EFAUDIT_ALIGN}`,
		`{F(1,2),3,RIGHT}`:    `{F(1, 2), 3, RIGHT}`,
		`{1,2,F(1,2)}`:        `{1, 2, F(1, 2)}`,
	}
	for input, want := range valid {
		edits, err := formEdits([]byte(input), rules, formLine)
		if err != nil {
			t.Fatalf("valid FORM %q: %v", input, err)
		}
		if got := string(applyEdits([]byte(input), edits)); got != want {
			t.Fatalf("valid FORM %q changed to %q, want %q", input, got, want)
		}
	}

	input := `x {1,2,RIGHT,4} % RESULTS+CSTR %`
	edits, err := formEdits([]byte(input), rules, formLine)
	if err == nil {
		t.Fatal("too many FORM fields were accepted")
	}
	if got, want := string(applyEdits([]byte(input), edits)), `x {1,2,RIGHT,4} %RESULTS + CSTR%`; got != want {
		t.Fatalf("partial FORM output = %q, want %q", got, want)
	}

	input = `x {1,,RIGHT} % RESULTS+CSTR %`
	edits, err = formEdits([]byte(input), rules, formLine)
	if err == nil {
		t.Fatal("empty FORM width was accepted")
	}
	if got, want := string(applyEdits([]byte(input), edits)), `x {1,,RIGHT} %RESULTS + CSTR%`; got != want {
		t.Fatalf("empty-width FORM output = %q, want %q", got, want)
	}
	for _, alignment := range []string{`0`, `"LEFT"`, `@"LEFT"`, `(RIGHT)`, `\@1?LEFT#RIGHT\@`} {
		input := `x {1,2,` + alignment + `} % RESULTS+CSTR %`
		edits, err := formEdits([]byte(input), rules, formLine)
		want := `x {1,2,` + alignment + `} %RESULTS + CSTR%`
		if err == nil || string(applyEdits([]byte(input), edits)) != want {
			t.Fatalf("literal alignment %q: edits=%+v error=%v", alignment, edits, err)
		}
	}
}

func TestFormEditsConditionalNestedAndEscaped(t *testing.T) {
	input := `\@ A+1 ? left \# { B+2 } # right \@`
	want := `\@ A + 1 ? left \# {B + 2} # right \@`
	if got := applyForm(t, input, formLine); got != want {
		t.Fatalf("conditional FORM = %q, want %q", got, want)
	}
	input = `*** { A+B } +++ \{literal\}`
	want = `*** {A + B} +++ \{literal\}`
	if got := applyForm(t, input, formLine); got != want {
		t.Fatalf("escaped/triple FORM = %q, want %q", got, want)
	}
}

func TestFormConditionalModeAndSpacing(t *testing.T) {
	rules, err := resolveRules(Options{Select: []RuleID{BinarySpacing, FormConditionSpacing}})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		input string
		want  string
	}{
		{`\@A+1?yes#no\@`, `\@ A + 1 ?yes#no\@`},
		{`\@  A + 1  ?  yes  \@`, `\@ A + 1 ?  yes  \@`},
		{`\@A?yes#right  \@`, `\@ A ?yes#right  \@`},
	} {
		edits, err := formEdits([]byte(test.input), rules, formConditional)
		if err != nil {
			t.Fatalf("formEdits(%q): %v", test.input, err)
		}
		if got := string(applyEdits([]byte(test.input), edits)); got != test.want {
			t.Fatalf("conditional FORM = %q, want %q", got, test.want)
		}
	}

	input := `\@A?yes  \@`
	fingerprint, err := formFingerprintMode([]byte(input), formConditional)
	if err != nil {
		t.Fatal(err)
	}
	formatted := `\@ A ?yes  \@`
	compact, err := formFingerprintMode([]byte(formatted), formConditional)
	if err != nil || !bytes.Equal(fingerprint, compact) {
		t.Fatalf("conditional fingerprint changed with condition trivia: %q vs %q", fingerprint, compact)
	}
}

func TestFormModesAndFullWidthLiteral(t *testing.T) {
	call := `name { A+1 }(ARG, next`
	end, err := scanForm([]byte(call), formCall)
	if err != nil || string([]byte(call)[:end]) != `name { A+1 }` {
		t.Fatalf("scanForm(call) = %d, %v", end, err)
	}
	comma := `name { A+1 }, next`
	end, err = scanForm([]byte(comma), formComma)
	if err != nil || string([]byte(comma)[:end]) != `name { A+1 }` {
		t.Fatalf("scanForm(comma) = %d, %v", end, err)
	}
	input := "literal　{ A+B }"
	want := "literal　{A + B}"
	if got := applyForm(t, input, formLine); got != want {
		t.Fatalf("full-width FORM literal = %q, want %q", got, want)
	}
}

func TestFormExpressionAtomAndFingerprint(t *testing.T) {
	input := `A+@"literal { B+1 }"`
	want := `A + @"literal {B + 1}"`
	rules := allFormRules(t)
	edits, err := expressionEdits([]byte(input), false, rules)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(applyEdits([]byte(input), edits)); got != want {
		t.Fatalf("FORM expression atom = %q, want %q", got, want)
	}
	fingerprint, err := formFingerprint([]byte(`@"x { A + 1 }"`))
	if err != nil || len(fingerprint) == 0 {
		t.Fatalf("form fingerprint = %q, %v", fingerprint, err)
	}
	compact, err := formFingerprint([]byte(`@"x {A+1}"`))
	if err != nil || !bytes.Equal(fingerprint, compact) {
		t.Fatalf("FORM fingerprint changed with island trivia: %q vs %q", fingerprint, compact)
	}
}

func TestExpressionFormAtomEditsNestedAndPaddingOnly(t *testing.T) {
	all := allFormRules(t)
	input := `A+@"x { A+B }"`
	edits, err := expressionEdits([]byte(input), false, all)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(applyEdits([]byte(input), edits)), `A + @"x {A + B}"`; got != want {
		t.Fatalf("nested expression FORM = %q, want %q", got, want)
	}
	rules, err := resolveRules(Options{Select: []RuleID{FormPadding}})
	if err != nil {
		t.Fatal(err)
	}
	edits, err = expressionEdits([]byte(input), false, rules)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(applyEdits([]byte(input), edits)), `A+@"x {A+B}"`; got != want {
		t.Fatalf("S017-only nested FORM = %q, want %q", got, want)
	}
}

func TestFormIslandErrorAfterConfirmedIslands(t *testing.T) {
	rules := allFormRules(t)
	input := `x { A+B } % A+@ % { C+D }`
	edits, err := formEdits([]byte(input), rules, formLine)
	if err == nil {
		t.Fatal("formEdits returned nil error for invalid middle island")
	}
	got := string(applyEdits([]byte(input), edits))
	if got != `x {A + B} % A+@ % {C + D}` {
		t.Fatalf("confirmed islands = %q", got)
	}

	input = `x { A+B } % A+@`
	edits, err = formEdits([]byte(input), rules, formLine)
	if err == nil {
		t.Fatal("formEdits returned nil error for unclosed container")
	}
	if len(edits) != 0 {
		t.Fatalf("unclosed container retained edits: %+v", edits)
	}
}

func TestFormErrorsPreserveInput(t *testing.T) {
	for _, input := range []string{`x { A+1`, `x {   }`, `x % A+1`, `x %   %`, `\@A+1?left#right`, `\@A?left\@B?x#y\@#right\@`} {
		edits, err := formEdits([]byte(input), allFormRules(t), formLine)
		if err == nil {
			t.Fatalf("formEdits(%q) returned nil error", input)
		}
		if got := applyEdits([]byte(input), edits); !bytes.Equal(got, []byte(input)) {
			t.Fatalf("error changed %q to %q", input, got)
		}
	}
}

func TestFormEditsIdempotent(t *testing.T) {
	rules := allFormRules(t)
	input := []byte(`@"x {  A+B  } % F( A,-1 ) %"`)
	first, err := formEdits(input, rules, formQuoted)
	if err != nil {
		t.Fatal(err)
	}
	formatted := applyEdits(input, first)
	second, err := formEdits(formatted, rules, formQuoted)
	if err != nil {
		t.Fatal(err)
	}
	if got := applyEdits(formatted, second); !bytes.Equal(got, formatted) {
		t.Fatalf("FORM not idempotent: %q -> %q", formatted, got)
	}
}

func TestReturnFormErrorPosition(t *testing.T) {
	for _, input := range []string{"RETURNFORM a, % A", "RETURNFORM €, % A"} {
		r := FormatWithOptions([]byte("@F\n"+input+"\n"), ERB, Options{})
		if len(r.Diagnostics) != 1 || r.Diagnostics[0].Line != 2 || r.Diagnostics[0].Column != 16 {
			t.Fatalf("%q: diagnostics=%+v", input, r.Diagnostics)
		}
	}
}

func TestNestedFormErrorPosition(t *testing.T) {
	input := []byte("@F\nRETURN @\"text { 0b2 }\"\n")
	r := FormatWithOptions(input, ERB, Options{})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Line != 2 || r.Diagnostics[0].Column != 19 || r.Diagnostics[0].Code != "unsupported_expression" {
		t.Fatalf("diagnostic position %+v", r.Diagnostics)
	}
}

func TestFormMandatoryInlineMarkerPreservesOnlyAffectedIsland(t *testing.T) {
	rules := allFormRules(t)
	input := `x {  A ;!; B  } % RESULTS+CSTR %`
	edits, err := formEdits([]byte(input), rules, formLine)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(applyEdits([]byte(input), edits)), `x {  A ;!; B  } %RESULTS + CSTR%`; got != want {
		t.Fatalf("FORM marker output = %q, want %q", got, want)
	}
}

func TestFormMandatoryInlineMarkerWorksInConditionalAndFingerprint(t *testing.T) {
	rules := allFormRules(t)
	input := `x \@  A ;!; B  ? yes # no \@ { C+D }`
	edits, err := formEdits([]byte(input), rules, formLine)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(applyEdits([]byte(input), edits)), `x \@  A ;!; B  ? yes # no \@ {C + D}`; got != want {
		t.Fatalf("conditional marker output = %q, want %q", got, want)
	}

	withPadding, err := formFingerprint([]byte(`@"x {  A ;!; B  }"`))
	if err != nil {
		t.Fatal(err)
	}
	withoutPadding, err := formFingerprint([]byte(`@"x {A ;!; B}"`))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(withPadding, withoutPadding) {
		t.Fatal("mandatory marker island fingerprint discarded raw padding")
	}
	if _, err := formFingerprint([]byte(`@"x { A + @"nested { B ;!; C }" }"`)); err != nil {
		t.Fatalf("nested marker FORM fingerprint: %v", err)
	}
}

func TestExpressionSessionBoundsAndSourceIsolation(t *testing.T) {
	src := []byte(`@"x { A+B }" trailing`)
	session := newExpressionSession(src)
	limit := strings.Index(string(src), " trailing")
	end, err := session.scanFormAt(expressionSourceOriginal, 0, limit, formQuoted)
	if err != nil || end != limit {
		t.Fatalf("bounded scan = %d, %v; want %d", end, err, limit)
	}
	largerEnd, err := session.scanFormAt(expressionSourceOriginal, 0, len(src), formQuoted)
	if err != nil || largerEnd != limit || len(session.scans) != 1 {
		t.Fatalf("explicit scan reuse = %d, %v, cache size %d", largerEnd, err, len(session.scans))
	}
	wide := newExpressionSession(src)
	if end, err := wide.scanFormAt(expressionSourceOriginal, 0, len(src), formQuoted); err != nil || end != limit {
		t.Fatalf("wide scan = %d, %v; want %d", end, err, limit)
	}
	if end, err := wide.scanFormAt(expressionSourceOriginal, 0, limit, formQuoted); err != nil || end != limit || len(wide.scans) != 1 {
		t.Fatalf("narrow explicit reuse = %d, %v, cache size %d", end, err, len(wide.scans))
	}
	if _, err := session.scanFormAt(expressionSourceOriginal, 0, 3, formQuoted); err == nil {
		t.Fatal("bounded scan accepted an unterminated FORM")
	}

	first, err := session.formFingerprintAt(expressionSourceOriginal, 0, limit, formQuoted)
	if err != nil {
		t.Fatal(err)
	}
	session.setSource(expressionSourceFormatted, []byte(`@"x { A+B }"`))
	second, err := session.formFingerprintAt(expressionSourceFormatted, 0, len(session.sources[expressionSourceFormatted]), formQuoted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("trivia-only source changed fingerprint: %q vs %q", first, second)
	}
	session.setSource(expressionSourceFormatted, []byte(`@"x { A ;!; B }"`))
	third, err := session.formFingerprintAt(expressionSourceFormatted, 0, len(session.sources[expressionSourceFormatted]), formQuoted)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(second, third) {
		t.Fatal("formatted source reused a stale fingerprint")
	}
}

func TestExpressionSessionAliasIndexOverlaps(t *testing.T) {
	session := newExpressionSession([]byte(`[[[`))
	if !session.hasAlias(expressionSourceOriginal, 1, 3) {
		t.Fatal("overlapping alias marker was not indexed")
	}
	if session.hasAlias(expressionSourceOriginal, 2, 3) {
		t.Fatal("partial alias marker was reported")
	}
}

func BenchmarkNestedFormExpression(b *testing.B) {
	rules, err := resolveRules(Options{})
	if err != nil {
		b.Fatal(err)
	}
	for _, depth := range []int{16, 64, 120} {
		b.Run(fmt.Sprintf("depth-%d", depth), func(b *testing.B) {
			source := `@"leaf"`
			for i := 0; i < depth; i++ {
				source = `@"x % ` + source + ` %"`
			}
			input := []byte(source)
			b.SetBytes(int64(len(input)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := expressionEdits(input, false, rules); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
