package cli

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestPrintVNumericIdentifierBoundaryPreservation(t *testing.T) {
	valid := []string{"000a00", "000αβ", "000€", "000😀", "0\x60", "0b10a00", "0x1g00", "12e3a00", "12e+3a00", "12p3a00", "0x1p2g00"}
	for _, body := range valid {
		if !printVHasNumericIdentifierBoundary([]byte(" " + body)) {
			t.Errorf("numeric identifier boundary not recognized: %s", body)
		}
		for _, allowFullwidth := range []bool{false, true} {
			original := []byte("@F\nPRINTV " + body + "\n")
			result := format.FormatWithOptions(original, format.ERB, format.Options{AllowFullwidthSpace: allowFullwidth})
			if err := checkSourcePreservedOptions(original, result.Output, sourcePreservationOptions{allowFullwidthSpace: allowFullwidth}); err != nil {
				t.Errorf("%s fullwidth=%v: formatter output rejected: %v", body, allowFullwidth, err)
			}
		}
	}

	for _, body := range []string{"12efoo", "0b102a", "0x1pga", "0b10e2a00", "foo000a00", "000 456", "　000a00"} {
		if printVHasNumericIdentifierBoundary([]byte(" " + body)) {
			t.Errorf("invalid numeric boundary accepted: %s", body)
		}
	}

	for _, tc := range []struct{ body, formatted string }{
		{"000a00 b00", "000 a00 b00"},
		{"0 0a00", "0 0 a00"},
		{"000a00\x00", "000 a00\x00"},
	} {
		for _, allowFullwidth := range []bool{false, true} {
			original := []byte("@F\nPRINTV " + tc.body + "\n")
			result := format.FormatWithOptions(original, format.ERB, format.Options{AllowFullwidthSpace: allowFullwidth})
			if !bytes.Contains(result.Output, []byte("PRINTV "+tc.formatted)) {
				t.Errorf("%q fullwidth=%v output = %q, want body %q", tc.body, allowFullwidth, result.Output, tc.formatted)
			}
			if err := checkSourcePreservedOptions(original, result.Output, sourcePreservationOptions{allowFullwidthSpace: allowFullwidth}); err != nil {
				t.Errorf("%q fullwidth=%v formatter output rejected: %v", tc.body, allowFullwidth, err)
			}
		}
	}
	commented := []byte("@F\nPRINTV 000a00 ; keep  \n")
	commentedResult := format.FormatWithOptions(commented, format.ERB, format.Options{})
	if err := checkSourcePreserved(commented, commentedResult.Output); err != nil {
		t.Errorf("commented numeric boundary rejected: %v", err)
	}

	for _, pair := range [][2]string{
		{" 000a00", " 000 a00"},
		{" 0\x60", " 0 \x60"},
		{" 0b10a00", " 0b10 a00"},
		{" 0x1g00", " 0x1 g00"},
		{" 12e3a00", " 12e3 a00"},
		{" 12e+3a00", " 12e+3 a00"},
		{" 12p3a00", " 12p3 a00"},
		{" 0x1p2g00", " 0x1p2 g00"},
		{" 0x1p-2g00", " 0x1p-2 g00"},
	} {
		if got, want := printVFingerprintDialect([]byte(pair[0]), false, format.CommonDialect), printVFingerprintDialect([]byte(pair[1]), false, format.CommonDialect); !bytes.Equal(got, want) {
			t.Errorf("numeric boundary fingerprint mismatch: %q != %q", got, want)
		}
	}
	invalidExponent := []byte("@F\nPRINTV 0x1p-2g00\n")
	invalidExponentResult := format.FormatWithOptions(invalidExponent, format.ERB, format.Options{})
	if err := checkSourcePreserved(invalidExponent, invalidExponentResult.Output); err != nil {
		t.Fatalf("invalid non-decimal exponent producer output rejected: %v", err)
	}
	if err := checkSourcePreserved(invalidExponent, []byte("@F\nPRINTV 0x1p-2 g00\n")); err == nil {
		t.Fatal("invalid non-decimal exponent payload mutation accepted")
	}

	protected := [][2]string{
		{" 'raw  text'", " 'raw text'"},
		{" \"quoted  text\"", " \"quoted text\""},
	}
	for _, pair := range protected {
		if got, want := printVFingerprintDialect([]byte(pair[0]), false, format.CommonDialect), printVFingerprintDialect([]byte(pair[1]), false, format.CommonDialect); bytes.Equal(got, want) {
			t.Errorf("PRINTV payload mutation accepted: %q -> %q", pair[0], pair[1])
		}
	}
	rawSpacing := []byte("@F\nPRINTV raw  text\n")
	rawSpacingResult := format.FormatWithOptions(rawSpacing, format.ERB, format.Options{AllowFullwidthSpace: true})
	if err := checkSourcePreservedOptions(rawSpacing, rawSpacingResult.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
		t.Fatalf("plain PRINTV spacing rejected: %v", err)
	}
	raw := []byte("@F\nPRINTV raw  000a00  suffix\n")
	changed := []byte("@F\nPRINTV changed  000a00  suffix\n")
	if err := checkSourcePreserved(raw, changed); err == nil {
		t.Fatal("raw identifier mutation accepted")
	}
	apostrophe := []byte("@F\nPRINTV lead 'raw  text'  suffix\n")
	apostropheChanged := []byte("@F\nPRINTV lead 'raw text' suffix\n")
	if err := checkSourcePreserved(apostrophe, apostropheChanged); err == nil {
		t.Fatal("apostrophe payload or prefix/suffix spacing mutation accepted")
	}
	for _, pair := range [][2][]byte{
		{[]byte("@F\nPRINTV foo`bar\n"), []byte("@F\nPRINTV foo `bar\n")},
		{[]byte("@F\nPRINTV foo\x00\n"), []byte("@F\nPRINTV foo \x00\n")},
		{[]byte("@F\nPRINTV 0A,foo`bar\n"), []byte("@F\nPRINTV 0A,foo `bar\n")},
		{[]byte("@F\nPRINTV 0A,foo\x00bar\n"), []byte("@F\nPRINTV 0A,foo \x00bar\n")},
	} {
		if err := checkSourcePreserved(pair[0], pair[1]); err == nil {
			t.Fatalf("runtime identifier split accepted: %q -> %q", pair[0], pair[1])
		}
	}
}

