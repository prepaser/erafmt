package format

import (
	"bytes"
	"slices"
	"testing"
)

func TestRuleSelection(t *testing.T) {
	tests := []struct {
		rule        RuleID
		input, want string
	}{
		{FunctionIndent, "  @F\nRETURN 0\n", "@F\nRETURN 0\n"},
		{DeclarationIndent, "@F\n#DIM USR_X\nRETURN 0\n", "@F\n    #DIM USR_X\nRETURN 0\n"},
		{LabelIndent, "@F\n$L\nRETURN 0\n", "@F\n    $L\nRETURN 0\n"},
		{InstructionIndent, "@F\nIF A+B\nRETURN 0\nENDIF\n", "@F\n    IF A+B\n        RETURN 0\n    ENDIF\n"},
		{SIFIndent, "@F\nSIF A\nRETURN 0\n", "@F\nSIF A\n        RETURN 0\n"},
		{CommentIndent, "@F\n; text  \nRETURN 0\n", "@F\n    ; text  \nRETURN 0\n"},
		{ContinuationIndent, "@F\n{\nPRINTL  text\n}\n", "@F\n    {\nPRINTL  text\n    }\n"},
		{CommandSpacing, "@F\nIF\t\tA+B\nENDIF\n", "@F\nIF A+B\nENDIF\n"},
		{BinarySpacing, "@F\nIF A+B?C#D\nENDIF\n", "@F\nIF A + B?C#D\nENDIF\n"},
		{TernarySpacing, "@F\nIF A+B?C#D\nENDIF\n", "@F\nIF A+B ? C # D\nENDIF\n"},
		{UnarySpacing, "@F\nIF - A+! B\nENDIF\n", "@F\nIF -A+!B\nENDIF\n"},
		{CommaSpacing, "@F\nIF F( A ,B,,C, )\nENDIF\n", "@F\nIF F( A, B,, C, )\nENDIF\n"},
		{DelimiterSpacing, "@F\nIF F ( A , B[ 0 ] )\nENDIF\n", "@F\nIF F(A , B[0])\nENDIF\n"},
		{IndexSpacing, "@F\nIF USR_A[ 0 : 1 ]\nENDIF\n", "@F\nIF USR_A[ 0:1 ]\nENDIF\n"},
		{CaseSpacing, "@F\nSELECTCASE USR_X\nCASE IS    +1,2   TO   3\nENDSELECT\n", "@F\nSELECTCASE USR_X\nCASE IS +1,2 TO 3\nENDSELECT\n"},
		{TokenSpacing, "@F\nIF USR_A  USR_B\nENDIF\n", "@F\nIF USR_A USR_B\nENDIF\n"},
		{PreprocessorIndent, "@F\n  [IF USR_X]\nRETURN 0\n  [ENDIF]\n", "@F\n[IF USR_X]\nRETURN 0\n[ENDIF]\n"},
		{AssignmentLeftSpacing, "@F\nA=  B+1\n", "@F\nA =  B+1\n"},
		{AssignmentRightSpacing, "@F\nA=  B+1\n", "@F\nA= B+1\n"},
		{DeclarationWordSpacing, "@F\n#DIM  CONST\tUSR_X,1=1\n", "@F\n#DIM CONST USR_X,1=1\n"},
		{DeclarationDefaultSpacing, "@F\n#DIM USR_X,2=1+2\n", "@F\n#DIM USR_X,2 = 1+2\n"},
		{InlineCommentSpacing, "@F\nRETURN 1; keep\n", "@F\nRETURN 1 ; keep\n"},
		{CommentPrefixSpacing, "@F\n;\t  text  \n", "@F\n; text  \n"},
		{PreprocessorFieldSpacing, "@F\n[ IF\t USR_X ]\nRETURN 0\n[ ENDIF ]\n", "@F\n[IF USR_X]\nRETURN 0\n[ENDIF]\n"},
		{FormPadding, "@F\nPRINTFORM raw { A+1 } % USR_TEXT %\n", "@F\nPRINTFORM raw {A+1} %USR_TEXT%\n"},
		{LabelMarkerSpacing, "@  F\n$\t LOOP\nRETURN 0\n", "@F\n$LOOP\nRETURN 0\n"},
		{FormConditionSpacing, "@F\nPRINTFORM \\@A? yes # no\\@\n", "@F\nPRINTFORM \\@ A ? yes # no\\@\n"},
		{PixelUnitSpacing, "@F\nPRINT_SPACE (A+1)    px\n", "@F\nPRINT_SPACE (A+1) px\n"},
		{BlankLineWhitespace, "@F\n \t\nRETURN 0\n", "@F\n\nRETURN 0\n"},
		{CodeTrailingWhitespace, "@F\nRETURN 0 \t\n", "@F\nRETURN 0\n"},
		{CommentTrailingWhitespace, "@F\n; text \t\n", "@F\n; text\n"},
		{FunctionSeparation, "@F\nRETURN 0\n; next\n@G\nRETURN 1\n", "@F\nRETURN 0\n\n; next\n@G\nRETURN 1\n"},
		{BlankLineRuns, "@F\n\n\nRETURN 0\n", "@F\n\nRETURN 0\n"},
		{FileBoundaryBlankLines, " \t\n\n@F\nRETURN 0\n \t\n\n", "@F\nRETURN 0\n"},
		{DeclarationBodySeparation, "@F\n#DIM USR_X\nRETURN USR_X\n", "@F\n#DIM USR_X\n\nRETURN USR_X\n"},
		{FinalNewline, "@F\nRETURN 0", "@F\nRETURN 0\n"},
		{LineEndingNormalization, "@F\r\nRETURN 0\r", "@F\nRETURN 0\n"},
	}
	if len(tests) != len(Rules()) {
		t.Fatal("missing rule isolation fixture")
	}
	for _, tt := range tests {
		t.Run(string(tt.rule), func(t *testing.T) {
			options := Options{Select: []RuleID{tt.rule}}
			if tt.rule == PixelUnitSpacing {
				options.Dialect = EMEE
			}
			r := FormatWithOptions([]byte(tt.input), ERB, options)
			if string(r.Output) != tt.want || len(r.Diagnostics) != 0 || len(r.Changes) == 0 {
				t.Fatalf("output %q, diagnostics %+v, changes %+v; want %q", r.Output, r.Diagnostics, r.Changes, tt.want)
			}
			for _, c := range r.Changes {
				if c.Rule != tt.rule || c.Line < 1 || c.Column < 1 {
					t.Fatalf("unexpected change: %+v", c)
				}
			}
			again := FormatWithOptions(r.Output, ERB, options)
			if !bytes.Equal(r.Output, again.Output) || len(again.Changes) != 0 {
				t.Fatalf("not idempotent: %+v", again)
			}
			options.Ignore = []RuleID{tt.rule}
			r = FormatWithOptions([]byte(tt.input), ERB, options)
			if string(r.Output) != tt.input || len(r.Changes) != 0 {
				t.Fatal("ignored rule produced changes")
			}
		})
	}
}

