package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestInputOracleTrailingComma(t *testing.T) {
	for _, tc := range []struct {
		name, body, separator string
		continued             bool
	}{
		{"two fields", "INPUTS prompt;  text,0,1+2,\n", "", false},
		{"omitted first", "ONEINPUTS prompt,,1+2,\n", "", false},
		{"omitted second", "BINPUTS prompt,1+2,,\n", "", false},
		{"padding", "ONEBINPUTS prompt,0,1+2, \t \n", "", false},
		{"comma separator", "INPUTS prompt;  text,\n1+2\n", ",", true},
		{"space separator", "INPUTS prompt,0,\n1+2, \t \n", " ", true},
		{"split literal", "INPUTS prompt raw  \ntext,0,1+2,\n", "", true},
	} {
		for _, wide := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/wide=%t", tc.name, wide), func(t *testing.T) {
				body := tc.body
				if wide {
					body = strings.ReplaceAll(body, " ", "\u3000")
				}
				if tc.continued {
					body = "{\n" + body + "}\n"
				}
				source := []byte("@F\n" + body)
				options := format.Options{Dialect: format.EMEE, AllowFullwidthSpace: wide}
				proof := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: wide}
				if tc.continued {
					options.ContinuationSeparator = &tc.separator
					proof.continuationSeparator = &tc.separator
				}
				result := format.FormatWithOptions(source, format.ERB, options)
				if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte("1 + 2")) {
					t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
				}
				if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
					t.Fatalf("formatted output rejected: %v\noutput=%q", err, result.Output)
				}
				for _, mutation := range [][2]string{{"prompt", "PROMPT"}, {"1 + 2", "1 + 3"}} {
					changed := bytes.Replace(result.Output, []byte(mutation[0]), []byte(mutation[1]), 1)
					if bytes.Equal(changed, result.Output) || checkSourcePreservedOptions(source, changed, proof) == nil {
						t.Fatalf("payload mutation absent or accepted: %q", changed)
					}
				}
				if comma := bytes.LastIndexByte(result.Output, ','); comma >= 0 {
					changed := append(append([]byte(nil), result.Output[:comma]...), result.Output[comma+1:]...)
					if checkSourcePreservedOptions(source, changed, proof) == nil {
						t.Fatalf("comma removal accepted: %q", changed)
					}
				}
				if tc.name == "split literal" {
					from, to := "raw  \ntext", "raw \n text"
					if wide {
						from, to = strings.ReplaceAll(from, " ", "\u3000"), strings.ReplaceAll(to, " ", "\u3000")
					}
					changed := bytes.Replace(result.Output, []byte(from), []byte(to), 1)
					if bytes.Equal(changed, result.Output) || checkSourcePreservedOptions(source, changed, proof) == nil {
						t.Fatalf("physical payload mutation absent or accepted: %q", changed)
					}
				}
			})
		}
	}
}

func TestInputOraclePreservesSurplusFields(t *testing.T) {
	for _, body := range []string{"INPUTS prompt,0,1+2,3", "INPUTS prompt,0,1+2,,", "INPUTS prompt,0,1+2,\u3000"} {
		for _, continued := range []bool{false, true} {
			source := "@F\n" + body + "\n"
			separator := ""
			proof := sourcePreservationOptions{dialect: format.EMEE}
			if continued {
				source = "@F\n{\n" + body + "\n}\n"
				proof.continuationSeparator = &separator
			}
			changed := strings.Replace(source, "1+2", "1 + 2", 1)
			if checkSourcePreservedOptions([]byte(source), []byte(changed), proof) == nil {
				t.Fatalf("surplus payload mutation accepted: %q", source)
			}
		}
	}
}