func TestPrintVUnknownBodyPreservation(t *testing.T) {
	options := format.Options{Select: []format.RuleID{format.CommandSpacing, format.TokenSpacing, format.FinalNewline}, EOL: format.DefaultEOL, Dialect: format.EMEE, AllowFullwidthSpace: true}
	oracleOptions := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}
	original := []byte("PRINTV  !")
	result := format.FormatWithOptions(original, format.ERB, options)
	if got, want := string(result.Output), "PRINTV !\n"; got != want {
		t.Fatalf("unsupported PRINTV output = %q, want %q", got, want)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Severity != format.Unsupported || result.Diagnostics[0].Code != "unsupported_expression" {
		t.Fatalf("unsupported PRINTV diagnostics = %+v", result.Diagnostics)
	}
	if err := checkSourcePreservedOptions(original, result.Output, oracleOptions); err != nil {
		t.Fatalf("unsupported PRINTV layout rejected: %v", err)
	}
	if err := checkSourcePreservedOptions(original, []byte("PRINTV ?\n"), oracleOptions); err == nil {
		t.Fatal("invalid PRINTV body mutation accepted")
	}
	printVN := []byte("PRINTVN  !")
	printVNResult := format.FormatWithOptions(printVN, format.ERB, options)
	if got, want := string(printVNResult.Output), "PRINTVN !\n"; got != want {
		t.Fatalf("unsupported PRINTVN output = %q, want %q", got, want)
	}
	if len(printVNResult.Diagnostics) != 1 || printVNResult.Diagnostics[0].Severity != format.Unsupported || printVNResult.Diagnostics[0].Code != "unsupported_expression" {
		t.Fatalf("unsupported PRINTVN diagnostics = %+v", printVNResult.Diagnostics)
	}
	if err := checkSourcePreservedOptions(printVN, printVNResult.Output, oracleOptions); err != nil {
		t.Fatalf("unsupported PRINTVN layout rejected: %v", err)
	}

	fullwidth := []byte("@F\nPRINTV　!\n")
	fullwidthResult := format.FormatWithOptions(fullwidth, format.ERB, options)
	if !bytes.Contains(fullwidthResult.Output, []byte("PRINTV !")) || len(fullwidthResult.Diagnostics) != 1 || fullwidthResult.Diagnostics[0].Severity != format.Unsupported || fullwidthResult.Diagnostics[0].Code != "unsupported_expression" {
		t.Fatalf("fullwidth unsupported PRINTV result = output %q diagnostics %+v", fullwidthResult.Output, fullwidthResult.Diagnostics)
	}
	if err := checkSourcePreservedOptions(fullwidth, fullwidthResult.Output, sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}); err != nil {
		t.Fatalf("fullwidth unsupported PRINTV layout rejected: %v", err)
	}

	for _, source := range [][]byte{[]byte("PRINTV !"), []byte("@F\nPRINTV !\n")} {
		result := format.FormatWithOptions(source, format.ERB, format.Options{})
		if err := checkSourcePreserved(source, result.Output); err != nil {
			t.Fatalf("unsupported PRINTV layout rejected: %v", err)
		}
		mutated := bytes.Replace(result.Output, []byte("!"), []byte("?"), 1)
		if err := checkSourcePreserved(source, mutated); err == nil {
			t.Fatalf("unsupported PRINTV body mutation accepted: %q", mutated)
		}
	}
	control := []byte("@F\nPRINTV \x04 \n")
	controlResult := format.FormatWithOptions(control, format.ERB, format.Options{AllowFullwidthSpace: true})
	if len(controlResult.Diagnostics) != 0 {
		t.Fatalf("control PRINTV diagnostics = %+v", controlResult.Diagnostics)
	}
	if err := checkSourcePreservedOptions(control, controlResult.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
		t.Fatalf("control PRINTV layout rejected: %v", err)
	}
	unary := []byte("@F\nPRINTV ~  1\n")
	unaryResult := format.FormatWithOptions(unary, format.ERB, format.Options{AllowFullwidthSpace: true})
	if err := checkSourcePreservedOptions(unary, unaryResult.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
		t.Fatalf("unary PRINTV layout rejected: %v", err)
	}
	controlOperand := []byte("@F\nPRINTV 0#\x10\n")
	controlOperandResult := format.FormatWithOptions(controlOperand, format.ERB, format.Options{AllowFullwidthSpace: true})
	if len(controlOperandResult.Diagnostics) != 0 {
		t.Fatalf("control operand PRINTV diagnostics = %+v", controlOperandResult.Diagnostics)
	}
	if err := checkSourcePreservedOptions(controlOperand, controlOperandResult.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
		t.Fatalf("control operand PRINTV layout rejected: %v", err)
	}
	commaFields := []byte("@F\nPRINTV UNKNOWN,a\n")
	commaFieldsResult := format.FormatWithOptions(commaFields, format.ERB, format.Options{AllowFullwidthSpace: true})
	if len(commaFieldsResult.Diagnostics) != 0 {
		t.Fatalf("comma-field PRINTV diagnostics = %+v", commaFieldsResult.Diagnostics)
	}
	if err := checkSourcePreservedOptions(commaFields, commaFieldsResult.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
		t.Fatalf("comma-field PRINTV layout rejected: %v", err)
	}
}

