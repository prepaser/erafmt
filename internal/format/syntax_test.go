package format

import (
	"bytes"
	"strings"
	"testing"
)

func TestSyntaxAssignmentPreservesPlainPayload(t *testing.T) {
	input := []byte("@F\nA=  raw  ; keep\nB+= 1+2 ; keep\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{
		AssignmentLeftSpacing, AssignmentRightSpacing, BinarySpacing, InlineCommentSpacing,
	}})
	want := []byte("@F\nA = raw  ; keep\nB += 1 + 2 ; keep\n")
	if !bytes.Equal(result.Output, want) || len(result.Diagnostics) != 0 {
		t.Fatalf("output = %q, diagnostics = %#v, want %q", result.Output, result.Diagnostics, want)
	}
}

func TestSyntaxPrintVAndFormBoundaries(t *testing.T) {
	input := []byte("@F\nPRINTV A+B,' raw  ,C+D\nPRINTFORM  raw { A+B }\nCALL F [ 1 ,2 ] ( 3 )\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{
		CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing, FormPadding,
	}})
	want := []byte("@F\nPRINTV A + B, ' raw  , C + D\nPRINTFORM  raw {A + B}\nCALL F[1, 2](3)\n")
	if !bytes.Equal(result.Output, want) || len(result.Diagnostics) != 0 {
		t.Fatalf("output = %q, diagnostics = %#v, want %q", result.Output, result.Diagnostics, want)
	}
}

func TestSyntaxNoArgAndEndDataMetadata(t *testing.T) {
	if !partial("ENDDATA") {
		t.Fatal("ENDDATA must be a partial block instruction")
	}
	input := []byte("@F\nPRINTDATA\nENDDATA  \n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{CodeTrailingWhitespace}})
	want := []byte("@F\nPRINTDATA\nENDDATA\n")
	if !bytes.Equal(result.Output, want) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "empty_data" || result.Diagnostics[0].Severity != Warning {
		t.Fatalf("output = %q, diagnostics = %#v, want %q", result.Output, result.Diagnostics, want)
	}
}

func TestSyntaxConsumedSemicolonStaysOpaque(t *testing.T) {
	input := []byte("@F\nIF A\nENDIF;comment  \n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{
		InlineCommentSpacing, CodeTrailingWhitespace,
	}})
	if !bytes.Equal(result.Output, input) || len(result.Diagnostics) != 0 {
		t.Fatalf("output = %q, diagnostics = %#v, want unchanged", result.Output, result.Diagnostics)
	}
}

func TestSyntaxNestedFormWithPaddingOnly(t *testing.T) {
	input := []byte("@F\nIF A+@\"x { A+B }\"\nENDIF\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{FormPadding}})
	want := []byte("@F\nIF A+@\"x {A+B}\"\nENDIF\n")
	if !bytes.Equal(result.Output, want) || len(result.Diagnostics) != 0 {
		t.Fatalf("output = %q, diagnostics = %#v, want %q", result.Output, result.Diagnostics, want)
	}
}

func TestSyntaxCodeEndStopsBeforeCommentTail(t *testing.T) {
	input := []byte("@F\nRETURN 1   ; keep  \n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{CodeTrailingWhitespace}})
	if !bytes.Equal(result.Output, input) || len(result.Diagnostics) != 0 {
		t.Fatalf("L002 changed comment tail: %q, diagnostics = %#v", result.Output, result.Diagnostics)
	}
	result = FormatWithOptions(input, ERB, Options{Select: []RuleID{InlineCommentSpacing}})
	want := []byte("@F\nRETURN 1 ; keep  \n")
	if !bytes.Equal(result.Output, want) || len(result.Diagnostics) != 0 {
		t.Fatalf("inline comment output = %q, diagnostics = %#v, want %q", result.Output, result.Diagnostics, want)
	}
}

func TestSyntaxRawPayloadBoundaries(t *testing.T) {
	input := []byte("@F\nPRINTL   \nPRINTFORM   \nPRINTFORM ; literal\nPRINTDATA\nDATAFORM ; literal\nENDDATA\n")
	output, diagnostics := Format(input, ERB)
	want := []byte("@F\n    PRINTL   \n    PRINTFORM   \n    PRINTFORM ; literal\n    PRINTDATA\n        DATAFORM ; literal\n    ENDDATA\n")
	if !bytes.Equal(output, want) || len(diagnostics) != 0 {
		t.Fatalf("output = %q, diagnostics = %#v, want %q", output, diagnostics, want)
	}
}

func TestAssignmentIndexAndStringExpression(t *testing.T) {
	input := []byte("@F\nA : F( 1+2 )=  literal  ; tail\nS'=\"a\"+\"b\"\nA : 0 ++ ; note\nPRINTL : raw++  \n")
	want := "@F\n    A:F(1 + 2) = literal  ; tail\n    S '= \"a\" + \"b\"\n    A:0++ ; note\n    PRINTL : raw++  \n"
	r := FormatWithOptions(input, ERB, Options{})
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output %q, diagnostics %+v", r.Output, r.Diagnostics)
	}
}

func TestFormHostsAndEmptyCallArguments(t *testing.T) {
	input := []byte("@F\nCALL F (  )\nCALLFORM target\\;name{ A+1 }( 2+3 )\nRETURNFORM { 1+2 },\\@ A+1 ?{ 3+4 }#{ 5+6 }\\@\n")
	want := "@F\n    CALL F()\n    CALLFORM target\\;name{A + 1}(2 + 3)\n    RETURNFORM {1 + 2},\\@ A + 1 ?{3 + 4}#{5 + 6}\\@\n"
	r := FormatWithOptions(input, ERB, Options{})
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output %q, diagnostics %+v", r.Output, r.Diagnostics)
	}
}

