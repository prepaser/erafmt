package format

import (
	"bytes"
	"strings"
	"testing"
)

func TestValidateHeaderGrammar(t *testing.T) {
	valid := []string{
		"@F",
		"@  F",
		"@F ; note ) [",
		"@F( )",
		"@F(ARG:0, ARGS:0)",
		"@F(ARG:0,)",
		"@F(ARG:0 = 1 + 2, S = \"x\")",
		"@F(ARG=1==2)",
		"@F( A[0], F(1, 2), @\"x { A + 1 }\", \\@A?yes#no\\@ ) ; note",
		"@F(\"x;)\") ; note",
		"@F(1) ; full　width",
		"@F, ARG:0 ; note, x=2",
		"@F(\"full　width\") ; note",
		"@F;!;",
		"@F;#;",
		"@F;^;",
		"@F(ARG:0);^; RETURN 2",
		"@F;!; ; note",
		"@F(1);!;",
		"@F, ARG:0;#;",
		"@１２３",
		"@١ABC(ARG:0)",
	}
	for _, src := range valid {
		result := validateHeader([]byte(src))
		if result.status != headerValid {
			t.Errorf("validateHeader(%q) = %v %q, want valid", src, result.status, result.message)
		}
	}
}

func TestValidateHeaderRejectsMalformedBoundaries(t *testing.T) {
	invalid := []string{
		"@",
		"@123",
		"@F+BAD",
		"@F garbage",
		"@F[1",
		"@F[]",
		"@F[1]",
		"@F[1+2](ARG:0)",
		"@F(1",
		"@F(1]",
		"@F(, ARG)",
		"@F(ARG:0,,)",
		"@F(ARG:0,,ARG:1)",
		"@F(ARG=)",
		"@F(ARG==1)",
		"@F(1) garbage",
		"@F[1] garbage",
		"@F(\"unterminated)",
	}
	for _, src := range invalid {
		result := validateHeader([]byte(src))
		if result.status != headerInvalid {
			t.Errorf("validateHeader(%q) = %v %q, want invalid", src, result.status, result.message)
		}
	}
}

func TestBracketFunctionHeaderPreservesWholeFile(t *testing.T) {
	source := []byte("@GOOD\nRETURN 1+2\n@F[1+2]\nRETURN 3+4\n")
	for _, options := range []Options{{}, {Select: []RuleID{}}} {
		result := FormatWithOptions(source, ERB, options)
		if !bytes.Equal(result.Output, source) {
			t.Fatalf("options=%+v output=%q, want unchanged source", options, result.Output)
		}
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Severity != Error || result.Diagnostics[0].Code != "invalid_header" {
			t.Fatalf("options=%+v diagnostics=%+v, want one invalid_header error", options, result.Diagnostics)
		}
	}
}

func TestValidateHeaderRejectsExactEventArguments(t *testing.T) {
	for _, src := range []string{"@EVENTTRAIN()", "@EVENTTRAIN[]", "@EVENTTRAIN,", "@EVENTTRAIN(ARG:0)"} {
		result := validateHeader([]byte(src))
		if result.status != headerInvalid {
			t.Errorf("validateHeader(%q) = %v %q, want invalid", src, result.status, result.message)
		}
	}
	for _, src := range []string{"@eventtrain(ARG:0)", "@EVENT_HELPER(ARG:0)"} {
		if result := validateHeader([]byte(src)); result.status != headerValid {
			t.Errorf("validateHeader(%q) = %v %q, want valid", src, result.status, result.message)
		}
	}
}

func TestValidateHeaderSeparatesMarkerPayloads(t *testing.T) {
	for _, src := range []string{
		"@F;!; marker",
		"@F;#; PRINTL body",
	} {
		result := validateHeader([]byte(src))
		if result.status != headerUnsupported {
			t.Errorf("validateHeader(%q) = %v %q, want unsupported", src, result.status, result.message)
		}
	}
}

func TestValidateHeaderTreatsFullwidthWhitespaceAsUnsupported(t *testing.T) {
	for _, src := range []string{
		"@　F",
		"@F　(ARG:0)",
		"@F(1　+ 2)",
		"@F(1)　; note",
	} {
		result := validateHeader([]byte(src))
		if result.status != headerUnsupported {
			t.Errorf("validateHeader(%q) = %v %q, want unsupported", src, result.status, result.message)
		}
	}
}

func TestValidateHeaderSeparatesUnprovenExpressions(t *testing.T) {
	for _, src := range []string{
		"@F[[ALIAS]]",
		"@F(A + [[ALIAS]])",
	} {
		result := validateHeader([]byte(src))
		if result.status != headerUnsupported {
			t.Errorf("validateHeader(%q) = %v %q, want unsupported", src, result.status, result.message)
		}
	}
}

func TestValidateHeaderWithSessionMatchesStandaloneValidation(t *testing.T) {
	for _, src := range []string{
		"@F(ARG:0 = 1 + 2, S = \"x\")",
		"@F( A[0], @\"x { A + 1 }\", \\@A?yes#no\\@ ) ; note",
		"@F(1]",
		"@F(A + [[ALIAS]])",
	} {
		source := []byte(src)
		session := newExpressionSession(source)
		got := validateHeaderWithSession(source, session)
		want := validateHeader(source)
		if got != want {
			t.Errorf("session validation %q = %v, want %v", src, got, want)
		}
		if strings.Contains(src, "@\"") && len(session.scans) == 0 {
			t.Errorf("session validation %q did not cache FORM scanning", src)
		}
		if strings.Contains(src, "ARG") && len(session.lexed) == 0 {
			t.Errorf("session validation %q did not cache expression lexing", src)
		}
	}
}

func TestHeaderSessionAtomHonorsExplicitLimit(t *testing.T) {
	source := []byte(`@"unterminated"`)
	session := newExpressionSession(source)
	limit := len(`@"unterminated`)
	if _, _, result := scanHeaderAtomWithSession(source, 0, limit, session); result.status != headerInvalid {
		t.Fatalf("limited FORM scan status = %v %q, want invalid", result.status, result.message)
	}
	if _, _, result := scanHeaderAtomWithSession(source, 0, len(source), session); result.status != headerValid {
		t.Fatalf("whole FORM scan status = %v %q, want valid", result.status, result.message)
	}
}
