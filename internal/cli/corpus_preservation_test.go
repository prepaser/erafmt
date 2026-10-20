package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestCheckSourcePreservedAllowsLayoutTrivia(t *testing.T) {
	original := []byte("@F\r\nIF A+B\r\nPRINTV  raw  \r\n{\r\n  raw   \r\n}\r\n;note  \r\n\r\n")
	formatted := []byte("@F\n    IF A + B\n        PRINTV  raw  \n    {\n  raw   \n        }\n; note\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() error = %v", err)
	}
}

func TestCheckSourcePreservedAllowsFormConditionalSpacing(t *testing.T) {
	original := []byte("@F\nPRINTFORM before \\@ A + B ? yes # no \\@ after\n")
	formatted := []byte("@F\nPRINTFORM before \\@ A+B? yes # no \\@ after\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() error = %v", err)
	}
	original = []byte("@F\nPRINTFORM \\@ A+B ? \"literal ( # in branch\" # result \\@\n")
	formatted = []byte("@F\nPRINTFORM \\@ A + B ? \"literal ( # in branch\" # result \\@\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() literal branch error = %v", err)
	}
}

func TestCheckSourcePreservedRejectsPayloadChanges(t *testing.T) {
	tests := []struct {
		name, original, formatted string
	}{
		{"printv raw text", "@F\nPRINTV raw text\n", "@F\nPRINTV changed text\n"},
		{"form literal", "@F\nPRINTFORM literal  { A + 1 }  tail\n", "@F\nPRINTFORM changed  {A+1}  tail\n"},
		{"quoted expression", "@F\nIF A + \"quoted text\"\n", "@F\nIF A+\"quoted  text\"\n"},
		{"macro body", "@F\nIF [[A + B]]\n", "@F\nIF [[A+B]]\n"},
		{"assignment rhs", "@F\nA = \"raw  value\"\n", "@F\nA = \"raw value\"\n"},
		{"assignment rhs tail", "@F\nA = raw  \n", "@F\nA = raw\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkSourcePreserved([]byte(tt.original), []byte(tt.formatted)); err == nil {
				t.Fatal("checkSourcePreserved() accepted a payload change")
			}
		})
	}
}

func TestCheckSourcePreservedAllowsAssignmentSpacing(t *testing.T) {
	original := []byte("@F\nA= \"raw value\"\n")
	formatted := []byte("@F\nA = \"raw value\"\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() error = %v", err)
	}
}

func TestCheckSourcePreservedAllowsIndexedAndUnicodeAssignments(t *testing.T) {
	original := []byte("@F\nLOCAL:(OFFSET+LOCAL:20) = TEQUIP:ARG:OFFSET\nαβ:(IDX+1)=\"raw\"\n")
	formatted := []byte("@F\nLOCAL : ( OFFSET + LOCAL : 20 ) = TEQUIP:ARG:OFFSET\nαβ : ( IDX + 1 ) =\"raw\"\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() error = %v", err)
	}
	numeric := []byte("@F\nFLAG:2X2 = 1\n")
	numericFormatted := []byte("@F\nFLAG:2 X2 = 1\n")
	if err := checkSourcePreserved(numeric, numericFormatted); err != nil {
		t.Fatalf("checkSourcePreserved() numeric boundary error = %v", err)
	}
	ternary := []byte("@F\nVALUE:(A == B:C?D#E) = 0\n")
	ternaryFormatted := []byte("@F\nVALUE:(A == B:C ? D # E) = 0\n")
	if err := checkSourcePreserved(ternary, ternaryFormatted); err != nil {
		t.Fatalf("checkSourcePreserved() ternary LHS error = %v", err)
	}
}

func TestCheckSourcePreservedAllowsIncrementSpacing(t *testing.T) {
	original := []byte("@F\n++LOCAL:(IDX+1)\nCOUNT:IDX-- ; keep  \n")
	formatted := []byte("@F\n++ LOCAL : ( IDX + 1 )\nCOUNT : IDX --  ; keep\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() error = %v", err)
	}
}

func TestCheckSourcePreservedAllowsCompoundAssignmentExpressionSpacing(t *testing.T) {
	original := []byte("@F\nA += B + \"raw text\"\n")
	formatted := []byte("@F\nA += B+\"raw text\"\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() error = %v", err)
	}
}

func TestCheckSourcePreservedAllowsCommandSeparatorSpacing(t *testing.T) {
	original := []byte("@F\nPRINTV   raw text\n")
	formatted := []byte("@F\nPRINTV raw text\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() error = %v", err)
	}
	if err := checkSourcePreserved([]byte("@F\nPRINTV \n"), []byte("@F\nPRINTV\n")); err != nil {
		t.Fatalf("checkSourcePreserved() empty PRINTV error = %v", err)
	}
	raw := []byte("@F\nPRINTV 'raw  '\n")
	changed := []byte("@F\nPRINTV 'raw'\n")
	if err := checkSourcePreserved(raw, changed); err == nil {
		t.Fatal("checkSourcePreserved() accepted PRINTV raw trailing payload change")
	}
}

func TestCheckSourcePreservedRejectsFormCommandSeparatorChange(t *testing.T) {
	original := []byte("@F\nPRINTFORM   literal\n")
	formatted := []byte("@F\nPRINTFORM literal\n")
	if err := checkSourcePreserved(original, formatted); err == nil {
		t.Fatal("checkSourcePreserved() accepted a FORM command separator change")
	}
}

func TestCheckSourcePreservedFormPayloadIsNotAssignment(t *testing.T) {
	original := []byte("@F\nPRINTFORML =%CENTER(F(A,B),42)%=\n")
	r := format.FormatWithOptions(original, format.ERB, format.Options{})
	if len(r.Diagnostics) != 0 || bytes.Equal(original, r.Output) {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	if err := checkSourcePreserved(original, r.Output); err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(r.Output, []byte("%=\n"), []byte("%X\n"), 1)
	if checkSourcePreserved(original, changed) == nil {
		t.Fatal("FORM literal mutation accepted")
	}
}

func TestCheckSourcePreservedAllowsFormCallArgumentSpacing(t *testing.T) {
	original := []byte("@F\nTRYCALLFORM PUNISHMENT_{LOCAL:1}(ARG,ARG:1)\n")
	formatted := []byte("@F\nTRYCALLFORM PUNISHMENT_{LOCAL:1}(ARG, ARG:1)\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() error = %v", err)
	}
	changedTarget := []byte("@F\nTRYCALLFORM PUNISHMENT_{LOCAL:2}(ARG, ARG:1)\n")
	if err := checkSourcePreserved(original, changedTarget); err == nil {
		t.Fatal("checkSourcePreserved() accepted a FORM target change")
	}
	changedLiteral := []byte("@F\nTRYCALLFORM PUNISHMENT_{LOCAL:1}(ARG, CHANGED)\n")
	if err := checkSourcePreserved(original, changedLiteral); err == nil {
		t.Fatal("checkSourcePreserved() accepted a call argument change")
	}
	funcOriginal := []byte("@F\nFUNC PUNISHMENT_{LOCAL:1}(ARG,ARG:1)\n")
	funcFormatted := []byte("@F\nFUNC PUNISHMENT_{LOCAL:1}(ARG, ARG:1)\n")
	if err := checkSourcePreserved(funcOriginal, funcFormatted); err != nil {
		t.Fatalf("checkSourcePreserved() FUNC error = %v", err)
	}
}

func TestCheckSourcePreservedAllowsNoArgTailWhitespace(t *testing.T) {
	original := []byte("@F\nDRAWLINE \nDRAWLINE ; keep  \n")
	formatted := []byte("@F\nDRAWLINE\nDRAWLINE ; keep\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() error = %v", err)
	}
	changed := []byte("@F\nDRAWLINE argument\nDRAWLINE ; keep\n")
	if err := checkSourcePreserved(original, changed); err == nil {
		t.Fatal("checkSourcePreserved() accepted a no-argument command payload")
	}
	if err := checkSourcePreserved([]byte("@F\nREUSELASTLINE   \n"), []byte("@F\nREUSELASTLINE\n")); err == nil {
		t.Fatal("checkSourcePreserved() treated RAW REUSELASTLINE as a VOID command")
	}
	if err := checkSourcePreserved([]byte("@F\nLOOP A+B\n"), []byte("@F\nLOOP A + B\n")); err != nil {
		t.Fatalf("checkSourcePreserved() LOOP expression error = %v", err)
	}
}