func TestDeclarationCommentRuleOwnership(t *testing.T) {
	input := []byte("@F(ARG:0) ; note  \n#DIM USR_X,1=2 ; note  \n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{CodeTrailingWhitespace}})
	if !bytes.Equal(r.Output, input) || len(r.Changes) != 0 {
		t.Fatalf("code rule changed comment: %+v", r)
	}
	r = FormatWithOptions(input, ERB, Options{Select: []RuleID{InlineCommentSpacing, CommentTrailingWhitespace}})
	want := "@F(ARG:0) ; note\n#DIM USR_X,1=2 ; note\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output %q, diagnostics %+v", r.Output, r.Diagnostics)
	}
}

func TestPrintVRawAtomsInsideExpressions(t *testing.T) {
	input := []byte("@F\nPRINTV 'first\\, literal  ,F('nested; raw  ,A+B),C+D\n")
	want := "@F\n    PRINTV 'first\\, literal  , F('nested; raw  , A + B), C + D\n"
	r := FormatWithOptions(input, ERB, Options{})
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output %q, diagnostics %+v", r.Output, r.Diagnostics)
	}
	again := FormatWithOptions(r.Output, ERB, Options{})
	if !bytes.Equal(r.Output, again.Output) || len(again.Changes) != 0 {
		t.Fatalf("not idempotent: %+v", again)
	}
}

func TestPrintVLargeRawAtomPreservesInput(t *testing.T) {
	for _, size := range []int{1 << 20, (1 << 20) + 1} {
		src := []byte("@F\nPRINTV '" + strings.Repeat("x", size) + "\n")
		result := FormatWithOptions(src, ERB, Options{Dialect: Emuera1824, Select: []RuleID{CodeTrailingWhitespace}})
		if len(result.Diagnostics) != 0 || !bytes.Equal(result.Output, src) {
			t.Fatalf("raw size=%d output=%d diagnostics=%+v", size, len(result.Output), result.Diagnostics)
		}
	}
}

func TestPrintVRawProjectionRetainsPhysicalOffsets(t *testing.T) {
	for input, want := range map[string]string{
		"@F\nPRINTV A+B,'raw\\, text  ,C+D\n": "@F\nPRINTV A + B, 'raw\\, text  , C + D\n",
		"@F\nPRINTV A    +    B\n":            "@F\nPRINTV A + B\n",
		"@F\nPRINTV 'raw',A+B\n":              "@F\nPRINTV 'raw', A + B\n",
	} {
		result := FormatWithOptions([]byte(input), ERB, Options{Select: []RuleID{BinarySpacing, CommaSpacing}})
		if !bytes.Equal(result.Output, []byte(want)) || len(result.Diagnostics) != 0 {
			t.Fatalf("output=%q diagnostics=%+v want=%q", result.Output, result.Diagnostics, want)
		}
	}
}

