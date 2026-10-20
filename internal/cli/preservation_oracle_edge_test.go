package cli

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestPreservationOracleVerifierGrammarEdges(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		dialect format.Dialect
		full    bool
	}{
		{"inline-double-semicolon", "@F\nRETURN 1 ;;note\n", format.CommonDialect, false},
		{"printv-unicode", "@F\nPRINTV αβ  \n", format.CommonDialect, false},
		{"printv-nul", "@F\nPRINTV A\x00B  \n", format.CommonDialect, false},
		{"printv-fullwidth-plain", "@F\nPRINTV raw　text  \n", format.CommonDialect, true},
		{"graphics-marker-tail", "@F\nPRINT_SPACE 1 ;!; payload  \n", format.CommonDialect, false},
		{"split-fullwidth-comment-gap", "@F\nSPLIT \"a\",\",\",RESULTS　;note　　\n", format.CommonDialect, true},
		{"printbutton-form-string", "@F\nPRINTBUTTON @\"x { 1+2 }\",1\n", format.CommonDialect, false},
		{"setfont-form-string", "@F\nSETFONT @\"x { 1+2 }\"\n", format.EMEE, false},
		{"savevar-form-destination", "@F\nSAVEVAR \"f\",\"m\",@\"a { 1+2 }\"\n", format.CommonDialect, false},
		{"loaddata-modulo", "@F\nLOADDATA 1　%　2\n", format.CommonDialect, true},
		{"loaddata-nested-form", "@F\nLOADDATA GETNUM(A,@\"{1　+　2}\")\n", format.CommonDialect, true},
		{"loadvar-invalid-form-neighbor", "@F\nLOADVAR @\"{A　+}\" {1　+　2}\n", format.CommonDialect, true},
		{"setcolor-conditional-comment", "@F\nSETCOLOR GETCOLOR() == 0xA0A0A0 ? COLOR_FN(ARG:0) # GETCOLOR()\t;note\n", format.EMEE, true},
		{"parenthesized-string-comparison", "@F\nPRINTFORM \\@(RESULTS:0 != \"\") ? yes # no \\@\n", format.EMEE, true},
		{"string-builtin-repetition", "@F\nPRINTFORM %CALLNAME*MASTER%\n", format.EMEE, true},
		{"nested-form-repetition", "@F\nARGS '= @\"%@\"%\"x\"*9%\"*7%\"\n", format.EMEE, true},
		{"numeric-string-index", "@F\nPRINTFORM {ABL:(\"key\"+TOSTR(LOCAL:0))}\n", format.EMEE, true},
		{"percent-indexed-string", "@F\nPRINTFORM %ARGS:0%\n", format.EMEE, true},
		{"percent-literal-commas", "@F\nPRINTFORM %@\"text, 5, 7\",20,LEFT%\n", format.EMEE, true},
		{"percent-ternary-form", "@F\nARGS '= @\"%ARG==0?@\"%CALLNAME:0%'s\"#@\"%LOCALS%'s\"%\"\n", format.EMEE, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			options := format.Options{Dialect: tc.dialect, AllowFullwidthSpace: tc.full}
			result := format.FormatWithOptions([]byte(tc.source), format.ERB, options)
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == format.Error {
					t.Fatalf("unexpected fatal diagnostic: %+v", diagnostic)
				}
			}
			preserve := sourcePreservationOptions{dialect: tc.dialect, allowFullwidthSpace: tc.full}
			if err := checkSourcePreservedOptions([]byte(tc.source), result.Output, preserve); err != nil {
				t.Fatalf("output=%q: %v", result.Output, err)
			}
		})
	}
	if err := checkSourcePreserved([]byte("@F\n;;standalone\n;---\n;#;marker\n"), []byte("@F\n;;standalone\n;---\n;#;marker\n")); err != nil {
		t.Fatalf("protected comment controls: %v", err)
	}
	if err := checkSourcePreserved([]byte("@F\nRETURN 1 ;;note\n"), []byte("@F\nRETURN 1 ; ;note\n")); err != nil {
		t.Fatalf("inline double-semicolon canonicalization: %v", err)
	}
}