func TestCheckSourcePreservedOutputLogDialectRoles(t *testing.T) {
	original := []byte("@F\n    OUTPUTLOG @\"payload\"\n")
	formatted := []byte("@F\nOUTPUTLOG @\"payload\"\n")
	mutated := []byte("@F\nOUTPUTLOG @\"changed\"\n")
	for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
		if err := checkSourcePreservedDialect(original, formatted, dialect); err != nil {
			t.Fatalf("%v accepted formatting changes: %v", dialect, err)
		}
		if err := checkSourcePreservedDialect(original, mutated, dialect); err == nil {
			t.Fatalf("%v accepted OUTPUTLOG payload mutation", dialect)
		}
	}
	for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824} {
		if !isNoArgCommandDialect("OUTPUTLOG", dialect) {
			t.Fatalf("%v did not classify OUTPUTLOG as no-argument", dialect)
		}
	}
	if isNoArgCommandDialect("OUTPUTLOG", format.EMEE) {
		t.Fatal("EMEE classified OUTPUTLOG as no-argument")
	}
}

func TestCheckSourcePreservedReuseLastLineForm(t *testing.T) {
	source := []byte("@F\nREUSELASTLINE  text { A+1 } ; literal\x00  \nREUSELASTLINE\nREUSELASTLINE   \n")
	result := format.FormatWithOptions(source, format.ERB, format.Options{})
	if bytes.Equal(source, result.Output) {
		t.Fatal("formatter did not change the REUSELASTLINE FORM expression")
	}
	if err := checkSourcePreserved(source, result.Output); err != nil {
		t.Fatalf("checkSourcePreserved() rejected formatter changes: %v", err)
	}

	mutations := []struct {
		name string
		out  []byte
	}{
		{
			name: "command separator padding",
			out:  []byte("@F\nREUSELASTLINE text { A+1 } ; literal\x00  \nREUSELASTLINE\nREUSELASTLINE   \n"),
		},
		{
			name: "literal payload",
			out:  []byte("@F\nREUSELASTLINE  text { A+1 } ; changed\x00  \nREUSELASTLINE\nREUSELASTLINE   \n"),
		},
		{
			name: "semicolon payload",
			out:  []byte("@F\nREUSELASTLINE  text { A+1 } ;literal\x00  \nREUSELASTLINE\nREUSELASTLINE   \n"),
		},
		{
			name: "nul payload",
			out:  []byte("@F\nREUSELASTLINE  text { A+1 } ; literal\x01  \nREUSELASTLINE\nREUSELASTLINE   \n"),
		},
		{
			name: "nullable padding",
			out:  []byte("@F\nREUSELASTLINE  text { A+1 } ; literal\x00  \nREUSELASTLINE\nREUSELASTLINE\n"),
		},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			if err := checkSourcePreserved(source, mutation.out); err == nil {
				t.Fatal("checkSourcePreserved() accepted a REUSELASTLINE payload mutation")
			}
		})
	}
}

func TestCheckSourcePreservedDistinguishesConsumedSemicolon(t *testing.T) {
	original := []byte("@F\nIF;A\n")
	formatted := []byte("@F\nIF ;A\n")
	if err := checkSourcePreserved(original, formatted); err == nil {
		t.Fatal("checkSourcePreserved() collapsed consumed semicolon spacing")
	}
	commentOriginal := []byte("@F\n;comment  \n")
	commentFormatted := []byte("@F\n; comment\n")
	if err := checkSourcePreserved(commentOriginal, commentFormatted); err != nil {
		t.Fatalf("checkSourcePreserved() comment trailing whitespace error = %v", err)
	}
	commentChanged := []byte("@F\n;changed\n")
	if err := checkSourcePreserved(commentOriginal, commentChanged); err == nil {
		t.Fatal("checkSourcePreserved() accepted comment interior change")
	}
}

func TestCheckSourcePreservedProtectsDefineAndContinuationBodies(t *testing.T) {
	tests := []struct {
		name, original, formatted string
	}{
		{"define body", "#DEFINE X  raw  body\n", "#DEFINE X raw body\n"},
		{"continuation blank", "@F\n{\n\nraw\n}\n", "@F\n{\nraw\n}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkSourcePreserved([]byte(tt.original), []byte(tt.formatted)); err == nil {
				t.Fatal("checkSourcePreserved() accepted a protected body change")
			}
		})
	}
}

func TestCheckSourcePreservedAllowsDefineBoundarySpacing(t *testing.T) {
	if err := checkSourcePreserved([]byte("#DEFINE\tX\t raw  body\n"), []byte("#DEFINE X raw  body\n")); err != nil {
		t.Fatal(err)
	}
}

func TestCheckSourcePreservedAllowsPrintVExpressionSpacing(t *testing.T) {
	original := []byte("@F\nPRINTV A + B , C\n")
	formatted := []byte("@F\nPRINTV A+B, C\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() error = %v", err)
	}
}

func TestCheckSourcePreservedAllowsFormConditionalApostrophe(t *testing.T) {
	original := []byte("@F\nCALL F(@\"literal\\@ GETBIT(LOOKUP_VALUE(A,B),C) ? isn't it? # false\\@literal\",2,\"l\")\n")
	formatted := []byte("@F\nCALL F(@\"literal\\@ GETBIT(LOOKUP_VALUE(A, B), C) ? isn't it? # false\\@literal\", 2, \"l\")\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() error = %v", err)
	}
	original = []byte("@F\nCALL F(@\"x \\@ VALUE!=\"\"?yes#\\@\")\n")
	formatted = []byte("@F\nCALL F(@\"x \\@ VALUE != \"\"?yes#\\@\")\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() empty branch error = %v", err)
	}
}

func TestCheckSourcePreservedAllowsConditionalInsideFormIsland(t *testing.T) {
	original := []byte(`@F
PRINTFORML before %\@ CFLAG:KEY ? %JOIN_TEXT(LOOKUP_TEXT(30),"suffix")% # fallback \@% after
`)
	formatted := []byte(`@F
PRINTFORML before %\@ CFLAG:KEY ? %JOIN_TEXT(LOOKUP_TEXT(30), "suffix")% # fallback \@% after
`)
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("conditional FORM island spacing: %v", err)
	}
	changed := []byte(`@F
PRINTFORML before %\@ CFLAG:KEY ? %JOIN_TEXT(LOOKUP_TEXT(31), "suffix")% # fallback \@% after
`)
	if err := checkSourcePreserved(original, changed); err == nil {
		t.Fatal("checkSourcePreserved() accepted a nested FORM island payload change")
	}
}

func TestCheckSourcePreservedAllowsNestedDataFormConditionalSpacing(t *testing.T) {
	original := []byte(`@F
DATAFORM %WRAP_TEXT(@"text %GET_TEXT()% \@ ARGS == "a" ? first %SELECT_TEXT("alpha")% # second %SELECT_TEXT("beta",,TARGET)% \@ tail")%`)
	formatted := []byte(`@F
DATAFORM %WRAP_TEXT(@"text %GET_TEXT()% \@ ARGS == "a" ? first %SELECT_TEXT("alpha")% # second %SELECT_TEXT("beta",, TARGET)% \@ tail")%`)
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() error = %v", err)
	}
}

func TestCheckSourcePreservedProtectsNestedPrintVAtoms(t *testing.T) {
	original := []byte("@F\nPRINTV OUTER(INNER('raw\\,  text',A+B),C+D)\n")
	formatted := []byte("@F\nPRINTV OUTER(INNER( 'raw\\,  text', A + B ), C + D)\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() error = %v", err)
	}
	changed := []byte("@F\nPRINTV OUTER(INNER( 'raw\\, text', A + B ), C + D)\n")
	if err := checkSourcePreserved(original, changed); err == nil {
		t.Fatal("checkSourcePreserved() accepted a PRINTV atom payload change")
	}
	quoted := []byte("@F\nPRINTV \"quoted ' text\",A+B\n")
	quotedFormatted := []byte("@F\nPRINTV \"quoted ' text\", A + B\n")
	if err := checkSourcePreserved(quoted, quotedFormatted); err != nil {
		t.Fatalf("checkSourcePreserved() misclassified quoted apostrophe: %v", err)
	}
}

func TestCheckSourcePreservedFingerprintsQuotedFormAtoms(t *testing.T) {
	original := []byte(`@F
CALL F(@"prefix%G(@"<x>%H("literal")%</x>", ,)%suffix")
`)
	formatted := []byte(`@F
CALL F(@"prefix%G(@"<x>%H("literal")%</x>",,)%suffix")
`)
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() error = %v", err)
	}
	changed := []byte(`@F
CALL F(@"prefix%G(@"<x>%H("changed")%</x>",,)%suffix")
`)
	if err := checkSourcePreserved(original, changed); err == nil {
		t.Fatal("checkSourcePreserved() accepted a quoted FORM literal change")
	}
}