func TestPrintVUnaryOperatorLongestMatch(t *testing.T) {
	source := []byte("@F\nPRINTV a ! !0\n")
	result := format.FormatWithOptions(source, format.ERB, format.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreserved(source, result.Output); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
}

func TestPrintVSupportedBinaryOperators(t *testing.T) {
	for _, op := range []string{"!&", "!|", "^^", "&&", "||", "!=", "==", "<=", ">="} {
		for _, operands := range [][2]string{{"0", "0"}, {"RESULT", "ARG"}} {
			source := []byte("@F\nPRINTV " + operands[0] + op + operands[1] + "\n")
			result := format.FormatWithOptions(source, format.ERB, format.Options{})
			if len(result.Diagnostics) != 0 {
				t.Fatalf("op=%q operands=%q diagnostics=%+v output=%q", op, operands, result.Diagnostics, result.Output)
			}
			if err := checkSourcePreserved(source, result.Output); err != nil {
				t.Fatalf("op=%q operands=%q output=%q: %v", op, operands, result.Output, err)
			}
		}
	}
	for _, pair := range [][2]string{
		{"@F\nPRINTV 0 ! & 0\n", "@F\nPRINTV 0 !& 0\n"},
		{"@F\nPRINTV 0 ^ ^ 0\n", "@F\nPRINTV 0 ^^ 0\n"},
	} {
		if err := checkSourcePreserved([]byte(pair[0]), []byte(pair[1])); err == nil {
			t.Fatalf("accepted longest-match mutation: %q -> %q", pair[0], pair[1])
		}
	}
}

func TestSaveVarNumericIdentifierBoundaryPreservation(t *testing.T) {
	original := []byte("SAVEVAR (0A0)")
	result := format.FormatWithOptions(original, format.ERB, format.Options{AllowFullwidthSpace: true})
	if got, want := string(result.Output), "SAVEVAR (0 A0)\n"; got != want {
		t.Fatalf("SAVEVAR output = %q, want %q", got, want)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("SAVEVAR diagnostics = %+v", result.Diagnostics)
	}
	if err := checkSourcePreservedOptions(original, result.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
		t.Fatal(err)
	}
}

func TestPrintVNulIdentifierGapPreservation(t *testing.T) {
	original := []byte("@F\nPRINTV 0a\x00b  c\n")
	result := format.FormatWithOptions(original, format.ERB, format.Options{})
	if err := checkSourcePreserved(original, result.Output); err != nil {
		t.Fatal(err)
	}
	boundary := []byte("@F\nPRINTV 000\x00a00  c\n")
	boundaryResult := format.FormatWithOptions(boundary, format.ERB, format.Options{})
	if err := checkSourcePreserved(boundary, boundaryResult.Output); err != nil {
		t.Fatal(err)
	}
}

func TestPrintVNulTerminalIdentifierSpacing(t *testing.T) {
	for _, source := range [][]byte{
		[]byte("@F\nPRINTV foo\x00+1\n"),
		[]byte("@F\nPRINTV foo\x00\x00bar+1\n"),
		[]byte("@F\nPRINTV foo\x00bar\x00+1\n"),
	} {
		result := format.FormatWithOptions(source, format.ERB, format.Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
		if err := checkSourcePreserved(source, result.Output); err != nil {
			t.Fatalf("source=%q output=%q: %v", source, result.Output, err)
		}
	}
}

func TestPrintVNulCommaFieldProfiles(t *testing.T) {
	options := format.Options{AllowFullwidthSpace: true}
	source := []byte("@F\nPRINTV A,0\x00\n")
	result := format.FormatWithOptions(source, format.ERB, options)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
	invalidSource := []byte("@F\nPRINTV A#.,0\x00\n")
	if err := checkSourcePreserved(invalidSource, []byte("@F\nPRINTV A # .,0\x00\n")); err == nil {
		t.Fatal("accepted known-invalid field before NUL")
	}
	suffixSource := []byte("@F\nPRINTV A,0\x00raw\n")
	suffixOutput := []byte("@F\nPRINTV A, 0\x00raw\n")
	if err := checkSourcePreserved(suffixSource, suffixOutput); err != nil {
		t.Fatalf("legitimate comma gap rejected: %v", err)
	}
	if err := checkSourcePreserved(suffixSource, []byte("@F\nPRINTV A, 0\x00 raw\n")); err == nil {
		t.Fatal("accepted ignored NUL suffix mutation")
	}
}

func TestPrintVNulEmbeddedDottedNumericIdentifier(t *testing.T) {
	source := []byte("@F\nPRINTV 0x0.0A\x00\n")
	result := format.FormatWithOptions(source, format.ERB, format.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreserved(source, result.Output); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
}

func TestPrintVNumericPrefixNulIdentifierSpacing(t *testing.T) {
	options := format.Options{AllowFullwidthSpace: true}
	source := []byte("@F\nPRINTV 000A\x00!0\n")
	result := format.FormatWithOptions(source, format.ERB, options)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
}

func TestPrintVControlDeferredDotSpacing(t *testing.T) {
	source := []byte("@F\nPRINTV 0\x01.00\n")
	for _, allowFullwidth := range []bool{false, true} {
		result := format.FormatWithOptions(source, format.ERB, format.Options{AllowFullwidthSpace: allowFullwidth})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("fullwidth=%v diagnostics=%+v output=%q", allowFullwidth, result.Diagnostics, result.Output)
		}
		if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{allowFullwidthSpace: allowFullwidth}); err != nil {
			t.Fatalf("fullwidth=%v: %v", allowFullwidth, err)
		}
	}
	for _, changed := range [][]byte{
		[]byte("@F\nPRINTV 0\x01 .00\n"),
		[]byte("@F\nPRINTV 0\x02.00\n"),
	} {
		if err := checkSourcePreservedOptions(source, changed, sourcePreservationOptions{allowFullwidthSpace: true}); err == nil {
			t.Fatalf("accepted deferred control mutation: %q", changed)
		}
	}
}

func TestPrintVControlLeadingDottedIdentifier(t *testing.T) {
	source := []byte("@F\nPRINTV \x01.0A\n")
	result := format.FormatWithOptions(source, format.ERB, format.Options{AllowFullwidthSpace: true})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
}

func TestPrintVControlPunctuationSpacing(t *testing.T) {
	source := []byte("@F\nPRINTV 0#\x0f0\n")
	result := format.FormatWithOptions(source, format.ERB, format.Options{AllowFullwidthSpace: true})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
		t.Fatal(err)
	}
	if err := checkSourcePreservedOptions(source, []byte("@F\nPRINTV 0 # \x10 0\n"), sourcePreservationOptions{allowFullwidthSpace: true}); err == nil {
		t.Fatal("accepted deferred control payload mutation")
	}
	invalid := []byte("@F\nPRINTV 9223372036854775808#\x0f0\n")
	if err := checkSourcePreserved(invalid, []byte("@F\nPRINTV 9223372036854775808 # \x0f0\n")); err == nil {
		t.Fatal("accepted known-invalid numeric mutation with deferred control")
	}
	for _, changed := range [][]byte{
		[]byte("@F\nPRINTV A # .\t\n"),
		[]byte("@F\nPRINTV A # .\x00\x0f\n"),
	} {
		original := bytes.Replace(changed, []byte("A # ."), []byte("A#."), 1)
		if err := checkSourcePreserved(original, changed); err == nil {
			t.Fatalf("accepted known-invalid PRINTV mutation: %q", changed)
		}
	}
}