func TestPreservationOracleVerifierFormProofs(t *testing.T) {
	tests := []struct {
		name, source string
		full         bool
	}{
		{"invalid-ascii", "@F\nPRINTFORM { A + }\n", false},
		{"invalid-string-type", "@F\nPRINTFORM { \"x\" }\n", false},
		{"invalid-string-index-key", "@F\nPRINTFORM { ARGS:\"key\" } {1+2}\n", false},
		{"invalid-indexed-string", "@F\nPRINTFORM { ARGS:0 } {1+2}\n", false},
		{"warning-negative-width-neighbor", "@F\nPRINTFORM { 1 , -1 } {1+2}\n", false},
		{"invalid-fullwidth", "@F\nPRINTFORM　{A　+}\n", true},
		{"unterminated-fullwidth-form", "@F\nPRINTFORMS　\"x　{A　+　B}\n", true},
		{"escaped-outer-quote", "@F\nPRINTFORMS \"prefix \\\"x {A　+　B}\\\"\"\n", true},
		{"conditional-invalid-shape", "@F\nPRINTV　\\@A　+?yes#no\\@\n", true},
		{"conditional-string-condition", "@F\nPRINTFORM \\@\"x\"?yes#no\\@\n", false},
		{"conditional-string-identifier", "@F\nPRINTFORM \\@ARGS?yes#no\\@\n", false},
		{"conditional-macro-string", "@F\nPRINTFORM \\@\"a\"+UNKNOWN?yes#no\\@\n", false},
		{"conditional-macro-words", "@F\nPRINTFORM \\@UNKNOWN OTHER?yes#no\\@\n", false},
		{"conditional-string-subtraction", "@F\nPRINTFORM \\@\"a\"-\"b\"?yes#no\\@\n", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			options := format.Options{AllowFullwidthSpace: tc.full}
			result := format.FormatWithOptions([]byte(tc.source), format.ERB, options)
			preserve := sourcePreservationOptions{allowFullwidthSpace: tc.full}
			if err := checkSourcePreservedOptions([]byte(tc.source), result.Output, preserve); err != nil {
				t.Fatalf("output=%q: %v", result.Output, err)
			}
			mutated := bytes.ReplaceAll([]byte(tc.source), []byte("\u3000"), []byte(" "))
			if !bytes.Equal(mutated, []byte(tc.source)) {
				mutatedOutput := format.FormatWithOptions(mutated, format.ERB, options).Output
				if err := checkSourcePreservedOptions([]byte(tc.source), mutatedOutput, preserve); err == nil {
					t.Fatalf("accepted protected fullwidth mutation: %q", mutatedOutput)
				}
			}
			if tc.name == "conditional-string-condition" || tc.name == "conditional-string-identifier" || tc.name == "conditional-string-subtraction" {
				changed := result.Output
				if tc.name == "conditional-string-condition" {
					changed = bytes.ReplaceAll(changed, []byte(`"x"`), []byte(`"y"`))
				} else if tc.name == "conditional-string-subtraction" {
					changed = bytes.ReplaceAll(changed, []byte(`"a"`), []byte(`"z"`))
				} else {
					changed = bytes.ReplaceAll(changed, []byte("ARGS?"), []byte("ARGS ?"))
				}
				if err := checkSourcePreservedOptions([]byte(tc.source), changed, preserve); err == nil {
					t.Fatalf("accepted invalid string condition mutation: %q", changed)
				}
			}
			if tc.name == "invalid-ascii" {
				mutatedOutput := bytes.Replace(result.Output, []byte("A +"), []byte("A+"), 1)
				if bytes.Equal(mutatedOutput, result.Output) {
					t.Fatalf("invalid ASCII island mutation did not change output: %q", result.Output)
				}
				if err := checkSourcePreservedOptions([]byte(tc.source), mutatedOutput, preserve); err == nil {
					t.Fatalf("accepted invalid ASCII island mutation: %q", mutatedOutput)
				}
			}
			if tc.name == "invalid-string-index-key" {
				changed := bytes.Replace(result.Output, []byte("{ ARGS:\"key\" }"), []byte("{ARGS:\"key\"}"), 1)
				if bytes.Equal(changed, result.Output) {
					t.Fatalf("string index mutation did not change output: %q", result.Output)
				}
				if err := checkSourcePreservedOptions([]byte(tc.source), changed, preserve); err == nil {
					t.Fatalf("accepted invalid string index mutation: %q", changed)
				}
			}
			if tc.name == "invalid-indexed-string" {
				changed := bytes.Replace(result.Output, []byte("{ ARGS:0 }"), []byte("{ARGS:0}"), 1)
				if bytes.Equal(changed, result.Output) {
					t.Fatalf("indexed string mutation did not change output: %q", result.Output)
				}
				if err := checkSourcePreservedOptions([]byte(tc.source), changed, preserve); err == nil {
					t.Fatalf("accepted invalid indexed string mutation: %q", changed)
				}
			}
			if tc.name == "invalid-string-type" {
				mutatedOutput := bytes.Replace(result.Output, []byte("{ \"x\" }"), []byte("{\"x\"}"), 1)
				if bytes.Equal(mutatedOutput, result.Output) {
					t.Fatalf("invalid string island mutation did not change output: %q", result.Output)
				}
				if err := checkSourcePreservedOptions([]byte(tc.source), mutatedOutput, preserve); err == nil {
					t.Fatalf("accepted invalid string island mutation: %q", mutatedOutput)
				}
			}
		})
	}

	positive := []byte("@F\nPRINTFORM {1, 2-1}\n")
	positiveResult := format.FormatWithOptions(positive, format.ERB, format.Options{})
	if err := checkSourcePreserved(positive, positiveResult.Output); err != nil {
		t.Fatalf("positive width expression rejected: %v", err)
	}
	negative := []byte("@F\nPRINTFORM {1, 2147483648}\n")
	negativeResult := format.FormatWithOptions(negative, format.ERB, format.Options{})
	if err := checkSourcePreserved(negative, negativeResult.Output); err != nil {
		t.Fatalf("warning width expression rejected: %v", err)
	}
	conditional := []byte("@F\nPRINTFORM \\@\"x\"?{1+2}#no\\@ {3+4}\n")
	conditionalResult := format.FormatWithOptions(conditional, format.ERB, format.Options{})
	if err := checkSourcePreserved(conditional, conditionalResult.Output); err != nil {
		t.Fatalf("invalid condition with editable branches rejected: output=%q err=%v", conditionalResult.Output, err)
	}
	conditionChanged := bytes.ReplaceAll(conditionalResult.Output, []byte(`"x"`), []byte(`"y"`))
	if err := checkSourcePreserved(conditional, conditionChanged); err == nil {
		t.Fatalf("accepted invalid conditional condition mutation: %q", conditionChanged)
	}
}