func TestCheckSourcePreservedAllowsQuotedFormModuloSpacing(t *testing.T) {
	o := []byte("@F\nRETURNF @\"%ARGS+(\" \"*(ARG-(STRLENS(ARGS)%ARG)))%\"\n")
	f := []byte("@F\nRETURNF @\"%ARGS + (\" \" * (ARG - (STRLENS(ARGS) % ARG)))%\"\n")
	if err := checkSourcePreserved(o, f); err != nil {
		t.Fatalf("checkSourcePreserved() error = %v", err)
	}
	original := []byte("@F\nPRINTFORM \"literal { A + B }\"\n")
	formatted := []byte("@F\nPRINTFORM \"literal {A + B}\"\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("checkSourcePreserved() did not recognize FORM island: %v", err)
	}
}

func TestCheckSourcePreservedAllowsQualifiedVariablesAndBareCondition(t *testing.T) {
	original := []byte("@ F\nLOCAL @ FUNC : 0 = A+B\nRETURNF \\@ A+B?yes;keep  \\@\n")
	formatted := []byte("@F\nLOCAL@FUNC:0 = A+B\nRETURNF \\@ A + B ?yes;keep  \\@\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("qualified variable or bare FORM condition: %v", err)
	}
	changed := []byte("@F\nLOCAL@OTHER:0 = A+B\nRETURNF \\@ A + B ?yes;keep  \\@\n")
	if err := checkSourcePreserved(original, changed); err == nil {
		t.Fatal("checkSourcePreserved() accepted a qualified variable change")
	}
	changedBranch := []byte("@F\nLOCAL@FUNC:0 = A+B\nRETURNF \\@ A + B ?changed;keep  \\@\n")
	if err := checkSourcePreserved(original, changedBranch); err == nil {
		t.Fatal("checkSourcePreserved() accepted a bare FORM branch change")
	}
}

func TestCheckSourcePreservedDialectProfiles(t *testing.T) {
	common := []byte("@F\nINPUTS text, 1\nPRINTVN A+B\nPRINTFORMN literal {A+B}\nCUSTOMDRAWLINE  raw  \n")
	changed := []byte("@F\nINPUTS text, 2\nPRINTVN A + B\nPRINTFORMN literal {A + B}\nCUSTOMDRAWLINE raw\n")
	if err := checkSourcePreservedDialect(common, changed, format.CommonDialect); err == nil {
		t.Fatal("common dialect accepted unsupported INPUTS/N or raw payload changes")
	}
	if err := checkSourcePreservedDialect(common, changed, format.EMEE); err == nil {
		t.Fatal("EMEE dialect accepted a changed INPUTS value or raw payload")
	}
	plain := []byte("@F\nPRINTPLAINFORM raw {A+1}\n")
	plainFormatted := []byte("@F\nPRINTPLAINFORM raw {A + 1}\n")
	if err := checkSourcePreserved(plain, plainFormatted); err != nil {
		t.Fatalf("checkSourcePreserved() rejected a PRINTPLAINFORM island edit: %v", err)
	}
	plainChanged := []byte("@F\nPRINTPLAINFORM changed {A + 1}\n")
	if err := checkSourcePreserved(plain, plainChanged); err == nil {
		t.Fatal("checkSourcePreserved() accepted a PRINTPLAINFORM literal change")
	}
	plainSeparator := []byte("@F\nPRINTPLAINFORM  raw {A + 1}\n")
	if err := checkSourcePreserved(plain, plainSeparator); err == nil {
		t.Fatal("checkSourcePreserved() accepted a PRINTPLAINFORM separator change")
	}
	emee := []byte("@F\nINPUTS raw {A+1}, 1+0, 2\nPRINTVN A+B\nPRINTFORMN literal {A+B}\n")
	emeeFormatted := []byte("@F\nINPUTS raw {A + 1}, 1 + 0, 2\nPRINTVN A + B\nPRINTFORMN literal {A + B}\n")
	if err := checkSourcePreservedDialect(emee, emeeFormatted, format.EMEE); err != nil {
		t.Fatalf("EMEE mixed/input profiles: %v", err)
	}
	commonInput := []byte("@F\nINPUTS raw {A+1}, 1+0, 2\n")
	commonInputFormatted := []byte("@F\nINPUTS raw {A + 1}, 1 + 0, 2\n")
	if err := checkSourcePreservedDialect(commonInput, commonInputFormatted, format.CommonDialect); err == nil {
		t.Fatal("common dialect accepted top-level INPUTS fields")
	}
}

func TestCheckSourcePreservedNextRegisteredProfiles(t *testing.T) {
	times := []byte("@F\nTIMES X : 0 , 1.25e-2\n")
	timesFormatted := []byte("@F\nTIMES X:0, 1.25e-2\n")
	if err := checkSourcePreserved(times, timesFormatted); err != nil {
		t.Fatalf("TIMES spacing: %v", err)
	}
	timesChanged := []byte("@F\nTIMES X:0, 1.25e-3\n")
	if err := checkSourcePreserved(times, timesChanged); err == nil {
		t.Fatal("checkSourcePreserved() accepted a TIMES coefficient change")
	}
	data := []byte("@F\nPRINTDATA X : 0\nSTRDATA NAME : 0\n")
	dataFormatted := []byte("@F\nPRINTDATA X:0\nSTRDATA NAME:0\n")
	if err := checkSourcePreserved(data, dataFormatted); err != nil {
		t.Fatalf("variable data command spacing: %v", err)
	}
	dataChanged := []byte("@F\nPRINTDATA X:1\nSTRDATA NAME:0\n")
	if err := checkSourcePreserved(data, dataChanged); err == nil {
		t.Fatal("checkSourcePreserved() accepted a variable payload change")
	}
	extra := []byte("@F\nSPLIT A+B, \"x\", OUT : 0, COUNT : 1\nSWAP OUT : 0, OTHER : 1\n")
	extraFormatted := []byte("@F\nSPLIT A + B, \"x\", OUT:0, COUNT:1\nSWAP OUT:0, OTHER:1\n")
	if err := checkSourcePreserved(extra, extraFormatted); err != nil {
		t.Fatalf("SPLIT/SWAP spacing: %v", err)
	}
	binput := []byte("@F\nBINPUT A+B, 1\nBINPUTS raw {A+1}, 2\n")
	binputFormatted := []byte("@F\nBINPUT A + B, 1\nBINPUTS raw {A + 1}, 2\n")
	if err := checkSourcePreservedDialect(binput, binputFormatted, format.EMEE); err != nil {
		t.Fatalf("EMEE BINPUT spacing: %v", err)
	}
	if err := checkSourcePreservedDialect(binput, binputFormatted, format.CommonDialect); err == nil {
		t.Fatal("common dialect accepted EMEE-only BINPUT syntax")
	}
}

