package erabasic

import (
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
		if result.Status != headerValid {
			t.Errorf("validateHeader(%q) = %v %q, want valid", src, result.Status, result.Message)
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
		if result.Status != headerInvalid {
			t.Errorf("validateHeader(%q) = %v %q, want invalid", src, result.Status, result.Message)
		}
	}
}

func TestValidateHeaderRejectsExactEventArguments(t *testing.T) {
	for _, src := range []string{"@EVENTTRAIN()", "@EVENTTRAIN[]", "@EVENTTRAIN,", "@EVENTTRAIN(ARG:0)"} {
		result := validateHeader([]byte(src))
		if result.Status != headerInvalid {
			t.Errorf("validateHeader(%q) = %v %q, want invalid", src, result.Status, result.Message)
		}
	}
	for _, src := range []string{"@eventtrain(ARG:0)", "@EVENT_HELPER(ARG:0)"} {
		if result := validateHeader([]byte(src)); result.Status != headerValid {
			t.Errorf("validateHeader(%q) = %v %q, want valid", src, result.Status, result.Message)
		}
	}
}

func TestValidateHeaderSeparatesMarkerPayloads(t *testing.T) {
	for _, src := range []string{"@F;!; marker", "@F;#; PRINTL body"} {
		result := validateHeader([]byte(src))
		if result.Status != headerUnsupported {
			t.Errorf("validateHeader(%q) = %v %q, want unsupported", src, result.Status, result.Message)
		}
	}
}

func TestValidateHeaderTreatsFullwidthWhitespaceAsUnsupported(t *testing.T) {
	for _, src := range []string{"@　F", "@F　(ARG:0)", "@F(1　+ 2)", "@F(1)　; note"} {
		result := validateHeader([]byte(src))
		if result.Status != headerUnsupported {
			t.Errorf("validateHeader(%q) = %v %q, want unsupported", src, result.Status, result.Message)
		}
	}
}

func TestValidateHeaderSeparatesUnprovenExpressions(t *testing.T) {
	for _, src := range []string{"@F[[ALIAS]]", "@F(A + [[ALIAS]])"} {
		result := validateHeader([]byte(src))
		if result.Status != headerUnsupported {
			t.Errorf("validateHeader(%q) = %v %q, want unsupported", src, result.Status, result.Message)
		}
	}
}

func TestValidateHeaderWithSessionMatchesStandaloneValidation(t *testing.T) {
	for _, src := range []string{"@F(ARG:0 = 1 + 2, S = \"x\")", "@F( A[0], @\"x { A + 1 }\", \\@A?yes#no\\@ ) ; note", "@F(1]", "@F(A + [[ALIAS]])"} {
		source := []byte(src)
		session := NewExpressionSession(source)
		got := validateHeaderWithSession(source, session)
		want := validateHeader(source)
		if got != want {
			t.Errorf("session validation %q = %v, want %v", src, got, want)
		}
		if strings.Contains(src, "@\"") && len(session.Scans) == 0 {
			t.Errorf("session validation %q did not cache FORM scanning", src)
		}
		if strings.Contains(src, "ARG") && len(session.Lexed) == 0 {
			t.Errorf("session validation %q did not cache expression lexing", src)
		}
	}
}

func TestHeaderSessionAtomHonorsExplicitLimit(t *testing.T) {
	source := []byte(`@"unterminated"`)
	session := NewExpressionSession(source)
	limit := len(`@"unterminated`)
	if _, _, result := scanHeaderAtomWithSession(source, 0, limit, session); result.Status != headerInvalid {
		t.Fatalf("limited FORM scan status = %v %q, want invalid", result.Status, result.Message)
	}
	if _, _, result := scanHeaderAtomWithSession(source, 0, len(source), session); result.Status != headerValid {
		t.Fatalf("whole FORM scan status = %v %q, want valid", result.Status, result.Message)
	}
}