func TestInputOracleDialectBoundaries(t *testing.T) {
	emuera := []byte("@F\nINPUTS raw {1+2},1+2\n")
	result := format.FormatWithOptions(emuera, format.ERB, format.Options{Dialect: format.Emuera1824})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("Emuera INPUTS diagnostics = %v", result.Diagnostics)
	}
	if err := checkSourcePreservedDialect(emuera, result.Output, format.Emuera1824); err != nil {
		t.Fatalf("Emuera INPUTS form with comma: %v\noutput=%q", err, result.Output)
	}
	mutated := bytes.Replace(result.Output, []byte("{1 + 2}"), []byte("{1 + 3}"), 1)
	if err := checkSourcePreservedDialect(emuera, mutated, format.Emuera1824); err == nil {
		t.Fatal("Emuera INPUTS form mutation was accepted")
	}

	emee := []byte("@F\nINPUTS prompt;  text,A+B; note\n")
	result = format.FormatWithOptions(emee, format.ERB, format.Options{Dialect: format.EMEE})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("EMEE INPUTS diagnostics = %v", result.Diagnostics)
	}
	if err := checkSourcePreservedDialect(emee, result.Output, format.EMEE); err != nil {
		t.Fatalf("EMEE INPUTS form semicolon: %v\noutput=%q", err, result.Output)
	}
	mutated = bytes.Replace(result.Output, []byte(";  text"), []byte("; text"), 1)
	if err := checkSourcePreservedDialect(emee, mutated, format.EMEE); err == nil {
		t.Fatal("EMEE INPUTS literal spacing mutation was accepted")
	}
	mutated = bytes.Replace(result.Output, []byte("A + B"), []byte("A + C"), 1)
	if err := checkSourcePreservedDialect(emee, mutated, format.EMEE); err == nil {
		t.Fatal("EMEE INPUTS expression mutation was accepted")
	}

	common := []byte("@F\nINPUTS raw {1+2},1+2\n")
	commonOutput := []byte("@F\n    INPUTS raw {1 + 2},1+2\n")
	if err := checkSourcePreservedDialect(common, commonOutput, format.CommonDialect); err == nil {
		t.Fatal("Common INPUTS comma form was treated as supported")
	}
	commonLiteral := []byte("@F\nINPUTS prompt;  text\n")
	commonLiteralChanged := []byte("@F\n    INPUTS prompt; text\n")
	if err := checkSourcePreservedDialect(commonLiteral, commonLiteralChanged, format.CommonDialect); err == nil {
		t.Fatal("Common INPUTS literal spacing mutation was accepted")
	}

	for _, source := range []string{
		"@F\nINPUTS prompt(,A+B\n",
		"@F\nINPUTS prompt[,A+B\n",
		"@F\nINPUTS @\"prompt\",A+B\n",
	} {
		result = format.FormatWithOptions([]byte(source), format.ERB, format.Options{Dialect: format.EMEE})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("EMEE raw FORM boundary diagnostics for %q = %v", source, result.Diagnostics)
		}
		if err := checkSourcePreservedDialect([]byte(source), result.Output, format.EMEE); err != nil {
			t.Fatalf("EMEE raw FORM boundary for %q: %v\noutput=%q", source, err, result.Output)
		}
	}
}

func TestInputOracleContinuationBoundaries(t *testing.T) {
	for _, separator := range []string{"", " "} {
		emuera := []byte("@F\n{\nINPUTS raw {1+\n2},1+2\n}\n")
		options := format.Options{Dialect: format.Emuera1824, ContinuationSeparator: &separator}
		proof := sourcePreservationOptions{dialect: format.Emuera1824, continuationSeparator: &separator}
		result := format.FormatWithOptions(emuera, format.ERB, options)
		if len(result.Diagnostics) != 0 {
			t.Fatalf("Emuera continuation separator %q diagnostics = %v", separator, result.Diagnostics)
		}
		if err := checkSourcePreservedOptions(emuera, result.Output, proof); err != nil {
			t.Fatalf("Emuera continuation separator %q: %v\noutput=%q", separator, err, result.Output)
		}

		emee := []byte("@F\n{\nINPUTS prompt;  text,\nA+\nB\n}\n")
		options.Dialect = format.EMEE
		proof.dialect = format.EMEE
		result = format.FormatWithOptions(emee, format.ERB, options)
		if len(result.Diagnostics) != 0 {
			t.Fatalf("EMEE continuation separator %q diagnostics = %v", separator, result.Diagnostics)
		}
		if err := checkSourcePreservedOptions(emee, result.Output, proof); err != nil {
			t.Fatalf("EMEE continuation separator %q: %v\noutput=%q", separator, err, result.Output)
		}
		mutated := bytes.Replace(result.Output, []byte(";  text"), []byte("; text"), 1)
		if err := checkSourcePreservedOptions(emee, mutated, proof); err == nil {
			t.Fatalf("EMEE continuation literal mutation separator %q was accepted", separator)
		}
	}
}

func TestInputOracleFullwidthFormBoundaries(t *testing.T) {
	source := []byte("@F\nINPUTS　prompt;　text,　A　+　B\n")
	options := format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true}
	proof := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}
	result := format.FormatWithOptions(source, format.ERB, options)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("fullwidth INPUTS diagnostics = %v", result.Diagnostics)
	}
	if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
		t.Fatalf("fullwidth INPUTS form: %v\noutput=%q", err, result.Output)
	}
	mutated := bytes.Replace(result.Output, []byte("text"), []byte("TEXT"), 1)
	if err := checkSourcePreservedOptions(source, mutated, proof); err == nil {
		t.Fatal("fullwidth INPUTS literal mutation was accepted")
	}
}

func TestInputOraclePreservesMalformedForms(t *testing.T) {
	for _, body := range []string{"{1+2,3+4", "%text,3+4", "\\@1?raw,3+4"} {
		source := []byte("@F\nINPUTS " + body + "\n")
		changed := bytes.Replace(source, []byte("3+4"), []byte("3 + 4"), 1)
		for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
			if err := checkSourcePreservedDialect(source, changed, dialect); err == nil {
				t.Fatalf("dialect=%d malformed FORM mutation accepted: %q", dialect, source)
			}
		}
	}
}
