package format

import (
	"bytes"
	"testing"
)

func commandOptions() Options {
	return Options{Select: []RuleID{
		FunctionIndent, InstructionIndent,
		CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing,
		IndexSpacing, FormPadding, AssignmentLeftSpacing, AssignmentRightSpacing,
	}}
}

func TestRegisteredCommandsFormatBoundedArguments(t *testing.T) {
	input := []byte("@F\nSETCOLOR 1+2,3*4,5\nPRINTBUTTON \"x\", A : 0\nBAR A, 1+2, 3\nARRAYREMOVE ARR : 0,1+2,3\nARRAYSORT ARR, FORWARD, 1+2, 3\nSORTCHARA ARR, BACK\nHTML_TAGSPLIT \"<b>\", OUT, COUNT\n")
	want := []byte("@F\n    SETCOLOR 1 + 2, 3 * 4, 5\n    PRINTBUTTON \"x\", A:0\n    BAR A, 1 + 2, 3\n    ARRAYREMOVE ARR:0, 1 + 2, 3\n    ARRAYSORT ARR, FORWARD, 1 + 2, 3\n    SORTCHARA ARR, BACK\n    HTML_TAGSPLIT \"<b>\", OUT, COUNT\n")
	r := FormatWithOptions(input, ERB, commandOptions())
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
	again := FormatWithOptions(r.Output, ERB, commandOptions())
	if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
		t.Fatalf("not idempotent: %+v", again)
	}
}

func TestRegisteredFormCommandsPreserveLiteralPayload(t *testing.T) {
	input := []byte("@F\nTHROW  raw { A + 1 } ; literal\nPUTFORM  raw { B+2 }\nDEBUGPRINTFORM  raw ; literal\nPRINTN  raw  ; literal\nPRINTFORMN  raw { C+3 }\n")
	common := FormatWithOptions(input, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, FormPadding, BinarySpacing}, Dialect: CommonDialect})
	wantCommon := []byte("@F\n    THROW  raw {A + 1} ; literal\n    PUTFORM  raw {B + 2}\n    DEBUGPRINTFORM  raw ; literal\nPRINTN  raw  ; literal\nPRINTFORMN  raw { C+3 }\n")
	if !bytes.Equal(common.Output, wantCommon) || len(common.Diagnostics) != 2 {
		t.Fatalf("common output=%q diagnostics=%+v want=%q", common.Output, common.Diagnostics, wantCommon)
	}
	emee := FormatWithOptions(input, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, FormPadding, BinarySpacing}, Dialect: EMEE})
	wantEMEE := []byte("@F\n    THROW  raw {A + 1} ; literal\n    PUTFORM  raw {B + 2}\n    DEBUGPRINTFORM  raw ; literal\n    PRINTN  raw  ; literal\n    PRINTFORMN  raw {C + 3}\n")
	if !bytes.Equal(emee.Output, wantEMEE) || len(emee.Diagnostics) != 0 {
		t.Fatalf("emee output=%q diagnostics=%+v want=%q", emee.Output, emee.Diagnostics, wantEMEE)
	}
}

func TestRegisteredInputDialectShapes(t *testing.T) {
	input := []byte("@F\nINPUT 1+2, 3+4, 5+6\nTINPUT 1,2,,4,5\nINPUTS raw { A+1 }, 1+2\n")
	common := FormatWithOptions(input, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing, CommaSpacing, FormPadding}, Dialect: CommonDialect})
	if len(common.Diagnostics) != 2 || !bytes.Contains(common.Output, []byte("INPUTS raw { A+1 }, 1+2")) {
		t.Fatalf("common=%+v", common)
	}
	emuera := FormatWithOptions(input, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing, CommaSpacing, FormPadding}, Dialect: Emuera1824})
	if len(emuera.Diagnostics) != 2 {
		t.Fatalf("emuera diagnostics=%+v", emuera.Diagnostics)
	}
	if !bytes.Contains(emuera.Output, []byte("INPUTS raw {A + 1}, 1+2")) {
		t.Fatalf("emuera output=%q", emuera.Output)
	}
	emee := FormatWithOptions(input, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing, CommaSpacing, FormPadding}, Dialect: EMEE})
	if len(emee.Diagnostics) != 1 || !bytes.Contains(emee.Output, []byte("INPUTS raw {A + 1}, 1 + 2")) {
		t.Fatalf("emee=%+v", emee)
	}
}