func TestCheckSourcePreservedExtendedCommandProfiles(t *testing.T) {
	base := []byte("@F\nRESETCOLOR   ; keep  \nBEGIN  raw text  \nSTRLEN raw;keep\n")
	baseFormatted := []byte("@F\nRESETCOLOR ; keep\nBEGIN  raw text  \nSTRLEN raw;keep\n")
	if err := checkSourcePreservedDialect(base, baseFormatted, format.CommonDialect); err != nil {
		t.Fatalf("base command profiles: %v", err)
	}
	baseChanged := []byte("@F\nRESETCOLOR argument\nBEGIN  raw text  \nSTRLEN raw;keep\n")
	if err := checkSourcePreservedDialect(base, baseChanged, format.CommonDialect); err == nil {
		t.Fatal("accepted a no-argument command payload")
	}
	rawChanged := []byte("@F\nRESETCOLOR   ; keep  \nBEGIN  changed text  \nSTRLEN raw;keep\n")
	if err := checkSourcePreservedDialect(base, rawChanged, format.CommonDialect); err == nil {
		t.Fatal("accepted a raw command payload change")
	}

	modern := []byte("@F\nFORCE_BEGIN  raw text\nAWAIT 1+2\nPLAYSOUND \"sound\", 1+0\nTOOLTIP_SETCOLOR 1+2, 3+4\nSETBGIMAGE  image ; raw { A+1 } tail\nVARSIZE FLAG\nDT_COLUMN_OPTIONS \"db\", \"col\", DEFAULT, 1+0\n")
	modernFormatted := []byte("@F\nFORCE_BEGIN  raw text\nAWAIT 1 + 2\nPLAYSOUND \"sound\", 1 + 0\nTOOLTIP_SETCOLOR 1 + 2, 3 + 4\nSETBGIMAGE image ; raw {A + 1} tail\nVARSIZE FLAG\nDT_COLUMN_OPTIONS \"db\", \"col\", DEFAULT, 1 + 0\n")
	if err := checkSourcePreservedDialect(modern, modernFormatted, format.EMEE); err != nil {
		t.Fatalf("modern command profiles: %v", err)
	}
	modernChanged := []byte("@F\nFORCE_BEGIN  raw text\nAWAIT 1 + 3\nPLAYSOUND \"sound\", 1 + 0\nTOOLTIP_SETCOLOR 1 + 2, 3 + 4\nSETBGIMAGE image ; raw {A + 1} tail\nVARSIZE FLAG\nDT_COLUMN_OPTIONS \"db\", \"col\", DEFAULT, 1 + 0\n")
	if err := checkSourcePreservedDialect(modern, modernChanged, format.EMEE); err == nil {
		t.Fatal("accepted a scalar value change")
	}
	keywordChanged := []byte("@F\nFORCE_BEGIN  raw text\nAWAIT 1 + 2\nPLAYSOUND \"sound\", 1 + 0\nTOOLTIP_SETCOLOR 1 + 2, 3 + 4\nSETBGIMAGE image ; raw {A + 1} tail\nVARSIZE FLAG\nDT_COLUMN_OPTIONS \"db\", \"col\", DEFAULT, 2 + 0\n")
	if err := checkSourcePreservedDialect(modern, keywordChanged, format.EMEE); err == nil {
		t.Fatal("accepted a keyword/value change")
	}
	keywordOnlyChanged := []byte("@F\nFORCE_BEGIN  raw text\nAWAIT 1 + 2\nPLAYSOUND \"sound\", 1 + 0\nTOOLTIP_SETCOLOR 1 + 2, 3 + 4\nSETBGIMAGE image ; raw {A + 1} tail\nVARSIZE FLAG\nDT_COLUMN_OPTIONS \"db\", \"col\", DEFAULT2, 1 + 0\n")
	if err := checkSourcePreservedDialect(modern, keywordOnlyChanged, format.EMEE); err == nil {
		t.Fatal("accepted a keyword change")
	}
	if err := checkSourcePreservedDialect(modern, modernFormatted, format.CommonDialect); err == nil {
		t.Fatal("accepted modern-only command profiles in the common dialect")
	}
	formLiteralChanged := []byte("@F\nFORCE_BEGIN  raw text\nAWAIT 1 + 2\nPLAYSOUND \"sound\", 1 + 0\nTOOLTIP_SETCOLOR 1 + 2, 3 + 4\nSETBGIMAGE image ; changed {A + 1} tail\nVARSIZE FLAG\nDT_COLUMN_OPTIONS \"db\", \"col\", DEFAULT, 1 + 0\n")
	if err := checkSourcePreservedDialect(modern, formLiteralChanged, format.EMEE); err == nil {
		t.Fatal("accepted a FORM_STR_ANY literal change")
	}
}

func TestCheckSourcePreservedProtectsMarkedFormIsland(t *testing.T) {
	original := []byte("@F\nPRINTFORM text { A + ;!; B } tail\n")
	changed := []byte("@F\nPRINTFORM text { A + ;!; C } tail\n")
	if err := checkSourcePreserved(original, changed); err == nil {
		t.Fatal("accepted a changed ;!; FORM island")
	}
	spacing := []byte("@F\nPRINTFORM text { A + ;!; B } tail\n")
	if err := checkSourcePreserved(original, spacing); err != nil {
		t.Fatalf("unchanged ;!; FORM island: %v", err)
	}
}

func TestCheckSourcePreservedMethodsGraphicsAndInlineComments(t *testing.T) {
	base := []byte("@F\nGETCHARA A+B\nGETCHARA \"literal  text\"\nLOG10 A+B\n")
	baseFormatted := []byte("@F\nGETCHARA A + B\nGETCHARA \"literal  text\"\nLOG10 A + B\n")
	if err := checkSourcePreservedDialect(base, baseFormatted, format.CommonDialect); err != nil {
		t.Fatalf("base builtin method spacing: %v", err)
	}
	baseLiteral := []byte("@F\nGETCHARA \"literal text\"\n")
	if err := checkSourcePreservedDialect(base, baseLiteral, format.CommonDialect); err == nil {
		t.Fatal("checkSourcePreserved() accepted a method string literal change")
	}
	modern := []byte("@F\nMOUSEB A+B\n")
	modernFormatted := []byte("@F\nMOUSEB A + B\n")
	if err := checkSourcePreservedDialect(modern, modernFormatted, format.EMEE); err != nil {
		t.Fatalf("modern builtin method spacing: %v", err)
	}
	if err := checkSourcePreservedDialect(modern, modernFormatted, format.CommonDialect); err == nil {
		t.Fatal("common dialect accepted a modern-only method")
	}
	shadow := []byte("@F\nVARSIZE A+B\n")
	shadowChanged := []byte("@F\nVARSIZE A + B\n")
	if err := checkSourcePreservedDialect(shadow, shadowChanged, format.EMEE); err == nil {
		t.Fatal("checkSourcePreserved() accepted a shadowed builtin command")
	}
	graphics := []byte("@F\nPRINT_SPACE (A+1)    px\nPRINT_RECT 1+2, 3, 4, 5\n")
	graphicsFormatted := []byte("@F\nPRINT_SPACE (A + 1) px\nPRINT_RECT 1 + 2, 3, 4, 5\n")
	if err := checkSourcePreservedDialect(graphics, graphicsFormatted, format.EMEE); err != nil {
		t.Fatalf("graphics spacing: %v", err)
	}
	graphicsChanged := []byte("@F\nPRINT_SPACE (A + 2) px\nPRINT_RECT 1 + 2, 3, 4, 5\n")
	if err := checkSourcePreservedDialect(graphics, graphicsChanged, format.EMEE); err == nil {
		t.Fatal("checkSourcePreserved() accepted a graphics expression change")
	}
	exponent := []byte("@F\nPRINT_RECT 1p2px, 1e2PX, 3, 4\n")
	exponentFormatted := []byte("@F\nPRINT_RECT 1p2 px, 1e2 PX, 3, 4\n")
	if err := checkSourcePreservedDialect(exponent, exponentFormatted, format.EMEE); err != nil {
		t.Fatalf("graphics exponent unit spacing: %v", err)
	}
	invalidUnit := []byte("@F\nPRINT_RECT 10px, 2, 3, 4\n")
	invalidUnitChanged := []byte("@F\nPRINT_RECT 10 px, 2, 3, 4\n")
	if err := checkSourcePreservedDialect(invalidUnit, invalidUnitChanged, format.EMEE); err == nil {
		t.Fatal("checkSourcePreserved() accepted an invalid glued pixel unit repair")
	}
	inline := []byte("@F\nRETURN A;note\nPRINT raw;note\n")
	inlineFormatted := []byte("@F\nRETURN A ; note\nPRINT raw;note\n")
	if err := checkSourcePreserved(inline, inlineFormatted); err != nil {
		t.Fatalf("inline comment prefix spacing: %v", err)
	}
	rawChanged := []byte("@F\nRETURN A ; note\nPRINT raw; note\n")
	if err := checkSourcePreserved(inline, rawChanged); err == nil {
		t.Fatal("checkSourcePreserved() accepted a raw PRINT comment change")
	}
	protected := []byte("@F\n;!;note  \n;---\n")
	protectedChanged := []byte("@F\n;!; note\n;--\n")
	if err := checkSourcePreserved(protected, protectedChanged); err == nil {
		t.Fatal("checkSourcePreserved() accepted a protected comment body change")
	}
	alias := []byte("@F\n;!; [[ALIAS  X]]\n")
	aliasChanged := []byte("@F\n;!; [[ALIAS X]]\n")
	if err := checkSourcePreserved(alias, aliasChanged); err == nil {
		t.Fatal("checkSourcePreserved() accepted a marked alias comment change")
	}
}

