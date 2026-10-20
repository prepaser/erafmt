package cli

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestPreservationOracleProtectsInlineCommentEdges(t *testing.T) {
	cases := []string{
		"RETURN 1 ;#; payload  ",
		"PRINTV 1 ;#; +2  ",
		"RETURN 1 ;^; payload  ",
		"RETURN 1 ;--- payload  ",
		"RETURN 1 ;#; ;#; payload  ",
		"RETURN 1 ;!; +2 ;#; payload  ",
		"RETURN 1 ;^; ;#; payload  ",
	}
	for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
		for _, line := range cases {
			source := []byte("@F\n" + line + "\n")
			options := sourcePreservationOptions{dialect: dialect}
			result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: dialect})
			if len(result.Diagnostics) != 0 {
				t.Fatalf("dialect=%v source=%q diagnostics=%v", dialect, line, result.Diagnostics)
			}
			if err := checkSourcePreservedOptions(source, result.Output, options); err != nil {
				t.Fatalf("dialect=%v source=%q formatted output rejected: %v\noutput=%q", dialect, line, err, result.Output)
			}
			mutated := bytes.Replace(result.Output, []byte("  \n"), []byte("\n"), 1)
			if bytes.Equal(mutated, result.Output) {
				t.Fatalf("dialect=%v source=%q protected tail was not present: %q", dialect, line, result.Output)
			}
			if checkSourcePreservedOptions(source, mutated, options) == nil {
				t.Fatalf("dialect=%v source=%q accepted protected tail mutation: %q", dialect, line, mutated)
			}
		}
	}
	ordinary := []byte("@F\nRETURN 1 ; payload  \n")
	if err := checkSourcePreserved(ordinary, []byte("@F\nRETURN 1 ; payload\n")); err != nil {
		t.Fatalf("ordinary comment normalization rejected: %v", err)
	}
}

func TestPreservationOracleFullwidthGrammarCases(t *testing.T) {
	cases := []struct {
		name, source string
		dialect      format.Dialect
	}{
		{"omitted-if-field", "@F\nIF RESULT,,10　\nENDIF\n", format.CommonDialect},
		{"omitted-returnf-field", "@F\nRETURNF 1,,2　\n", format.CommonDialect},
		{"method-omitted-field", "@F\nRAND ,1　\n", format.CommonDialect},
		{"printforms-literal-suffix", "@F\nPRINTFORMS \"a + b\"　\n", format.CommonDialect},
		{"savevar-deferred-form", "@F\nSAVEVAR @\"a {1+2}\"　\n", format.CommonDialect},
		{"graphics-unknown-field-comment", "@F\nPRINT_IMG \"x\",RESULTS,RESULT　;note\n", format.EMEE},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte(tc.source)
			result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: tc.dialect, AllowFullwidthSpace: true})
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == format.Error {
					t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
				}
			}
			if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{dialect: tc.dialect, allowFullwidthSpace: true}); err != nil {
				t.Fatalf("preservation: %v\noutput=%q", err, result.Output)
			}
		})
	}
}

func TestPreservationOracleKeepsFullwidthRawPrintVPayload(t *testing.T) {
	for _, source := range []string{
		"@F\nPRINTV 'raw　text\n",
		"@F\nPRINTV 'raw　\n",
	} {
		original := []byte(source)
		result := format.FormatWithOptions(original, format.ERB, format.Options{AllowFullwidthSpace: true})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%v", source, result.Diagnostics)
		}
		options := sourcePreservationOptions{allowFullwidthSpace: true}
		if err := checkSourcePreservedOptions(original, result.Output, options); err != nil {
			t.Fatalf("source=%q output rejected: %v\noutput=%q", source, err, result.Output)
		}
		mutated := bytes.Replace(result.Output, []byte("　"), []byte(" "), 1)
		if bytes.Equal(mutated, result.Output) {
			t.Fatalf("source=%q payload was absent: %q", source, result.Output)
		}
		if checkSourcePreservedOptions(original, mutated, options) == nil {
			t.Fatalf("source=%q accepted raw payload mutation: %q", source, mutated)
		}
	}
}

