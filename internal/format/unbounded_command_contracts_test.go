package format

import (
	"bytes"
	"testing"
)

func TestCharacterListCommandsValidateArgumentsBeyondLegacyLimit(t *testing.T) {
	commands := []string{"ADDCHARA", "ADDSPCHARA", "ADDCOPYCHARA", "PICKUPCHARA", "DELCHARA"}
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing}}
	for _, command := range commands {
		src := []byte("@F\n" + command + " " + commandNumericFields(256) + "\n")
		r := FormatWithOptions(src, ERB, options)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("%s valid tail: diagnostics=%+v output=%q", command, r.Diagnostics, r.Output)
		}
		invalid := []byte("@F\n" + command + " " + commandNumericFields(255) + ",1+(\n")
		r = FormatWithOptions(invalid, ERB, options)
		if !bytes.Equal(r.Output, invalid) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%s invalid tail: output=%q diagnostics=%+v", command, r.Output, r.Diagnostics)
		}
	}
	validSave := []byte("@F\nSAVECHARA " + saveCharaFields(256, false) + "\n")
	r := FormatWithOptions(validSave, ERB, options)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("SAVECHARA valid tail: diagnostics=%+v output=%q", r.Diagnostics, r.Output)
	}
	invalidSave := []byte("@F\nSAVECHARA " + saveCharaFields(256, true) + "\n")
	r = FormatWithOptions(invalidSave, ERB, options)
	if !bytes.Equal(r.Output, invalidSave) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("SAVECHARA invalid tail: output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestRegisteredFixedArgumentCommandsKeepSurplusHandling(t *testing.T) {
	src := []byte("@F\nSETCOLOR " + commandNumericFields(4) + "\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing}})
	if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "ignored_arguments" || r.Diagnostics[0].Severity != Warning {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestDTColumnOptionsAcceptsOneTerminalComma(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing}, Dialect: EMEE}
	valid := []byte("@F\nDT_COLUMN_OPTIONS DT,COL,DEFAULT,1+2,\n")
	r := FormatWithOptions(valid, ERB, options)
	if len(r.Diagnostics) != 0 || !bytes.Contains(r.Output, []byte("DT_COLUMN_OPTIONS DT, COL, DEFAULT, 1 + 2,")) {
		t.Fatalf("valid output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	for _, body := range []string{"DT_COLUMN_OPTIONS DT,COL,DEFAULT,", "DT_COLUMN_OPTIONS DT,COL,DEFAULT,,", "DT_COLUMN_OPTIONS DT,COL,,1,", "DT_COLUMN_OPTIONS DT,COL,DEFAULT,1,,"} {
		src := []byte("@F\n" + body + "\n")
		r := FormatWithOptions(src, ERB, options)
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q output=%q diagnostics=%+v", body, r.Output, r.Diagnostics)
		}
	}
}
