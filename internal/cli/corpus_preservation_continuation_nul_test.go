package cli

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestContinuationPreservationFullwidthCallComment(t *testing.T) {
	for _, tc := range []struct {
		name, command, want string
	}{
		{"plain", "TRYCALL", "TRYCALL F, ; note"},
		{"form", "CALLFORM", "CALLFORM F, ;note"},
	} {
		for _, separator := range []string{"", " ", "\t"} {
			t.Run(tc.name+"-"+continuationSeparatorName(separator), func(t *testing.T) {
				source := []byte("@F\n{\n" + tc.command + " F,\u3000;note\n}\n")
				options := format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true, ContinuationSeparator: &separator}
				result := format.FormatWithOptions(source, format.ERB, options)
				if len(result.Diagnostics) != 0 {
					t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
				}
				if !bytes.Contains(result.Output, []byte(tc.want)) {
					t.Fatalf("want %q in output %q", tc.want, result.Output)
				}
				oracle := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true, continuationSeparator: &separator}
				if err := checkSourcePreservedOptions(source, result.Output, oracle); err != nil {
					t.Fatalf("output rejected: %v\noutput=%q", err, result.Output)
				}
			})
		}
	}
}

func TestContinuationPreservationCallCommentShape(t *testing.T) {
	for _, tc := range []struct {
		name, command, want string
	}{
		{"plain", "CALL", "CALL F, 1 ; A 1"},
		{"form", "CALLFORM", "CALLFORM F, 1;A 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			separator := ""
			source := []byte("@F\n{\n" + tc.command + " F,1;A 1\n}\n")
			options := format.Options{Dialect: format.EMEE, ContinuationSeparator: &separator}
			result := format.FormatWithOptions(source, format.ERB, options)
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
			}
			if !bytes.Contains(result.Output, []byte(tc.want)) {
				t.Fatalf("want %q in output %q", tc.want, result.Output)
			}
			oracle := sourcePreservationOptions{dialect: format.EMEE, continuationSeparator: &separator}
			if err := checkSourcePreservedOptions(source, result.Output, oracle); err != nil {
				t.Fatalf("output rejected: %v\noutput=%q", err, result.Output)
			}
		})
	}
}

func TestContinuationPreservationNulSuffix(t *testing.T) {
	cases := []struct {
		name, source, from, to, prefix string
	}{
		{"call-inline", "@F\n{\nTRYCALL F,1+2\x00ignored\n}\n", "ignored", "ignored ", "1 + 2"},
		{"call-rows", "@F\n{\nTRYCALL F\x00x\nyz\n}\n", "yz", "y z", ""},
		{"form-call-rows", "@F\n{\nCALLFORM F\x00\n,1\n}\n", ",1", ", 1", ""},
		{"expression-rows", "@F\n{\nRETURN 1\x00\n;note\n}\n", ";note", "; note", ""},
		{"assignment-rows", "@F\n{\nRESULT +=1\x00\n;note\n}\n", ";note", "; note", ""},
		{"printv-rows", "@F\n{\nPRINTV 1\x00\n0\n}\n", "0", " 0", ""},
		{"form-inline", "@F\n{\nPRINTFORM {1+2}\x00{3+4}\n}\n", "{3+4}", "{3 + 4}", "{1 + 2}"},
		{"form-rows", "@F\n{\nPRINTFORM raw\x00\n{1+2}\n\n}\n", "{1+2}", "{1 + 2}", ""},
	}
	for _, separator := range []string{"", " ", "\t"} {
		for _, tc := range cases {
			t.Run(tc.name+"-"+continuationSeparatorName(separator), func(t *testing.T) {
				source := []byte(tc.source)
				options := format.Options{Dialect: format.EMEE, ContinuationSeparator: &separator}
				result := format.FormatWithOptions(source, format.ERB, options)
				if len(result.Diagnostics) != 0 {
					t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
				}
				oracle := sourcePreservationOptions{dialect: format.EMEE, continuationSeparator: &separator}
				if err := checkSourcePreservedOptions(source, result.Output, oracle); err != nil {
					t.Fatalf("source rejected: %v\noutput=%q", err, result.Output)
				}
				if tc.prefix != "" && !bytes.Contains(result.Output, []byte(tc.prefix)) {
					t.Fatalf("safe prefix was not formatted: want %q in %q", tc.prefix, result.Output)
				}
				mutated := bytes.Replace(result.Output, []byte(tc.from), []byte(tc.to), 1)
				if bytes.Equal(mutated, result.Output) {
					t.Fatal("suffix mutation was not applied")
				}
				if err := checkSourcePreservedOptions(source, mutated, oracle); err == nil {
					t.Fatalf("accepted ignored suffix mutation: %q", mutated)
				}
			})
		}
	}
}