func TestPrintVDelIdentifierProfiles(t *testing.T) {
	options := format.Options{AllowFullwidthSpace: true}
	embedded := []byte("@F\nPRINTV A\x7f#\n")
	result := format.FormatWithOptions(embedded, format.ERB, options)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreservedOptions(embedded, result.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
	if err := checkSourcePreserved(embedded, []byte("@F\nPRINTV A\x7f!\n")); err == nil {
		t.Fatal("accepted embedded DEL suffix mutation")
	}
	boundary := []byte("@F\nPRINTV A \x7f#\n")
	boundaryResult := format.FormatWithOptions(boundary, format.ERB, options)
	if len(boundaryResult.Diagnostics) != 0 {
		t.Fatalf("boundary diagnostics=%+v output=%q", boundaryResult.Diagnostics, boundaryResult.Output)
	}
	if err := checkSourcePreservedOptions(boundary, boundaryResult.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
		t.Fatalf("boundary output=%q: %v", boundaryResult.Output, err)
	}
	if err := checkSourcePreserved([]byte("@F\nPRINTV A#.\n"), []byte("@F\nPRINTV A # .\n")); err == nil {
		t.Fatal("accepted known-invalid A#. mutation")
	}
}

func TestPrintVKnownPrefixIdentifierProfiles(t *testing.T) {
	source := []byte("@F\nPRINTV #A`\n")
	result := format.FormatWithOptions(source, format.ERB, format.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreserved(source, result.Output); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
}

func TestPrintVNumericDottedIdentifierSpacing(t *testing.T) {
	for _, source := range [][]byte{
		[]byte("@F\nPRINTV 0A0.\n"),
		[]byte("@F\nPRINTV 000A00.0\n"),
		[]byte("@F\nPRINTV a .\n"),
	} {
		result := format.FormatWithOptions(source, format.ERB, format.Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
		if err := checkSourcePreserved(source, result.Output); err != nil {
			t.Fatalf("source=%q output=%q: %v", source, result.Output, err)
		}
	}
	for _, tc := range []struct {
		source, changed string
		wantErr         bool
	}{
		{"@F\nPRINTV 0A0.\n", "@F\nPRINTV 0 A0.\n", false},
		{"@F\nPRINTV 0A0.\n", "@F\nPRINTV 0 A1.\n", true},
		{"@F\nPRINTV 000A00.0\n", "@F\nPRINTV 000 A00.1\n", true},
		{"@F\nPRINTV 0.00\n", "@F\nPRINTV 0 .00\n", true},
		{"@F\nPRINTV 0A0.\n", "@F\nPRINTV 0 A0 .\n", false},
		{"@F\nPRINTV A .\n", "@F\nPRINTV A.\n", true},
	} {
		err := checkSourcePreserved([]byte(tc.source), []byte(tc.changed))
		if (err != nil) != tc.wantErr {
			t.Fatalf("source=%q changed=%q wantErr=%v got=%v", tc.source, tc.changed, tc.wantErr, err)
		}
	}
}

func TestPrintVDottedUnknownPrefixFuzzProfile(t *testing.T) {
	separator := ""
	options := format.Options{AllowFullwidthSpace: true, ContinuationSeparator: &separator}
	for i, rule := range format.Rules() {
		if 0x11fffffffa7&(uint64(1)<<i) != 0 {
			options.Select = append(options.Select, rule.ID)
		}
	}
	source := []byte("@F\nPRINTV .0A0\n")
	result := format.FormatWithOptions(source, format.ERB, options)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if !bytes.Contains(result.Output, []byte("PRINTV .0 A0")) {
		t.Fatalf("output=%q, want numeric identifier gap", result.Output)
	}
	if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{allowFullwidthSpace: true, continuationSeparator: &separator}); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
	changed := []byte("@F\nPRINTV .0 A0\n")
	if err := checkSourcePreservedOptions(source, changed, sourcePreservationOptions{allowFullwidthSpace: true, continuationSeparator: &separator}); err != nil {
		t.Fatalf("legitimate numeric identifier gap rejected: %v", err)
	}
}

func TestPrintVModernBuiltinDottedSpacing(t *testing.T) {
	options := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}
	source := []byte("@F\nPRINTV DAYNAME .\n")
	changed := []byte("@F\nPRINTV DAYNAME.\n")
	if err := checkSourcePreservedOptions(source, changed, options); err == nil {
		t.Fatal("accepted known EMEE builtin dotted spacing mutation")
	}
}

func TestPrintVUnsupportedPrefixEmbeddedIdentifierProfiles(t *testing.T) {
	for _, source := range [][]byte{
		[]byte("@F\nPRINTV .0A\x00\n"),
		[]byte("@F\nPRINTV .0\x01A\n"),
	} {
		result := format.FormatWithOptions(source, format.ERB, format.Options{AllowFullwidthSpace: true})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
		if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
			t.Fatalf("source=%q output=%q: %v", source, result.Output, err)
		}
	}
}