func TestPreservationOracleVerifierFormShapeMatrix(t *testing.T) {
	for _, source := range []string{
		"@F\nPRINTFORM { 1, }\n",
		"@F\nPRINTFORM {1,2,}\n",
		"@F\nPRINTFORM {1,~1+2}\n",
		"@F\nPRINTFORM {++A}\n",
		"@F\nPRINTFORM {A+(++B)}\n",
		"@F\nPRINTFORM {++MAXBASE}\n",
		"@F\nPRINTFORM {++RAND}\n",
		"@F\nPRINTFORM {UNKNOWN,1,LOCAL}\n",
		"@F\nPRINTFORM {UNKNOWN,\"x\"}\n",
		"@F\nPRINTFORM {UNKNOWN,A+}\n",
		"@F\nPRINTFORM {UNKNOWN OTHER}\n",
		"@F\nPRINTFORM {1, UNKNOWN OTHER}\n",
		"@F\nPRINTFORM {UNKNOWN+}\n",
		"@F\nPRINTFORM {RESULTS}\n",
		"@F\nPRINTFORM {1,UNKNOWN,LOCAL}\n",
		"@F\nPRINTFORM {1,9223372036854775808}\n",
		"@F\nPRINTFORM {1,1 / 0}\n",
		"@F\nPRINTFORM {( )}\n",
		"@F\nPRINTFORM {A += B}\n",
		"@F\nPRINTFORM {A = B}\n",
		"@F\nPRINTFORM {(A = B)}\n",
		"@F\nPRINTFORM {F(A += B)}\n",
		"@F\nPRINTFORM %\"a\",-1%\n",
		"@F\nPRINTFORM %\"x\",1,LOCAL%\n",
		"@F\nPRINTFORM % UNKNOWN OTHER %\n",
	} {
		result := format.FormatWithOptions([]byte(source), format.ERB, format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true})
		if err := checkSourcePreservedOptions([]byte(source), result.Output, sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}); err != nil {
			t.Fatalf("source=%q output=%q diagnostics=%v: %v", source, result.Output, result.Diagnostics, err)
		}
	}
	invalid := []byte("@F\nPRINTFORM { ++RAND }\n")
	result := format.FormatWithOptions(invalid, format.ERB, format.Options{})
	mutated := bytes.Replace(result.Output, []byte("++RAND"), []byte("++ RAND"), 1)
	if bytes.Equal(mutated, result.Output) {
		t.Fatalf("read-only increment mutation did not change output: %q", result.Output)
	}
	if err := checkSourcePreserved(invalid, mutated); err == nil {
		t.Fatalf("accepted read-only increment mutation: %q", mutated)
	}
}