func TestPreservationOracleKeepsUnsupportedPrintVFullwidthInterior(t *testing.T) {
	original := []byte("@F\nPRINTV A　B\n")
	result := format.FormatWithOptions(original, format.ERB, format.Options{AllowFullwidthSpace: true})
	options := sourcePreservationOptions{allowFullwidthSpace: true}
	if err := checkSourcePreservedOptions(original, result.Output, options); err != nil {
		t.Fatalf("formatted output rejected: %v\noutput=%q diagnostics=%v", err, result.Output, result.Diagnostics)
	}
	mutated := bytes.Replace(result.Output, []byte("　"), []byte(" "), 1)
	if bytes.Equal(mutated, result.Output) {
		t.Fatalf("fullwidth interior was absent: %q", result.Output)
	}
	if checkSourcePreservedOptions(original, mutated, options) == nil {
		t.Fatalf("accepted unsupported PRINTV interior mutation: %q", mutated)
	}
}

func TestPreservationOracleContinuationTriviaBoundaries(t *testing.T) {
	separator := ""
	source := []byte("@F\n{\nDO　;comment　\n}\nLOOP 1\n")
	result := format.FormatWithOptions(source, format.ERB, format.Options{AllowFullwidthSpace: true, ContinuationSeparator: &separator})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{allowFullwidthSpace: true, continuationSeparator: &separator}); err != nil {
		t.Fatalf("continuation trivia rejected: %v\noutput=%q", err, result.Output)
	}
}

func TestPreservationOracleGraphicsCommentBoundaryWithoutFullwidthMode(t *testing.T) {
	cases := []struct {
		name, source string
		dialect      format.Dialect
	}{
		{"common-expression", "@F\nPRINT_SPACE  %RESULTS%　;note  \n", format.CommonDialect},
		{"modern-form", "@F\nPRINT_SPACE  @\"a {1+2}\"　;note  \n", format.EMEE},
		{"modern-image", "@F\nPRINT_IMG  \"x\",RESULTS,RESULT　;note  \n", format.EMEE},
		{"modern-image-unknown", "@F\nPRINT_IMG  1,　;note  \n", format.EMEE},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte(tc.source)
			result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: tc.dialect})
			if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{dialect: tc.dialect}); err != nil {
				t.Fatalf("preservation: %v\noutput=%q diagnostics=%v", err, result.Output, result.Diagnostics)
			}
		})
	}
}

func TestPreservationOracleKeepsKnownInvalidGraphicsBodiesExact(t *testing.T) {
	cases := []struct {
		name, source, formatted string
		dialect                 format.Dialect
	}{
		{"wrong-space-arity", "@F\nPRINT_SPACE 1,2 ;note\n", "@F\nPRINT_SPACE 1,2 ; note\n", format.CommonDialect},
		{"known-space-string", "@F\nPRINT_SPACE \"abc\" ;note\n", "@F\nPRINT_SPACE \"abc\" ; note\n", format.CommonDialect},
		{"known-space-builtin-string", "@F\nPRINT_SPACE RESULTS ;note\n", "@F\nPRINT_SPACE RESULTS ; note\n", format.CommonDialect},
		{"known-rect-string", "@F\nPRINT_RECT \"abc\",1,2,3 ;note\n", "@F\nPRINT_RECT \"abc\",1,2,3 ; note\n", format.CommonDialect},
		{"wrong-image-count", "@F\nPRINT_IMG \"x\",1,2,3,4,5,6 ;note\n", "@F\nPRINT_IMG \"x\",1,2,3,4,5,6 ; note\n", format.EMEE},
		{"wrong-image-role", "@F\nPRINT_IMG 1 ;note\n", "@F\nPRINT_IMG 1 ; note\n", format.CommonDialect},
		{"opaque-space-command-gap", "@F\nPRINT_SPACE　1 ;note\n", "@F\nPRINT_SPACE　1 ; note\n", format.CommonDialect},
		{"opaque-image-command-gap", "@F\nPRINT_IMG　\"x\",RESULTS,RESULT ;note\n", "@F\nPRINT_IMG　\"x\",RESULTS,RESULT ; note\n", format.EMEE},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			options := sourcePreservationOptions{dialect: tc.dialect}
			if err := checkSourcePreservedOptions([]byte(tc.source), []byte(tc.formatted), options); err == nil {
				t.Fatal("accepted invalid graphics comment-only mutation")
			}
		})
	}
}

