package format

import (
	"bytes"

	"testing"
)

func TestDeclarationInitializersWarnOnProvenLiteralErrors(t *testing.T) {
	input := []byte("@F\n#DIM NUM,2=\"x\"\n#DIMS TEXT,2=1\n#DIM CONST MISSING\n#DIM CONST TOTAL,2=1\n#DIM REF REFVALUE=1\n#DIM OK,2=1,2\nRETURN 0\n")
	result := FormatWithOptions(input, ERB, Options{})
	if len(result.Diagnostics) != 5 {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code != "suspicious_initializer" || diagnostic.Severity != Warning {
			t.Fatalf("diagnostics=%+v", result.Diagnostics)
		}
	}
	for _, line := range []string{"#DIM NUM,2=\"x\"", "#DIMS TEXT,2=1", "#DIM CONST MISSING", "#DIM CONST TOTAL,2=1", "#DIM REF REFVALUE=1"} {
		if !bytes.Contains(result.Output, []byte(line)) {
			t.Fatalf("output=%q missing preserved declaration %q", result.Output, line)
		}
	}
	if !bytes.Contains(result.Output, []byte("#DIM OK, 2 = 1, 2")) {
		t.Fatalf("valid declaration was not formatted: %q", result.Output)
	}
}

func TestDeclarationInitializerValidationDefersUnknownFields(t *testing.T) {
	input := []byte("@F\n#DIM NUM,UNKNOWN=\"x\"\n#DIM VALUE=UNKNOWN,\"x\"\n#DIMS TEXT,2=UNKNOWN,\"x\"\n")
	result := FormatWithOptions(input, ERB, Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
	if !bytes.Contains(result.Output, []byte("#DIM NUM, UNKNOWN = \"x\"")) || !bytes.Contains(result.Output, []byte("#DIM VALUE = UNKNOWN, \"x\"")) {
		t.Fatalf("output=%q", result.Output)
	}
}

func TestDeclarationInitializerSizeUsesConstantExpressions(t *testing.T) {
	input := []byte("@F\n#DIM CONST EXPR,1+2=1,2\n#DIMS LABELS,1+1=\"a\",\"b\",\"c\"\n#DIM CONST OK,1+2=1,2,3\n#DIM UNKNOWN,UNKNOWN=1,2\n")
	result := FormatWithOptions(input, ERB, Options{})
	if len(result.Diagnostics) != 2 {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	for n, diagnostic := range result.Diagnostics {
		if diagnostic.Code != "suspicious_initializer" || diagnostic.Severity != Warning || diagnostic.Line != n+2 {
			t.Fatalf("diagnostics=%+v", result.Diagnostics)
		}
	}
	for _, line := range []string{"#DIM CONST EXPR,1+2=1,2", "#DIMS LABELS,1+1=\"a\",\"b\",\"c\"", "#DIM UNKNOWN, UNKNOWN = 1, 2"} {
		if !bytes.Contains(result.Output, []byte(line)) {
			t.Fatalf("output=%q missing %q", result.Output, line)
		}
	}
	if !bytes.Contains(result.Output, []byte("#DIM CONST OK, 1 + 2 = 1, 2, 3")) {
		t.Fatalf("valid expression-sized initializer was not formatted: %q", result.Output)
	}
}

func TestDeclarationModifierScopeAndConflictDiagnostics(t *testing.T) {
	input := []byte("@F\n#DIM GLOBAL USER_X\n#DIM SAVEDATA USER_Y\n#DIM CHARADATA USER_Z\n#DIM REF STATIC USER_R\n#DIM DYNAMIC STATIC USER_D\n")
	result := FormatWithOptions(input, ERB, Options{})
	if len(result.Diagnostics) != 5 {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code != "unsupported_syntax" || diagnostic.Severity != Unsupported {
			t.Fatalf("diagnostics=%+v", result.Diagnostics)
		}
	}
	erh := FormatWithOptions([]byte("#DIM DYNAMIC USER_X\n#DIM STATIC USER_Y\n"), ERH, Options{})
	if len(erh.Diagnostics) != 2 || !bytes.Equal(erh.Output, []byte("#DIM DYNAMIC USER_X\n#DIM STATIC USER_Y\n")) {
		t.Fatalf("ERH result=%+v", erh)
	}
}

func TestDeclarationMacroPrefixesRemainOpaque(t *testing.T) {
	input := []byte("#DEFINE MODIFIER_A CONST\n#DIM MODIFIER_A PRIVATE_MAC_A = 7\n#DEFINE INIT_E = 7\n#DIM CONST NAME_E INIT_E\n")
	result := FormatWithOptions(input, ERH, Options{Select: []RuleID{DeclarationWordSpacing, DeclarationDefaultSpacing, CommaSpacing, BinarySpacing}})
	if !bytes.Equal(result.Output, input) || len(result.Diagnostics) != 0 {
		t.Fatalf("macro-shaped declarations were not preserved: output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
}
