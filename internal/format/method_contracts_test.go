package format

import (
	"bytes"
	"strings"
	"testing"
)

func methodContractResult(dialect Dialect, line string) Result {
	return FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(dialect))
}

func assertMethodContractValid(t *testing.T, dialect Dialect, lines ...string) {
	t.Helper()
	for _, line := range lines {
		r := methodContractResult(dialect, line)
		if len(r.Diagnostics) != 0 {
			t.Errorf("dialect=%v valid %q: diagnostics=%+v output=%q", dialect, line, r.Diagnostics, r.Output)
		}
	}
}

func assertMethodContractInvalid(t *testing.T, dialect Dialect, lines ...string) {
	t.Helper()
	for _, line := range lines {
		r := methodContractResult(dialect, line)
		if len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "method") {
			t.Errorf("dialect=%v invalid %q: diagnostics=%+v output=%q", dialect, line, r.Diagnostics, r.Output)
		}
	}
}

func assertMethodContractDeferred(t *testing.T, dialect Dialect, lines ...string) {
	t.Helper()
	for _, line := range lines {
		r := methodContractResult(dialect, line)
		if len(r.Diagnostics) != 0 {
			t.Errorf("dialect=%v deferred %q: diagnostics=%+v output=%q", dialect, line, r.Diagnostics, r.Output)
		}
	}
}

func TestExpandedMethodContractsByDialect(t *testing.T) {
	assertMethodContractValid(t, CommonDialect,
		"GCLEAR 1,2", `GCREATEFROMFILE 1,"x"`, `CBGSETSPRITE "x",1,2,3`,
		`CBGSETBUTTONSPRITE 1,"a","b",2,3,4,"c"`,
		`CBGSETBUTTONSPRITE 1,"a","b",2,3,4`,
	)
	assertMethodContractInvalid(t, CommonDialect,
		"GCLEAR 1", `GCREATEFROMFILE 1,"x",0`, `CBGSETSPRITE 1,1,2,3`,
		`CBGSETBUTTONSPRITE 1,"a","b",2,3`,
		`CBGSETBUTTONSPRITE 1,"a","b",2,3,4,,`,
	)

	assertMethodContractValid(t, EMEE,
		"GCLEAR 1,2", "GCLEAR 1,2,3,4,5,6", `GCREATEFROMFILE 1,"x"`, `GCREATEFROMFILE 1,"x",0`,
		`CBGSETSPRITE "x",1,2,3`, `CBGSETBUTTONSPRITE 1,"a","b",2,3,4`,
		`CBGSETBUTTONSPRITE 1,"a","b",2,3,4,"c"`, "GDRAWTEXT 1,\"x\"", "GDRAWTEXT 1,\"x\",2,3",
		`GGETTEXTSIZE "x","Arial",12`, `GGETTEXTSIZE "x","Arial",12,1`,
		"GDRAWGWITHROTATE 1,2,3", "GDRAWGWITHROTATE 1,2,3,4,5", "GDRAWLINE 1,2,3,4,5",
		"GDASHSTYLE 1,2,3", `SETTEXTBOX "x"`, "HOTKEY_STATE 1,2",
	)
	assertMethodContractInvalid(t, EMEE,
		"GCLEAR 1,2,3", `GCREATEFROMFILE 1,"x",0,1`, `CBGSETBUTTONSPRITE 1,"a","b",2,3`,
		`CBGSETBUTTONSPRITE 1,"a","b",2,3,4,,`, "GDRAWTEXT 1", `GDRAWTEXT 1,"x",2,"bad"`, `GGETTEXTSIZE "x",1,12`,
		"GDRAWGWITHROTATE 1,2", "GDRAWLINE 1,2,3,4", "GDASHSTYLE 1,2", `SETTEXTBOX 1`, "HOTKEY_STATE", "HOTKEY_STATE 1",
	)
}