func TestCheckSourcePreservedDirectSTRFORMLiteral(t *testing.T) {
	original := []byte("@F\nSTRFORM  \"x { 1+2 }\"\n")
	formatted := []byte("@F\n    STRFORM \"x {1 + 2}\"\n")
	if err := checkSourcePreservedDialect(original, formatted, format.CommonDialect); err != nil {
		t.Fatalf("direct STRFORM literal: %v", err)
	}
	changed := []byte("@F\n    STRFORM \"changed {1 + 2}\"\n")
	if err := checkSourcePreservedDialect(original, changed, format.CommonDialect); err == nil {
		t.Fatal("accepted a direct STRFORM literal mutation")
	}
	for _, source := range []string{
		`STRFORM "x \\{ 1+2 \\}"`,
		`STRFORM "x { 1+2 }" + CSTR`,
		`STRFORM ;!; "x { 1+2 }"`,
		`STRFORM "x [[ALIAS]] { 1+2 }"`,
		`RETURN STRFORM("x { 1+2 }")`,
	} {
		before := []byte("@F\n" + source + "\n")
		after := bytes.Replace(before, []byte("1+2"), []byte("1 + 2"), 1)
		if err := checkSourcePreservedDialect(before, after, format.CommonDialect); err == nil {
			t.Fatalf("accepted an opaque STRFORM literal edit: %q", source)
		}
	}

	separator := " "
	continued := []byte("@F\n{\nSTRFORM \"x { 1+2 }\"\n}\n")
	result := format.FormatWithOptions(continued, format.ERB, format.Options{ContinuationSeparator: &separator})
	if err := checkSourcePreservedOptions(continued, result.Output, sourcePreservationOptions{dialect: format.CommonDialect, continuationSeparator: &separator}); err != nil {
		t.Fatalf("continued direct STRFORM literal: %v\noutput=%q", err, result.Output)
	}

	fullwidth := []byte("@F\nSTRFORM　\"x { 1+2 }\"\n")
	fullwidthResult := format.FormatWithOptions(fullwidth, format.ERB, format.Options{AllowFullwidthSpace: true})
	if err := checkSourcePreservedOptions(fullwidth, fullwidthResult.Output, sourcePreservationOptions{dialect: format.CommonDialect, allowFullwidthSpace: true}); err != nil {
		t.Fatalf("fullwidth direct STRFORM literal: %v\noutput=%q", err, fullwidthResult.Output)
	}
	for _, test := range []struct {
		name, source, output string
	}{
		{"dynamic", "@F\nSTRFORM　CSTR\n", "@F\n    STRFORM CSTR\n"},
		{"concatenated", "@F\nSTRFORM　\"x { 1+2 }\"　+　CSTR\n", "@F\n    STRFORM \"x { 1+2 }\" + CSTR\n"},
		{"explicit form", "@F\nSTRFORM　@\"x { 1+2 }\"\n", "@F\n    STRFORM @\"x {1 + 2}\"\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := format.FormatWithOptions([]byte(test.source), format.ERB, format.Options{AllowFullwidthSpace: true})
			if !bytes.Equal(result.Output, []byte(test.output)) {
				t.Fatalf("output=%q want=%q diagnostics=%+v", result.Output, test.output, result.Diagnostics)
			}
			if err := checkSourcePreservedOptions([]byte(test.source), result.Output, sourcePreservationOptions{dialect: format.CommonDialect, allowFullwidthSpace: true}); err != nil {
				t.Fatalf("preservation: %v", err)
			}
		})
	}
}

func TestCheckSourcePreservedDialectRestrictedCommands(t *testing.T) {
	original := []byte("@F\nCALLSHARP A+B\nFORCE_QUIT\nFORCE_QUIT_AND_RESTART\nQUIT_AND_RESTART\nSKIPLOG A+B\nTRYCALLF A+B\nTRYCALLFORMF F(A+B)\nOUTPUTLOG A+B\n")
	emee := []byte("@F\nCALLSHARP A+B\nFORCE_QUIT\nFORCE_QUIT_AND_RESTART\nQUIT_AND_RESTART\nSKIPLOG A + B\nTRYCALLF A+B\nTRYCALLFORMF F(A + B)\nOUTPUTLOG A + B\n")
	if err := checkSourcePreservedDialect(original, emee, format.EMEE); err != nil {
		t.Fatalf("EMEE dialect profiles: %v", err)
	}
	for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824} {
		if err := checkSourcePreservedDialect(original, emee, dialect); err == nil {
			t.Fatalf("%v accepted unavailable command edits", dialect)
		}
	}
	common := []byte("@F\nCALLSHARP = A+B\nFORCE_QUIT = A+B\nSKIPLOG = A+B\nTRYCALLF = A+B\nTRYCALLFORMF = A+B\n")
	assignment := []byte("@F\nCALLSHARP =A+B\nFORCE_QUIT =A+B\nSKIPLOG =A+B\nTRYCALLF =A+B\nTRYCALLFORMF =A+B\n")
	for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
		if err := checkSourcePreservedDialect(common, assignment, dialect); err != nil {
			t.Fatalf("%v implicit assignment profile: %v", dialect, err)
		}
	}
	lower := []byte("@F\ncallsharp = A+B\nforce_quit = A+B\nskiplog = A+B\n")
	lowerFormatted := []byte("@F\ncallsharp =A+B\nforce_quit =A+B\nskiplog =A+B\n")
	for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
		if err := checkSourcePreservedDialect(lower, lowerFormatted, dialect); err != nil {
			t.Fatalf("%v lowercase implicit assignment profile: %v", dialect, err)
		}
	}
}

func TestCheckSourcePreservedGraphicsCallLiterals(t *testing.T) {
	original := []byte("@F\nPRINT_SPACE STRLENS(\"abc\")    px\nPRINT_RECT STRLENS(@\"{A+B}\"), 1, 2, 3\n")
	formatted := []byte("@F\nPRINT_SPACE STRLENS(\"abc\") px\nPRINT_RECT STRLENS(@\"{A + B}\"), 1, 2, 3\n")
	if err := checkSourcePreservedDialect(original, formatted, format.EMEE); err != nil {
		t.Fatalf("nested graphics call literals: %v", err)
	}
	topLevel := []byte("@F\nPRINT_SPACE \"a\"+\"b\"\n")
	topLevelChanged := []byte("@F\nPRINT_SPACE \"a\" + \"b\"\n")
	if err := checkSourcePreservedDialect(topLevel, topLevelChanged, format.CommonDialect); err == nil {
		t.Fatal("accepted a string expression as a base numeric field")
	}
	invalidUnit := []byte("@F\nPRINT_SPACE 10px\n")
	invalidUnitChanged := []byte("@F\nPRINT_SPACE 10 px\n")
	if err := checkSourcePreservedDialect(invalidUnit, invalidUnitChanged, format.EMEE); err == nil {
		t.Fatal("accepted an invalid glued graphics unit")
	}
}

func TestCorpusVerifierSeparatesModuloFromFormIslands(t *testing.T) {
	expression := []byte("A%1, B%2, C%3, D%4")
	if got := topLevelCommas(expression); len(got) != 3 {
		t.Fatalf("expression commas = %v, want three separators", got)
	}
	form := []byte("%F(A,B)%, C")
	if got := formTopLevelCommas(form); len(got) != 1 {
		t.Fatalf("FORM commas = %v, want one separator", got)
	}

	graphics := []byte("@F\nPRINT_RECT A%1, B%2, C%3, D%4\n")
	graphicsFormatted := []byte("@F\nPRINT_RECT A % 1, B % 2, C % 3, D % 4\n")
	if err := checkSourcePreservedDialect(graphics, graphicsFormatted, format.EMEE); err != nil {
		t.Fatalf("PRINT_RECT modulo expressions: %v", err)
	}

	tooltip := []byte("@F\nTOOLTIP_SETCOLOR A%1, B%2\n")
	tooltipFormatted := []byte("@F\nTOOLTIP_SETCOLOR A % 1, B % 2\n")
	if err := checkSourcePreservedDialect(tooltip, tooltipFormatted, format.EMEE); err != nil {
		t.Fatalf("TOOLTIP_SETCOLOR modulo expressions: %v", err)
	}
}

