package cli

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestSourcePreservationFullwidthContext(t *testing.T) {
	separator := " "
	options := format.Options{AllowFullwidthSpace: true, ContinuationSeparator: &separator}
	verification := sourcePreservationOptions{dialect: format.CommonDialect, continuationSeparator: &separator, allowFullwidthSpace: true}
	for _, source := range []string{
		"@F\nIF A　+　B\nENDIF\n",
		"@F\nIF \"A　B\"+FOO\nENDIF\n",
		"@F\nIF A　+　FOO ; note　literal\nENDIF\n",
		"@F\nUSR = A　+　B\n",
		"@F\nUSR　++\n",
		"@F\n#DIM　STATIC　USR_X　,　1\n[IF　USR_X]\nRETURN　1\n[ENDIF]\n",
		"@F\n{\n　RETURN　1　+　2\n}\n",
		"@F\n　{\n　RETURN　1　+　2\n}\u3000\n",
		"@F\n{\nRETURNF @\"outside　{　1+2　}　tail\"\n}\n",
		"@F\n{\n　RETURN　1　+　2\n}\t\n",
		"@F\n{\n　RETURN　1　+　2\n}\u3000\n",
	} {
		input := []byte(source)
		result := format.FormatWithOptions(input, format.ERB, options)
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%v", source, result.Diagnostics)
		}
		if err := checkSourcePreservedOptions(input, result.Output, verification); err != nil {
			t.Fatalf("source=%q output=%q: %v", source, result.Output, err)
		}
	}

	mutated := []byte("@F\nIF \"A B\" + FOO\nENDIF\n")
	original := []byte("@F\nIF \"A　B\"+FOO\nENDIF\n")
	if err := checkSourcePreservedOptions(original, mutated, verification); err == nil {
		t.Fatal("fullwidth literal mutation accepted")
	}
	for _, tc := range []struct {
		name string
		orig []byte
		out  []byte
	}{
		{name: "assignment-rhs", orig: []byte("@F\nUSR = \"A　B\"\n"), out: []byte("@F\nUSR = \"A B\"\n")},
		{name: "macro-body", orig: []byte("@F\n#DEFINE X　A　+　B\n"), out: []byte("@F\n#DEFINE X A + B\n")},
		{name: "macro-name-and-body", orig: []byte("@F\n#DEFINE TEXT　A　+　B\n"), out: []byte("@F\n#DEFINE TEXT A + B\n")},
		{name: "continued-rhs", orig: []byte("@F\n{\nUSR = \"A　B\"\n}\n"), out: []byte("@F\n{\nUSR = \"A B\"\n}\n")},
		{name: "increment-operator", orig: []byte("@F\nUSR　++\n"), out: []byte("@F\nUSR　--\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkSourcePreservedOptions(tc.orig, tc.out, verification); err == nil {
				t.Fatal("raw payload mutation accepted")
			}
		})
	}
	macro := []byte("@F\n#DEFINE TEXT　A　+　B\n")
	if err := checkSourcePreservedOptions(macro, []byte("@F\n#DEFINE TEXT A　+　B\n"), verification); err != nil {
		t.Fatalf("fullwidth DEFINE separator rejected: %v", err)
	}
}