func TestVariadicMethodContracts(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		assertMethodContractValid(t, dialect,
			"MIN 1", "MIN 1,2,3", "MIN 1,", "MAX 1,2",
			"GROUPMATCH 1,2,3", `GROUPMATCH "a","b",`, "NOSAMES 1,2", `ALLSAMES "a","b"`,
		)
		assertMethodContractInvalid(t, dialect,
			"MIN", `MIN "x"`, `MIN 1,"x"`, "MIN 1,,2", "GROUPMATCH",
			`GROUPMATCH 1,"x"`, "GROUPMATCH 1,,2", "NOSAMES 1,2,,3",
		)
		assertMethodContractDeferred(t, dialect,
			"MIN UNKNOWN", `MIN UNKNOWN,"x"`, "MIN 1,UNKNOWN,\"x\"",
			"GROUPMATCH UNKNOWN,1", `GROUPMATCH 1,UNKNOWN,"x"`, "GROUPMATCH ABS(1),1",
			"NOSAMES 1,UNKNOWN,2", "ALLSAMES UNKNOWN,1,\"x\"",
		)
	}
}

func TestMatrixMethodContracts(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		assertMethodContractValid(t, dialect,
			`GDRAWSPRITE 1,"x"`, `GDRAWSPRITE 1,"x",2,3`, `GDRAWSPRITE 1,"x",2,3,4,5`,
			`GDRAWSPRITE 1,"x",2,3,4,5,DA`, `GDRAWSPRITE 1,"x",2,3,4,5,CDFLAG`,
			`GDRAWG 1,2,3,4,5,6,7,8,9,10`, `GDRAWG 1,2,3,4,5,6,7,8,9,10,TA:0:0:0`,
			`GDRAWSPRITE 1,"x",2,3,4,5,CDFLAG:0:0:0`,
		)
		assertMethodContractInvalid(t, dialect,
			`GDRAWSPRITE 1`, `GDRAWSPRITE 1,"x",2`, `GDRAWSPRITE 1,"x",2,3,4,5,6`,
			`GDRAWSPRITE 1,"x",2,3,4,5,1`, `GDRAWSPRITE 1,"x",2,3,4,5,"x"`,
			`GDRAWSPRITE 1,"x",2,3,4,5,CDFLAG:0:0`, `GDRAWSPRITE 1,"x",2,3,4,5,DA:1`,
			`GDRAWG 1,2,3,4,5,6,7,8,9`, `GDRAWG 1,2,3,4,5,6,7,8,9,10,1`,
		)
		assertMethodContractDeferred(t, dialect,
			`GDRAWSPRITE 1,"x",2,3,4,5,UNKNOWN`, `GDRAWSPRITE 1,"x",2,3,4,5,F(1)`,
			`GDRAWG 1,2,3,4,5,6,7,8,9,10,UNKNOWN`, `GDRAWG 1,2,3,4,5,6,7,8,9,10,USR_X:0`,
			`GDRAWSPRITE 1,"x",UNKNOWN`, `GDRAWSPRITE 1,"x",2,3,4,5,DA:RESULT:0`,
			`GDRAWG 1,2,3,4,5,6,7,8,9,10,TA:(1):0:0`,
		)
		if dialect == EMEE {
			assertMethodContractInvalid(t, dialect,
				`GDRAWSPRITE 1,"x",2,3,4,5,,`,
			)
			assertMethodContractDeferred(t, dialect,
				`GDRAWG 1,2,3,4,5,6,7,8,9,10,,`,
			)
		} else {
			assertMethodContractInvalid(t, dialect,
				`GDRAWSPRITE 1,"x",2,3,4,5,,`,
				`GDRAWG 1,2,3,4,5,6,7,8,9,10,,`,
			)
		}
	}
}

func TestMethodContractsRespectSelectionAndIdempotence(t *testing.T) {
	input := []byte("@F\n    GCLEAR 1\n    GCLEAR 1 + 2,3\n")
	quiet := FormatWithOptions(input, ERB, Options{Select: []RuleID{}})
	if !bytes.Equal(quiet.Output, input) || len(quiet.Diagnostics) != 0 {
		t.Fatalf("selection-none changed or diagnosed input: %+v", quiet)
	}
	indented := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
	if !bytes.Equal(indented.Output, input) || len(indented.Diagnostics) != 0 {
		t.Fatalf("indent-only changed or diagnosed input: %+v", indented)
	}
	formatted := methodContractResult(CommonDialect, "GCLEAR 1 + 2,3")
	again := FormatWithOptions(formatted.Output, ERB, methodOptions(CommonDialect))
	if len(formatted.Diagnostics) != 0 || !bytes.Equal(formatted.Output, again.Output) || len(again.Changes) != 0 {
		t.Fatalf("method formatting not idempotent: first=%+v again=%+v", formatted, again)
	}
}