func TestRegisteredModernCommandsAreDialectGated(t *testing.T) {
	input := []byte("@F\nPRINTVN A+1\nPRINTSN RESULTS+\"x\"\nPRINTFORMN raw { B+2 }\n")
	common := FormatWithOptions(input, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing, FormPadding}, Dialect: CommonDialect})
	if !bytes.Equal(common.Output, input) || len(common.Diagnostics) != 3 {
		t.Fatalf("common=%+v", common)
	}
	emee := FormatWithOptions(input, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing, FormPadding}, Dialect: EMEE})
	want := []byte("@F\n    PRINTVN A + 1\n    PRINTSN RESULTS + \"x\"\n    PRINTFORMN raw {B + 2}\n")
	if !bytes.Equal(emee.Output, want) || len(emee.Diagnostics) != 0 {
		t.Fatalf("emee output=%q diagnostics=%+v want=%q", emee.Output, emee.Diagnostics, want)
	}
}

func TestRegisteredCommandShapeFailureKeepsLine(t *testing.T) {
	input := []byte("@F\nSETCOLOR 1,2\nARRAYREMOVE 1+2,3,4\nSORTCHARA 1+2, BACK\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing, CommaSpacing}})
	want := []byte("@F\n    SETCOLOR 1,2\n    ARRAYREMOVE 1+2,3,4\n    SORTCHARA 1+2, BACK\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 3 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
}

func TestRegisteredCommaOwnershipAndRawSeparators(t *testing.T) {
	input := []byte("@F\nINPUT    ,1+2\nINPUTS  raw { A+1 } ,1+2\nWAIT;raw  \nTHROW; raw { A+1 }\n")
	options := Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing, CommaSpacing, FormPadding, InlineCommentSpacing, CodeTrailingWhitespace}, Dialect: EMEE}
	r := FormatWithOptions(input, ERB, options)
	want := []byte("@F\n    INPUT    , 1 + 2\n    INPUTS  raw {A + 1} , 1 + 2\n    WAIT;raw  \n    THROW; raw {A + 1}\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
}

func TestRegisteredCommandRuleSelection(t *testing.T) {
	input := []byte("@F\nSETCOLOR 1+2,3,4\n")
	binary := FormatWithOptions(input, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing}})
	if !bytes.Contains(binary.Output, []byte("SETCOLOR 1 + 2,3,4")) {
		t.Fatalf("binary output=%q", binary.Output)
	}
	comma := FormatWithOptions(input, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, CommaSpacing}})
	if !bytes.Contains(comma.Output, []byte("SETCOLOR 1+2, 3, 4")) {
		t.Fatalf("comma output=%q", comma.Output)
	}
}

func TestRegisteredNoArgAndRequiredScalarBoundaries(t *testing.T) {
	input := []byte("@F\nWAIT;raw  \nWAIT ;comment  \nWAIT payload  \nSETFONT\nLOADDATA\nDELCHARA 1+2,3+4\n")
	options := Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing, CommaSpacing, InlineCommentSpacing, CommentTrailingWhitespace, CodeTrailingWhitespace}}
	r := FormatWithOptions(input, ERB, options)
	want := []byte("@F\n    WAIT;raw  \n    WAIT ;comment\n    WAIT payload  \n    SETFONT\n    LOADDATA\n    DELCHARA 1 + 2, 3 + 4\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 1 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
}