func TestPreservationOracleKeepsUnsupportedExpressionPayloadExact(t *testing.T) {
	tests := []struct {
		name, source, changed string
		acceptChanged         bool
	}{
		{"form-call-trailing-operator", "@F\nCALLFORM F(A+)\n", "@F\nCALLFORM F(A +)\n", false},
		{"form-call-comma-trailing-operator", "@F\nFUNC TARGET, A+\n", "@F\nFUNC TARGET, A +\n", false},
		{"form-call-adjacent-strings", "@F\nCALLFORM F(\"x\" \"y\")\n", "@F\nCALLFORM F(\"x\"  \"y\")\n", false},
		{"form-call-mixed-unknown-string", "@F\nCALLFORM F(A+\"x\")\n", "@F\nCALLFORM F(A + \"x\")\n", false},
		{"form-call-unknown-macro-words", "@F\nCALLFORM F(UNKNOWN OTHER)\n", "@F\nCALLFORM F(UNKNOWN  OTHER)\n", true},
		{"form-call-string-comparison", "@F\nCALLFORM F(\"x\"==\"y\")\n", "@F\nCALLFORM F(\"x\" == \"y\")\n", true},
		{"form-call-dual-role-rand", "@F\nCALLFORM F(RAND(2))\n", "@F\nCALLFORM F(RAND (2))\n", true},
		{"form-call-dual-role-rand-expression", "@F\nCALLFORM F(RAND(2)+1)\n", "@F\nCALLFORM F(RAND (2) + 1)\n", true},
		{"form-call-string-method", "@F\nCALLFORM F(STRLENS(\"x\"))\n", "@F\nCALLFORM F(STRLENS (\"x\"))\n", true},
		{"form-call-numeric-method-string-arg", "@F\nCALLFORM F(STRLENS(\"x\")+1)\n", "@F\nCALLFORM F(STRLENS (\"x\") + 1)\n", true},
		{"form-call-string-method-conversion", "@F\nCALLFORM F(TOSTR(RESULT+1)+\"x\")\n", "@F\nCALLFORM F(TOSTR (RESULT + 1) + \"x\")\n", true},
		{"form-call-string-repetition-concat", "@F\nCALLFORM F(\" \"*(ARG:2-STRLENS(LOCALS)-4)+LOCALS)\n", "@F\nCALLFORM F(\" \" * (ARG:2 - STRLENS(LOCALS) - 4) + LOCALS)\n", true},
		{"form-call-valid-argument", "@F\nCALLFORM F(A,B)\n", "@F\nCALLFORM F(A, B)\n", true},
		{"prefix-readonly-increment", "@F\nRETURN ++GAMEBASE_VERSION+1\n", "@F\nRETURN ++ GAMEBASE_VERSION + 1\n", false},
		{"adjacent-increments", "@F\nRETURN RESULT++--\n", "@F\nRETURN RESULT++ --\n", false},
		{"prefix-postfix-increment", "@F\nRETURN ++RESULT++\n", "@F\nRETURN ++ RESULT++\n", false},
		{"postfix-operand-adjacency", "@F\nRETURN RESULT++1\n", "@F\nRETURN RESULT++ 1\n", false},
		{"logical-prefix-postfix-increment", "@F\nRETURN !RESULT++\n", "@F\nRETURN ! RESULT++\n", false},
		{"compound-logical-prefix-postfix-increment", "@F\nRETURN RESULT + !RESULT++\n", "@F\nRETURN RESULT + ! RESULT++\n", false},
		{"ternary-logical-prefix-postfix-increment", "@F\nRETURN RESULT ? !RESULT++ # 1\n", "@F\nRETURN RESULT ? ! RESULT++ # 1\n", false},
		{"compound-readonly-increment", "@F\nRETURN ++GAMEBASE_VERSION + RESULT++\n", "@F\nRETURN ++ GAMEBASE_VERSION + RESULT ++\n", false},
		{"deferred-increment", "@F\nRETURN UNKNOWN++ + ++LOCAL\n", "@F\nRETURN UNKNOWN ++ + ++ LOCAL\n", true},
		{"unary-plus-increment", "@F\nRETURN +RESULT++\n", "@F\nRETURN + RESULT++\n", true},
		{"unary-minus-increment", "@F\nRETURN -RESULT++\n", "@F\nRETURN - RESULT++\n", true},
		{"unary-tilde-increment", "@F\nRETURN ~RESULT++\n", "@F\nRETURN ~ RESULT++\n", true},
		{"grouped-postfix-increment", "@F\nRETURN (RESULT++)+1\n", "@F\nRETURN (RESULT ++) + 1\n", true},
		{"nested-call-increment", "@F\nRETURN ABS(RESULT++)+1\n", "@F\nRETURN ABS(RESULT ++) + 1\n", true},
		{"case-to-increment", "@F\nSELECTCASE 1\nCASE RESULT++ TO 2\nENDSELECT\n", "@F\nSELECTCASE 1\nCASE RESULT ++ TO 2\nENDSELECT\n", true},
		{"valid-increment-expression", "@F\nRETURN RESULT+++1\n", "@F\nRETURN RESULT++ + 1\n", true},
		{"scoped-case-name", "@F\nRETURN LOCAL@IS\n", "@F\nRETURN LOCAL @ IS\n", true},
		{"unknown-sharp-directive", "@F\n#LOCALSIZE1+2\n", "@F\n#LOCALSIZE1 + 2\n", false},
		{"known-sharp-directive", "@F\n#LOCALSIZE 1+2\n", "@F\n#LOCALSIZE 1 + 2\n", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkSourcePreserved([]byte(tc.source), []byte(tc.changed))
			if tc.acceptChanged && err != nil {
				t.Fatalf("valid trivia mutation rejected: %v", err)
			}
			if !tc.acceptChanged && err == nil {
				t.Fatalf("invalid trivia mutation accepted: %q", tc.changed)
			}
		})
	}
}

