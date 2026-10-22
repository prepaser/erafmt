package format

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestDeclarationDirectiveBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind erabasic.Kind
		src  string
		code string
		sev  erabasic.Severity
	}{
		{"bare dim", erabasic.ERB, "@F\n#DIM\nRETURN 0\n", "unsupported_syntax", erabasic.Unsupported},
		{"bare dims", erabasic.ERH, "#DIMS\n", "unsupported_syntax", erabasic.Unsupported},
		{"bare const", erabasic.ERB, "@F\n#DIM CONST\nRETURN 0\n", "unsupported_syntax", erabasic.Unsupported},
		{"bare ref", erabasic.ERB, "@F\n#DIM REF\nRETURN 0\n", "unsupported_syntax", erabasic.Unsupported},
		{"bare localsize", erabasic.ERB, "@F\n#LOCALSIZE\nRETURN 0\n", "missing_local_size", erabasic.Warning},
		{"bare localssize", erabasic.ERB, "@F\n#LOCALSSIZE\nRETURN 0\n", "missing_local_size", erabasic.Warning},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := FormatWithOptions([]byte(tc.src), Kind(tc.kind), Options{})
			if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != tc.code || r.Diagnostics[0].Severity != Severity(tc.sev) {
				t.Fatalf("diagnostics=%+v output=%q", r.Diagnostics, r.Output)
			}
		})
	}
}

func TestDuplicateGlobalModifierIsAccepted(t *testing.T) {
	r := FormatWithOptions([]byte("#DIM GLOBAL GLOBAL VALUE\n"), ERH, Options{})
	if len(r.Diagnostics) != 0 || !bytes.Equal(r.Output, []byte("#DIM GLOBAL GLOBAL VALUE\n")) {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestEventLocalSizeOnlyWarnsWhenEmpty(t *testing.T) {
	r := FormatWithOptions([]byte("@EVENTTRAIN\n#LOCALSIZE\nRETURN 0\n"), ERB, Options{})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "missing_local_size" {
		t.Fatalf("empty event directive diagnostics=%+v", r.Diagnostics)
	}
	r = FormatWithOptions([]byte("@EVENTTRAIN\n#LOCALSIZE 1+2\nRETURN 0\n"), ERB, Options{})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "event_local_size" || !bytes.Contains(r.Output, []byte("#LOCALSIZE 1+2")) {
		t.Fatalf("nonempty event directive output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestReservedDeclarationNamesRequireExactSpelling(t *testing.T) {
	reserved := FormatWithOptions([]byte("@F\n#DIM RETURN\nRETURN 0\n"), ERB, Options{})
	if len(reserved.Diagnostics) != 1 || reserved.Diagnostics[0].Code != "unsupported_syntax" {
		t.Fatalf("reserved diagnostics=%+v output=%q", reserved.Diagnostics, reserved.Output)
	}
	deferred := FormatWithOptions([]byte("@F\n#DIM return\nRETURN 0\n"), ERB, Options{})
	if len(deferred.Diagnostics) != 0 || !bytes.Contains(deferred.Output, []byte("#DIM return")) {
		t.Fatalf("case variant diagnostics=%+v output=%q", deferred.Diagnostics, deferred.Output)
	}
	base := FormatWithOptions([]byte("@F\n#DIM SETVAR\nRETURN 0\n"), ERB, Options{Dialect: Emuera1824})
	if len(base.Diagnostics) != 0 {
		t.Fatalf("base-only declaration diagnostics=%+v output=%q", base.Diagnostics, base.Output)
	}
	modern := FormatWithOptions([]byte("@F\n#DIM SETVAR\nRETURN 0\n"), ERB, Options{Dialect: EMEE})
	if len(modern.Diagnostics) != 1 || modern.Diagnostics[0].Code != "unsupported_syntax" {
		t.Fatalf("modern reserved diagnostics=%+v output=%q", modern.Diagnostics, modern.Output)
	}
}

func TestRuntimeReservedDeclarationNames(t *testing.T) {
	for _, name := range []string{"IS", "TO", "INT", "REFFUNC", "PRIVATE", "__DEBUG__", "__SKIP__", "_"} {
		for _, directive := range []string{"#DIM", "#DIMS"} {
			init := `,1+1="x","y"`
			if directive == "#DIM" {
				init = ",1+1=1,2"
			}
			src := "@F\n" + directive + " " + name + init + "\nRETURN 0\n"
			result := FormatWithOptions([]byte(src), ERB, Options{})
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_syntax" || !bytes.Contains(result.Output, []byte(directive+" "+name+init)) {
				t.Fatalf("name=%q directive=%s output=%q diagnostics=%+v", name, directive, result.Output, result.Diagnostics)
			}
		}
	}
	for _, name := range []string{"IS", "__DEBUG__", "_"} {
		for _, directive := range []string{"#DIM", "#DIMS"} {
			init := `,1+1="x","y"`
			if directive == "#DIM" {
				init = ",1+1=1,2"
			}
			src := directive + " " + name + init + "\n"
			result := FormatWithOptions([]byte(src), ERH, Options{})
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_syntax" || !bytes.Contains(result.Output, []byte(directive+" "+name+init)) {
				t.Fatalf("ERH name=%q directive=%s output=%q diagnostics=%+v", name, directive, result.Output, result.Diagnostics)
			}
		}
	}
}

func TestPrivateDeclarationsDeferBMPDigitLeadingMacroNames(t *testing.T) {
	for _, name := range []string{"１２A", "١A"} {
		for _, directive := range []string{"#DIM", "#DIMS"} {
			src := "@F\n" + directive + " " + name + ",1+2\nRETURN 1+2\n"
			result := FormatWithOptions([]byte(src), ERB, Options{})
			if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte(directive+" "+name+",1+2")) {
				t.Fatalf("directive=%s name=%q output=%q diagnostics=%+v", directive, name, result.Output, result.Diagnostics)
			}
		}
	}
	for _, name := range []string{"1A", "2"} {
		src := "@F\n#DIM " + name + ",1+2\nRETURN 1+2\n"
		result := FormatWithOptions([]byte(src), ERB, Options{})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_syntax" || !bytes.Contains(result.Output, []byte("#DIM "+name+",1+2")) {
			t.Fatalf("ASCII name=%q output=%q diagnostics=%+v", name, result.Output, result.Diagnostics)
		}
	}
	for _, name := range []string{"１２A", "١A", "𝟙A"} {
		for _, directive := range []string{"#DIM", "#DIMS"} {
			src := directive + " " + name + ",1+2\n"
			result := FormatWithOptions([]byte(src), ERH, Options{})
			if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte(directive+" "+name+", 1 + 2")) {
				t.Fatalf("ERH directive=%s name=%q output=%q diagnostics=%+v", directive, name, result.Output, result.Diagnostics)
			}
		}
	}
	result := FormatWithOptions([]byte("@F\n#DIM CONST １２A,1+2\nRETURN 0\n"), ERB, Options{})
	if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte("#DIM CONST １２A,1+2")) {
		t.Fatalf("modifier output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
}

func TestPrivateDigitLeadingDeclarationDefersUnknownContexts(t *testing.T) {
	for _, src := range []string{"@F\n[IF FLAG]\n#DIM １２A,1+2\n[ENDIF]\nRETURN 1+2\n", "@F\n? #DIM １２A,1+2\nRETURN 1+2\n"} {
		result := FormatWithOptions([]byte(src), ERB, Options{})
		if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte("#DIM １２A, 1 + 2")) && !bytes.Contains(result.Output, []byte("#DIM １２A,1+2")) {
			t.Fatalf("source=%q output=%q diagnostics=%+v", src, result.Output, result.Diagnostics)
		}
	}
}