func TestPreservationOraclePrefixIncrementGap(t *testing.T) {
	source := []byte("@F\nSIF ++ LOCAL:11 > 3\nPRINTV x\n")
	result := format.FormatWithOptions(source, format.ERB, format.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("prefix increment diagnostics=%v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreserved(source, result.Output); err != nil {
		t.Fatalf("prefix increment gap rejected: %v output=%q", err, result.Output)
	}
	invalid := []byte("@F\nSIF ++ ARGS:1 > 3\nPRINTV x\n")
	invalidResult := format.FormatWithOptions(invalid, format.ERB, format.Options{})
	mutated := bytes.Replace(invalidResult.Output, []byte("++ ARGS:1"), []byte("++ARGS:1"), 1)
	if bytes.Equal(mutated, invalidResult.Output) {
		t.Fatal("invalid string increment mutation was not applied")
	}
	if err := checkSourcePreserved(invalid, mutated); err == nil {
		t.Fatalf("accepted invalid string increment mutation: %q", mutated)
	}
}

func TestPreservationOracleVerifierFormWarningWidths(t *testing.T) {
	for _, tc := range []struct{ source, from, to string }{
		{"@F\nPRINTFORM {1,0-2*1}\n", "0-2*1", "0 - 2 * 1"},
		{"@F\nPRINTFORM {1,~1+1}\n", "~1+1", "~ 1 + 1"},
		{"@F\nPRINTFORM {1,1<<31}\n", "1<<31", "1 << 31"},
		{"@F\nPRINTFORM {1,1p31}\n", "1p31", "1p31"},
		{"@F\nPRINTFORM {1,4294967295}\n", "4294967295", "4294967295"},
		{"@F\nPRINTFORM {1,0?1#-1}\n", "0?1#-1", "0 ? 1 # -1"},
	} {
		result := format.FormatWithOptions([]byte(tc.source), format.ERB, format.Options{})
		if err := checkSourcePreserved([]byte(tc.source), result.Output); err != nil {
			t.Fatalf("source=%q output=%q diagnostics=%v: %v", tc.source, result.Output, result.Diagnostics, err)
		}
		mutated := bytes.Replace(result.Output, []byte(tc.from), []byte(tc.to), 1)
		if bytes.Equal(mutated, result.Output) {
			continue
		}
		if err := checkSourcePreserved([]byte(tc.source), mutated); err == nil {
			t.Fatalf("accepted warning width mutation source=%q output=%q mutated=%q diagnostics=%v", tc.source, result.Output, mutated, result.Diagnostics)
		}
	}
}

func TestPreservationOracleVerifierConfirmedFormCases(t *testing.T) {
	for _, source := range []string{
		"@F\nPRINTFORM {1,1<<1}\n",
		"@F\nPRINTFORM {1,1>>1}\n",
		"@F\nPRINTFORM {1,0b10<<1}\n",
		"@F\nPRINTFORM \\@(1<<1)?yes#no\\@\n",
	} {
		result := format.FormatWithOptions([]byte(source), format.ERB, format.Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("valid shift source=%q diagnostics=%v output=%q", source, result.Diagnostics, result.Output)
		}
		if err := checkSourcePreserved([]byte(source), result.Output); err != nil {
			t.Fatalf("valid shift source=%q output=%q: %v", source, result.Output, err)
		}
	}

	for _, tc := range []struct {
		name, source, from, to string
	}{
		{"mixed builtin", "@F\nPRINTFORM % A+\"x\" %\n", `% A+"x" %`, `% A + "x" %`},
		{"mixed constant", "@F\nPRINTFORM % 1+\"x\" %\n", `% 1+"x" %`, `% 1 + "x" %`},
		{"mixed reverse", "@F\nPRINTFORM % \"x\"+1 %\n", `% "x"+1 %`, `% "x" + 1 %`},
	} {
		result := format.FormatWithOptions([]byte(tc.source), format.ERB, format.Options{Dialect: format.EMEE})
		if len(result.Diagnostics) == 0 {
			t.Fatalf("invalid mixed source=%q had no diagnostic; output=%q", tc.source, result.Output)
		}
		options := sourcePreservationOptions{dialect: format.EMEE}
		if err := checkSourcePreservedOptions([]byte(tc.source), result.Output, options); err != nil {
			t.Fatalf("invalid mixed source=%q output=%q: %v", tc.source, result.Output, err)
		}
		mutated := bytes.Replace(result.Output, []byte(tc.from), []byte(tc.to), 1)
		if bytes.Equal(mutated, result.Output) {
			t.Fatalf("invalid mixed source=%q mutation was not applied: %q", tc.source, result.Output)
		}
		if err := checkSourcePreservedOptions([]byte(tc.source), mutated, options); err == nil {
			t.Fatalf("accepted invalid mixed whitespace mutation source=%q output=%q", tc.source, mutated)
		}
	}

	validIndex := []byte("@F\nPRINTFORM {ARGS:(TOSTR(0))}\n")
	validIndexResult := format.FormatWithOptions(validIndex, format.ERB, format.Options{Dialect: format.EMEE})
	if len(validIndexResult.Diagnostics) != 0 {
		t.Fatalf("valid string index diagnostics=%v output=%q", validIndexResult.Diagnostics, validIndexResult.Output)
	}
	if err := checkSourcePreservedDialect(validIndex, validIndexResult.Output, format.EMEE); err != nil {
		t.Fatalf("valid string index output=%q: %v", validIndexResult.Output, err)
	}
	validPercentIndex := []byte("@F\nPRINTFORM % ARGS:LOCAL %\n")
	validPercentIndexResult := format.FormatWithOptions(validPercentIndex, format.ERB, format.Options{Dialect: format.EMEE})
	if len(validPercentIndexResult.Diagnostics) != 0 {
		t.Fatalf("valid percent string index diagnostics=%v output=%q", validPercentIndexResult.Diagnostics, validPercentIndexResult.Output)
	}
	if err := checkSourcePreservedDialect(validPercentIndex, validPercentIndexResult.Output, format.EMEE); err != nil {
		t.Fatalf("valid percent string index output=%q: %v", validPercentIndexResult.Output, err)
	}
	unknownFirst := []byte("@F\nPRINTFORM {UNKNOWN,ARGS:LOCAL}\n")
	unknownFirstResult := format.FormatWithOptions(unknownFirst, format.ERB, format.Options{Dialect: format.EMEE})
	if len(unknownFirstResult.Diagnostics) != 0 {
		t.Fatalf("unknown-first string width diagnostics=%v output=%q", unknownFirstResult.Diagnostics, unknownFirstResult.Output)
	}
	if err := checkSourcePreservedDialect(unknownFirst, unknownFirstResult.Output, format.EMEE); err != nil {
		t.Fatalf("unknown-first string width output=%q: %v", unknownFirstResult.Output, err)
	}

	invalidIndex := []byte("@F\nPRINTFORM {ARGS:LOCAL}\n")
	invalidIndexResult := format.FormatWithOptions(invalidIndex, format.ERB, format.Options{Dialect: format.EMEE})
	if len(invalidIndexResult.Diagnostics) == 0 {
		t.Fatalf("invalid string index had no diagnostic; output=%q", invalidIndexResult.Output)
	}
	if err := checkSourcePreservedDialect(invalidIndex, invalidIndexResult.Output, format.EMEE); err != nil {
		t.Fatalf("invalid string index output=%q: %v", invalidIndexResult.Output, err)
	}
	mutatedIndex := bytes.Replace(invalidIndexResult.Output, []byte("{ARGS:LOCAL}"), []byte("{ ARGS:LOCAL }"), 1)
	if bytes.Equal(mutatedIndex, invalidIndexResult.Output) {
		t.Fatalf("invalid string index mutation was not applied: %q", invalidIndexResult.Output)
	}
	if err := checkSourcePreservedDialect(invalidIndex, mutatedIndex, format.EMEE); err == nil {
		t.Fatalf("accepted invalid string index whitespace mutation: %q", mutatedIndex)
	}

	invalidWidth := []byte("@F\nPRINTFORM {1,ARGS:LOCAL}\n")
	invalidWidthResult := format.FormatWithOptions(invalidWidth, format.ERB, format.Options{Dialect: format.EMEE})
	if len(invalidWidthResult.Diagnostics) == 0 {
		t.Fatalf("invalid string width had no diagnostic; output=%q", invalidWidthResult.Output)
	}
	if err := checkSourcePreservedDialect(invalidWidth, invalidWidthResult.Output, format.EMEE); err != nil {
		t.Fatalf("invalid string width output=%q: %v", invalidWidthResult.Output, err)
	}
	mutatedWidth := bytes.Replace(invalidWidthResult.Output, []byte("{1,ARGS:LOCAL}"), []byte("{1, ARGS:LOCAL}"), 1)
	if bytes.Equal(mutatedWidth, invalidWidthResult.Output) {
		t.Fatalf("invalid string width mutation was not applied: %q", invalidWidthResult.Output)
	}
	if err := checkSourcePreservedDialect(invalidWidth, mutatedWidth, format.EMEE); err == nil {
		t.Fatalf("accepted invalid string width whitespace mutation: %q", mutatedWidth)
	}
	for _, tc := range []struct{ source, from, to string }{
		{"@F\nPRINTFORM {(ARGS:LOCAL)}\n", "{(ARGS:LOCAL)}", "{ (ARGS:LOCAL) }"},
		{"@F\nPRINTFORM {ARGS:(LOCAL)}\n", "{ARGS:(LOCAL)}", "{ ARGS:(LOCAL) }"},
		{"@F\nPRINTFORM {ARGS:(LOCAL:0)}\n", "{ARGS:(LOCAL:0)}", "{ ARGS:(LOCAL:0) }"},
		{"@F\nPRINTFORM {ARGS:(LOCAL+1)}\n", "{ARGS:(LOCAL+1)}", "{ ARGS:(LOCAL+1) }"},
	} {
		result := format.FormatWithOptions([]byte(tc.source), format.ERB, format.Options{Dialect: format.EMEE})
		if len(result.Diagnostics) == 0 {
			t.Fatalf("grouped invalid index had no diagnostic source=%q output=%q", tc.source, result.Output)
		}
		if err := checkSourcePreservedDialect([]byte(tc.source), result.Output, format.EMEE); err != nil {
			t.Fatalf("grouped invalid index output=%q: %v", result.Output, err)
		}
		mutated := bytes.Replace(result.Output, []byte(tc.from), []byte(tc.to), 1)
		if bytes.Equal(mutated, result.Output) {
			t.Fatalf("grouped invalid index mutation was not applied source=%q output=%q", tc.source, result.Output)
		}
		if err := checkSourcePreservedDialect([]byte(tc.source), mutated, format.EMEE); err == nil {
			t.Fatalf("accepted grouped invalid index whitespace mutation: %q", mutated)
		}
	}
	percentChain := []byte("@F\nPRINTFORM % \"x\"+A+\"y\" %\n")
	percentChainResult := format.FormatWithOptions(percentChain, format.ERB, format.Options{Dialect: format.EMEE})
	if len(percentChainResult.Diagnostics) == 0 {
		t.Fatalf("mixed percent chain had no diagnostic; output=%q", percentChainResult.Output)
	}
	if err := checkSourcePreservedDialect(percentChain, percentChainResult.Output, format.EMEE); err != nil {
		t.Fatalf("mixed percent chain output=%q: %v", percentChainResult.Output, err)
	}
	mutatedChain := bytes.Replace(percentChainResult.Output, []byte(`% "x"+A+"y" %`), []byte(`% "x" + A + "y" %`), 1)
	if bytes.Equal(mutatedChain, percentChainResult.Output) {
		t.Fatalf("mixed percent chain mutation was not applied: %q", percentChainResult.Output)
	}
	if err := checkSourcePreservedDialect(percentChain, mutatedChain, format.EMEE); err == nil {
		t.Fatalf("accepted mixed percent chain whitespace mutation: %q", mutatedChain)
	}

	nulForm := []byte("@F\nPRINTFORM {A\x00B+1}\n")
	nulFormResult := format.FormatWithOptions(nulForm, format.ERB, format.Options{})
	if len(nulFormResult.Diagnostics) != 0 {
		t.Fatalf("valid NUL form diagnostics=%v output=%q", nulFormResult.Diagnostics, nulFormResult.Output)
	}
	if bytes.Equal(nulForm, nulFormResult.Output) || !bytes.Contains(nulFormResult.Output, []byte("A\x00B + 1")) {
		t.Fatalf("valid NUL form was not formatted: %q", nulFormResult.Output)
	}
	if err := checkSourcePreserved(nulForm, nulFormResult.Output); err != nil {
		t.Fatalf("valid NUL form output=%q: %v", nulFormResult.Output, err)
	}
}

func TestPreservationOracleVerifierUnknownMacroFields(t *testing.T) {
	for _, source := range []string{
		"@F\nPRINTFORM % A+\"x\"+UNKNOWN %\n",
		"@F\nPRINTFORM % A+@\"x\"+\"y\" %\n",
		"@F\nPRINTFORM { UNKNOWN OTHER }\n",
		"@F\nPRINTFORM % UNKNOWN OTHER %\n",
		"@F\nPRINTFORM % \"x\", UNKNOWN OTHER %\n",
		"@F\nPRINTFORM % UNKNOWN, \"x\" %\n",
		"@F\nPRINTFORM {1, UNKNOWN OTHER}\n",
		"@F\nPRINTFORM { UNKNOWN+ }\n",
	} {
		result := format.FormatWithOptions([]byte(source), format.ERB, format.Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%v", source, result.Diagnostics)
		}
		if err := checkSourcePreserved([]byte(source), result.Output); err != nil {
			t.Fatalf("source=%q output=%q: %v", source, result.Output, err)
		}
	}

	for _, tc := range []struct{ source, from, to string }{
		{"@F\nSETCOLOR (\"x\"*A)\n", `("x"*A)`, `("x" * A)`},
		{"@F\nDT_CELL_SET \"t\",(\"x\"*A),\"c\",1\n", `("x"*A)`, `("x" * A)`},
	} {
		source := []byte(tc.source)
		result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: format.EMEE})
		if len(result.Diagnostics) == 0 {
			t.Fatalf("numeric consumer accepted invalid string multiplication: source=%q output=%q", tc.source, result.Output)
		}
		if err := checkSourcePreservedDialect(source, result.Output, format.EMEE); err != nil {
			t.Fatalf("formatted invalid numeric consumer rejected: %v\noutput=%q", err, result.Output)
		}
		mutated := bytes.Replace(result.Output, []byte(tc.from), []byte(tc.to), 1)
		if bytes.Equal(mutated, result.Output) {
			t.Fatalf("numeric consumer mutation was not applied: output=%q", result.Output)
		}
		if err := checkSourcePreservedDialect(source, mutated, format.EMEE); err == nil {
			t.Fatalf("accepted numeric consumer mutation: %q", mutated)
		}
	}
}