func TestCorpusStripOuterParensDeepNesting(t *testing.T) {
	const depth = 20000
	source := make([]byte, 0, depth*2+1)
	for i := 0; i < depth; i++ {
		source = append(source, '(')
	}
	source = append(source, '1')
	for i := 0; i < depth; i++ {
		source = append(source, ')')
	}
	if got := corpusStripOuterParens(source); !bytes.Equal(got, []byte("1")) {
		t.Fatalf("deep nesting result=%q", got)
	}
}

func TestCorpusStripOuterParensControls(t *testing.T) {
	for _, tc := range []struct {
		source, want string
	}{
		{"(1)", "1"},
		{"((1))", "1"},
		{"( ( 1 ) )", "1"},
		{"((1)+2)", "(1)+2"},
		{"((1) + (2))", "(1) + (2)"},
		{"(\";\")", "\";\""},
		{"(\\))", "\\)"},
		{"(;!; 1)", "(;!; 1)"},
		{"(1))", "(1))"},
		{"((1)", "((1)"},
		{"([1])", "[1]"},
	} {
		if got := corpusStripOuterParens([]byte(tc.source)); !bytes.Equal(got, []byte(tc.want)) {
			t.Fatalf("source=%q got=%q want=%q", tc.source, got, tc.want)
		}
	}
}

func TestPreservationOracleUnknownSharpLayout(t *testing.T) {
	for _, source := range []string{"@F\n#FOO\n", "@F\n#FOO  \n"} {
		input := []byte(source)
		result := format.FormatWithOptions(input, format.ERB, format.Options{})
		if err := checkSourcePreserved(input, result.Output); err != nil {
			t.Fatalf("source=%q output=%q diagnostics=%v: %v", source, result.Output, result.Diagnostics, err)
		}
	}
}

func TestPreservationOracleFullwidthFormCallMalformedArguments(t *testing.T) {
	source := []byte("@F\nCALLFORM F(\"x\"　\"y\")\n")
	changed := []byte("@F\nCALLFORM F(\"x\"　　\"y\")\n")
	options := sourcePreservationOptions{allowFullwidthSpace: true}
	if err := checkSourcePreservedOptions(source, changed, options); err == nil {
		t.Fatalf("accepted malformed fullwidth argument mutation: %q", changed)
	}
}

func TestPreservationOracleContinuedFormCallMalformedArguments(t *testing.T) {
	separator := " "
	source := []byte("@F\n{\nFUNC TARGET,\nA+\n}\n")
	changed := []byte("@F\n{\nFUNC TARGET,\nA +\n}\n")
	options := sourcePreservationOptions{continuationSeparator: &separator}
	if err := checkSourcePreservedOptions(source, changed, options); err == nil {
		t.Fatalf("accepted malformed continued argument mutation: %q", changed)
	}
}