func TestRegisteredInputFormRollback(t *testing.T) {
	for _, arg := range []string{"raw { A+1 },1+2,3+4,5+6", "raw { A+1 },1+2,B+("} {
		input := []byte("@F\nINPUTS " + arg + "\n")
		options := Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing, CommaSpacing, FormPadding}, Dialect: EMEE}
		r := FormatWithOptions(input, ERB, options)
		want := []byte("@F\n    INPUTS " + arg + "\n")
		if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 1 {
			t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
		}
	}
}

func TestRegisteredInputFormKeepsWhitespaceLiteral(t *testing.T) {
	input := []byte("@F\nINPUTS   \nONEINPUTS; \t\n")
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		r := FormatWithOptions(input, ERB, Options{Dialect: dialect})
		want := []byte("@F\n    INPUTS   \n    ONEINPUTS; \t\n")
		if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
			t.Fatalf("dialect=%v output=%q diagnostics=%+v", dialect, r.Output, r.Diagnostics)
		}
	}
}

func TestRegisteredProvidedFieldsMustBeNonempty(t *testing.T) {
	for _, body := range []string{
		"SETCOLOR 1+2,,3",
		"ARRAYSHIFT ARR,1+2,B,,5",
		"ADDCHARA 1+2,,",
		"DRAWLINEFORM",
	} {
		src := []byte("@F\n" + body + "\n")
		r := FormatWithOptions(src, ERB, Options{Select: []RuleID{BinarySpacing, CommaSpacing, FormPadding}})
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || len(r.Changes) != 0 {
			t.Fatalf("%q output=%q diagnostics=%+v changes=%+v", body, r.Output, r.Diagnostics, r.Changes)
		}
	}
}

func TestRegisteredCommandTerminalCommas(t *testing.T) {
	input := []byte("@F\n" +
		"TINPUT 15,-1,0,\n" +
		"PRINTBUTTON \"x\",1,\n" +
		"SAVEDATA 1,\"x\",\n" +
		"BAR 1,2,3,\n" +
		"ADDCHARA 1+2,\n" +
		"SETCOLOR 1,\n" +
		"PRINTBUTTON \"x\",1,,\n" +
		"SAVEDATA 1,\"x\",,\n" +
		"BAR 1,2,3,,\n" +
		"TINPUT 15,-1,0,,\n" +
		"ADDCHARA 1+2,,\n" +
		"SETCOLOR 1,,3\n")
	r := FormatWithOptions(input, ERB, commandOptions())
	want := []byte("@F\n" +
		"    TINPUT 15, -1, 0,\n" +
		"    PRINTBUTTON \"x\", 1,\n" +
		"    SAVEDATA 1, \"x\",\n" +
		"    BAR 1, 2, 3,\n" +
		"    ADDCHARA 1 + 2,\n" +
		"    SETCOLOR 1,\n" +
		"    PRINTBUTTON \"x\",1,,\n" +
		"    SAVEDATA 1,\"x\",,\n" +
		"    BAR 1,2,3,,\n" +
		"    TINPUT 15,-1,0,,\n" +
		"    ADDCHARA 1+2,,\n" +
		"    SETCOLOR 1,,3\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 6 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
	for _, line := range []string{
		"    TINPUT 15, -1, 0,",
		"    PRINTBUTTON \"x\", 1,",
		"    SAVEDATA 1, \"x\",",
		"    BAR 1, 2, 3,",
		"    ADDCHARA 1 + 2,",
		"    SETCOLOR 1,",
	} {
		if !bytes.Contains(r.Output, []byte(line+"\n")) {
			t.Fatalf("missing formatted terminal comma line %q in %q", line, r.Output)
		}
	}
	again := FormatWithOptions(r.Output, ERB, commandOptions())
	if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 || len(again.Diagnostics) != len(r.Diagnostics) {
		t.Fatalf("not idempotent: output=%q changes=%+v diagnostics=%+v", again.Output, again.Changes, again.Diagnostics)
	}
}

func TestSAVEVARFormatsStringAndVariableFields(t *testing.T) {
	input := []byte("@F\nSAVEVAR  \"file\",\"message\",RESULT, RESULTS\n")
	want := []byte("@F\n    SAVEVAR \"file\", \"message\", RESULT, RESULTS\n")
	r := FormatWithOptions(input, ERB, commandOptions())
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
	again := FormatWithOptions(r.Output, ERB, commandOptions())
	if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 || len(again.Diagnostics) != 0 {
		t.Fatalf("not idempotent: %+v", again)
	}
}

func TestSAVEVARRequiresVariableTail(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing, FormPadding}}
	for _, body := range []string{
		"SAVEVAR \"file\",\"message\"",
		"SAVEVAR \"file\",\"message\",RESULT,,RESULTS",
	} {
		src := []byte("@F\n" + body + "\n")
		r := FormatWithOptions(src, ERB, options)
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || len(r.Changes) != 0 {
			t.Fatalf("%q output=%q diagnostics=%+v changes=%+v", body, r.Output, r.Diagnostics, r.Changes)
		}
	}
}

