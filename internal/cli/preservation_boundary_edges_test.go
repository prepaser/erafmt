package cli

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestPreservationOracleEscapedCallBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, source, changed string
		wantErr               bool
	}{
		{"terminal-space-loss", "@F\nTRYCALL F\\ \n", "@F\nTRYCALL F\\\n", true},
		{"terminal-tab-loss", "@F\nTRYCALL F\\\t\n", "@F\nTRYCALL F\\\n", true},
		{"escaped-space-kept-before-padding", "@F\nTRYCALL F\\ \t\n", "@F\nTRYCALL F\\ \n", false},
		{"escaped-comma", "@F\nTRYCALL F\\,1\n", "@F\nTRYCALL F\\, 1\n", true},
		{"escaped-semicolon", "@F\nTRYCALL F\\;tail\n", "@F\nTRYCALL F\\; tail\n", true},
		{"plain-form-like-target", "@F\nTRYCALL F@\"(1+2)\n", "@F\nTRYCALL F@\"(1 + 2)\n", false},
		{"form-terminal-space-loss", "@F\nCALLFORM F\\ \n", "@F\nCALLFORM F\\\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkSourcePreserved([]byte(tc.source), []byte(tc.changed))
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v got=%v: %v", tc.wantErr, err != nil, err)
			}
		})
	}

	for _, dialect := range []format.Dialect{format.CommonDialect, format.EMEE} {
		options := sourcePreservationOptions{dialect: dialect}
		source := []byte("@F\nTRYCALL F\\ \t\n")
		output := []byte("@F\nTRYCALL F\\ \n")
		if err := checkSourcePreservedOptions(source, output, options); err != nil {
			t.Fatalf("dialect=%v preserved escape rejected: %v", dialect, err)
		}
	}
	for _, tc := range []struct {
		name, source, changed string
	}{
		{"plain-space", "@F\nTRYCALL F\\ \n", "@F\nTRYCALL F\\\n"},
		{"plain-tab", "@F\nTRYCALL F\\\t\n", "@F\nTRYCALL F\\\n"},
		{"form-space", "@F\nCALLFORM F\\ \n", "@F\nCALLFORM F\\\n"},
	} {
		for _, dialect := range []format.Dialect{format.CommonDialect, format.EMEE} {
			for _, allowFullwidth := range []bool{false, true} {
				options := sourcePreservationOptions{dialect: dialect, allowFullwidthSpace: allowFullwidth}
				if err := checkSourcePreservedOptions([]byte(tc.source), []byte(tc.changed), options); err == nil {
					t.Fatalf("accepted escaped target loss case=%s dialect=%v fullwidth=%v", tc.name, dialect, allowFullwidth)
				}
			}
		}
	}
}

func TestPreservationOracleSAVEVARTerminalComma(t *testing.T) {
	valid := []struct {
		name, source, output string
		options              sourcePreservationOptions
	}{
		{"common", "@F\nSAVEVAR \"x\",\"y\",RESULT,\n", "@F\nSAVEVAR \"x\", \"y\", RESULT,\n", sourcePreservationOptions{}},
		{"fullwidth", "@F\nSAVEVAR \"x\",\"y\",RESULT,\n", "@F\nSAVEVAR \"x\", \"y\", RESULT,\n", sourcePreservationOptions{allowFullwidthSpace: true}},
		{"comment", "@F\nSAVEVAR \"x\",\"y\",RESULT,;note\n", "@F\nSAVEVAR \"x\", \"y\", RESULT, ; note\n", sourcePreservationOptions{}},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkSourcePreservedOptions([]byte(tc.source), []byte(tc.output), tc.options); err != nil {
				t.Fatalf("valid terminal comma rejected: %v", err)
			}
		})
	}
	for _, changed := range []string{
		"@F\nSAVEVAR \"x\", \"y\", RESULT,,\n",
		"@F\nSAVEVAR \"x\", \"y\", , RESULT\n",
	} {
		if err := checkSourcePreserved([]byte("@F\nSAVEVAR \"x\",\"y\",RESULT,\n"), []byte(changed)); err == nil {
			t.Fatalf("accepted malformed SAVEVAR mutation: %q", changed)
		}
	}
}