func TestAdditionalMethodContracts(t *testing.T) {
	assertMethodContractValid(t, CommonDialect,
		`SAVETEXT "text",1`, `SAVETEXT "text",1,0,1`,
		`LOADTEXT 1`, `LOADTEXT 1,0,1`,
	)
	assertMethodContractInvalid(t, CommonDialect,
		"SAVETEXT", `SAVETEXT 1,1`, `SAVETEXT "text",1,,3`, `SAVETEXT "text",1,2,3,4`,
		"LOADTEXT", `LOADTEXT "path"`, `LOADTEXT 1,,2`, `LOADTEXT 1,2,3,4`,
	)

	assertMethodContractValid(t, EMEE,
		`SAVETEXT "text","path"`,
		`LOADTEXT "path"`,
		`SETVAR "RESULT",1+2`, `VARSETEX "RESULT",1,1,2,3`,
		`ENUMFILES "data","*.csv",1,RESULTS`,
		`DT_COLUMN_ADD "table","column"`, `DT_COLUMN_ADD "table","column",STRING,1`,
		`DT_ROW_ADD "table"`, `DT_ROW_ADD "table","column",1,"other",2`, `DT_ROW_ADD "table","column",1,`,
		`DT_ROW_ADD "table",RESULTS,RESULT,1`,
		`DT_ROW_REMOVE "table",1`, `DT_ROW_REMOVE "table",1,`, `DT_ROW_REMOVE "table",RESULT,1`,
		`DT_CELL_SET "table",1,"column"`, `DT_CELL_SET "table",1,"column",1,0`,
		`DT_SELECT "table"`, `DT_SELECT "table","filter","sort",RESULT`,
		`DT_TOXML "table"`, `DT_TOXML "table",RESULTS`,
		`MOVETEXTBOX 1,2,3`, `RESUMETEXTBOX`,
		`GETMETH "METHOD"`, `GETMETH "METHOD",1,2`,
		`GETMETHS "METHOD"`, `GETMETHS "METHOD","fallback",1`,
	)
	assertMethodContractInvalid(t, EMEE,
		`SAVETEXT 1,"path"`, `SAVETEXT "text","path",,1`, `SAVETEXT "text","path",1,2,3`,
		`LOADTEXT "path",,`, `LOADTEXT 1,2,3,4`, `SETVAR 1,2`, `SETVAR "RESULT"`,
		`VARSETEX 1,2`, `ENUMFILES 1`,
		`DT_COLUMN_ADD 1,"column"`, `DT_COLUMN_ADD "table",1`,
		`DT_ROW_ADD 1,"column",1`, `DT_ROW_ADD "table",1,2`,
		`DT_ROW_ADD "table","column",1,"dangling"`, `DT_ROW_ADD "table","column",1,,`,
		`DT_ROW_REMOVE`, `DT_ROW_REMOVE "table"`, `DT_ROW_REMOVE "table",,1`, `DT_ROW_REMOVE "table",1,2`, `DT_ROW_REMOVE "table",1,2,3`,
		`DT_CELL_SET "table","row","column"`, `DT_CELL_SET "table",1,1`,
		`DT_SELECT 1`, `DT_TOXML 1`, `DT_TOXML "table",1`, `DT_TOXML "table",GAMEBASE_TITLE`, `DT_TOXML "table",NAME`, `DT_TOXML "table",NAME:0`, `DT_TOXML "table",1,2`,
		`ENUMFILES "dir","*.csv",1,1`,
		`MOVETEXTBOX 1,2`, `MOVETEXTBOX 1,2,3,4`, `RESUMETEXTBOX 1`,
		`GETMETH 1`, `GETMETH "METHOD","fallback"`,
		`GETMETHS 1`, `GETMETHS "METHOD",1`,
	)
	assertMethodContractDeferred(t, EMEE,
		`SETVAR "RESULT",UNKNOWN`, `DT_COLUMN_ADD "table","column",UNKNOWN`,
		`DT_ROW_ADD UNKNOWN,"column",1`, `DT_ROW_ADD "table",RESULTS:0,RESULT:0,1`,
		`DT_ROW_REMOVE "table",UNKNOWN,1`, `DT_ROW_REMOVE "table",RESULT:0,1`,
		`DT_TOXML "table",UNKNOWN`,
		`ENUMFILES "dir","*.csv",1,UNKNOWN`, `ENUMFILES "dir","*.csv",1,RESULTS:0`,
		`GETMETH UNKNOWN,UNKNOWN`,
	)
}

