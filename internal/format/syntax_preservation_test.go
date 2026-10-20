package format

import (
	"bytes"
	"testing"
)

func TestMalformedReturnCommaAssignmentStaysOpaque(t *testing.T) {
	for _, source := range []string{
		"@F\nRETURN A0,0=\n",
		"@F\nRETURN A0,=\n",
		"@F\nRETURN UNKNOWN,A=1\n",
		"@F\nRETURN UNKNOWN,=1\n",
	} {
		result := FormatWithOptions([]byte(source), ERB, Options{})
		want := bytes.Replace([]byte(source), []byte("\nRETURN"), []byte("\n    RETURN"), 1)
		if !bytes.Equal(result.Output, want) {
			t.Fatalf("source=%q output=%q diagnostics=%+v", source, result.Output, result.Diagnostics)
		}
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%+v", source, result.Diagnostics)
		}
	}
}

func TestMalformedSetterQuotedOperandsStayOpaque(t *testing.T) {
	for _, source := range []string{
		"@F\nVARSET a,\"\"A\"\"\n",
		"@F\nVARSET A0,\"\"A\"\"\n",
	} {
		result := FormatWithOptions([]byte(source), ERB, Options{})
		want := bytes.Replace([]byte(source), []byte("\nVARSET"), []byte("\n    VARSET"), 1)
		if !bytes.Equal(result.Output, want) {
			t.Fatalf("source=%q output=%q diagnostics=%+v", source, result.Output, result.Diagnostics)
		}
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%+v", source, result.Diagnostics)
		}
	}
}

func TestProtectedMetadataTailsStayOpaque(t *testing.T) {
	for _, source := range []string{
		"@A\n#00000000;;00\n",
		"@F\n$A00000;;;0\n",
	} {
		result := FormatWithOptions([]byte(source), ERB, Options{})
		want := bytes.Replace([]byte(source), []byte("\n#"), []byte("\n    #"), 1)
		want = bytes.Replace(want, []byte("\n$"), []byte("\n    $"), 1)
		if !bytes.Equal(result.Output, want) {
			t.Fatalf("source=%q output=%q diagnostics=%+v", source, result.Output, result.Diagnostics)
		}
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%+v", source, result.Diagnostics)
		}
	}
}

func TestMalformedTrailingHashStaysOpaque(t *testing.T) {
	source := "@F\nRETURN _0#\n"
	result := FormatWithOptions([]byte(source), ERB, Options{})
	want := "@F\n    RETURN _0#\n"
	if !bytes.Equal(result.Output, []byte(want)) {
		t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
}

func TestProtectedSyntaxRetainsProvenDiagnostics(t *testing.T) {
	for _, body := range []string{"#DIM X;;metadata", "RETURN 1,0=", "RETURN 1#"} {
		result := FormatWithOptions([]byte("@F\n"+body+"\n"), ERB, Options{})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("body=%q diagnostics=%+v", body, result.Diagnostics)
		}
	}
}