func TestPreservationOracleVerifierContinuationKinds(t *testing.T) {
	separator := " "
	for _, tc := range []struct {
		name, source string
		dialect      format.Dialect
	}{
		{"printforms", "@F\n{\nPRINTFORMS \"x { A+\nB }\"\n}\n", format.CommonDialect},
		{"strform", "@F\n{\nSTRFORM \"x { A+\nB }\"\n}\n", format.CommonDialect},
		{"printv-form-atom", "@F\n{\nPRINTV @\"x { A+\nB }\"\n}\n", format.CommonDialect},
		{"graphics-form-atom", "@F\n{\nPRINT_SPACE @\"x { A+\nB }\"\n}\n", format.EMEE},
		{"continued-call-closing-form", "@F\n{\nCALL F(@\"\ntext\n\", ,40)\n}\n", format.EMEE},
		{"form-call-arguments", "@F\n{\nTRYCALLFORM WALL_{FLAG:0},\nA,B+C,D+E,\nF\n}\n", format.EMEE},
		{"form-call-target-split", "@F\n{\nTRYCALLFORM WALL_{A+\nB},C+D\n}\n", format.EMEE},
		{"continued-percent-form", "@F\n{\nPRINTFORM %\"x\",1　+\n2%\n}\n", format.EMEE},
		{"continued-percent-form-call", "@F\n{\nCALL HTMLPRINTW(@\"x %HTMLFONT(\"b\", ,\nCOLOR2(\"A\"))%\")\n}\n", format.CommonDialect},
		{"continued-percent-rand", "@F\n{\nPRINTFORM %RAND_SPLIT(\n\"a\"+\n\"b\") %\n}\n", format.CommonDialect},
		{"continued-conditional-call", "@F\n{\nCALL F(@\"text\\@ \n!A \t? yes # no \\@tail\")\n}\n", format.EMEE},
		{"continued-conditional-group", "@F\n{\nCALL F(@\"text\\@ (A?1#0)\n? yes # no \\@tail\")\n}\n", format.EMEE},
		{"continued-form-comment", "@F\n{\nCALL F(\"text\" +\n@\"%FONT(\"b\")%\")\n;note\n}\n", format.EMEE},
		{"continued-nested-form", "@F\n{\nCALL F(@\"%FONT(@\"text\\@ A \n?yes#no\\@\", , ,)%\")\n}\n", format.EMEE},
	} {
		t.Run(tc.name, func(t *testing.T) {
			join := separator
			if tc.name == "form-call-arguments" {
				join = ""
			}
			options := format.Options{Dialect: tc.dialect, AllowFullwidthSpace: tc.name == "continued-percent-form", ContinuationSeparator: &join}
			result := format.FormatWithOptions([]byte(tc.source), format.ERB, options)
			preserve := sourcePreservationOptions{dialect: tc.dialect, allowFullwidthSpace: tc.name == "continued-percent-form", continuationSeparator: &join}
			if err := checkSourcePreservedOptions([]byte(tc.source), result.Output, preserve); err != nil {
				t.Fatalf("output=%q: %v", result.Output, err)
			}
		})
	}

	for _, tc := range []struct {
		name, source string
		dialect      format.Dialect
		full         bool
	}{
		{"findelement-same-type", "@F\nFINDELEMENT RESULTS,(\"x\"*A)\n", format.CommonDialect, false},
		{"groupmatch-same-type", "@F\nGROUPMATCH \"x\",(\"x\"*A)\n", format.CommonDialect, false},
		{"varset-fullwidth-any", "@F\nVARSET　ARGS　,　(\"x\"*A)\n", format.EMEE, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte(tc.source)
			result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: tc.dialect, AllowFullwidthSpace: tc.full})
			if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{dialect: tc.dialect, allowFullwidthSpace: tc.full}); err != nil {
				t.Fatalf("same-type/Any output rejected: diagnostics=%v output=%q: %v", result.Diagnostics, result.Output, err)
			}
			if !bytes.Contains(result.Output, []byte(` * A`)) {
				t.Fatalf("string multiplication was not formatted: output=%q diagnostics=%v", result.Output, result.Diagnostics)
			}
		})
	}
}