func TestChangesReflectCommittedEdits(t *testing.T) {
	input := []byte("@BAD\nIF A+B\n[[UNKNOWN]]\nENDIF\n@GOOD\nIF USR_X+Y\nRETURN 1\nENDIF\n")
	r := FormatWithOptions(input, ERB, Options{})
	if len(r.Changes) == 0 {
		t.Fatal("missing successful function changes")
	}
	lastLine, lastColumn := 0, 0
	var lastRule RuleID
	for _, c := range r.Changes {
		if c.Line <= 5 || c.Line < lastLine || (c.Line == lastLine && (c.Column < lastColumn || c.Column == lastColumn && c.Rule <= lastRule)) {
			t.Fatalf("rolled-back or unordered change: %+v", c)
		}
		lastLine, lastColumn = c.Line, c.Column
		lastRule = c.Rule
	}
	for _, src := range []string{"@F\nIF A+B\nWEND\n", "@F\nRETURN 1\n[IF USR_X]\n@G\n[ENDIF]\n"} {
		r = FormatWithOptions([]byte(src), ERB, Options{})
		if string(r.Output) != src || len(r.Changes) != 0 {
			t.Fatal("preserved file reported changes")
		}
	}
	input = []byte("@F\nIF A+(B\nRETURN 0\nENDIF\n")
	r = FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
	if len(r.Diagnostics) != 0 || len(r.Changes) != 3 {
		t.Fatalf("disabled expression rules affected indentation: %+v", r)
	}
}