func TestPrintVAttachedSemicolonSpacing(t *testing.T) {
	attached := []byte("@F\nPRINTV;0A0.\n")
	result := format.FormatWithOptions(attached, format.ERB, format.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("attached diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreserved(attached, result.Output); err != nil {
		t.Fatalf("attached output=%q: %v", result.Output, err)
	}
	if err := checkSourcePreserved(attached, []byte("@F\nPRINTV;0 A0.\n")); err != nil {
		t.Fatalf("attached numeric gap rejected: %v", err)
	}
	comment := []byte("@F\nPRINTV ;0A0.\n")
	if err := checkSourcePreserved(comment, []byte("@F\nPRINTV ;0 A0.\n")); err == nil {
		t.Fatal("accepted ordinary comment payload mutation")
	}
}

func TestPrintVAttachedSeparatorArgumentProfiles(t *testing.T) {
	for _, source := range [][]byte{
		[]byte("@F\nPRINTV;00 ,\n"),
		[]byte("@F\nPRINTV;\"quoted  text\"\n"),
		[]byte("@F\nPRINTV;'raw  text'\n"),
		[]byte("@F\nPRINTV;0#\x10\n"),
	} {
		result := format.FormatWithOptions(source, format.ERB, format.Options{AllowFullwidthSpace: true})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
		if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
			t.Fatalf("source=%q output=%q: %v", source, result.Output, err)
		}
	}
	if err := checkSourcePreserved([]byte("@F\nPRINTV;A#.\n"), []byte("@F\nPRINTV;A # .\n")); err == nil {
		t.Fatal("accepted attached known-invalid A#. mutation")
	}
	if err := checkSourcePreserved([]byte("@F\nPRINTV;0x1p-2g00\n"), []byte("@F\nPRINTV;0x1p-2 g00\n")); err == nil {
		t.Fatal("accepted attached invalid numeric mutation")
	}
	if err := checkSourcePreserved([]byte("@F\nPRINTV ;00 ,\n"), []byte("@F\nPRINTV ;00,\n")); err == nil {
		t.Fatal("accepted spaced-semicolon comment mutation")
	}
}