func TestPreservationOracleGenericExpressionProofs(t *testing.T) {
	for _, tc := range []struct {
		name, source, from, to string
		dialect                format.Dialect
	}{
		{"return malformed", "@F\nRETURN A+\n", "A+", "A +", format.CommonDialect},
		{"call malformed", "@F\nCALL F(A+)\n", "A+", "A +", format.CommonDialect},
		{"printv malformed", "@F\nPRINTV A+\n", "A+", "A +", format.CommonDialect},
		{"printvn malformed", "@F\nPRINTVN A+\n", "A+", "A +", format.EMEE},
		{"setcolor malformed", "@F\nSETCOLOR A+\n", "A+", "A +", format.EMEE},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte(tc.source)
			result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: tc.dialect})
			if len(result.Diagnostics) == 0 {
				t.Fatalf("malformed source produced no diagnostic: output=%q", result.Output)
			}
			options := sourcePreservationOptions{dialect: tc.dialect}
			if err := checkSourcePreservedOptions(source, result.Output, options); err != nil {
				t.Fatalf("formatted malformed source rejected: %v\noutput=%q", err, result.Output)
			}
			mutated := bytes.Replace(result.Output, []byte(tc.from), []byte(tc.to), 1)
			if bytes.Equal(mutated, result.Output) {
				t.Fatalf("mutation was not applied: output=%q", result.Output)
			}
			if err := checkSourcePreservedOptions(source, mutated, options); err == nil {
				t.Fatalf("accepted malformed expression mutation: %q", mutated)
			}
		})
	}

	for _, source := range []string{
		"@F\nPRINTV ARGS:LOCAL ;note\n",
		"@F\nPRINTV (ARGS:LOCAL)\n",
		"@F\nPRINTV RESULTS:0\n",
		"@F\nPRINTV UNKNOWN+\"x\"\n",
	} {
		result := format.FormatWithOptions([]byte(source), format.ERB, format.Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("deferred PRINTV source=%q diagnostics=%v output=%q", source, result.Diagnostics, result.Output)
		}
		if err := checkSourcePreserved([]byte(source), result.Output); err != nil {
			t.Fatalf("deferred PRINTV source=%q output=%q: %v", source, result.Output, err)
		}
	}
}

func TestPreservationOracleStringIndexQuoteBoundaries(t *testing.T) {
	for _, tc := range []struct{ source, from, to string }{
		{"@F\nPRINTFORM % ARGS:\"bad key\" %\n", `ARGS:"bad key"`, `ARGS : "bad key"`},
		{"@F\nPRINTFORM % ARGS:(\"bad key\") %\n", `ARGS:("bad key")`, `ARGS : ( "bad key" )`},
		{"@F\nPRINTFORM % \"x\",ARGS:\"bad key\" %\n", `ARGS:"bad key"`, `ARGS : "bad key"`},
	} {
		source := []byte(tc.source)
		result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: format.EMEE})
		if len(result.Diagnostics) == 0 {
			t.Fatalf("invalid index source produced no diagnostic: source=%q output=%q", tc.source, result.Output)
		}
		if err := checkSourcePreservedDialect(source, result.Output, format.EMEE); err != nil {
			t.Fatalf("formatted invalid index rejected: source=%q output=%q: %v", tc.source, result.Output, err)
		}
		mutated := bytes.Replace(result.Output, []byte(tc.from), []byte(tc.to), 1)
		if bytes.Equal(mutated, result.Output) {
			t.Fatalf("index mutation was not applied: output=%q", result.Output)
		}
		if err := checkSourcePreservedDialect(source, mutated, format.EMEE); err == nil {
			t.Fatalf("accepted invalid index mutation: %q", mutated)
		}
	}

	for _, source := range []string{
		"@F\nPRINTFORM % ARGS:LOCAL %\n",
		"@F\nPRINTFORM % ARGS:(LOCAL) %\n",
		"@F\nPRINTFORM % ARGS:(TOSTR(0)) %\n",
	} {
		result := format.FormatWithOptions([]byte(source), format.ERB, format.Options{Dialect: format.EMEE})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("valid index source=%q diagnostics=%v output=%q", source, result.Diagnostics, result.Output)
		}
		if err := checkSourcePreservedDialect([]byte(source), result.Output, format.EMEE); err != nil {
			t.Fatalf("valid index source=%q output=%q: %v", source, result.Output, err)
		}
	}
}