func TestArrayMethodContracts(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		assertMethodContractValid(t, dialect,
			"RAND 1", "RAND 1,2", "RAND __LINE__", "RAND ITEMPRICE", "RAND RAND(1)",
			"RAND (RAND(1))", "RAND 1+RAND(1)",
			`GETNUM RESULT,"x"`, `GETNUM ITEMPRICE:0,"x"`,
			"FINDELEMENT RESULT,1", `FINDELEMENT ITEMNAME,"x"`,
			"FINDELEMENT RESULT,1,0,2,1", "FINDLASTELEMENT RESULT,1,0,2,1",
			"ARRAYMSORT RESULT,RESULT", "ARRAYMSORT RESULT:0,RESULT:0",
			"SUMARRAY RESULT", "SUMARRAY CDFLAG", "SUMCARRAY CDFLAG",
			"MATCH RESULT,1", `MATCH CSTR,"x"`, `CMATCH NAME,"x"`,
			"INRANGEARRAY RESULT,1,2",
			"STRJOIN RESULT", `STRJOIN RESULTS,"",,2`, "STRJOIN CFLAG", "STRJOIN CFLAG:0:0",
			`STRJOIN CFLAG:0:0,"",0,0`, `STRJOIN (CFLAG:0:0),"",0,0`,
			`STRJOIN CDFLAG:0:0:0,"",0,0`, "STRJOIN DA", "STRJOIN ITEMPRICE",
			`STRJOIN DA,"",0,1`, `STRJOIN ITEMPRICE,"",0,1`,
		)
		assertMethodContractInvalid(t, dialect,
			"RAND", "RAND 1,2,3", `RAND "x"`, "RAND 1,,2", "RAND RAND",
			"GETNUM", `GETNUM RAND,"x"`, `GETNUM RESULT,1`, `GETNUM "x","x"`,
			"FINDELEMENT", `FINDELEMENT NAME:0,"x"`,
			`FINDELEMENT RESULT,"x"`, "FINDELEMENT RESULT,1,0,2,1,2",
			"ARRAYMSORT", "ARRAYMSORT ITEMPRICE,RESULT", "ARRAYMSORT CFLAG,RESULT",
			`ARRAYMSORT RESULT,"x"`, "SUMARRAY", "SUMARRAY RAND", "SUMCARRAY NAME",
			"MATCH", "MATCH RESULT,\"x\"", "MATCH RESULT,1,2,3,4",
			"CMATCH", "CMATCH RESULT,1", "INRANGEARRAY", "INRANGEARRAY RAND,1,2",
			"INRANGEARRAY CDFLAG,1,2", "INRANGECARRAY NAME,1,2", "STRJOIN",
			"STRJOIN RAND", "STRJOIN NAME", `STRJOIN RESULTS,1`, "STRJOIN CSTR", "STRJOIN CSTR:0:0",
			`STRJOIN CSTR:(0),"",0,1`, `STRJOIN (CSTR:(1+1)),"",0,1`,
			`STRJOIN CSTR:(LOCAL+1),"",0,1`, `STRJOIN CFLAG:0:0,"",0,1`,
			`STRJOIN (CFLAG:0:0),"",0,1`, `STRJOIN CDFLAG:0:0:0,"",0,1`,
		)
		assertMethodContractDeferred(t, dialect,
			`GETNUM UNKNOWN,"x"`, "FINDELEMENT UNKNOWN,1", "ARRAYMSORT UNKNOWN,RESULT",
			"SUMARRAY UNKNOWN", `MATCH UNKNOWN,"x"`, "INRANGEARRAY UNKNOWN,1,2",
			"STRJOIN UNKNOWN", "SUMARRAY RESULT:INDEX", `STRJOIN CFLAG:INDEX,"",0,1`,
			`STRJOIN CFLAG:0:INDEX,"",0,1`, `STRJOIN (CSTR:0:INDEX),"",0,1`,
			`STRJOIN CFLAG:0:0,"",0,UNKNOWN`,
			"MATCH CFLAG:INDEX,1",
		)
		for _, line := range []string{`GETNUM RESULT@F,"x"`, "STRJOIN NAME@F"} {
			r := methodContractResult(dialect, line)
			if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_expression" {
				t.Errorf("dialect=%v qualified builtin %q: diagnostics=%+v", dialect, line, r.Diagnostics)
			}
		}
	}
	assertMethodContractValid(t, CommonDialect, "INRANGECARRAY ISASSI,1,2")
	assertMethodContractValid(t, Emuera1824, "INRANGECARRAY ISASSI,1,2")
	assertMethodContractInvalid(t, EMEE, "INRANGECARRAY ISASSI,1,2")
	assertMethodContractValid(t, EMEE, `GETNUM RESULT,"x",1`)
	assertMethodContractInvalid(t, CommonDialect, `GETNUM RESULT,"x",1`)
	assertMethodContractInvalid(t, EMEE, `GETNUM RESULT,"x",1,2`, `GETNUM RESULT,"x","scope"`)
	assertMethodContractValid(t, Emuera1824, "INRANGEARRAY RESULT,1,2,0,2,3", `INRANGEARRAY RESULT,1,2,0,2,"ignored"`)
	assertMethodContractInvalid(t, CommonDialect, "INRANGEARRAY RESULT,1,2,0,2,3")
	assertMethodContractInvalid(t, EMEE, "INRANGEARRAY RESULT,1,2,0,2,3")
}