func TestPreservationOracleNumericRuntimeBoundaries(t *testing.T) {
	tests := []struct {
		name, source, changed string
		allowFullwidth        bool
		wantErr               bool
	}{
		{"negative-radix-exponent", "@F\nPRINTV 0x1p-2g00\n", "@F\nPRINTV 0x1p-2 g00\n", false, true},
		{"decimal-overflow", "@F\nPRINTV 9223372036854775808g00\n", "@F\nPRINTV 9223372036854775808 g00\n", false, true},
		{"radix-overflow", "@F\nPRINTV 0x10000000000000000g00\n", "@F\nPRINTV 0x10000000000000000 g00\n", false, true},
		{"bmp-digit", "@F\nPRINTV 12１g00\n", "@F\nPRINTV 12 １g00\n", false, true},
		{"radix-prefix-plus", "@F\nPRINTV 0x+1g00\n", "@F\nPRINTV 0x +1g00\n", false, true},
		{"radix-prefix-plus-expression", "@F\nRETURN 0x+1g00\n", "@F\nRETURN 0x +1g00\n", false, true},
		{"radix-prefix-plus-savevar", "@F\nSAVEVAR \"f\",\"m\",0x+1g00\n", "@F\nSAVEVAR \"f\",\"m\",0x +1g00\n", false, true},
		{"negative-decimal-exponent", "@F\nPRINTV 1e-999g00\n", "@F\nPRINTV 1e-999 g00\n", false, false},
		{"signed64-decimal-exponent", "@F\nPRINTV 9223372036854775807e-1g00\n", "@F\nPRINTV 9223372036854775807e-1 g00\n", false, false},
		{"signed64-radix-boundary", "@F\nPRINTV 0x8000000000000000g00\n", "@F\nPRINTV 0x8000000000000000 g00\n", false, false},
		{"fullwidth-suffix", "@F\nPRINTV foo \x00 0x1p3f\u3000g00\n", "@F\nPRINTV foo \x00 0x1p3f g00\n", true, true},
		{"nul-token-boundary", "@F\nPRINTV foo \x00 1+2\n", "@F\nPRINTV foo \x00 1 + 2\n", false, true},
		{"nul-embedded-identifier", "@F\nPRINTV foo\x001+2\n", "@F\nPRINTV foo\x001 + 2\n", false, false},
		{"quoted-then-invalid", "@F\nPRINTV \"has'apostrophe\",0x1p-2g00\n", "@F\nPRINTV \"has'apostrophe\",0x1p-2 g00\n", false, true},
		{"raw-then-invalid", "@F\nPRINTV 'raw,0x1p-2g00\n", "@F\nPRINTV 'raw,0x1p-2 g00\n", false, true},
		{"form-then-invalid", "@F\nPRINTV @\"x {1+2}\",0x1p-2g00\n", "@F\nPRINTV @\"x {1+2}\",0x1p-2 g00\n", false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			options := sourcePreservationOptions{allowFullwidthSpace: tc.allowFullwidth}
			err := checkSourcePreservedOptions([]byte(tc.source), []byte(tc.changed), options)
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v got=%v: %v", tc.wantErr, err != nil, err)
			}
		})
	}
}

func TestPreservationOracleUnknownSharpComments(t *testing.T) {
	ordinary := []byte("@A\n#0000;0\n")
	formatted := []byte("@A\n#0000 ; 0\n")
	if err := checkSourcePreserved(ordinary, formatted); err != nil {
		t.Fatalf("ordinary unknown-sharp comment rejected: %v", err)
	}
	for _, tail := range []string{";!;0", ";#;0", ";^;0", ";---0", "+0", "(0"} {
		source := []byte("@A\n#0000" + tail + "\n")
		changed := []byte("@A\n#0000 " + tail + "\n")
		if err := checkSourcePreserved(source, changed); err == nil {
			t.Fatalf("accepted protected unknown-sharp mutation tail=%q", tail)
		}
	}
}