func TestExpandedRegisteredCommands(t *testing.T) {
	input := []byte("@F\nRESETCOLOR ; keep\nRESTART ; keep\nBEGIN   title ; raw  \nSETCOLORBYNAME  name ; raw  \nVARSIZE ARR\nTIMES (A:0),0.5\nSWAP (A:0),(B:0)\n")
	r := FormatWithOptions(input, ERB, commandOptions())
	want := []byte("@F\n    RESETCOLOR ; keep\n    RESTART ; keep\n    BEGIN   title ; raw  \n    SETCOLORBYNAME  name ; raw  \n    VARSIZE ARR\n    TIMES (A:0), 0.5\n    SWAP (A:0), (B:0)\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
}

func TestModernRegisteredCommandsAndDialectGate(t *testing.T) {
	input := []byte("@F\nPLAYBGM \"sound\"\nPLAYSOUND \"x\",1+2\nTOOLTIP_SETCOLOR 1+2,3+4\nHTML_PRINT_ISLAND \"html\"\nVARSIZE ARR\n")
	common := FormatWithOptions(input, ERB, Options{Select: commandOptions().Select, Dialect: CommonDialect})
	commonWant := []byte("@F\nPLAYBGM \"sound\"\nPLAYSOUND \"x\",1+2\n    TOOLTIP_SETCOLOR 1 + 2, 3 + 4\nHTML_PRINT_ISLAND \"html\"\n    VARSIZE ARR\n")
	if len(common.Diagnostics) != 3 || !bytes.Equal(common.Output, commonWant) {
		t.Fatalf("common output=%q diagnostics=%+v", common.Output, common.Diagnostics)
	}
	emee := FormatWithOptions(input, ERB, Options{Select: commandOptions().Select, Dialect: EMEE})
	want := []byte("@F\n    PLAYBGM \"sound\"\n    PLAYSOUND \"x\", 1 + 2\n    TOOLTIP_SETCOLOR 1 + 2, 3 + 4\n    HTML_PRINT_ISLAND \"html\"\n    VARSIZE ARR\n")
	if !bytes.Equal(emee.Output, want) || len(emee.Diagnostics) != 0 {
		t.Fatalf("emee output=%q diagnostics=%+v want=%q", emee.Output, emee.Diagnostics, want)
	}
}

func TestDTColumnOptionsGrammar(t *testing.T) {
	input := []byte("@F\nDT_COLUMN_OPTIONS DT,COL,default,1+2\nDT_COLUMN_OPTIONS DT,COL,unknown,1\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing, CommaSpacing}, Dialect: EMEE})
	want := []byte("@F\n    DT_COLUMN_OPTIONS DT, COL, default, 1 + 2\n    DT_COLUMN_OPTIONS DT,COL,unknown,1\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 1 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
}

func TestVARSIZEDefersUnknownAndWarnsKnownSuffix(t *testing.T) {
	input := []byte("@F\nVARSIZE ARR:0\nVARSIZE A+B\n")
	r := FormatWithOptions(input, ERB, commandOptions())
	want := []byte("@F\n    VARSIZE ARR:0\n    VARSIZE A+B\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "ignored_arguments" || len(r.Changes) != 2 {
		t.Fatalf("output=%q diagnostics=%+v changes=%+v", r.Output, r.Diagnostics, r.Changes)
	}
}

func TestVARSIZEPreservesRuntimeIgnoredSuffixes(t *testing.T) {
	input := []byte("@F\nVARSIZE RESULT,1\nVARSIZE DA,1+2\nVARSIZE ITEMPRICE,1\n")
	r := FormatWithOptions(input, ERB, commandOptions())
	want := []byte("@F\n    VARSIZE RESULT,1\n    VARSIZE DA,1+2\n    VARSIZE ITEMPRICE,1\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 3 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Code != "ignored_arguments" || diagnostic.Severity != Warning {
			t.Fatalf("diagnostics=%+v", r.Diagnostics)
		}
	}
}

func TestVARSIZERejectsScalarsAndRand(t *testing.T) {
	input := []byte("@F\nVARSIZE RAND\nVARSIZE RAND,1\nVARSIZE NO,1\n")
	r := FormatWithOptions(input, ERB, commandOptions())
	want := []byte("@F\n    VARSIZE RAND\n    VARSIZE RAND,1\n    VARSIZE NO,1\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 3 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Code != "unsupported_syntax" || diagnostic.Severity != Unsupported {
			t.Fatalf("diagnostics=%+v", r.Diagnostics)
		}
	}
}

func TestVARSIZEDefersMarkedAndUnknownSuffixes(t *testing.T) {
	input := []byte("@F\nVARSIZE ;!; RESULT,1\nVARSIZE UNKNOWN,1\n")
	r := FormatWithOptions(input, ERB, commandOptions())
	want := []byte("@F\n    VARSIZE ;!; RESULT,1\n    VARSIZE UNKNOWN,1\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v want=%q", r.Output, r.Diagnostics, want)
	}
}

