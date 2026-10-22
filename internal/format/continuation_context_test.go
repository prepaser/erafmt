package format

import (
	"bytes"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestContinuationTrustedContext(t *testing.T) {
	for _, tc := range []struct {
		src, msg string
		line     int
	}{
		{"@F\n#FUNCTIONS\n{\nRETURNF 1+2\n}\n", "#FUNCTIONS requires a string RETURNF expression", 4},
		{"@F\n#FUNCTION\n{\nRETURNF \"x\"\n}\n", "#FUNCTION requires a numeric RETURNF expression", 4},
		{"@F\n#FUNCTION\n{\nRETURN 1\n}\n", "RETURN cannot be used in #FUNCTION", 4},
		{"@F\n#FUNCTIONS\n{\nRETURNFORM text\n}\n", "RETURNFORM cannot be used in #FUNCTIONS", 4},
		{"@F\nSELECTCASE \"x\"\n{\nCASE 1\n}\nENDSELECT\n", "CASE has an incompatible type", 4},
		{"@F\n{\nSELECTCASE \"x\"\n}\nCASE 1\nENDSELECT\n", "CASE has an incompatible type", 5},
		{"@F\nTRYGOTOLIST\n{\nFUNC TARGET\n(1)\n}\nENDFUNC\n", "TRYGOTOLIST FUNC cannot have subnames or arguments", 5},
	} {
		for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
			for _, sep := range []string{"", " ", "\t"} {
				options := Options{Dialect: Dialect(dialect), ContinuationSeparator: &sep}
				result := FormatWithOptions([]byte(tc.src), ERB, options)
				if len(result.Diagnostics) != 1 || !strings.Contains(result.Diagnostics[0].Message, tc.msg) || result.Diagnostics[0].Line != tc.line {
					t.Fatalf("source=%q dialect=%v separator=%q diagnostics=%+v", tc.src, dialect, sep, result.Diagnostics)
				}
				again := FormatWithOptions(result.Output, ERB, options)
				if !bytes.Equal(again.Output, result.Output) || len(again.Changes) != 0 {
					t.Fatalf("unstable source=%q separator=%q", tc.src, sep)
				}
			}
		}
	}
}

func TestContinuationContextDefersUncertainty(t *testing.T) {
	sep := " "
	for _, src := range []string{
		"@F\n#FUNCTIONS\n{\nRETURNF \"x\"\n}\n",
		"@F\n#FUNCTIONS\n{\nRETURNF UNKNOWN\n}\n",
		"@F\n#FUNCTIONS\n[IF X]\n{\nRETURNF 1+2\n}\n[ENDIF]\n",
		"@F\n#FUNCTIONS\n{\nRETURNF ;!; 1+2\n}\n",
		"@F\nSELECTCASE UNKNOWN\n{\nCASE 1\n}\nENDSELECT\n",
		"@F\nSELECTCASE \"x\"\n[IF X]\n{\nCASE 1\n}\n[ENDIF]\nENDSELECT\n",
		"@F\nTRYGOTOLIST\n{\nFUNC TARGET(UNKNOWN)\n}\nENDFUNC\n",
	} {
		result := FormatWithOptions([]byte(src), ERB, Options{ContinuationSeparator: &sep})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%+v", src, result.Diagnostics)
		}
	}
	result := FormatWithOptions([]byte("@F\n[IF X]\n{\nSELECTCASE \"x\"\n}\n[ELSE]\n{\nSELECTCASE \"x\"\n}\n[ENDIF]\nCASE 1\nENDSELECT\n"), ERB, Options{ContinuationSeparator: &sep})
	if len(result.Diagnostics) != 1 || !strings.Contains(result.Diagnostics[0].Message, "CASE has an incompatible type") {
		t.Fatalf("agreeing selector diagnostics=%+v", result.Diagnostics)
	}
	src := []byte("@F\n#FUNCTIONS\n{\nRETURNF 1+2\n}\n")
	for _, options := range []Options{{}, {Select: []RuleID{}, ContinuationSeparator: &sep}, {Select: []RuleID{ContinuationIndent}, ContinuationSeparator: &sep}} {
		result := FormatWithOptions(src, ERB, options)
		if len(result.Diagnostics) != 0 {
			t.Fatalf("options=%+v diagnostics=%+v", options, result.Diagnostics)
		}
	}
}

func TestContinuationUnsupportedDeclarationPreservesFunction(t *testing.T) {
	separator := " "
	src := []byte("@１２３\n{\n#FUNCTION\n}\nRETURNF 1\n@G\nRETURN 0\n")
	result := FormatWithOptions(src, ERB, Options{ContinuationSeparator: &separator})
	if !bytes.HasPrefix(result.Output, []byte("@１２３\n{\n#FUNCTION\n}\nRETURNF 1\n")) {
		t.Fatalf("continued function was formatted: output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_declaration" || result.Diagnostics[0].Line != 3 {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
}

func TestContinuationWarningKeepsDeclarationContext(t *testing.T) {
	separator := " "
	for _, tc := range []struct {
		name string
		src  string
		code string
	}{
		{name: "duplicate dimension", src: "@F\n#FUNCTION\n#DIM USR_X\n{\n#DIM USR_X\n}\n#DIM USR_X\nRETURN 0\n", code: "duplicate_declaration"},
		{name: "suspicious dimension", src: "@F\n#FUNCTION\n{\n#DIM USR_X,0\n}\nRETURN 0\n", code: "suspicious_dimension"},
		{name: "suspicious initializer", src: "@F\n#FUNCTION\n{\n#DIM USR_X=\n}\nRETURN 0\n", code: "suspicious_initializer"},
		{name: "suspicious local size", src: "@F\n#FUNCTION\n{\n#LOCALSIZE 1/0\n}\nRETURN 0\n", code: "suspicious_local_size"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := FormatWithOptions([]byte(tc.src), ERB, Options{ContinuationSeparator: &separator})
			warning := false
			returnDiagnostic := false
			for _, diagnostic := range result.Diagnostics {
				warning = warning || diagnostic.Code == tc.code
				returnDiagnostic = returnDiagnostic || strings.Contains(diagnostic.Message, "RETURN cannot be used in #FUNCTION")
			}
			if !warning || !returnDiagnostic {
				t.Fatalf("diagnostics=%+v", result.Diagnostics)
			}
		})
	}
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"unknown declaration", "@F\n#FUNCTION\n{\n#UNKNOWN\n}\nRETURN 0\n"},
		{"aliased declaration", "@F\n#FUNCTION\n{\n#DIM [[USR_X]]\n}\nRETURN 0\n"},
		{"conditional declaration", "@F\n#FUNCTION\n[IF X]\n{\n#DIM USR_X,0\n}\n[ENDIF]\nRETURN 0\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := FormatWithOptions([]byte(tc.src), ERB, Options{ContinuationSeparator: &separator})
			for _, diagnostic := range result.Diagnostics {
				if strings.Contains(diagnostic.Message, "RETURN cannot be used in #FUNCTION") {
					t.Fatalf("uncertain declaration committed sharp context: diagnostics=%+v", result.Diagnostics)
				}
			}
		})
	}
}