func TestPreservationOracleKnownCommandFallbacks(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		dialect      format.Dialect
	}{
		{"selectcase", "SELECTCASE  0 0\nENDSELECT\n", format.EMEE},
		{"fullwidth-selectcase", "SELECTCASE　 0 0\nENDSELECT\n", format.EMEE},
		{"savevar-unknown-macro", "SAVEVAR \"\",\"\",LOCA,.(0!A)\n", format.CommonDialect},
		{"savevar-deferred-del", "SAVEVAR \"0\"00(DEL)\n", format.CommonDialect},
		{"savevar-deferred-control", "SAVEVAR \"0\"00(\x7f)\n", format.CommonDialect},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := []byte(tc.source)
			result := format.FormatWithOptions(original, format.ERB, format.Options{Dialect: tc.dialect, AllowFullwidthSpace: true})
			if err := checkSourcePreservedOptions(original, result.Output, sourcePreservationOptions{dialect: tc.dialect, allowFullwidthSpace: true}); err != nil {
				t.Fatalf("producer output rejected: %v\nsource=%q\noutput=%q\ndiagnostics=%v", err, original, result.Output, result.Diagnostics)
			}
		})
	}
	for _, tc := range []struct{ source, changed string }{
		{"@F\nSAVEVAR \"0\"00(1+2)\n", "@F\nSAVEVAR \"0\" 00(1 + 2)\n"},
		{"@F\nSAVEVAR \"0\"00(RESULT)\n", "@F\nSAVEVAR \"0\" 00(RESULT )\n"},
	} {
		if err := checkSourcePreserved([]byte(tc.source), []byte(tc.changed)); err == nil {
			t.Fatalf("accepted known-invalid SAVEVAR mutation: %q -> %q", tc.source, tc.changed)
		}
	}
	invalid := []byte("@F\nPRINTV A#.\n")
	changed := []byte("@F\nPRINTV A # .\n")
	if err := checkSourcePreserved(invalid, changed); err == nil {
		t.Fatal("accepted known-invalid PRINTV body mutation")
	}
}

func TestPreservationOracleSelectCaseNumericIdentifierGap(t *testing.T) {
	separator := ""
	options := format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true, ContinuationSeparator: &separator}
	for i, rule := range format.Rules() {
		if 0x31fffffffa2&(uint64(1)<<i) != 0 {
			options.Select = append(options.Select, rule.ID)
		}
	}
	source := []byte("SELECTCASE 0++00A0\nENDSELECT\n")
	result := format.FormatWithOptions(source, format.ERB, options)
	if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true, continuationSeparator: &separator}); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
	for _, pair := range [][2]string{
		{"SELECTCASE 0++RESULT\nENDSELECT\n", "SELECTCASE 0++ RESULT\nENDSELECT\n"},
		{"SELECTCASE 0++9223372036854775808A0\nENDSELECT\n", "SELECTCASE 0++9223372036854775808 A0\nENDSELECT\n"},
	} {
		if err := checkSourcePreserved([]byte(pair[0]), []byte(pair[1])); err == nil {
			t.Fatalf("accepted invalid increment mutation: %q -> %q", pair[0], pair[1])
		}
	}
}