func TestCorpusVerifierRecognizesProvenNumericImageFields(t *testing.T) {
	modulo := []byte("@F\nPRINT_IMG \"x\", \"y\", A%1, B%2, C%3\n")
	moduloFormatted := []byte("@F\nPRINT_IMG \"x\", \"y\", A % 1, B % 2, C % 3\n")
	if err := checkSourcePreservedDialect(modulo, moduloFormatted, format.EMEE); err != nil {
		t.Fatalf("PRINT_IMG modulo fields: %v", err)
	}

	known := []byte("@F\nPRINT_IMG \"x\", \"y\", A+1\n")
	knownFormatted := []byte("@F\nPRINT_IMG \"x\", \"y\", A + 1\n")
	if err := checkSourcePreservedDialect(known, knownFormatted, format.EMEE); err != nil {
		t.Fatalf("PRINT_IMG builtin numeric field: %v", err)
	}
	for _, name := range []string{"LOCAL", "RAND", "CDFLAG"} {
		original := []byte("@F\nPRINT_IMG \"x\", \"y\", " + name + "+1\n")
		formatted := []byte("@F\nPRINT_IMG \"x\", \"y\", " + name + " + 1\n")
		if err := checkSourcePreservedDialect(original, formatted, format.EMEE); err != nil {
			t.Fatalf("PRINT_IMG %s numeric field: %v", name, err)
		}
	}

	unknown := []byte("@F\nPRINT_IMG \"x\", \"y\", NAME+1\n")
	unknownFormatted := []byte("@F\nPRINT_IMG \"x\", \"y\", NAME + 1\n")
	if err := checkSourcePreservedDialect(unknown, unknownFormatted, format.EMEE); err == nil {
		t.Fatal("accepted an unresolved string-valued image field")
	}
	for _, source := range []string{"F(A)", "F (A)", "F@FUNC"} {
		if !graphicsNumericUnknown([]byte(source)) {
			t.Fatalf("classified unresolved call/reference %q as numeric", source)
		}
	}
}

func TestCheckSourcePreservedContinuationOptions(t *testing.T) {
	separator := " "
	options := sourcePreservationOptions{dialect: format.CommonDialect, continuationSeparator: &separator}
	original := []byte("@F\n{\n;OriginalString : HTML_HEART(, COLOR2_K39(\"PINK\")) \n}\n")
	result := format.FormatWithOptions(original, format.ERB, format.Options{ContinuationSeparator: &separator})
	if err := checkSourcePreservedOptions(original, result.Output, options); err != nil {
		t.Fatalf("explicit continuation verification: %v\noutput=%q", err, result.Output)
	}
	if err := checkSourcePreserved(original, result.Output); err == nil {
		t.Fatal("default byte-sensitive verification accepted continuation comment layout")
	}
}

func TestCheckSourcePreservedContinuationMutations(t *testing.T) {
	separator := " "
	options := sourcePreservationOptions{dialect: format.EMEE, continuationSeparator: &separator}
	original := []byte("@F\n{\nPRINTFORM prefix { A +\nB } \\\"quoted text\\\" tail\n}\n")
	formatted := []byte("@F\n    {\nPRINTFORM prefix { A+\nB } \\\"quoted text\\\" tail\n    }\n")
	if err := checkSourcePreservedOptions(original, formatted, options); err != nil {
		t.Fatalf("valid FORM expression spacing: %v", err)
	}
	tests := []struct {
		name string
		out  string
	}{
		{
			name: "quoted FORM literal",
			out:  "@F\n    {\nPRINTFORM prefix { A+\nB } \\\"changed text\\\" tail\n    }\n",
		},
		{
			name: "FORM literal whitespace",
			out:  "@F\n    {\nPRINTFORM prefix { A+\nB } \\\"quoted  text\\\" tail\n    }\n",
		},
		{
			name: "executable marker",
			out:  "@F\n    {\nPRINTFORM prefix { A+\nB } \\\"quoted text\\\" tail ;!;changed\n    }\n",
		},
		{
			name: "protected row",
			out:  "@F\n    {\nPRINTFORM prefix { A+\nB } \\\"quoted text\\\" tail\n; [changed protected row]\n    }\n",
		},
		{
			name: "tokens moved between rows",
			out:  "@F\n    {\nPRINTFORM prefix { A+B } \\\"quoted\ntext\\\" tail\n    }\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkSourcePreservedOptions(original, []byte(tt.out), options); err == nil {
				t.Fatal("continuation mutation was accepted")
			}
		})
	}
}

func TestCheckSourcePreservedCorpusCommandArity(t *testing.T) {
	for _, test := range []struct {
		name, original, formatted string
	}{
		{"setfont suffix", "@F\nSETFONT \"font\", 1+2\n", "@F\n    SETFONT \"font\", 1+2\n"},
		{"fontstyle suffix", "@F\nFONTSTYLE 1+2, 3+4\n", "@F\n    FONTSTYLE 1+2, 3+4\n"},
		{"loaddata suffix", "@F\nLOADDATA 1+2, 3+4\n", "@F\n    LOADDATA 1+2, 3+4\n"},
		{"html print suffix", "@F\nHTML_PRINT \"x\", 1+2, 3+4\n", "@F\n    HTML_PRINT \"x\", 1+2, 3+4\n"},
		{"print per line suffix", "@F\nPRINTCPERLINE RESULTS, 1+2\n", "@F\n    PRINTCPERLINE RESULTS, 1+2\n"},
		{"twait suffix", "@F\nTWAIT 1+2, 3+4, 5+6\n", "@F\n    TWAIT 1+2, 3+4, 5+6\n"},
		{"swapchara suffix", "@F\nSWAPCHARA 1+2, 3+4, 5+6\n", "@F\n    SWAPCHARA 1+2, 3+4, 5+6\n"},
		{"arrayremove suffix", "@F\nARRAYREMOVE RESULTS, 1+2, 3+4, 5+6\n", "@F\n    ARRAYREMOVE RESULTS, 1+2, 3+4, 5+6\n"},
		{"arrayshift suffix", "@F\nARRAYSHIFT RESULTS, 1+2, 3+4, 5+6, 7+8, 9+10\n", "@F\n    ARRAYSHIFT RESULTS, 1+2, 3+4, 5+6, 7+8, 9+10\n"},
		{"arraycopy suffix", "@F\nARRAYCOPY \"x\", \"y\", \"z\"\n", "@F\n    ARRAYCOPY \"x\", \"y\", \"z\"\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := checkSourcePreserved([]byte(test.original), []byte(test.formatted)); err != nil {
				t.Fatalf("valid ignored suffix: %v", err)
			}
			changed := strings.Replace(test.formatted, "3+4", "3+5", 1)
			if changed == test.formatted {
				changed = strings.Replace(test.formatted, "1+2", "1+3", 1)
			}
			if changed == test.formatted {
				changed = strings.Replace(test.formatted, `"z"`, `"q"`, 1)
			}
			if err := checkSourcePreserved([]byte(test.original), []byte(changed)); err == nil {
				t.Fatal("accepted ignored suffix mutation")
			}
		})
	}

	for _, test := range []struct {
		name, original, formatted string
	}{
		{"twait terminal comma", "@F\nTWAIT 1+2,3+4,\n", "@F\n    TWAIT 1 + 2, 3 + 4,\n"},
		{"arrayshift terminal comma", "@F\nARRAYSHIFT RESULTS,1+2,3+4,\n", "@F\n    ARRAYSHIFT RESULTS, 1 + 2, 3 + 4,\n"},
		{"graphics terminal comma", "@F\nPRINT_RECT 1+2,3,4,5,\n", "@F\n    PRINT_RECT 1 + 2, 3, 4, 5,\n"},
		{"structured terminal comma", "@F\nTOOLTIP_SETCOLOR 1+2,3+4,\n", "@F\n    TOOLTIP_SETCOLOR 1 + 2, 3 + 4,\n"},
		{"times terminal comma", "@F\nTIMES RESULT,1.5,\n", "@F\n    TIMES RESULT, 1.5,\n"},
		{"split terminal comma", "@F\nSPLIT \"x\",\",\",RESULTS,\n", "@F\n    SPLIT \"x\", \",\", RESULTS,\n"},
		{"variable terminal comma", "@F\nPRINTDATA RESULT,\n", "@F\n    PRINTDATA RESULT,\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := checkSourcePreserved([]byte(test.original), []byte(test.formatted)); err != nil {
				t.Fatalf("terminal comma: %v", err)
			}
		})
	}

	invalid := []byte("@F\nFONTSTYLE 1+2,3+4,5+6\n")
	invalidChanged := []byte("@F\n    FONTSTYLE 1 + 2, 3 + 5, 5 + 6\n")
	if err := checkSourcePreserved(invalid, invalidChanged); err == nil {
		t.Fatal("accepted invalid-shape expression edits")
	}
	for _, test := range []struct {
		name, original, formatted string
		dialect                   format.Dialect
	}{
		{"numeric field with string value", "@F\nFONTSTYLE \"x\"+\"y\"\n", "@F\n    FONTSTYLE \"x\" + \"y\"\n", format.CommonDialect},
		{"numeric field with string builtin", "@F\nFONTSTYLE CSTR+CSTR\n", "@F\n    FONTSTYLE CSTR + CSTR\n", format.CommonDialect},
		{"numeric field with parenthesized string", "@F\nFONTSTYLE (CSTR)\n", "@F\n    FONTSTYLE ( CSTR )\n", format.CommonDialect},
		{"string field with numeric value", "@F\nPLAYSOUND 1+2,3+4\n", "@F\n    PLAYSOUND 1 + 2, 3 + 4\n", format.EMEE},
		{"SAVEVAR literal destination", "@F\nSAVEVAR \"f\",\"m\",1+2\n", "@F\n    SAVEVAR \"f\", \"m\", 1 + 2\n", format.CommonDialect},
		{"SAVEVAR read-only destination", "@F\nSAVEVAR \"f\",\"m\",ITEMPRICE\n", "@F\n    SAVEVAR \"f\", \"m\", ITEMPRICE\n", format.CommonDialect},
		{"SAVEVAR character destination", "@F\nSAVEVAR \"f\",\"m\",CSTR\n", "@F\n    SAVEVAR \"f\", \"m\", CSTR\n", format.CommonDialect},
		{"SAVEVAR duplicate destination", "@F\nSAVEVAR \"f\",\"m\",RESULT,RESULT\n", "@F\n    SAVEVAR \"f\", \"m\", RESULT, RESULT\n", format.CommonDialect},
		{"SAVEVAR parenthesized read-only", "@F\nSAVEVAR \"f\",\"m\",(ITEMPRICE)\n", "@F\n    SAVEVAR \"f\", \"m\", (ITEMPRICE)\n", format.CommonDialect},
		{"SAVEVAR parenthesized character", "@F\nSAVEVAR \"f\",\"m\",(CSTR)\n", "@F\n    SAVEVAR \"f\", \"m\", (CSTR)\n", format.CommonDialect},
		{"SAVEVAR parenthesized duplicate", "@F\nSAVEVAR \"f\",\"m\",(RESULT),RESULT\n", "@F\n    SAVEVAR \"f\", \"m\", (RESULT), RESULT\n", format.CommonDialect},
		{"SAVEVAR unary string destination", "@F\nSAVEVAR \"f\",\"m\",+RESULTS\n", "@F\n    SAVEVAR \"f\", \"m\", +RESULTS\n", format.CommonDialect},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := checkSourcePreservedDialect([]byte(test.original), []byte(test.formatted), test.dialect); err == nil {
				t.Fatal("accepted invalid typed field edits")
			}
		})
	}
	if err := checkSourcePreserved([]byte("@F\nPRINTDATA ,\n"), []byte("@F\nPRINTDATA\n")); err == nil {
		t.Fatal("accepted removal of a sole empty field")
	}
	for _, test := range []struct{ name, original, formatted string }{
		{"lowercase numeric", "@F\nFONTSTYLE cstr+cstr\n", "@F\n    FONTSTYLE cstr + cstr\n"},
		{"lowercase string", "@F\nSETFONT itemprice+itemprice\n", "@F\n    SETFONT itemprice + itemprice\n"},
		{"lowercase SAVEVAR", "@F\nSAVEVAR \"f\",\"m\",itemprice\n", "@F\n    SAVEVAR \"f\", \"m\", itemprice\n"},
		{"parenthesized SAVEVAR", "@F\nSAVEVAR \"f\",\"m\",(RESULT)\n", "@F\n    SAVEVAR \"f\", \"m\", (RESULT)\n"},
		{"unary SAVEVAR", "@F\nSAVEVAR \"f\",\"m\",+RESULT\n", "@F\n    SAVEVAR \"f\", \"m\", +RESULT\n"},
		{"unary parenthesized SAVEVAR", "@F\nSAVEVAR \"f\",\"m\",+(RESULT)\n", "@F\n    SAVEVAR \"f\", \"m\", +(RESULT)\n"},
		{"marked SAVEVAR", "@F\nSAVEVAR \"f\",\"m\",;!;RESULT\n", "@F\n    SAVEVAR \"f\", \"m\",;!;RESULT\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := checkSourcePreserved([]byte(test.original), []byte(test.formatted)); err != nil {
				t.Fatalf("valid case-sensitive or parenthesized field: %v", err)
			}
		})
	}
}