func TestPreservationOracleProtectedMixedOperators(t *testing.T) {
	for _, tc := range []struct{ source, from, to string }{
		{"@F\nPRINTFORM % A-\"x\" %\n", `% A-"x" %`, `% A - "x" %`},
		{"@F\nPRINTFORM % A/\"x\" %\n", `% A/"x" %`, `% A / "x" %`},
		{"@F\nPRINTFORM % A|\"x\" %\n", `% A|"x" %`, `% A | "x" %`},
		{"@F\nPRINTFORM % A&\"x\" %\n", `% A&"x" %`, `% A & "x" %`},
		{"@F\nPRINTFORM % A^\"x\" %\n", `% A^"x" %`, `% A ^ "x" %`},
		{"@F\nPRINTFORM % A<<\"x\" %\n", `% A<<"x" %`, `% A << "x" %`},
		{"@F\nPRINTFORM % A>>\"x\" %\n", `% A>>"x" %`, `% A >> "x" %`},
	} {
		source := []byte(tc.source)
		result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: format.EMEE})
		if len(result.Diagnostics) == 0 {
			t.Fatalf("invalid mixed source produced no diagnostic: source=%q output=%q", tc.source, result.Output)
		}
		if err := checkSourcePreservedDialect(source, result.Output, format.EMEE); err != nil {
			t.Fatalf("formatted invalid mixed source rejected: %v\noutput=%q", err, result.Output)
		}
		mutated := bytes.Replace(result.Output, []byte(tc.from), []byte(tc.to), 1)
		if bytes.Equal(mutated, result.Output) {
			t.Fatalf("mixed mutation was not applied: output=%q", result.Output)
		}
		if err := checkSourcePreservedDialect(source, mutated, format.EMEE); err == nil {
			t.Fatalf("accepted invalid mixed mutation: %q", mutated)
		}
	}

	for _, source := range []string{
		"@F\nPRINTFORM % A*\"x\" %\n",
		"@F\nPRINTFORM % A*RESULTS %\n",
	} {
		result := format.FormatWithOptions([]byte(source), format.ERB, format.Options{Dialect: format.EMEE})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("valid mixed multiplication source=%q diagnostics=%v output=%q", source, result.Diagnostics, result.Output)
		}
		if err := checkSourcePreservedDialect([]byte(source), result.Output, format.EMEE); err != nil {
			t.Fatalf("valid mixed multiplication source=%q output=%q: %v", source, result.Output, err)
		}
	}

	for _, tc := range []struct{ source, changed string }{
		{"@F\nPRINT_IMG \"x\", (\"x\"+A)\n", "@F\nPRINT_IMG \"x\", (\"x\" + A)\n"},
		{"@F\nPRINT_IMG \"x\", (\"x\"*RESULTS)\n", "@F\nPRINT_IMG \"x\", (\"x\" * RESULTS)\n"},
		{"@F\nPRINT_IMG \"x\", (UNKNOWN*\"x\")\n", "@F\nPRINT_IMG \"x\", (UNKNOWN * \"x\")\n"},
	} {
		source := []byte(tc.source)
		if err := checkSourcePreservedDialect(source, []byte(tc.changed), format.EMEE); err == nil {
			t.Fatalf("accepted protected graphics mutation: %q", tc.changed)
		}
	}
}

func TestPreservationOracleDeferredRepetitionAndGroupedTypes(t *testing.T) {
	for _, source := range []string{
		"@F\nPRINTV (\"x\"*A)\n",
		"@F\nCALL F((\"x\"*A))\n",
		"@F\nPRINTFORM {1,UNKNOWN+\"x\"}\n",
		"@F\nPRINTFORM % A*ARGS:LOCAL %\n",
	} {
		result := format.FormatWithOptions([]byte(source), format.ERB, format.Options{Dialect: format.EMEE})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("deferred source=%q diagnostics=%v output=%q", source, result.Diagnostics, result.Output)
		}
		if err := checkSourcePreservedDialect([]byte(source), result.Output, format.EMEE); err != nil {
			t.Fatalf("deferred source=%q output=%q: %v", source, result.Output, err)
		}
	}

	for _, tc := range []struct{ source, from, to string }{
		{"@F\nPRINTV (A==\"x\")\n", `(A=="x")`, `(A == "x")`},
		{"@F\nRETURN (A!=\"x\")\n", `(A!="x")`, `(A != "x")`},
		{"@F\nSETCOLOR (A<=\"x\")\n", `(A<="x")`, `(A <= "x")`},
		{"@F\nPRINTFORM {(A+ARGS:LOCAL)}\n", `(A+ARGS:LOCAL)`, `(A + ARGS:LOCAL)`},
	} {
		source := []byte(tc.source)
		result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: format.EMEE})
		if len(result.Diagnostics) == 0 {
			t.Fatalf("invalid grouped source produced no diagnostic: source=%q output=%q", tc.source, result.Output)
		}
		if err := checkSourcePreservedDialect(source, result.Output, format.EMEE); err != nil {
			t.Fatalf("formatted invalid grouped source rejected: %v\noutput=%q", err, result.Output)
		}
		mutated := bytes.Replace(result.Output, []byte(tc.from), []byte(tc.to), 1)
		if bytes.Equal(mutated, result.Output) {
			t.Fatalf("grouped mutation was not applied: output=%q", result.Output)
		}
		if err := checkSourcePreservedDialect(source, mutated, format.EMEE); err == nil {
			t.Fatalf("accepted invalid grouped mutation: %q", mutated)
		}
	}
}