func TestPrintVLeadingAssignmentDeferredProfile(t *testing.T) {
	mask := uint64(1236950581231)
	options := format.Options{Dialect: format.CommonDialect, AllowFullwidthSpace: true}
	for i, rule := range format.Rules() {
		if mask&(uint64(1)<<i) != 0 {
			options.Select = append(options.Select, rule.ID)
		}
	}
	source := []byte("@F\nPRINTV =a\n")
	result := format.FormatWithOptions(source, format.ERB, options)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{dialect: format.CommonDialect, allowFullwidthSpace: true}); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
	if err := checkSourcePreserved([]byte("@F\nPRINTV =A\n"), []byte("@F\nPRINTV = A\n")); err == nil {
		t.Fatal("accepted known leading-assignment mutation")
	}
	if err := checkSourcePreserved([]byte("@F\nPRINTV =,A\n"), []byte("@F\nPRINTV =, A\n")); err == nil {
		t.Fatal("accepted known leading-assignment comma mutation")
	}
}

func TestPrintVLeadingAssignmentCommaDeferredProfile(t *testing.T) {
	separator := ""
	options := format.Options{AllowFullwidthSpace: true, ContinuationSeparator: &separator}
	for i, rule := range format.Rules() {
		if 0x11fffffffef&(uint64(1)<<i) != 0 {
			options.Select = append(options.Select, rule.ID)
		}
	}
	source := []byte("@F\nPRINTV =,a\n")
	result := format.FormatWithOptions(source, format.ERB, options)
	if len(result.Diagnostics) == 0 {
		t.Fatalf("expected unsupported leading assignment diagnostic, output=%q", result.Output)
	}
	if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{allowFullwidthSpace: true, continuationSeparator: &separator}); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
	if err := checkSourcePreserved([]byte("@F\nPRINTV UNKNOWN,A#.\n"), []byte("@F\nPRINTV UNKNOWN, A # .\n")); err != nil {
		t.Fatalf("unknown-first deferred field mutation rejected: %v", err)
	}
	for _, pair := range [][2]string{
		{"@F\nPRINTV A#.,UNKNOWN\n", "@F\nPRINTV A # ., UNKNOWN\n"},
		{"@F\nPRINTV =,a,0x1p-2g00\n", "@F\nPRINTV =, a, 0x1p-2 g00\n"},
	} {
		if err := checkSourcePreserved([]byte(pair[0]), []byte(pair[1])); err == nil {
			t.Fatalf("accepted known-invalid mixed field mutation: %q -> %q", pair[0], pair[1])
		}
	}
}