func TestSourcePreservationPrintFormsContext(t *testing.T) {
	if kind, _ := statementFingerprintDialect("PRINTFORMSN", []byte("PRINTFORMSN"), []byte(` "x { 1+2 }"`), format.EMEE); kind != "printforms-literal" {
		t.Fatalf("EMEE PRINTFORMSN kind=%q", kind)
	}
	if kind, _ := statementFingerprintDialect("PRINTFORMSN", []byte("PRINTFORMSN"), []byte(` "x { 1+2 }"`), format.CommonDialect); kind == "printforms-literal" {
		t.Fatal("common PRINTFORMSN entered literal path")
	}
	original := []byte("@F\nPRINTFORMS   \"outside { 1+2 }\";note\n")
	formatted := []byte("@F\nPRINTFORMS \"outside {1 + 2}\" ; note\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("PRINTFORMS FORM trivia rejected: %v", err)
	}
	for _, changed := range [][]byte{
		[]byte("@F\nPRINTFORMS \"changed {1 + 2}\" ; note\n"),
		[]byte("@F\nPRINTFORMS \"outside {1 + 2} changed\" ; note\n"),
		[]byte("@F\nPRINTFORMS \"outside [[ALIAS]] {1 + 2}\" ; changed\n"),
	} {
		if err := checkSourcePreserved(original, changed); err == nil {
			t.Fatalf("PRINTFORMS mutation accepted: %q", changed)
		}
	}
	aliasOriginal := []byte("@F\nPRINTFORMS \"outside [[ALIAS]] { 1+2 }\"\n")
	if err := checkSourcePreserved(aliasOriginal, aliasOriginal); err != nil {
		t.Fatalf("PRINTFORMS alias fallback rejected: %v", err)
	}
	if err := checkSourcePreserved(aliasOriginal, []byte("@F\nPRINTFORMS \"outside [[ ALIAS ]] { 1+2 }\"\n")); err == nil {
		t.Fatal("PRINTFORMS alias mutation accepted")
	}

	fullwidthOriginal := []byte("@F\nPRINTFORMS \"outside　{　1+2　}　tail\"\n")
	fullwidthFormatted := []byte("@F\nPRINTFORMS \"outside　{1 + 2}　tail\"\n")
	result := format.FormatWithOptions(fullwidthOriginal, format.ERB, format.Options{AllowFullwidthSpace: true})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("fullwidth PRINTFORMS diagnostics: %v", result.Diagnostics)
	}
	if err := checkSourcePreservedOptions(fullwidthOriginal, result.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
		t.Fatalf("actual fullwidth PRINTFORMS output=%q: %v", result.Output, err)
	}
	if err := checkSourcePreservedOptions(fullwidthOriginal, fullwidthFormatted, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
		t.Fatalf("fullwidth PRINTFORMS FORM trivia rejected: %v", err)
	}
	if err := checkSourcePreservedOptions(fullwidthOriginal, []byte("@F\nPRINTFORMS \"changed　{1 + 2}　tail\"\n"), sourcePreservationOptions{allowFullwidthSpace: true}); err == nil {
		t.Fatal("PRINTFORMS literal mutation accepted")
	}
	separatorOriginal := []byte("@F\nPRINTFORMS　\"outside　{　1+2　}　tail\"\n")
	separatorResult := format.FormatWithOptions(separatorOriginal, format.ERB, format.Options{AllowFullwidthSpace: true})
	if len(separatorResult.Diagnostics) != 0 {
		t.Fatalf("fullwidth PRINTFORMS separator diagnostics: %v", separatorResult.Diagnostics)
	}
	if err := checkSourcePreservedOptions(separatorOriginal, separatorResult.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
		t.Fatalf("fullwidth PRINTFORMS separator output=%q: %v", separatorResult.Output, err)
	}
	if err := checkSourcePreservedOptions(separatorOriginal, []byte("@F\nPRINTFORMS \"changed　{1 + 2}　tail\"\n"), sourcePreservationOptions{allowFullwidthSpace: true}); err == nil {
		t.Fatal("fullwidth PRINTFORMS separator literal mutation accepted")
	}
	emeeSeparatorOriginal := []byte("@F\nPRINTFORMSN　\"outside　{　1+2　}　tail\"\n")
	emeeSeparatorResult := format.FormatWithOptions(emeeSeparatorOriginal, format.ERB, format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true})
	if len(emeeSeparatorResult.Diagnostics) != 0 {
		t.Fatalf("fullwidth PRINTFORMSN separator diagnostics: %v", emeeSeparatorResult.Diagnostics)
	}
	if err := checkSourcePreservedOptions(emeeSeparatorOriginal, emeeSeparatorResult.Output, sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}); err != nil {
		t.Fatalf("fullwidth PRINTFORMSN separator output=%q: %v", emeeSeparatorResult.Output, err)
	}
	if err := checkSourcePreserved(fullwidthOriginal, fullwidthFormatted); err == nil {
		t.Fatal("default verifier accepted fullwidth grammar trivia")
	}
	separator := " "
	continuedOriginal := []byte("@F\n{\nPRINTFORMS \"x { 1+2 }\"\n}\n")
	continuedFormatted := []byte("@F\n{\nPRINTFORMS \"x {1 + 2}\"\n}\n")
	if err := checkSourcePreservedOptions(continuedOriginal, continuedFormatted, sourcePreservationOptions{continuationSeparator: &separator}); err != nil {
		t.Fatalf("continued PRINTFORMS FORM trivia rejected: %v", err)
	}
	continuedFullwidthOriginal := []byte("@F\n{\nPRINTFORMS　\"x　{　1+2　}\"\n}\n")
	continuedFullwidthResult := format.FormatWithOptions(continuedFullwidthOriginal, format.ERB, format.Options{AllowFullwidthSpace: true, ContinuationSeparator: &separator})
	if len(continuedFullwidthResult.Diagnostics) != 0 {
		t.Fatalf("continued fullwidth PRINTFORMS diagnostics: %v", continuedFullwidthResult.Diagnostics)
	}
	if err := checkSourcePreservedOptions(continuedFullwidthOriginal, continuedFullwidthResult.Output, sourcePreservationOptions{continuationSeparator: &separator, allowFullwidthSpace: true}); err != nil {
		t.Fatalf("continued fullwidth PRINTFORMS output=%q: %v", continuedFullwidthResult.Output, err)
	}
}