func TestPreservationOracleCallCommentAndPrintVSeparators(t *testing.T) {
	for _, tc := range []struct {
		name, source, output string
		dialect              format.Dialect
	}{
		{"plain-call-comment", "@F\nTRYCALL F,;note\n", "@F\nTRYCALL F, ; note\n", format.CommonDialect},
		{"form-call-comment", "@F\nCALLFORM F,;note\n", "@F\nCALLFORM F, ; note\n", format.CommonDialect},
		{"modern-call-comment", "@F\nCALLFORM F,;note\n", "@F\nCALLFORM F, ; note\n", format.EMEE},
		{"attached-printv", "@F\nPRINTV; \n", "@F\nPRINTV;\n", format.CommonDialect},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkSourcePreservedDialect([]byte(tc.source), []byte(tc.output), tc.dialect); err != nil {
				t.Fatalf("safe separator formatting rejected: %v", err)
			}
		})
	}
	for _, tc := range []struct {
		name, source, output string
	}{
		{"mandatory-call-marker", "@F\nTRYCALL F,;!;note\n", "@F\nTRYCALL F, ;!; note\n"},
		{"protected-call-marker", "@F\nTRYCALL F,;#;note\n", "@F\nTRYCALL F, ;#; note\n"},
		{"printv-raw", "@F\nPRINTV;'raw  text\n", "@F\nPRINTV;'raw text\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkSourcePreserved([]byte(tc.source), []byte(tc.output)); err == nil {
				t.Fatal("accepted protected payload mutation")
			}
		})
	}
}

func TestPreservationOracleFullwidthTerminalNumericFields(t *testing.T) {
	options := sourcePreservationOptions{dialect: format.CommonDialect, allowFullwidthSpace: true}
	for _, tc := range []struct {
		name, source, output string
	}{
		{"arrayshift", "@F\nARRAYSHIFT RESULT,1,2,\n", "@F\nARRAYSHIFT RESULT, 1, 2,\n"},
		{"tinput", "@F\nTINPUT 15,-1,0,\n", "@F\nTINPUT 15, -1, 0,\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkSourcePreservedOptions([]byte(tc.source), []byte(tc.output), options); err != nil {
				t.Fatalf("terminal field formatting rejected: %v", err)
			}
		})
	}
	if err := checkSourcePreservedOptions([]byte("@F\nTOOLTIP_SETDELAY 1\n"), []byte("@F\nTOOLTIP_SETDELAY 1,\n"), options); err == nil {
		t.Fatal("accepted unsupported numeric terminal comma formatting")
	}
	for _, changed := range []string{
		"@F\nARRAYSHIFT RESULT,,2\n",
		"@F\nARRAYSHIFT RESULT,  , 2\n",
	} {
		if err := checkSourcePreservedOptions([]byte("@F\nARRAYSHIFT RESULT,1,2,\n"), []byte(changed), options); err == nil {
			t.Fatalf("accepted interior numeric omission: %q", changed)
		}
	}
}

func TestPreservationOracleGraphicsNulBoundary(t *testing.T) {
	source := []byte("@F\nPRINT_RECT 0%1\x00 0\n")
	output := []byte("@F\nPRINT_RECT 0 % 1\x00 0\n")
	for _, dialect := range []format.Dialect{format.CommonDialect, format.EMEE} {
		for _, allowFullwidth := range []bool{false, true} {
			options := sourcePreservationOptions{dialect: dialect, allowFullwidthSpace: allowFullwidth}
			if err := checkSourcePreservedOptions(source, output, options); err != nil {
				t.Fatalf("dialect=%v fullwidth=%v safe NUL prefix rejected: %v", dialect, allowFullwidth, err)
			}
		}
	}
	mutated := []byte("@F\nPRINT_RECT 0 % 2\x00 0\n")
	if err := checkSourcePreservedDialect(source, mutated, format.EMEE); err == nil {
		t.Fatal("accepted graphics expression mutation before NUL")
	}
	mutatedSuffix := []byte("@F\nPRINT_RECT 0 % 1\x00 changed\n")
	if err := checkSourcePreservedDialect(source, mutatedSuffix, format.EMEE); err == nil {
		t.Fatal("accepted graphics suffix mutation after NUL")
	}
	embedded := []byte("@F\nPRINT_RECT A\x00B,1,2,3\n")
	embeddedChanged := []byte("@F\nPRINT_RECT A \x00B,1,2,3\n")
	if err := checkSourcePreservedDialect(embedded, embeddedChanged, format.EMEE); err == nil {
		t.Fatal("accepted embedded-identifier NUL mutation")
	}
}