func TestPrivateDigitLeadingContinuationDeclarationIsPreserved(t *testing.T) {
	separator := " "
	src := "@F\n{\n#DIM １２A,1+\n2\n}\nRETURN 1+2\n"
	result := FormatWithOptions([]byte(src), ERB, Options{ContinuationSeparator: &separator})
	if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte("#DIM １２A,1+\n2")) {
		t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
}

func TestPrivateDeclarationDuplicatesAreScopedToAFunction(t *testing.T) {
	r := FormatWithOptions([]byte("@F\n#DIM VALUE\n#DIM VALUE\nRETURN 0\n"), ERB, Options{})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "duplicate_declaration" || r.Diagnostics[0].Severity != Warning {
		t.Fatalf("duplicate diagnostics=%+v output=%q", r.Diagnostics, r.Output)
	}
	r = FormatWithOptions([]byte("@F\n#DIM VALUE\nRETURN 0\n@G\n#DIM VALUE\nRETURN 0\n"), ERB, Options{})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("cross-function diagnostics=%+v output=%q", r.Diagnostics, r.Output)
	}
	r = FormatWithOptions([]byte("@F\n#DIM VALUE,0\n#DIM VALUE,1\nRETURN 0\n"), ERB, Options{})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "suspicious_dimension" {
		t.Fatalf("rejected declaration established state: %+v", r.Diagnostics)
	}
}