func TestArrayMethodOptionalFields(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		assertMethodContractValid(t, dialect,
			"RAND ,1", "RAND 1,", `GETNUM RESULT,"x",`,
			"FINDELEMENT RESULT,1,", "FINDELEMENT RESULT,1,,,,",
			"ARRAYMSORT RESULT,RESULT,", "SUMARRAY RESULT,,1",
			"MATCH RESULT,1,,", "INRANGEARRAY RESULT,1,2,,,", "STRJOIN RESULT,,,,",
		)
		assertMethodContractInvalid(t, dialect,
			"RAND ,", "GETNUM RESULT,,", "FINDELEMENT RESULT,1,,,,,",
			"ARRAYMSORT RESULT,,", "MATCH RESULT,,1", "INRANGEARRAY RESULT,1,,",
			"STRJOIN RESULT,1,2",
		)
	}
}

func TestArrayMethodContractsRespectSelection(t *testing.T) {
	src := []byte("@F\nFINDELEMENT\nARRAYMSORT RESULT,CFLAG\nGETNUM RESULT,1\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{InstructionIndent}})
	want := []byte("@F\n    FINDELEMENT\n    ARRAYMSORT RESULT,CFLAG\n    GETNUM RESULT,1\n")
	if len(r.Diagnostics) != 0 || !bytes.Equal(r.Output, want) {
		t.Fatalf("selection-only result=%+v output=%q want=%q", r.Diagnostics, r.Output, want)
	}
}

func TestArrayMethodContractsPreserveExpressionErrors(t *testing.T) {
	r := methodContractResult(CommonDialect, `GETNUM RESULT,(1`)
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_expression" {
		t.Fatalf("malformed method expression result=%+v output=%q", r.Diagnostics, r.Output)
	}
}

func TestGetNumOptionalScopeMustHaveAValue(t *testing.T) {
	assertMethodContractValid(t, EMEE, `GETNUM RESULT,"x",`, `GETNUM RESULT,"x",1,`)
	assertMethodContractInvalid(t, EMEE, `GETNUM RESULT,"x",,`)
}