func TestPreservationOracleContinuedCallEscape(t *testing.T) {
	separator := ""
	original := []byte("@F\n{\nTRYCALL F\\\n \n}\n")
	changed := []byte("@F\n{\nTRYCALL F\\\n\n}\n")
	options := sourcePreservationOptions{continuationSeparator: &separator}
	if err := checkSourcePreservedOptions(original, changed, options); err == nil {
		t.Fatal("accepted escaped terminal whitespace loss across an empty continuation join")
	}

	fullwidth := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true, continuationSeparator: &separator}
	if err := checkSourcePreservedOptions(original, original, fullwidth); err != nil {
		t.Fatalf("self comparison rejected continued call profile: %v", err)
	}
}

func TestPreservationOracleGraphicsNulFingerprintRetainsSuffix(t *testing.T) {
	before := graphicsNumericFieldMode([]byte("0%1\x00suffix"), true, format.EMEE)
	after := graphicsNumericFieldMode([]byte("0 % 1\x00suffix"), true, format.EMEE)
	if !before || !after {
		t.Fatalf("NUL-terminated numeric graphics fields not recognized: before=%v after=%v", before, after)
	}
	if bytes.Equal(graphicsFingerprint("PRINT_RECT", []byte(" 0%1\x00suffix,1,2,3"), format.EMEE), graphicsFingerprint("PRINT_RECT", []byte(" 0%2\x00suffix,1,2,3"), format.EMEE)) {
		t.Fatal("graphics fingerprint ignored executable prefix mutation")
	}
}

func TestPreservationOracleFullwidthRawCallProfiles(t *testing.T) {
	cases := []struct {
		name, source string
	}{
		{"escaped-space-before-comma", "@F\nTRYCALL F\\ ,1\n"},
		{"empty-argument-comment", "@F\nTRYCALL F,;note\n"},
	}
	for _, tc := range cases {
		for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
			for _, allowFullwidth := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/dialect=%d/fullwidth=%v", tc.name, dialect, allowFullwidth), func(t *testing.T) {
					options := format.Options{Dialect: dialect, AllowFullwidthSpace: allowFullwidth}
					result := format.FormatWithOptions([]byte(tc.source), format.ERB, options)
					if len(result.Diagnostics) != 0 {
						t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
					}
					preservation := sourcePreservationOptions{dialect: dialect, allowFullwidthSpace: allowFullwidth}
					if err := checkSourcePreservedOptions([]byte(tc.source), result.Output, preservation); err != nil {
						t.Fatalf("output=%q: %v", result.Output, err)
					}
				})
			}
		}
	}
}

func TestPreservationOracleFullwidthCaseTerminalComma(t *testing.T) {
	for _, source := range []string{
		"@F\nSELECTCASE RESULT\nCASE 1, ;note\nRETURN 1\nENDSELECT\n",
		"@F\nSELECTCASE LOCALS\nCASE \"text\", \nRETURN 1\nENDSELECT\n",
	} {
		input := []byte(source)
		result := format.FormatWithOptions(input, format.ERB, format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%v output=%q", source, result.Diagnostics, result.Output)
		}
		if err := checkSourcePreservedOptions(input, result.Output, sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}); err != nil {
			t.Fatalf("source=%q output=%q: %v", source, result.Output, err)
		}
	}
}

func TestPreservationOracleFullwidthMalformedCallArguments(t *testing.T) {
	source := []byte("@F\nTRYCALL F,1+\u3000\n")
	changed := []byte("@F\nTRYCALL F,1+ \n")
	options := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}
	if err := checkSourcePreservedOptions(source, changed, options); err == nil {
		t.Fatal("accepted malformed call argument mutation")
	}
}
