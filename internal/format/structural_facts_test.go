package format

import (
	"strings"
	"testing"
)

func TestContinuedStructuralState(t *testing.T) {
	separator := " "
	for _, tc := range []struct {
		name string
		src  string
		code string
	}{
		{"function", "@F\n{\n#FUNCTIONS\n}\nRETURNF 1\n", "#FUNCTIONS requires a string RETURNF expression"},
		{"dimension", "@F\n{\n#DIM\nUSR_X\n}\n#DIM USR_X\n", "duplicate_declaration"},
		{"event attribute", "@EVENTTRAIN\n{\n#SINGLE\n}\n#SINGLE\n", "duplicate_attribute"},
		{"event local size", "@EVENTTRAIN\n{\n#LOCALSIZE\n1\n}\n", "event_local_size"},
		{"local size", "@F\n{\n#LOCALSIZE\n1\n}\n#LOCALSIZE 2\n", "duplicate_local_size"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := FormatWithOptions([]byte(tc.src), ERB, Options{ContinuationSeparator: &separator})
			for _, d := range r.Diagnostics {
				if d.Code == tc.code || d.Message == tc.code {
					return
				}
			}
			t.Fatalf("diagnostics=%+v", r.Diagnostics)
		})
	}
}

func TestContinuedSystemHeaderContext(t *testing.T) {
	separator := " "
	r := FormatWithOptions([]byte("{\n@EVENTTRAIN\n}\n#FUNCTION\n"), ERB, Options{ContinuationSeparator: &separator})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_declaration" {
		t.Fatalf("diagnostics=%+v", r.Diagnostics)
	}
}

func TestBranchMetadata(t *testing.T) {
	separator := " "
	for _, tc := range []struct {
		name string
		src  string
		code string
	}{
		{"selector", "@F\n[IF X]\nSELECTCASE \"x\"\n[ELSE]\nSELECTCASE \"y\"\n[ENDIF]\nCASE 1\nENDSELECT\n", "CASE has an incompatible type"},
		{"continued selector", "@F\n[IF X]\n{\nSELECTCASE\n\"x\"\n}\n[ELSE]\n{\nSELECTCASE\n\"y\"\n}\n[ENDIF]\nCASE 1\nENDSELECT\n", "CASE has an incompatible type"},
		{"repeat", "@F\n[IF X]\nREPEAT 1\n[ELSE]\nREPEAT 2\n[ENDIF]\nREPEAT 3\n", "nested_repeat"},
		{"data", "@F\n[IF X]\nPRINTDATA\n[ELSE]\nPRINTDATA\n[ENDIF]\nENDDATA\n", "empty_data"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := FormatWithOptions([]byte(tc.src), ERB, Options{ContinuationSeparator: &separator})
			for _, d := range r.Diagnostics {
				if d.Code == tc.code || strings.Contains(d.Message, tc.code) {
					return
				}
			}
			t.Fatalf("diagnostics=%+v", r.Diagnostics)
		})
	}
}

func TestSIFMergeUsesCommonOrigin(t *testing.T) {
	src := "@F\n[IF X]\nSIF A\n[ELSE]\nSIF B\n[ENDIF]\n; comment\n"
	r := FormatWithOptions([]byte(src), ERB, Options{})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "invalid_structure" || r.Diagnostics[0].Line != 6 {
		t.Fatalf("diagnostics=%+v", r.Diagnostics)
	}
}