func TestPrintVRawProjectionMapsFormAdvisories(t *testing.T) {
	input := []byte("@F\nPRINTV 'raw', @\"x {1,-1}\"\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{FormPadding}})
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "suspicious_form_width" {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
	wantColumn := bytes.Index(input[bytes.IndexByte(input, '\n')+1:], []byte("-1")) + 1
	if result.Diagnostics[0].Column != wantColumn {
		t.Fatalf("diagnostic=%+v want column=%d", result.Diagnostics[0], wantColumn)
	}
}

func TestPrintVDisabledFullwidthSeparatorsPreserveInput(t *testing.T) {
	input := []byte("@F\nPRINTV A　　 B  +  C\n")
	options := Options{Select: []RuleID{BinarySpacing, CommaSpacing}}
	result := FormatWithOptions(input, ERB, options)
	if !bytes.Equal(result.Output, input) || len(result.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
	again := FormatWithOptions(result.Output, ERB, options)
	if !bytes.Equal(again.Output, input) || len(again.Changes) != 0 || len(again.Diagnostics) != 0 {
		t.Fatalf("second pass=%+v", again)
	}
	body := []byte("A　　B C")
	p := formatter{src: body}
	projection, _, _, ok := p.printVProjection(0, 0, len(body))
	if !ok || !bytes.Equal(projection.src, body) {
		t.Fatalf("disabled projection=%q want=%q", projection.src, body)
	}
}

func TestReservedOpaquePayloadsAreNotAssignments(t *testing.T) {
	input := []byte("@F\nPRINTDATA\nDATA : raw=payload++\nENDDATA\nPRINTLC : raw=payload++\nPRINTVN : raw=payload++\nCUSTOMDRAWLINE : raw=payload++\nREUSELASTLINE   \n")
	options := Options{Select: []RuleID{AssignmentLeftSpacing, AssignmentRightSpacing, UnarySpacing, CodeTrailingWhitespace}}
	r := FormatWithOptions(input, ERB, options)
	if !bytes.Equal(r.Output, input) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_command" || len(r.Changes) != 0 {
		t.Fatalf("opaque payload changed: %+v", r)
	}
}

func TestPrintVCommentsAndRawTail(t *testing.T) {
	input := []byte("@F\nPRINTV A+B \t; note  \nPRINTV A+B,'raw  \nPRINTV ; note  \n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{InlineCommentSpacing, CodeTrailingWhitespace, CommentTrailingWhitespace}})
	want := "@F\nPRINTV A+B ; note\nPRINTV A+B,'raw  \nPRINTV ; note\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output %q, diagnostics %+v", r.Output, r.Diagnostics)
	}
	r = FormatWithOptions([]byte("@F\nPRINTV \t ; note  \n"), ERB, Options{})
	if string(r.Output) != "@F\n    PRINTV ; note\n" || len(r.Diagnostics) != 0 {
		t.Fatalf("comment-only output %q, diagnostics %+v", r.Output, r.Diagnostics)
	}
}

func TestPrintVOmittedExpressionsAreDiagnosed(t *testing.T) {
	input := []byte("@F\nPRINTV ,A+B\nPRINTV A+B,,C\nPRINTV A+B,\nPRINTV\nPRINTV '\nPRINTV F(,A+B)\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{BinarySpacing, CommaSpacing}})
	if string(result.Output) != "@F\nPRINTV ,A+B\nPRINTV A+B,,C\nPRINTV A + B,\nPRINTV\nPRINTV '\nPRINTV F(, A + B)\n" {
		t.Fatalf("output=%q", result.Output)
	}
	if len(result.Diagnostics) != 2 {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code != "unsupported_syntax" || diagnostic.Message != "PRINTV contains an empty expression" {
			t.Fatalf("diagnostics=%+v", result.Diagnostics)
		}
	}
}

func TestInstructionIdentityAmbiguityPreservesBody(t *testing.T) {
	input := []byte("@F\nPRINTFORM=raw { A+B }\nprintv = raw+A\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{AssignmentLeftSpacing, AssignmentRightSpacing, BinarySpacing, FormPadding}})
	if !bytes.Equal(r.Output, input) || len(r.Changes) != 0 {
		t.Fatalf("instruction identity changed: %+v", r)
	}
	r = FormatWithOptions([]byte("@F\nvalue=  raw+A\n"), ERB, Options{Select: []RuleID{AssignmentLeftSpacing, AssignmentRightSpacing}})
	if string(r.Output) != "@F\nvalue = raw+A\n" {
		t.Fatalf("lowercase assignment output %q", r.Output)
	}

	input = []byte("@BAD\r\nIF　USR_X\r\nRETURN 0\r\nENDIF\r\n@GOOD\r\nRETURN 1\r\n")
	r = FormatWithOptions(input, ERB, Options{})
	want := "@BAD\nIF　USR_X\nRETURN 0\nENDIF\n@GOOD\n    RETURN 1\n"
	if string(r.Output) != want || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("output %q, diagnostics %+v", r.Output, r.Diagnostics)
	}
}
