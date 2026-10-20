package format

import (
	"bytes"
	"testing"
)

func TestEmptySharpDeclarationIsIdempotent(t *testing.T) {
	for _, gap := range []string{" ", "\t", "　"} {
		options := Options{Select: []RuleID{DeclarationIndent, CodeTrailingWhitespace}, AllowFullwidthSpace: gap == "　"}
		first := FormatWithOptions([]byte("@A\n#"+gap+"\n#LOCALSIZE 0\n"), ERB, options)
		second := FormatWithOptions(first.Output, ERB, options)
		if string(first.Output) != "@A\n    #\n    #LOCALSIZE 0\n" || !bytes.Equal(first.Output, second.Output) || len(second.Changes) != 0 {
			t.Fatalf("gap=%q first=%q second=%q changes=%+v", gap, first.Output, second.Output, second.Changes)
		}
	}
}

func TestSharpIdentifierBoundaryPreservesUnknownSuffixes(t *testing.T) {
	for _, suffix := range []string{"1", "_X", "αβ", "😀"} {
		t.Run(suffix, func(t *testing.T) {
			src := []byte("@EVENTTRAIN\n#SINGLE" + suffix + "\n#ONLY\n#SINGLE\n")
			result := FormatWithOptions(src, ERB, commandOptions())
			if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte("#SINGLE"+suffix)) {
				t.Fatalf("suffix=%q output=%q diagnostics=%+v", suffix, result.Output, result.Diagnostics)
			}
			second := FormatWithOptions(result.Output, ERB, commandOptions())
			if !bytes.Equal(result.Output, second.Output) {
				t.Fatalf("suffix=%q output changed on second pass: first=%q second=%q", suffix, result.Output, second.Output)
			}
		})
	}
}

func TestSharpIdentifierDelimitersRemainKnown(t *testing.T) {
	result := FormatWithOptions([]byte("@F\n#LOCALSIZE(1+2)\n#LOCALSIZE 3\n"), ERB, commandOptions())
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "duplicate_local_size" {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
}

func TestSharpLocalSizeSuffixRemainsOpaque(t *testing.T) {
	src := []byte("@F\n#LOCALSIZE1+2\n#LOCALSIZE 3\n")
	result := FormatWithOptions(src, ERB, commandOptions())
	if len(result.Diagnostics) != 0 || !bytes.Equal(result.Output, src) {
		t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
}

func TestSharpFullwidthDelimiterKeepsKnownAttributeState(t *testing.T) {
	src := []byte("@EVENTTRAIN\n#SINGLE　suffix\n#SINGLE\n")
	for _, allow := range []bool{false, true} {
		result := FormatWithOptions(src, ERB, Options{AllowFullwidthSpace: allow})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "duplicate_attribute" {
			t.Fatalf("allow=%t output=%q diagnostics=%+v", allow, result.Output, result.Diagnostics)
		}
	}
}