func TestCheckSourcePreservedCorpusUnsupportedPreprocessor(t *testing.T) {
	known := []byte("@F\n[IF   FEATURE]\nRETURN 1+2\n[ENDIF]\n")
	knownFormatted := []byte("@F\n[IF FEATURE]\n    RETURN 1 + 2\n[ENDIF]\n")
	if err := checkSourcePreserved(known, knownFormatted); err != nil {
		t.Fatalf("known preprocessor spacing: %v", err)
	}
	unknown := []byte("@F\n[FUTURE   FEATURE]\nRETURN 1+2\n")
	unknownChanged := []byte("@F\n[FUTURE FEATURE]\nRETURN 1 + 2\n")
	if err := checkSourcePreserved(unknown, unknownChanged); err == nil {
		t.Fatal("accepted unsupported preprocessor spacing mutation")
	}
	unknownIndented := []byte("@F\n    [FUTURE   FEATURE]\nRETURN 1+2\n")
	if err := checkSourcePreserved(unknown, unknownIndented); err == nil {
		t.Fatal("accepted unsupported preprocessor indentation mutation")
	}
	separator := " "
	continued := []byte("@F\n{\n[FUTURE   FEATURE]\n}\n")
	continuedChanged := []byte("@F\n    {\n[FUTURE FEATURE]\n    }\n")
	if err := checkSourcePreservedOptions(continued, continuedChanged, sourcePreservationOptions{continuationSeparator: &separator}); err == nil {
		t.Fatal("accepted unsupported continuation preprocessor mutation")
	}
	comment := []byte("@F\n[IF　FEATURE] ; keep　text\n[ENDIF]\n")
	commentFormatted := []byte("@F\n[IF FEATURE]; keep　text\n[ENDIF]\n")
	options := sourcePreservationOptions{allowFullwidthSpace: true}
	if err := checkSourcePreservedOptions(comment, commentFormatted, options); err != nil {
		t.Fatalf("known preprocessor comment spacing: %v", err)
	}
	commentChanged := bytes.Replace(commentFormatted, []byte("keep　text"), []byte("keep text"), 1)
	if err := checkSourcePreservedOptions(comment, commentChanged, options); err == nil {
		t.Fatal("accepted preprocessor comment payload mutation")
	}
}

