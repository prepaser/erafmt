package format

import (
	"bytes"

	"testing"
)

func TestLocalSizeInvalidConstants(t *testing.T) {
	for _, value := range []string{"1/0", "-0x8000000000000000/-1", "\"x\"", "\"x\"*2", "1,2", "1=2", "1?2#1/0", "0&&1/0"} {
		src := []byte("@F\n#LOCALSIZE " + value + "\nRETURN 0\n")
		result := FormatWithOptions(src, ERB, Options{})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "suspicious_local_size" || result.Diagnostics[0].Severity != Warning {
			t.Fatalf("value=%q diagnostics=%+v", value, result.Diagnostics)
		}
		if !bytes.Contains(result.Output, []byte("#LOCALSIZE "+value+"\n")) {
			t.Fatalf("invalid value changed: %q", result.Output)
		}
	}
}

func TestLocalSizeConstantProofDefersUnsupportedEvaluation(t *testing.T) {
	for _, value := range []string{"UNKNOWN", "[[SIZE]]", "\"x\"==\"x\"", "\"x\"!=\"y\"", "1?2#3", "(1<<64)+2"} {
		src := []byte("@F\n#LOCALSIZE " + value + "\nRETURN 0\n")
		result := FormatWithOptions(src, ERB, Options{})
		for _, d := range result.Diagnostics {
			if d.Code == "suspicious_local_size" {
				t.Fatalf("value=%q diagnostics=%+v", value, result.Diagnostics)
			}
		}
	}
	for _, src := range []string{"@F\n[IF X]\n#LOCALSIZE 1/0\n[ENDIF]\nRETURN 0\n"} {
		sep := " "
		result := FormatWithOptions([]byte(src), ERB, Options{ContinuationSeparator: &sep})
		for _, d := range result.Diagnostics {
			if d.Code == "suspicious_local_size" {
				t.Fatalf("source=%q diagnostics=%+v", src, result.Diagnostics)
			}
		}
	}
	sep := " "
	result := FormatWithOptions([]byte("@F\n{\n#LOCALSIZE 1/0\n}\nRETURN 0\n"), ERB, Options{ContinuationSeparator: &sep})
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "suspicious_local_size" {
		t.Fatalf("continued invalid constant diagnostics=%+v", result.Diagnostics)
	}
	result = FormatWithOptions([]byte("@EVENTTRAIN\n#LOCALSIZE 1/0\nRETURN 0\n"), ERB, Options{})
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "event_local_size" {
		t.Fatalf("event diagnostics=%+v", result.Diagnostics)
	}
	result = FormatWithOptions([]byte("@F\n#LOCALSIZE 1/0\nRETURN 0\n"), ERB, Options{Select: []RuleID{}})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("select-none diagnostics=%+v", result.Diagnostics)
	}
}
