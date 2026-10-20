package format

import "testing"

func TestConsumedEmptyMethodArgumentsRejected(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		assertMethodContractInvalid(t, dialect, `HTML_GETPRINTEDSTR ,`)
		assertMethodContractValid(t, dialect,
			`HTML_GETPRINTEDSTR`, `HTML_GETPRINTEDSTR 1`, `HTML_GETPRINTEDSTR 1,`,
		)
		assertMethodContractDeferred(t, dialect, `HTML_GETPRINTEDSTR UNKNOWN`)
	}
	assertMethodContractInvalid(t, EMEE,
		`DT_COLUMN_ADD "t","c",,1`,
		`DT_CELL_GET "t",1,"c",,`,
		`DT_CELL_GETS "t",1,"c",,`,
		`DT_CELL_ISNULL "t",1,"c",,`,
		`DT_CELL_SET "t",1,"c",1,,`,
		`DT_TOXML "t",,`,
		`ENUMFILES "dir",,1`,
		`ENUMFILES "dir","*.csv",,RESULTS`,
		`ENUMFILES "dir","*.csv",1,,`,
		`CBGSETBUTTONSPRITE 1,"a","b",2,3,4,,`,
		`HTML_STRINGLEN "x",,`,
		`EXISTFUNCTION "F",,`,
		`FLOWINPUT 1,,2`,
		`FLOWINPUTS 1,,`,
		`OUTPUTLOG "x",,`,
		`OUTPUTLOG ,1`,
		`HOTKEY_STATE 1`,
		`SAVETEXT "x","path",,1`,
		`LOADTEXT "path",,`,
		`GCREATEFROMFILE 1,"x",,`,
		`GGETTEXTSIZE "x","Arial",12,,`,
		`GDRAWGWITHROTATE 1,2,3,4`,
		`GDRAWGWITHROTATE 1,2,3,,5`,
		`GDRAWTEXT 1,"x",,2`,
		`DT_ROW_ADD "t",,RESULTS,1`,
		`DT_ROW_ADD "t",RESULTS,,1`,
		`DT_ROW_ADD "t",,1`,
		`VARSETEX "RESULT",1,,`,
		`VARSETEX "RESULT",1,,2`,
	)
}

func TestEMEENullableMethodArgumentsRemainValid(t *testing.T) {
	assertMethodContractValid(t, EMEE,
		`CBGSETBUTTONSPRITE 1,"a","b",2,3,4`,
		`CBGSETBUTTONSPRITE 1,"a","b",2,3,4,"tip"`,
		`DT_CELL_SET "t",1,"c",,1`,
		`DT_CELL_SET "t",1,"c",,`,
		`DT_SELECT "t",,,RESULT`,
		`DT_TOXML "t",RESULTS`,
		`ENUMFILES "dir","*.csv",1,RESULTS`,
		`DT_ROW_ADD "t","column",,`,
		`SAVETEXT "x","path"`,
		`SAVETEXT "x",1,0,1`,
		`LOADTEXT "path"`,
		`LOADTEXT 1,0,1`,
		`GDRAWTEXT 1,"x",,`,
		`GDRAWTEXT 1,"x",2,3`,
		`GDRAWGWITHROTATE 1,2,3`,
		`GDRAWGWITHROTATE 1,2,3,4,5`,
		`TOSTR 1,,`,
		`MONEYSTR 1,,`,
		`SUBSTRING "x",,`,
		`STRFIND "x","y",,`,
		`VARSETEX "RESULT",1`,
		`VARSETEX "RESULT",1,`,
		`VARSETEX "RESULT",1,2,`,
	)
}

func TestEMEEConsumedEmptyMethodArgumentsDeferUnknowns(t *testing.T) {
	assertMethodContractDeferred(t, EMEE,
		`DT_ROW_ADD "t",UNKNOWN,RESULTS,1`,
		`DT_ROW_ADD "t",RESULTS,UNKNOWN,1`,
		`DT_ROW_ADD UNKNOWN,,RESULTS,1`,
		`DT_ROW_ADD "t",UNKNOWN,,1`,
		`DT_CELL_GET "t",1,"c",UNKNOWN`,
		`ENUMFILES "dir","*.csv",1,UNKNOWN`,
		`SAVETEXT "x",UNKNOWN,UNKNOWN,UNKNOWN`,
		`VARSETEX "RESULT",1,UNKNOWN,2`,
	)
}