func TestCheckSourcePreservedCorpusSpecialPayloads(t *testing.T) {
	printV := []byte("@F\nPRINTV A+B,'raw\\,  ',C+D\n")
	printVFormatted := []byte("@F\n    PRINTV A + B, 'raw\\,  ', C + D\n")
	if err := checkSourcePreserved(printV, printVFormatted); err != nil {
		t.Fatalf("PRINTV raw plus expression: %v", err)
	}
	printVChanged := []byte("@F\n    PRINTV A + B, 'raw\\, ', C + D\n")
	if err := checkSourcePreserved(printV, printVChanged); err == nil {
		t.Fatal("accepted PRINTV apostrophe atom mutation")
	}

	separator := " "
	inputs := []byte("@F\n{\nINPUTS raw {A+\n1}, 1+0, 2\n}\n")
	result := format.FormatWithOptions(inputs, format.ERB, format.Options{Dialect: format.EMEE, ContinuationSeparator: &separator})
	if err := checkSourcePreservedOptions(inputs, result.Output, sourcePreservationOptions{dialect: format.EMEE, continuationSeparator: &separator}); err != nil {
		t.Fatalf("INPUTS continuation: %v\noutput=%q", err, result.Output)
	}

	for _, test := range []struct{ original, formatted string }{
		{"@F\nPRINT_SPACE (1+2)px\n", "@F\n    PRINT_SPACE (1 + 2) px\n"},
		{"@F\nPRINT_SPACE LOCAL@FUNC:0 px\n", "@F\n    PRINT_SPACE LOCAL@FUNC:0 px\n"},
	} {
		if err := checkSourcePreservedDialect([]byte(test.original), []byte(test.formatted), format.EMEE); err != nil {
			t.Fatalf("glued pixel unit: %v", err)
		}
	}
	invalidUnit := []byte("@F\nPRINT_SPACE 1px\n")
	invalidUnitChanged := []byte("@F\n    PRINT_SPACE 1 px\n")
	if err := checkSourcePreservedDialect(invalidUnit, invalidUnitChanged, format.EMEE); err == nil {
		t.Fatal("accepted invalid glued pixel unit repair")
	}

	for _, continued := range []bool{false, true} {
		source := []byte("@F\nINPUTS　\"prompt　text\",　A　+　B\n")
		options := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}
		formatOptions := format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true}
		if continued {
			source = []byte("@F\n{\nINPUTS　\"prompt　text\",　A　+\nB\n}\n")
			options.continuationSeparator = &separator
			formatOptions.ContinuationSeparator = &separator
		}
		result := format.FormatWithOptions(source, format.ERB, formatOptions)
		if err := checkSourcePreservedOptions(source, result.Output, options); err != nil {
			t.Fatalf("INPUTS continued=%v: %v\noutput=%q", continued, err, result.Output)
		}
		commandGap := bytes.Replace(result.Output, []byte("INPUTS　"), []byte("INPUTS "), 1)
		if err := checkSourcePreservedOptions(source, commandGap, options); err != nil {
			t.Fatalf("INPUTS command separator continued=%v: %v\noutput=%q", continued, err, commandGap)
		}
		mutated := bytes.Replace(result.Output, []byte("prompt　text"), []byte("prompt text"), 1)
		if bytes.Equal(mutated, result.Output) {
			t.Fatalf("INPUTS prompt mutation was not applied: %q", result.Output)
		}
		if err := checkSourcePreservedOptions(source, mutated, options); err == nil {
			t.Fatalf("accepted INPUTS prompt mutation, continued=%v: %q", continued, mutated)
		}
	}

	for _, continued := range []bool{false, true} {
		source := []byte("@F\nPRINTV　'raw'　text,A　+　1\n")
		options := sourcePreservationOptions{dialect: format.CommonDialect, allowFullwidthSpace: true}
		formatOptions := format.Options{AllowFullwidthSpace: true}
		if continued {
			source = []byte("@F\n{\nPRINTV　'raw'　text,A　+\n1\n}\n")
			options.continuationSeparator = &separator
			formatOptions.ContinuationSeparator = &separator
		}
		result := format.FormatWithOptions(source, format.ERB, formatOptions)
		if err := checkSourcePreservedOptions(source, result.Output, options); err != nil {
			t.Fatalf("PRINTV raw atom continued=%v: %v\noutput=%q", continued, err, result.Output)
		}
		mutated := bytes.Replace(result.Output, []byte("'raw'　text"), []byte("'raw' text"), 1)
		if bytes.Equal(mutated, result.Output) {
			t.Fatalf("PRINTV raw atom mutation was not applied: %q", result.Output)
		}
		if err := checkSourcePreservedOptions(source, mutated, options); err == nil {
			t.Fatalf("accepted PRINTV raw atom mutation, continued=%v: %q", continued, mutated)
		}
	}

	rawExpression := []byte("@F\nPRINTV　'raw,A　+　B\n")
	rawExpressionOptions := sourcePreservationOptions{dialect: format.CommonDialect, allowFullwidthSpace: true}
	rawExpressionResult := format.FormatWithOptions(rawExpression, format.ERB, format.Options{AllowFullwidthSpace: true})
	if err := checkSourcePreservedOptions(rawExpression, rawExpressionResult.Output, rawExpressionOptions); err != nil {
		t.Fatalf("PRINTV raw plus expression: %v\noutput=%q", err, rawExpressionResult.Output)
	}
}

func TestCheckSourcePreservedCorpusFullwidthOpaqueSuffixes(t *testing.T) {
	separator := " "
	graphics := []struct {
		command, tail string
	}{
		{"PRINT_RECT", "1,2,3,4,5　suffix"},
		{"PRINT_IMG", `"x",1,2,3,4,5,6　suffix`},
		{"PRINT_SPACE", "1,2　suffix"},
	}
	for _, continued := range []bool{false, true} {
		for _, test := range graphics {
			t.Run(fmt.Sprintf("graphics/%s/continued=%v", test.command, continued), func(t *testing.T) {
				source := []byte("@F\n" + test.command + "　" + test.tail + "\n")
				options := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}
				formatOptions := format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true}
				if continued {
					source = []byte("@F\n{\n" + test.command + "　" + test.tail + "\n}\n")
					options.continuationSeparator = &separator
					formatOptions.ContinuationSeparator = &separator
				}
				result := format.FormatWithOptions(source, format.ERB, formatOptions)
				if err := checkSourcePreservedOptions(source, result.Output, options); err != nil {
					t.Fatalf("opaque graphics suffix: %v\noutput=%q", err, result.Output)
				}
				mutated := bytes.Replace(result.Output, []byte("　suffix"), []byte(" suffix"), 1)
				if bytes.Equal(mutated, result.Output) {
					t.Fatalf("graphics suffix mutation was not applied: %q", result.Output)
				}
				if err := checkSourcePreservedOptions(source, mutated, options); err == nil {
					t.Fatal("accepted graphics overflow suffix mutation")
				}
			})
		}
	}
	typed := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}
	invalidTyped := []byte("@F\nPLAYSOUND 1　+　2,3\n")
	invalidTypedChanged := []byte("@F\nPLAYSOUND 1 + 2,3\n")
	if err := checkSourcePreservedOptions(invalidTyped, invalidTypedChanged, typed); err == nil {
		t.Fatal("accepted fullwidth edits in an invalid typed field")
	}

	fixed := []struct {
		command, valid, overflow string
	}{
		{"SETFONT", `"font"`, `"font",1　ignored`},
		{"FONTSTYLE", "1　+　2", "1,2　ignored"},
		{"LOADDATA", "1　+　2", "1,2　ignored"},
		{"HTML_PRINT", `"html",1　+　2`, `"html",1,2　ignored`},
		{"PRINTCPERLINE", "RESULTS", "RESULTS,1　ignored"},
		{"TWAIT", "1　+　2,3　+　4", "1,2,3　ignored"},
		{"SWAPCHARA", "1　+　2,3　+　4", "1,2,3　ignored"},
		{"ARRAYREMOVE", "RESULTS,1　+　2,3　+　4", "RESULTS,1,2,3　ignored"},
		{"ARRAYSHIFT", "RESULTS,1　+　2,3　+　4", "RESULTS,1,2,3,4,5,6　ignored"},
		{"ARRAYCOPY", `"x","y"`, `"x","y","z"　ignored`},
	}
	for _, continued := range []bool{false, true} {
		for _, test := range fixed {
			t.Run(fmt.Sprintf("fixed/%s/continued=%v", test.command, continued), func(t *testing.T) {
				options := sourcePreservationOptions{dialect: format.CommonDialect, allowFullwidthSpace: true}
				formatOptions := format.Options{Dialect: format.CommonDialect, AllowFullwidthSpace: true}
				validSource := []byte("@F\n" + test.command + "　" + test.valid + "\n")
				if continued {
					validSource = []byte("@F\n{\n" + test.command + "　" + test.valid + "\n}\n")
					options.continuationSeparator = &separator
					formatOptions.ContinuationSeparator = &separator
				}
				validResult := format.FormatWithOptions(validSource, format.ERB, formatOptions)
				if err := checkSourcePreservedOptions(validSource, validResult.Output, options); err != nil {
					t.Fatalf("valid prefix normalization: %v\noutput=%q", err, validResult.Output)
				}

				overflowSource := []byte("@F\n" + test.command + "　" + test.overflow + "\n")
				if continued {
					overflowSource = []byte("@F\n{\n" + test.command + "　" + test.overflow + "\n}\n")
				}
				overflowResult := format.FormatWithOptions(overflowSource, format.ERB, formatOptions)
				if err := checkSourcePreservedOptions(overflowSource, overflowResult.Output, options); err != nil {
					t.Fatalf("overflow suffix: %v\noutput=%q", err, overflowResult.Output)
				}
				mutated := bytes.Replace(overflowResult.Output, []byte("　ignored"), []byte(" ignored"), 1)
				if bytes.Equal(mutated, overflowResult.Output) {
					t.Fatalf("fixed suffix mutation was not applied: %q", overflowResult.Output)
				}
				if err := checkSourcePreservedOptions(overflowSource, mutated, options); err == nil {
					t.Fatal("accepted fixed-arity overflow suffix mutation")
				}
			})
		}
	}
}

func TestCheckSourcePreservedRetainsUnknownIndent(t *testing.T) {
	original := []byte("@F\nCUSTOM foo\n")
	formatted := []byte("@F\n    CUSTOM foo\n")
	if err := checkSourcePreserved(original, formatted); err == nil {
		t.Fatal("unknown statement indentation mutation was accepted")
	}
}

func isFingerprintWord(b byte) bool {
	return b >= 0x80 || b == '_' || b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
}