func TestPreservationOracleSaveVarDeferredCallProfile(t *testing.T) {
	separator := ""
	mask := uint64(0x31fffffffff)
	options := format.Options{AllowFullwidthSpace: true, ContinuationSeparator: &separator}
	for i, rule := range format.Rules() {
		if mask&(uint64(1)<<i) != 0 {
			options.Select = append(options.Select, rule.ID)
		}
	}
	oracleOptions := sourcePreservationOptions{allowFullwidthSpace: true, continuationSeparator: &separator}
	source := []byte("SAVEVAR A00(0%)")
	result := format.FormatWithOptions(source, format.ERB, options)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreservedOptions(source, result.Output, oracleOptions); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
	for _, tc := range []struct {
		name, source, changed string
	}{
		{"invalid-number-in-unknown-call", "SAVEVAR A00(0x1p-2g00)", "SAVEVAR A00(0x1p-2 g00)"},
		{"known-invalid-operator", `SAVEVAR "0"00(1+2)`, `SAVEVAR "0" 00(1 + 2)`},
		{"known-result-call", `SAVEVAR "0"00(RESULT)`, `SAVEVAR "0" 00(RESULT)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkSourcePreservedOptions([]byte(tc.source), []byte(tc.changed), oracleOptions); err == nil {
				t.Fatalf("accepted malformed SAVEVAR mutation: %q -> %q", tc.source, tc.changed)
			}
		})
	}
}

func TestPreservationOracleNulBoundaries(t *testing.T) {
	options := sourcePreservationOptions{allowFullwidthSpace: true}
	for _, tc := range []struct {
		name, source, changed string
		wantErr               bool
	}{
		{"numeric-boundary", "@F\nPRINTV 1\x00foo　bar\n", "@F\nPRINTV 1\x00foo bar\n", true},
		{"numeric-exponent-boundary", "@F\nPRINTV 1e2\x001+2\n", "@F\nPRINTV 1e2\x001 + 2\n", true},
		{"identifier-internal", "@F\nPRINTV foo\x00bar　baz\n", "@F\nPRINTV foo\x00bar baz\n", false},
		{"token-boundary", "@F\nPRINTV foo \x00 1+2\n", "@F\nPRINTV foo \x00 1 + 2\n", true},
		{"closing-delimiter-boundary", "@F\nPRINTV (1)\x00foo　bar\n", "@F\nPRINTV (1)\x00foo bar\n", true},
		{"embedded-then-boundary", "@F\nPRINTV foo\x00bar \x00 1+2\n", "@F\nPRINTV foo\x00bar \x00 1 + 2\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkSourcePreservedOptions([]byte(tc.source), []byte(tc.changed), options)
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v got=%v", tc.wantErr, err)
			}
		})
	}
}

func TestPreservationOracleKnownExpressionBeforeNul(t *testing.T) {
	for _, tc := range []struct {
		name, source, changed string
	}{
		{"assignment", "@F\nRETURN A=1\x00foo\n", "@F\nRETURN A = 1\x00foo\n"},
		{"builtin-call-assignment", "@F\nRETURN ABS(A=1\x00foo)\n", "@F\nRETURN ABS(A = 1\x00foo)\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkSourcePreserved([]byte(tc.source), []byte(tc.changed)); err == nil {
				t.Fatalf("accepted known expression mutation before token-boundary NUL: %q", tc.changed)
			}
		})
	}
}

func TestPreservationOracleDeferredLexicalCompleteness(t *testing.T) {
	for _, tc := range []struct {
		name, source, changed string
	}{
		{"unclosed-call", "@F\nRETURN User(1+\x00foo)\n", "@F\nRETURN User(1 +\x00foo)\n"},
		{"unterminated-quote", "@F\nRETURN User+\"unterminated\n", "@F\nRETURN User + \"unterminated\n"},
		{"opening-quote-only", "@F\nRETURN User+\"\n", "@F\nRETURN User + \"\n"},
		{"escaped-final-quote", "@F\nRETURN User+\"x\\\"\n", "@F\nRETURN User + \"x\\\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkSourcePreserved([]byte(tc.source), []byte(tc.changed)); err == nil {
				t.Fatalf("accepted deferred lexical mutation: %q", tc.changed)
			}
		})
	}
}

func TestPreservationOracleFormControlIdentifierSpacing(t *testing.T) {
	source := []byte("@F\nPRINTFORM {\x17 0 }0\n")
	result := format.FormatWithOptions(source, format.ERB, format.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreserved(source, result.Output); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
}

func TestPreservationOracleFormControlIdentifierFuzzProfile(t *testing.T) {
	separator := ""
	source := []byte("PRINTFORM {\x17 0 }0")
	options := format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true, ContinuationSeparator: &separator}
	mask := uint64(21028159881215)
	for i, rule := range format.Rules() {
		if mask&(uint64(1)<<i) != 0 {
			options.Select = append(options.Select, rule.ID)
		}
	}
	result := format.FormatWithOptions(source, format.ERB, options)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true, continuationSeparator: &separator}); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
}

func TestPreservationOracleFormNumericControlProfile(t *testing.T) {
	separator := ""
	source := []byte("PRINTFORM {0A\x10}0")
	options := format.Options{Dialect: format.Emuera1824, AllowFullwidthSpace: true, ContinuationSeparator: &separator}
	mask := uint64(0xb1fffffffff)
	for i, rule := range format.Rules() {
		if mask&(uint64(1)<<i) != 0 {
			options.Select = append(options.Select, rule.ID)
		}
	}
	result := format.FormatWithOptions(source, format.ERB, options)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{dialect: format.Emuera1824, allowFullwidthSpace: true, continuationSeparator: &separator}); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
}