func TestRuleValidationAndCatalogIsolation(t *testing.T) {
	for _, options := range []Options{{Select: []RuleID{"UNKNOWN"}}, {Ignore: []RuleID{"UNKNOWN"}}} {
		input := []byte("@F\nRETURN 0\n")
		r := FormatWithOptions(input, ERB, options)
		if !bytes.Equal(input, r.Output) || len(r.Changes) != 0 || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "invalid_rules" {
			t.Fatalf("invalid rule accepted: %+v", r)
		}
	}
	rules := Rules()
	rules[0].ID = "UNKNOWN"
	if Rules()[0].ID != FunctionIndent {
		t.Fatal("catalog can be modified by caller")
	}
	seen := make(map[RuleID]bool)
	for _, rule := range Rules() {
		if seen[rule.ID] || rule.Name == "" || rule.Description == "" {
			t.Fatalf("invalid catalog entry: %+v", rule)
		}
		seen[rule.ID] = true
	}
	r := FormatWithOptions([]byte("@F\r\nRETURN 0"), ERB, Options{EOL: EOL(255)})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "invalid_eol" || string(r.Output) != "@F\r\nRETURN 0" || len(r.Changes) != 0 {
		t.Fatalf("invalid EOL accepted: %+v", r)
	}
	r = FormatWithOptions([]byte("@F\r\nRETURN 0"), ERB, Options{Dialect: Dialect(255)})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "invalid_dialect" || string(r.Output) != "@F\r\nRETURN 0" || len(r.Changes) != 0 {
		t.Fatalf("invalid dialect accepted: %+v", r)
	}
}

func TestChangePositionsUseOriginalSource(t *testing.T) {
	r := FormatWithOptions([]byte("@F\nIF α+β\nENDIF\n"), ERB, Options{})
	want := []Change{
		{Rule: InstructionIndent, Line: 2, Column: 1},
		{Rule: BinarySpacing, Line: 2, Column: 5},
		{Rule: BinarySpacing, Line: 2, Column: 6},
		{Rule: InstructionIndent, Line: 3, Column: 1},
	}
	if !slices.Equal(r.Changes, want) {
		t.Fatalf("changes %+v, want %+v", r.Changes, want)
	}
}

func TestCommandSpacingDoesNotDependOnExpressionParsing(t *testing.T) {
	input := []byte("@F\nIF\t\tA+(B\nRETURN 0\nENDIF\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{CommandSpacing}})
	if string(r.Output) != "@F\nIF A+(B\nRETURN 0\nENDIF\n" || len(r.Diagnostics) != 0 || len(r.Changes) != 1 || r.Changes[0].Rule != CommandSpacing {
		t.Fatalf("separator correction depends on expression parsing: %+v", r)
	}
	r = FormatWithOptions(input, ERB, Options{})
	if string(r.Output) != "@F\n    IF A+(B\n        RETURN 0\n    ENDIF\n" || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_expression" {
		t.Fatalf("unsupported argument suppressed independent corrections: %+v", r)
	}
}

