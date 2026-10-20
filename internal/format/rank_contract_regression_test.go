package format

import "testing"

func rankContractResult(dialect Dialect, line string) Result {
	return FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(dialect))
}

func TestScalarRankProofAcrossConsumers(t *testing.T) {
	lines := []string{
		`FINDELEMENT RESULT,DA`,
		`MATCH RESULT,DA`,
		`FINDCHARA CFLAG,CDFLAG`,
		`ARRAYSORT RESULT,FORWARD,DA`,
		`PRINTV DA`,
		`PRINTFORM {DA}`,
		`PRINTFORM {1,DA}`,
		`PRINTFORM \@DA?yes#no\@`,
		`DT_ROW_ADD "t","c",DA`,
	}
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		for _, line := range lines {
			r := rankContractResult(dialect, line)
			if len(r.Diagnostics) == 0 {
				t.Errorf("dialect=%v accepted scalar-rank misuse %q: output=%q", dialect, line, r.Output)
			}
		}
	}
	for _, line := range []string{`PRINTV UNKNOWN,DA`, `DT_ROW_ADD UNKNOWN,"c",DA`, `DT_ROW_ADD "t",UNKNOWN,DA`, `ARRAYSORT UNKNOWN,FORWARD,DA`} {
		if r := rankContractResult(EMEE, line); len(r.Diagnostics) != 0 {
			t.Errorf("rejected deferred later field %q: %+v", line, r.Diagnostics)
		}
	}
	for _, line := range []string{`DT_ROW_ADD "t","c",,"d",1`, `DT_ROW_ADD "t","c",1,`} {
		if r := rankContractResult(EMEE, line); len(r.Diagnostics) != 0 {
			t.Errorf("rejected nullable row value %q: %+v", line, r.Diagnostics)
		}
	}
}

func TestModernAnySlotsRequireScalarValues(t *testing.T) {
	for _, line := range []string{
		`SAVETEXT "x",DA`, `LOADTEXT DA`, `SETVAR "X",DA`,
		`VARSETEX "X",DA`, `DT_COLUMN_ADD "t","c",DA`, `DT_CELL_SET "t",1,"c",DA`,
	} {
		r := rankContractResult(EMEE, line)
		if len(r.Diagnostics) == 0 {
			t.Errorf("accepted scalar-rank misuse %q: output=%q", line, r.Output)
		}
	}
}

func TestReplaceModeContracts(t *testing.T) {
	for _, line := range []string{
		`REPLACE "a","b","c",DA`,
		`REPLACE "a","b","c",1`,
		`REPLACE "a","b",GAMEBASE_TITLE,1`,
		`REPLACE "a","b",CSTR,1`,
		`REPLACE "a","b",CSTR:0:0,1`,
		`REPLACE "a","b",CSTR,4294967297`,
		`REPLACE "a","b",CSTR,-4294967295`,
	} {
		if r := rankContractResult(EMEE, line); len(r.Diagnostics) == 0 {
			t.Errorf("accepted invalid REPLACE %q: output=%q", line, r.Output)
		}
	}
	for _, line := range []string{
		`REPLACE "a","b",RESULTS,1`,
		`REPLACE "a","b",GAMEBASE_TITLE,2`,
		`REPLACE "a","b",CSTR,4294967296`,
		`REPLACE "a","b",CSTR,-4294967294`,
		`REPLACE "a","b",UNKNOWN,1`,
		`REPLACE "a","b",F(1),1`,
		`REPLACE "a","b",RESULTS,F(1)`,
	} {
		if r := rankContractResult(EMEE, line); len(r.Diagnostics) != 0 {
			t.Errorf("rejected valid/deferred REPLACE %q: %+v", line, r.Diagnostics)
		}
	}
}

func TestModernArrayOutputRoles(t *testing.T) {
	for _, line := range []string{
		`REGEXPMATCH "x","x",DA,RESULTS`,
		`REGEXPMATCH "x","x",RESULT,CSTR`,
		`REGEXPMATCH "x","x",RESULT,CSTR:0:0`,
		`XML_GET 1,"/x",CSTR`,
		`XML_GET_BYNAME "id","/x",CSTR`,
		`DT_ROW_ADD "t",CSTR,RESULTS,1`,
		`DT_ROW_ADD "t",RESULTS,CSTR,1`,
		`DT_SELECT "t","f","s",CFLAG`,
	} {
		if r := rankContractResult(EMEE, line); len(r.Diagnostics) == 0 {
			t.Errorf("accepted invalid array role %q: output=%q", line, r.Output)
		}
	}
	for _, line := range []string{
		`REGEXPMATCH "x","x",DA:0:0,RESULTS`,
		`REGEXPMATCH "x","x",RESULT,RESULTS`,
		`XML_GET 1,"/x",RESULTS`,
		`XML_GET_BYNAME "id","/x",RESULTS`,
		`DT_ROW_ADD "t",RESULTS,RESULTS,1`,
		`DT_SELECT "t","f","s",RESULT`,
		`XML_ADDNODE CSTR,"/x","<y/>"`,
	} {
		if r := rankContractResult(EMEE, line); len(r.Diagnostics) != 0 {
			t.Errorf("rejected valid array/scalar role %q: %+v", line, r.Diagnostics)
		}
	}
}

func TestInRangeCharacterArrayDialectRoles(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824} {
		for _, line := range []string{`INRANGECARRAY CDFLAG,1,2`, `INRANGECARRAY CDFLAG:0:0:0,1,2`, `INRANGECARRAY CFLAG,1,2`} {
			if r := rankContractResult(dialect, line); len(r.Diagnostics) != 0 {
				t.Errorf("dialect=%v rejected legacy character-array role %q: %+v", dialect, line, r.Diagnostics)
			}
		}
	}
	for _, line := range []string{`INRANGECARRAY CDFLAG,1,2`, `INRANGECARRAY CDFLAG:0:0:0,1,2`} {
		if r := rankContractResult(EMEE, line); len(r.Diagnostics) == 0 {
			t.Errorf("EMEE accepted multidimensional character-array role %q", line)
		}
	}
	if r := rankContractResult(EMEE, `INRANGECARRAY CFLAG,1,2`); len(r.Diagnostics) != 0 {
		t.Fatalf("EMEE rejected rank-1 character-array role: %+v", r.Diagnostics)
	}
}