func TestContinuationPreservationPrintVTerminalComma(t *testing.T) {
	separator := ""
	source := []byte("@F\n{\nPRINTV 'x,0,\x00\n}\n")
	options := format.Options{Dialect: format.EMEE, ContinuationSeparator: &separator}
	result := format.FormatWithOptions(source, format.ERB, options)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	oracle := sourcePreservationOptions{dialect: format.EMEE, continuationSeparator: &separator}
	if err := checkSourcePreservedOptions(source, result.Output, oracle); err != nil {
		t.Fatalf("terminal comma normalization rejected: %v\noutput=%q", err, result.Output)
	}
}

func TestContinuationPreservationNulIgnoredCallOperator(t *testing.T) {
	for _, command := range []string{"TRYCALL", "CALLFORM"} {
		t.Run(command, func(t *testing.T) {
			separator := ""
			source := []byte("@F\n{\n" + command + " F,1\x00 &\n}\n")
			options := format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true, ContinuationSeparator: &separator}
			result := format.FormatWithOptions(source, format.ERB, options)
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
			}
			oracle := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true, continuationSeparator: &separator}
			if err := checkSourcePreservedOptions(source, result.Output, oracle); err != nil {
				t.Fatalf("safe call prefix rejected: %v\noutput=%q", err, result.Output)
			}
		})
	}
}

func TestContinuationPreservationNulSuffixKeepsRowBoundaries(t *testing.T) {
	separator := ""
	source := []byte("@F\n{\nTRYCALL F\x00x\nyz\n}\n")
	options := format.Options{Dialect: format.EMEE, ContinuationSeparator: &separator}
	result := format.FormatWithOptions(source, format.ERB, options)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	oracle := sourcePreservationOptions{dialect: format.EMEE, continuationSeparator: &separator}
	if err := checkSourcePreservedOptions(source, result.Output, oracle); err != nil {
		t.Fatalf("source rejected: %v\noutput=%q", err, result.Output)
	}
	mutated := bytes.Replace(result.Output, []byte("\x00x\n"), []byte("\x00xy\n"), 1)
	mutated = bytes.Replace(mutated, []byte("\nyz\n"), []byte("\nz\n"), 1)
	if err := checkSourcePreservedOptions(source, mutated, oracle); err == nil {
		t.Fatalf("accepted ignored suffix row redistribution: %q", mutated)
	}
}

func TestContinuationPreservationNulSuffixAssignment(t *testing.T) {
	separator := ""
	source := []byte("@F\n{\nVALUE +=1\x00\n;note\n}\n")
	options := format.Options{Dialect: format.EMEE, ContinuationSeparator: &separator}
	result := format.FormatWithOptions(source, format.ERB, options)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	oracle := sourcePreservationOptions{dialect: format.EMEE, continuationSeparator: &separator}
	if err := checkSourcePreservedOptions(source, result.Output, oracle); err != nil {
		t.Fatalf("source rejected: %v\noutput=%q", err, result.Output)
	}
	mutated := bytes.Replace(result.Output, []byte(";note"), []byte("; note"), 1)
	if err := checkSourcePreservedOptions(source, mutated, oracle); err == nil {
		t.Fatalf("accepted ignored assignment suffix mutation: %q", mutated)
	}
}

func TestContinuationPreservationNulSuffixKeepsFullwidthBytes(t *testing.T) {
	separator := ""
	source := []byte("@F\n{\nPRINTFORM raw\x00\u3000{1+2}\n}\n")
	options := format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true, ContinuationSeparator: &separator}
	result := format.FormatWithOptions(source, format.ERB, options)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	oracle := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true, continuationSeparator: &separator}
	if err := checkSourcePreservedOptions(source, result.Output, oracle); err != nil {
		t.Fatalf("source rejected: %v\noutput=%q", err, result.Output)
	}
	mutated := bytes.Replace(result.Output, []byte("\u3000{1+2}"), []byte(" {1+2}"), 1)
	if err := checkSourcePreservedOptions(source, mutated, oracle); err == nil {
		t.Fatalf("accepted ignored fullwidth suffix mutation: %q", mutated)
	}
}

func continuationSeparatorName(separator string) string {
	switch separator {
	case "":
		return "empty"
	case " ":
		return "space"
	case "\t":
		return "tab"
	default:
		return "other"
	}
}
