package format

import (
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func characterMethodResult(line string) Result {
	return FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, Options{Dialect: CommonDialect})
}

func characterMethodDialectResult(dialect erabasic.Dialect, line string) Result {
	return FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, Options{Dialect: Dialect(dialect)})
}

func TestCharacterMethodCSVContracts(t *testing.T) {
	for _, line := range []string{"CSVNAME 1", "CSVNAME 1,", "CSVNAME 1,,", "CSVNAME RESULT:0", "CSVNAME ITEMPRICE", "CSVCSTR 1,2", "CSVCSTR 1,2,,", "CSVBASE 1,2,3", "EXISTCSV 1,2"} {
		if r := characterMethodResult(line); len(r.Diagnostics) != 0 {
			t.Errorf("valid %q: %+v", line, r.Diagnostics)
		}
	}
	for _, line := range []string{"CSVNAME", "CSVNAME 1,2,3", `CSVNAME "x"`, "CSVCSTR 1", "CSVCSTR 1,,2", "CSVBASE 1,2,3,4", `EXISTCSV "x"`, "EXISTCSV 1,2,3"} {
		if r := characterMethodResult(line); len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "method") {
			t.Errorf("invalid %q: %+v", line, r.Diagnostics)
		}
	}
}

func TestCharacterMethodFindCharaContracts(t *testing.T) {
	for _, line := range []string{"FINDCHARA CFLAG,1", "FINDCHARA CFLAG,1,,,", "FINDCHARA CFLAG,1,,RESULT", `FINDCHARA NAME,"x"`, "FINDCHARA CSTR,CSTR", "FINDLASTCHARA CFLAG,CFLAG", "FINDCHARA UNKNOWN,UNKNOWN"} {
		if r := characterMethodResult(line); len(r.Diagnostics) != 0 {
			t.Errorf("valid/deferred %q: %+v", line, r.Diagnostics)
		}
	}
	for _, line := range []string{"FINDCHARA", "FINDCHARA CFLAG", "FINDCHARA CFLAG,,,,", "FINDCHARA RESULT,1", "FINDCHARA CFLAG,CSTR", `FINDCHARA CFLAG,1,"x"`, "FINDCHARA DA,1"} {
		if r := characterMethodResult(line); len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "method") {
			t.Errorf("invalid %q: %+v", line, r.Diagnostics)
		}
	}
}

func TestCharacterMethodFindCharadataContract(t *testing.T) {
	for _, line := range []string{"FIND_CHARADATA", `FIND_CHARADATA "x"`, "FIND_CHARADATA ,"} {
		if r := characterMethodResult(line); len(r.Diagnostics) != 0 {
			t.Errorf("valid %q: %+v", line, r.Diagnostics)
		}
	}
	for _, line := range []string{`FIND_CHARADATA "x",1`, "FIND_CHARADATA 1", "FIND_CHARADATA 1,2"} {
		if r := characterMethodResult(line); len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "method") {
			t.Errorf("invalid %q: %+v", line, r.Diagnostics)
		}
	}
}

func TestCharacterMethodContractsDeferDynamicTails(t *testing.T) {
	for _, line := range []string{
		`CSVNAME UNKNOWN,UNKNOWN`,
		`CSVNAME 1,UNKNOWN`,
		`CSVCSTR 1,UNKNOWN,UNKNOWN`,
		`FINDCHARA UNKNOWN,UNKNOWN`,
		`FINDCHARA CFLAG,UNKNOWN,UNKNOWN`,
		`CSVNAME ;!; 1,2,3`,
		`FINDCHARA ;!; RESULT,1`,
		`CSVNAME F(1),F(2)`,
		`FINDCHARA F(1),F(2),F(3)`,
		`CSVNAME @"raw",1`,
		`FINDCHARA @"raw",1`,
	} {
		r := characterMethodResult(line)
		if len(r.Diagnostics) != 0 {
			t.Errorf("deferred %q: %+v", line, r.Diagnostics)
		}
		again := FormatWithOptions(r.Output, ERB, Options{Dialect: CommonDialect})
		if len(again.Diagnostics) != 0 || string(again.Output) != string(r.Output) || len(again.Changes) != 0 {
			t.Errorf("not idempotent %q: first=%+v second=%+v", line, r, again)
		}
	}
}

func TestCharacterMethodContractsAllDialects(t *testing.T) {
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		for _, line := range []string{"CSVNAME 1", "CSVBASE 1,2", "FINDCHARA CFLAG,1", "FIND_CHARADATA", "FIND_CHARADATA \"x\""} {
			if r := characterMethodDialectResult(dialect, line); len(r.Diagnostics) != 0 {
				t.Errorf("dialect=%v valid %q: %+v", dialect, line, r.Diagnostics)
			}
		}
		for _, line := range []string{"CSVNAME 1,2,3", "FINDCHARA RESULT,1", "FIND_CHARADATA 1"} {
			if r := characterMethodDialectResult(dialect, line); len(r.Diagnostics) != 1 {
				t.Errorf("dialect=%v invalid %q: %+v", dialect, line, r.Diagnostics)
			}
		}
	}
}

func TestCharacterMethodContractsMacroExpansionDefers(t *testing.T) {
	src := []byte("@F\n#DEFINE USR_M CFLAG\n#DEFINE USR_ARGS 1,2\nFINDCHARA USR_M,1\nCSVNAME USR_ARGS\n")
	r := FormatWithOptions(src, ERB, Options{Dialect: CommonDialect})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("macro-capable fields diagnosed: %+v", r.Diagnostics)
	}
}