func TestERHGlobalDeclarationDuplicatesShareDIMNamespace(t *testing.T) {
	r := FormatWithOptions([]byte("#DIM VALUE\n#DIMS VALUE\n"), ERH, Options{})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "duplicate_declaration" || r.Diagnostics[0].Severity != Warning {
		t.Fatalf("diagnostics=%+v output=%q", r.Diagnostics, r.Output)
	}
	r = FormatWithOptions([]byte("#DIM VALUE\n#DIM value\n"), ERH, Options{})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("case-dependent duplicate was diagnosed: %+v", r.Diagnostics)
	}
}

func TestRejectedOrUnresolvedDeclarationDoesNotPoisonDuplicateState(t *testing.T) {
	for _, first := range []string{
		`#DIM VALUE="x"`,
		"#DIM VALUE=UNKNOWN",
		"#DIM VALUE,UNKNOWN",
		"#DIM VALUE,1/0",
		"#DIM VALUE,2-2",
		"#DIM VALUE,1000000+1",
		"#DIM VALUE,1000+1,1000",
		"#DIM VALUE,1=RESULT",
		"#DIM VALUE,1=1/0",
		"#DIM VALUE,2+2=1,2,3,4,5",
		"#DIM CONST VALUE,2+2=1,2,3",
		"#DIM VALUE,2+2,2=1",
		`#DIMS VALUE,1="x" * -1`,
		`#DIMS VALUE,1="x" * 10000`,
		`#DIMS VALUE,1="x" * (1/0)`,
	} {
		for _, kind := range []erabasic.Kind{erabasic.ERB, erabasic.ERH} {
			src := first + "\n#DIM VALUE,1\n"
			if kind == erabasic.ERB {
				src = "@F\n" + src
			}
			r := FormatWithOptions([]byte(src), Kind(kind), Options{})
			for _, d := range r.Diagnostics {
				if d.Code == "duplicate_declaration" {
					t.Fatalf("kind=%v unaccepted declaration poisoned state for %q: %+v", kind, src, r.Diagnostics)
				}
			}
			if !bytes.Contains(r.Output, []byte("#DIM VALUE, 1\n")) {
				t.Fatalf("kind=%v later declaration was not formatted: %q", kind, r.Output)
			}
		}
	}
}

func TestProvenDeclarationValuesEstablishDuplicateState(t *testing.T) {
	for _, first := range []string{"#DIM VALUE,1+2", "#DIM VALUE,1=1+2", "#DIM CONST VALUE,1+1=1,2", "#DIM CONST VALUE=1,2", `#DIMS VALUE,1=("x")`} {
		for _, kind := range []erabasic.Kind{erabasic.ERB, erabasic.ERH} {
			src := first + "\n#DIM VALUE,1\n"
			if kind == erabasic.ERB {
				src = "@F\n" + src
			}
			r := FormatWithOptions([]byte(src), Kind(kind), Options{})
			if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "duplicate_declaration" {
				t.Fatalf("kind=%v source=%q diagnostics=%+v", kind, src, r.Diagnostics)
			}
		}
	}
	for _, first := range []string{"#DIM REF VALUE", "#DIM REF VALUE,0+0", "#DIM REF VALUE,,"} {
		r := FormatWithOptions([]byte("@F\n"+first+"\n#DIM VALUE,1\n"), ERB, Options{})
		if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "duplicate_declaration" {
			t.Fatalf("reference declaration %q diagnostics=%+v", first, r.Diagnostics)
		}
	}
}

func TestERHFunctionStyleDefineIsPreservedAsUnsupported(t *testing.T) {
	r := FormatWithOptions([]byte("#DEFINE FOO(x) 1\n#DIM VALUE\n"), ERH, Options{})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_declaration" || r.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("diagnostics=%+v output=%q", r.Diagnostics, r.Output)
	}
	for _, src := range []string{"#DEFINE FOO (x) 1\n", "#define FOO(x) 1\n", "#DEFINE [[FOO]](x) 1\n"} {
		r = FormatWithOptions([]byte(src), ERH, Options{})
		if len(r.Diagnostics) != 0 {
			t.Fatalf("uncertain define %q was diagnosed: %+v", src, r.Diagnostics)
		}
	}
}

func TestERHReferenceDeclarationPreservesFileExceptGlobalRules(t *testing.T) {
	src := []byte("#DIM REF VALUE\r\n#DIM OTHER,1+2")
	r := FormatWithOptions(src, ERH, Options{})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_declaration" || r.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("diagnostics=%+v output=%q", r.Diagnostics, r.Output)
	}
	if !bytes.Equal(r.Output, []byte("#DIM REF VALUE\n#DIM OTHER,1+2\n")) {
		t.Fatalf("output=%q", r.Output)
	}
}