func TestContinuationClosingTriviaIsUnicodeWhitespace(t *testing.T) {
	for _, allow := range []bool{false, true} {
		for _, trivia := range []string{"\t", "\u3000"} {
			original := []byte("@F\n{\nRETURN 1+2\n}" + trivia + "\n")
			formatted := []byte("@F\n{\nRETURN 1 + 2\n}\n")
			separator := " "
			options := sourcePreservationOptions{continuationSeparator: &separator, allowFullwidthSpace: allow}
			err := checkSourcePreservedOptions(original, formatted, options)
			wantPreserved := trivia == "\t" || allow
			if wantPreserved && err != nil {
				t.Fatalf("allow=%t trivia=%q: %v", allow, trivia, err)
			}
			if !wantPreserved && err == nil {
				t.Fatalf("allow=%t trivia=%q: non-grammar whitespace removal passed", allow, trivia)
			}
		}
	}
}

func TestContinuationBoundaryMarkerCollision(t *testing.T) {
	separator := " "
	options := sourcePreservationOptions{continuationSeparator: &separator}
	source := append([]byte("@F\n{\nRETURN 1+2"), continuationBoundaryMarker)
	source = append(source, []byte("\nRETURN 3+4\n}\n")...)
	formatted := format.FormatWithOptions(source, format.ERB, format.Options{ContinuationSeparator: &separator}).Output
	if err := checkSourcePreservedOptions(source, formatted, options); err != nil {
		t.Fatalf("collision output=%q: %v", formatted, err)
	}
	mutated := bytes.Replace(formatted, []byte{continuationBoundaryMarker}, []byte{'X'}, 1)
	if err := checkSourcePreservedOptions(source, mutated, options); err == nil {
		t.Fatal("continuation control-byte mutation accepted")
	}

	logical := make([]byte, 0, 32)
	for marker := byte(1); marker < 0x20; marker++ {
		if marker != '\r' && marker != '\n' {
			logical = append(logical, marker)
		}
	}
	if _, ok := continuationBoundaryMarkerFor(logical); ok {
		t.Fatal("all safe continuation markers were reported available")
	}
	eligible := make([]byte, 0, 29)
	for marker := byte(1); marker < 0x20; marker++ {
		if marker != '\t' && marker != '\v' && marker != '\f' && marker != '\r' && marker != '\n' {
			eligible = append(eligible, marker)
		}
	}
	if _, ok := continuationBoundaryMarkerFor(eligible); ok {
		t.Fatal("tab was offered as a continuation sentinel")
	}
}

func TestCorpusAllowFullwidthSpaceSetting(t *testing.T) {
	t.Setenv("ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE", "true")
	if allow, err := corpusAllowFullwidthSpace(); err != nil || !allow {
		t.Fatalf("true setting = %t, %v", allow, err)
	}
	t.Setenv("ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE", "false")
	if allow, err := corpusAllowFullwidthSpace(); err != nil || allow {
		t.Fatalf("false setting = %t, %v", allow, err)
	}
	t.Setenv("ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE", "maybe")
	if _, err := corpusAllowFullwidthSpace(); err == nil {
		t.Fatal("invalid fullwidth setting was accepted")
	}
}

func TestFullwidthPreservationUnterminatedQuotedEscape(t *testing.T) {
	src := []byte("@F\nIF \"x\\\nENDIF\n")
	r := format.FormatWithOptions(src, format.ERB, format.Options{AllowFullwidthSpace: true})
	if len(r.Diagnostics) == 0 {
		t.Fatal("unterminated quote was accepted")
	}
	if err := checkSourcePreservedOptions(src, r.Output, sourcePreservationOptions{allowFullwidthSpace: true}); err != nil {
		t.Fatalf("malformed quoted payload changed: %v", err)
	}
}