func TestRegisteredFormAnyCommandsPreserveLiteralPayload(t *testing.T) {
	input := []byte("@F\nSETBGIMAGE  image { A+1 }, level { B+2 }, opacity ; raw, text\nREMOVEBGIMAGE name { C+3 }; literal, text\n")
	emee := FormatWithOptions(input, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, CommandSpacing, BinarySpacing, FormPadding}, Dialect: EMEE})
	want := []byte("@F\n    SETBGIMAGE image {A + 1}, level {B + 2}, opacity ; raw, text\n    REMOVEBGIMAGE name {C + 3}; literal, text\n")
	if !bytes.Equal(emee.Output, want) || len(emee.Diagnostics) != 0 {
		t.Fatalf("emee output=%q diagnostics=%+v want=%q", emee.Output, emee.Diagnostics, want)
	}
	common := FormatWithOptions(input, ERB, Options{Select: []RuleID{FunctionIndent, InstructionIndent, BinarySpacing, FormPadding}, Dialect: CommonDialect})
	if !bytes.Equal(common.Output, input) || len(common.Diagnostics) != 2 {
		t.Fatalf("common output=%q diagnostics=%+v", common.Output, common.Diagnostics)
	}
}

func TestUnrecognizedUnicodeStatementPreservesIndent(t *testing.T) {
	for _, src := range []string{
		"@F\n\t\tαβ text = GETVAR(\"x\") + 1  \nRETURN 0\n",
		"@F\nSIF 1\n\t\tαβ text = 1  \nRETURN 0\n",
	} {
		result := FormatWithOptions([]byte(src), ERB, Options{})
		if !bytes.Contains(result.Output, []byte("\t\tαβ text = ")) || !bytes.Contains(result.Output, []byte("  \n")) {
			t.Fatalf("unrecognized line changed: %q", result.Output)
		}
		if !bytes.Contains(result.Output, []byte("    RETURN 0\n")) {
			t.Fatalf("subsequent statement not formatted: %q", result.Output)
		}
	}
}