func FuzzRuleSelection(f *testing.F) {
	f.Add("\n\ufeff", uint64(0x1fffffffff))
	for _, input := range []string{
		"@F\nIF - -A+F( 1,- B,,C[ 0 : 2 ])?Y#Z ; keep\nENDIF\n",
		"@F\nSELECTCASE USR_X\nCASE IS(-1),2 TO 3\nENDSELECT\n",
		"@F\nIF A+(B\nRETURN 0\nENDIF\n",
		"@F\nPRINTV 'raw\\, literal  ,F('nested, A+B)\nRETURNFORM { 1+2 },{3+4}\n",
		"@F (ARG:0=1+2)\n#DIM CONST USR_X,2+3=4*5\nCALLFORM F{ A+1 }( 2+3 )\n",
		"@F\nA : F( 1+2 )=  raw  \nS'= \"a\"+\"b\"\n++ A:0\n",
		"\r\n@  F\n$ L\nLOCAL @ F:0+=1+2\nRETURNF \\@ A?text;,(#other\\@\n \t\r\n",
		"@F\nINPUTS prompt,1+2\nPRINTBUTTON \"a,b\",1+2\nARRAYSORT LOCAL @ F, BACK, 1+2\n",
		"@F\n#DIM USR_X\n;body\nTIMES USR_X : 0 ,1.25e-2\nSETVAR @\"a{USR_X+1}\",1+2\nPRINT_SPACE (USR_X+1)  PX ;note\n",
		"@　F\n{\nRETURN 1+\n2\n}\nRETURN A\x00B+1\n",
		"@F\nRETURNF @\"text{1+2}\nPRINTFORM x\x00{1,,RIGHT}\n",
		"@A\rRETURN 1\r@B\rRETURN 2",
		"@F\nIF 1\nELSE\nELSEIF 2\nENDIF\nREPEAT 2\nREPEAT 1\nREND\nREND\n",
		"@F\nARRAYSORT RESULTS,FORWARD,\"offset\"\nSPLIT M,RESULTS,1\n",
		"@F\nPRINTFORM {CSTR:MACRO} %RESULT:MACRO% {1,\"bad\"}\n",
		"@F\nSELECTCASE \"x\"\nCASE 1,\nRETURN 1,,2\nENDSELECT\n",
		"@F\n#DIM CONST USR_X,1=1\n#DIM REF STATIC USR_R\nPRINTDATA\nDATALIST\nENDLIST\nENDDATA\n",
		"@F\nPRINTFORM {1+} {2+3} {1,2,RESULT}\nPRINT_SPACE !!1  px\nARRAYSORT RESULTS,FORWARD,1+\n",
		"@F\nSPLIT ;!; RESULT,RESULTS,1\nHTML_TAGSPLIT ;!; \"x\",RESULTS,RESULT\nARRAYSHIFT RESULT,1,2,3,4,ignored\n",
		"@F\nPOWER RESULT,1,\nABS 1,\nPRINT_SPACE RESULTS==\"x\",\nPRINT_IMG \"x\",UNKNOWN,1\n",
		"@F\n#LOCALSIZE\n#LOCALSIZE 1\n#DIM UserVar\n#DIM UserVar\nTRYGOTOLIST\nFUNC TARGET(1)\nENDFUNC\n",
		"@EVENT_HELPER\n#LOCALSIZE 1+2\n@eventtrain\n#LOCALSIZE \"ignored\"\n#SINGLE\n",
		"@F\n{\n#DIM UserVar,1+\n2\n}\n#DIM UserVar\nRETURN 0\n",
		"@F\n#LOCALSIZE (1<<64)+2\n#FUNCTIONS\nRETURNF 1\nPRINT_SPACE 1;!;\tpx\nPRINT_IMG RE;!;SULT\n",
		"@F\nSAVEVAR \"x\",\"y\",RESULT:0,RESULT:1\nVARSET ITEMPRICE:MACRO,1\nTIMES RESULT,1e+2\nREDRAW 1+2,!!\n",
		"#DIM EFA_DUP,1=MACRO\n#DIMS EFA_DUP,2\n#DIM EFA_DUP,3\n",
		"@F\nSELECTCASE A\nCASE IS > ++A, A++ TO 2\nENDSELECT\nRETURN CDFLAGNAME1:\"key\"\n",
		"@F\nDT_COLUMN_OPTIONS \"db\",\"col\",DEFAULT,1+2,\n",
		"@F\n#FUNCTION\n{\n#DIM USR_X,0\n}\n#DIM USR_X\nRETURN 0\n",
		"@F\nCALL TARGET(1+2),2\nTRYCALL F(RESULT) MACRO\nCALLFORM F{1+2}(3+4)(5)\n",
		"@１２３\n{\n#FUNCTION\n}\nRETURNF 1\n@G\nRETURN 0\n",
	} {
		for dialect := uint64(0); dialect < 3; dialect++ {
			f.Add(input, uint64(0x1fffffffff)|dialect<<62)
		}
		f.Add(input, uint64(0xaaaaaaaa))
		f.Add(input, uint64(0x1fffffffff)|1<<40|1<<41)
		f.Add(input, uint64(0x1fffffffff)|1<<40|1<<41|1<<42)
		f.Add(input, uint64(0x1fffffffff)|1<<43)
		f.Add(input, uint64(0x1fffffffff)|2<<43)
	}
	f.Add("@A0(% =:)\n", uint64(0x4000001fffffffff))
	f.Add("@A(%=:)", uint64(0x4000001fffffffff))
	f.Add("SETVAR= ", uint64(0x8000001fffffff5e))
	f.Fuzz(func(t *testing.T, src string, mask uint64) {
		selected := make([]RuleID, 0)
		for i, r := range Rules() {
			if mask&(1<<i) != 0 {
				selected = append(selected, r.ID)
			}
		}
		options := Options{Select: selected, Dialect: Dialect((mask >> 62) % 3), EOL: EOL((mask >> 43) % 3)}
		options.AllowFullwidthSpace = mask&(1<<40) != 0
		if mask&(1<<41) != 0 {
			separator := ""
			if mask&(1<<42) != 0 {
				separator = " "
			}
			options.ContinuationSeparator = &separator
		}
		first := FormatWithOptions([]byte(src), ERB, options)
		second := FormatWithOptions(first.Output, ERB, options)
		if hasFatalDiagnostic(first.Diagnostics) != hasFatalDiagnostic(second.Diagnostics) {
			t.Fatalf("fatal status changed for %04x: %+v -> %+v", mask, first.Diagnostics, second.Diagnostics)
		}
		if !bytes.Equal(first.Output, second.Output) || len(second.Changes) != 0 {
			t.Fatalf("not idempotent for %04x: %q -> %q", mask, first.Output, second.Output)
		}
		for _, c := range first.Changes {
			if !slices.Contains(selected, c.Rule) {
				t.Fatalf("disabled rule changed source: %+v", c)
			}
		}
	})
}