func TestPreservationOracleStringConsumers(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		dialect      format.Dialect
	}{
		{"setfont", "@F\nSETFONT \"x\"*A\n", format.CommonDialect},
		{"prints", "@F\nPRINTS \"x\"*A\n", format.CommonDialect},
		{"printforms", "@F\nPRINTFORMS \"x\"*A\n", format.CommonDialect},
		{"returnf", "@F\n#FUNCTIONS\nRETURNF \"x\"*A\n", format.CommonDialect},
		{"savetext", "@F\nSAVETEXT \"x\",(\"x\"*A)\n", format.EMEE},
		{"dt-cell", "@F\nDT_CELL_SET \"t\",1,\"c\",(\"x\"*A)\n", format.EMEE},
		{"toupper", "@F\nTOUPPER \"x\"*A\n", format.CommonDialect},
		{"tolower", "@F\nTOLOWER \"x\"*A\n", format.CommonDialect},
		{"tohalf", "@F\nTOHALF \"x\"*A\n", format.CommonDialect},
		{"substring", "@F\nSUBSTRING (\"x\"*A),1,2\n", format.CommonDialect},
		{"replace", "@F\nREPLACE \"a\",\"b\",(\"x\"*A),0\n", format.EMEE},
		{"loadtext", "@F\nLOADTEXT (\"x\"*A)\n", format.EMEE},
		{"printbutton", "@F\nPRINTBUTTON (\"x\"*A),1\n", format.CommonDialect},
		{"printbutton-reply", "@F\nPRINTBUTTON \"label\",(\"x\"*A)\n", format.CommonDialect},
		{"setvar-any", "@F\nSETVAR (\"x\"*A),(\"y\"*B)\n", format.EMEE},
		{"varsetex-any", "@F\nVARSETEX (\"x\"*A),(\"y\"*B),1,2\n", format.EMEE},
		{"dt-column-any", "@F\nDT_COLUMN_ADD (\"t\"*A),(\"c\"*B),(\"v\"*C),1\n", format.EMEE},
		{"dt-cell-string-fields", "@F\nDT_CELL_SET (\"t\"*A),1,(\"c\"*B),1\n", format.EMEE},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte(tc.source)
			result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: tc.dialect})
			if err := checkSourcePreservedDialect(source, result.Output, tc.dialect); err != nil {
				t.Fatalf("string consumer output rejected: diagnostics=%v output=%q: %v", result.Diagnostics, result.Output, err)
			}
			if !bytes.Contains(result.Output, []byte(` * A`)) {
				t.Fatalf("string multiplication was not formatted: output=%q diagnostics=%v", result.Output, result.Diagnostics)
			}
		})
	}
}

func TestPreservationOracleStringRepetitionConcat(t *testing.T) {
	separator := " "
	source := []byte("@F\nPRINTFORMDL %ARGS*3 + ARGS:1%\n")
	options := format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true, ContinuationSeparator: &separator}
	result := format.FormatWithOptions(source, format.ERB, options)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("string concatenation diagnostics=%v output=%q", result.Diagnostics, result.Output)
	}
	preservation := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true, continuationSeparator: &separator}
	if err := checkSourcePreservedOptions(source, result.Output, preservation); err != nil {
		t.Fatalf("formatted string concatenation rejected: %v output=%q", err, result.Output)
	}
	mutated := bytes.Replace(result.Output, []byte("ARGS:1"), []byte("ARGS:2"), 1)
	if bytes.Equal(mutated, result.Output) {
		t.Fatal("string index mutation was not applied")
	}
	if err := checkSourcePreservedOptions(source, mutated, preservation); err == nil {
		t.Fatalf("accepted string index mutation: %q", mutated)
	}
}

func TestPreservationOraclePercentQuotedStringIndexes(t *testing.T) {
	for _, name := range []string{"CSTR", "TRAINNAME", "STR", "TSTR", "SAVESTR", "GLOBALS", "CDFLAGNAME1", "CDFLAGNAME2"} {
		source := []byte("@F\nPRINTFORM % " + name + `:"key"` + " %\n")
		result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: format.EMEE})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("%s diagnostics=%v output=%q", name, result.Diagnostics, result.Output)
		}
		if err := checkSourcePreservedDialect(source, result.Output, format.EMEE); err != nil {
			t.Fatalf("%s output rejected: %v", name, err)
		}
	}
	invalid := []byte("@F\nPRINTFORM % CSTR:\"key\":0 %\n")
	result := format.FormatWithOptions(invalid, format.ERB, format.Options{Dialect: format.EMEE})
	mutated := bytes.Replace(result.Output, []byte(`CSTR:"key":0`), []byte(`CSTR: "key": 0`), 1)
	if bytes.Equal(mutated, result.Output) {
		t.Fatal("invalid quoted index mutation was not applied")
	}
	if err := checkSourcePreservedDialect(invalid, mutated, format.EMEE); err == nil {
		t.Fatalf("accepted invalid quoted index mutation: %q", mutated)
	}
}

func TestPreservationOracleFormCallPercentName(t *testing.T) {
	source := []byte("@F\nTRYCALLLIST\n\tFUNC %\"LUNA_KOJO_EVENT_\"+ARGS:0%,ARG:0,ARG:1,ARG:2,ARG:3\nENDFUNC\n")
	result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: format.EMEE})
	if len(result.Diagnostics) != 0 || bytes.Equal(source, result.Output) {
		t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreservedDialect(source, result.Output, format.EMEE); err != nil {
		t.Fatalf("trusted form-call output rejected: %v", err)
	}
	mutated := bytes.Replace(result.Output, []byte("ARGS:0"), []byte("ARGS:1"), 1)
	if bytes.Equal(mutated, result.Output) {
		t.Fatal("form-call argument mutation was not applied")
	}
	if err := checkSourcePreservedDialect(source, mutated, format.EMEE); err == nil {
		t.Fatalf("accepted form-call argument mutation: %q", mutated)
	}
}
