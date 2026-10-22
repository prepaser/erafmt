package format

import (
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestSimpleMethodArguments(t *testing.T) {
	src := []byte("@F\nABS\nABS \"x\"\nABS 1,\nABS UNKNOWN\n")
	r := FormatWithOptions(src, ERB, methodOptions(erabasic.CommonDialect))
	if len(r.Diagnostics) != 2 {
		t.Fatalf("diagnostics=%+v output=%q", r.Diagnostics, r.Output)
	}
	for _, diagnostic := range r.Diagnostics {
		if !strings.Contains(diagnostic.Message, "method ") {
			t.Fatalf("diagnostic=%+v", diagnostic)
		}
	}
	if !strings.Contains(string(r.Output), "ABS 1,") || !strings.Contains(string(r.Output), "ABS UNKNOWN") {
		t.Fatalf("output=%q", r.Output)
	}
}

func TestFixedMethodContracts(t *testing.T) {
	valid := []string{
		`CHKFONT "x"`,
		`CHKDATA 1`,
		`CHKCHARADATA "x"`,
		`BARSTR 1,2,3`,
		`COLOR_FROMNAME "red"`,
		`COLOR_FROMRGB 1,2,3`,
		`GETBIT 1,2`,
		`GETNUMB "x","y"`,
		`GETPALAMLV 1,2`,
		`GETEXPLV 1,2`,
		`GETLINESTR "x"`,
		`STRFORM "x"`,
		`GETCONFIG "x"`,
		`GETCONFIGS "x"`,
		`HTML_POPPRINTINGSTR`,
		`GETKEY 1`,
		`GETKEYTRIGGERED 1`,
		`SETANIMETIMER 1`,
	}
	for _, line := range valid {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(erabasic.CommonDialect))
		if len(r.Diagnostics) != 0 {
			t.Errorf("valid method %q: diagnostics=%+v", line, r.Diagnostics)
		}
	}
	for _, line := range []string{`CHKFONT 1`, `CHKDATA "x"`, `BARSTR 1,"x",3`, `GETBIT 1`, `GETCONFIG 1`} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(erabasic.CommonDialect))
		if len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "method") {
			t.Errorf("invalid method %q: diagnostics=%+v", line, r.Diagnostics)
		}
	}
}

func TestArrayExtremaMethodContracts(t *testing.T) {
	valid := []string{
		`MAXARRAY RESULT`,
		`MAXARRAY RESULT:0`,
		`MAXARRAY ITEMPRICE`,
		`MAXARRAY RESULT,1`,
		`MAXARRAY RESULT,1,2,`,
		`MINARRAY RESULT,1,,`,
		`MAXCARRAY CFLAG`,
		`MAXCARRAY CFLAG:0:0`,
		`MAXCARRAY CFLAG,1,2`,
		`MINCARRAY CFLAG,1,`,
	}
	invalid := []string{
		`MAXARRAY RESULT:0:0`,
		`MAXARRAY RESULT,"x"`,
		`MAXARRAY RESULT,1,"x"`,
		`MAXARRAY RESULT,1,2,,`,
		`MAXARRAY RESULTS`,
		`MAXARRAY CDFLAG`,
		`MAXCARRAY CDFLAG`,
		`MAXCARRAY ITEMPRICE`,
		`MAXCARRAY CSTR`,
		`MAXCARRAY CFLAG,"x"`,
		`MINCARRAY CFLAG,1,2,3`,
	}
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		for _, line := range valid {
			r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(dialect))
			if len(r.Diagnostics) != 0 {
				t.Errorf("dialect=%v valid array extrema %q: diagnostics=%+v", dialect, line, r.Diagnostics)
			}
		}
		for _, line := range invalid {
			r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(dialect))
			if len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported || !strings.Contains(r.Diagnostics[0].Message, "method") || !strings.Contains(string(r.Output), line+"\n") {
				t.Errorf("dialect=%v invalid array extrema %q: diagnostics=%+v output=%q", dialect, line, r.Diagnostics, r.Output)
			}
		}
	}
	for _, line := range []string{`MAXARRAY UNKNOWN`, `MAXCARRAY UNKNOWN`, `MAXARRAY RESULT:INDEX`} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(erabasic.CommonDialect))
		if len(r.Diagnostics) != 0 {
			t.Errorf("deferred array extrema %q: diagnostics=%+v", line, r.Diagnostics)
		}
	}
}

func TestGetBitLiteralRange(t *testing.T) {
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		for _, line := range []string{`GETBIT 1,0`, `GETBIT 1,63`, `GETBIT 1,0x3f`, `GETBIT 1,1+63`, `GETBIT 1,UNKNOWN`} {
			r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(dialect))
			if len(r.Diagnostics) != 0 {
				t.Errorf("dialect=%v valid/deferred GETBIT %q: diagnostics=%+v", dialect, line, r.Diagnostics)
			}
		}
		for _, line := range []string{`GETBIT 1,64`, `GETBIT 1,0x40`, `GETBIT 1,-1`, `GETBIT 1,(-64)`, `GETBIT 1,1e2`, `GETBIT 1,1p6`, `GETBIT 1,+((64))`, `GETBIT 1,-(1)`} {
			r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(dialect))
			if len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported || !strings.Contains(r.Diagnostics[0].Message, "bit index literal") || !strings.Contains(string(r.Output), line+"\n") {
				t.Errorf("dialect=%v invalid GETBIT %q: diagnostics=%+v output=%q", dialect, line, r.Diagnostics, r.Output)
			}
		}
		for _, line := range []string{`GETBIT 1,(1+63)`} {
			r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(dialect))
			if len(r.Diagnostics) != 0 {
				t.Errorf("dialect=%v deferred GETBIT %q: diagnostics=%+v", dialect, line, r.Diagnostics)
			}
		}
	}
}

func TestOptionalMethodContracts(t *testing.T) {
	for _, line := range []string{`SUBSTRING "x"`, `SUBSTRING "x",,2`, `SUBSTRINGU "x",1,`, `STRFIND "x","x"`, `STRFINDU "x","x",`, `TOSTR 1`, `TOSTR 1,`, `MONEYSTR 1`, `HTML_POPPRINTINGSTR`} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(erabasic.CommonDialect))
		if len(r.Diagnostics) != 0 {
			t.Errorf("valid optional method %q: diagnostics=%+v", line, r.Diagnostics)
		}
	}
	for _, line := range []string{`SUBSTRING "x",1,2,3`, `STRFIND "x"`, `TOSTR "x"`, `MONEYSTR 1,"x",2`} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(erabasic.CommonDialect))
		if len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "method") {
			t.Errorf("invalid optional method %q: diagnostics=%+v", line, r.Diagnostics)
		}
	}
	for _, line := range []string{`SUBSTRING UNKNOWN,1`, `SUBSTRING "x",F(1)`, `STRFIND UNKNOWN,"x"`, `TOSTR F(1)`} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(erabasic.CommonDialect))
		if len(r.Diagnostics) != 0 {
			t.Errorf("deferred optional method %q: diagnostics=%+v", line, r.Diagnostics)
		}
	}
}

func TestModernMethodContracts(t *testing.T) {
	valid := []string{
		`REPLACE "a","b","c",0`,
		`REPLACE "a","b",RESULTS,1`,
		`REPLACE "a","b",GAMEBASE_TITLE,2`,
		`HTML_STRINGLEN "x"`,
		`HTML_STRINGLEN "x",1`,
		`EXISTFILE "x"`,
		`OUTPUTLOG`,
		`OUTPUTLOG "x",1`,
		`REGEXPMATCH "x","x"`,
		`REGEXPMATCH "x","x",1`,
		`REGEXPMATCH "x","x",RESULT,RESULTS`,
	}
	for _, line := range valid {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(erabasic.EMEE))
		if len(r.Diagnostics) != 0 {
			t.Errorf("valid modern method %q: diagnostics=%+v", line, r.Diagnostics)
		}
	}
	deferred := FormatWithOptions([]byte("@F\nREPLACE \"a\",\"b\",UNKNOWN,1\nREGEXPMATCH \"x\",\"x\",UNKNOWN,UNKNOWN\n"), ERB, methodOptions(erabasic.EMEE))
	if len(deferred.Diagnostics) != 0 {
		t.Fatalf("deferred modern methods diagnostics=%+v", deferred.Diagnostics)
	}
	for _, line := range []string{`REPLACE "a","b","c", "x"`, `REPLACE "a","b",1,1`, `HTML_STRINGLEN 1`, `EXISTFILE 1`, `REGEXPMATCH "x"`} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(erabasic.EMEE))
		if len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "method") {
			t.Errorf("invalid modern method %q: diagnostics=%+v", line, r.Diagnostics)
		}
	}
	common := FormatWithOptions([]byte("@F\nREPLACE \"a\",\"b\",\"c\",0\nHTML_STRINGLEN \"x\"\n"), ERB, methodOptions(erabasic.CommonDialect))
	if len(common.Diagnostics) != 2 {
		t.Fatalf("common dialect diagnostics=%+v", common.Diagnostics)
	}
}

func TestAlternativeMethodContracts(t *testing.T) {
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		for _, line := range []string{`SPRITECREATE "sprite",1`, `SPRITECREATE "sprite",1,2,3,4,5`} {
			r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(dialect))
			if len(r.Diagnostics) != 0 {
				t.Errorf("dialect=%v valid alternative %q: diagnostics=%+v", dialect, line, r.Diagnostics)
			}
		}
		for _, line := range []string{`SPRITECREATE "sprite",1,2`, `SPRITECREATE "sprite",1,2,3,4`} {
			r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(dialect))
			if len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "method argument count") {
				t.Errorf("dialect=%v invalid alternative %q: diagnostics=%+v", dialect, line, r.Diagnostics)
			}
		}
		for _, line := range []string{`SPRITECREATE "sprite",1,`} {
			r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(dialect))
			if len(r.Diagnostics) != 0 {
				t.Errorf("dialect=%v valid trailing alternative %q: diagnostics=%+v", dialect, line, r.Diagnostics)
			}
		}
		for _, line := range []string{`SPRITECREATE`, `SPRITECREATE "sprite",,`, `SPRITECREATE "sprite",1,,3,4,5`} {
			r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(dialect))
			if len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "method") {
				t.Errorf("dialect=%v invalid empty/interior alternative %q: diagnostics=%+v", dialect, line, r.Diagnostics)
			}
		}
	}
}

func TestGraphicsMethodContractsByDialect(t *testing.T) {
	for _, line := range []string{`GSETBRUSH 1,2`, `GSETFONT 1,"Arial",12`, `GSETPEN 1,2`} {
		for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824} {
			r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(dialect))
			if len(r.Diagnostics) != 0 {
				t.Errorf("dialect=%v valid graphics method %q: diagnostics=%+v", dialect, line, r.Diagnostics)
			}
		}
	}
	for _, line := range []string{`GSETFONT 1,"Arial",12,1`, `GSETPEN 1,2,3`} {
		for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824} {
			r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(dialect))
			if len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "method argument count") {
				t.Errorf("dialect=%v invalid graphics method %q: diagnostics=%+v", dialect, line, r.Diagnostics)
			}
		}
	}
	for _, line := range []string{`GSETBRUSH 1,2`, `GSETFONT 1,"Arial",12`, `GSETFONT 1,"Arial",12,1`, `GSETPEN 1,2,3`} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(erabasic.EMEE))
		if len(r.Diagnostics) != 0 {
			t.Errorf("dialect=EMEE valid graphics method %q: diagnostics=%+v", line, r.Diagnostics)
		}
	}
	for _, line := range []string{`GSETFONT 1,"Arial",12,`} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(erabasic.EMEE))
		if len(r.Diagnostics) != 0 {
			t.Errorf("dialect=EMEE valid trailing graphics method %q: diagnostics=%+v", line, r.Diagnostics)
		}
	}
	for _, line := range []string{`GSETFONT`, `GSETFONT 1,,12`} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(erabasic.EMEE))
		if len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "method") {
			t.Errorf("dialect=EMEE invalid empty/interior graphics method %q: diagnostics=%+v", line, r.Diagnostics)
		}
	}
}

func TestExpandedMethodContracts(t *testing.T) {
	for _, line := range []string{
		`GETCHARA 1`,
		`GETCHARA 1,2`,
		`GETSPCHARA 1`,
		`HTML_GETPRINTEDSTR`,
		`HTML_GETPRINTEDSTR 1`,
		`GCREATE 1,2,3`,
		`GDISPOSE 1`,
		`GCREATED 1`,
		`GWIDTH 1`,
		`GHEIGHT 1`,
		`SPRITECREATED "x"`,
		`SPRITEWIDTH "x"`,
		`SPRITEMOVE "x",1,2`,
		`SPRITEGETCOLOR "x",1,2`,
		`SPRITEANIMECREATE "x",1,2`,
		`CBGREMOVERANGE 1,2`,
		`CBGSETG 1,2,3,4`,
		`CBGSETBMAPG 1`,
		`GSAVE 1,2`,
		`GLOAD 1,2`,
	} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(erabasic.CommonDialect))
		if len(r.Diagnostics) != 0 {
			t.Errorf("valid expanded method %q: diagnostics=%+v", line, r.Diagnostics)
		}
	}
	for _, line := range []string{
		`HTML_SUBSTRING "x",1`,
		`HTML_STRINGLINES "x",1`,
		`EXISTVAR "x"`,
		`ISDEFINED "x"`,
		`EXISTSOUND "x"`,
		`EXISTFUNCTION "x"`,
		`EXISTFUNCTION "x",1`,
		`GETVAR "x"`,
		`GETVARS "x"`,
		`MAP_CREATE "m"`,
		`MAP_GET "m","k"`,
		`MAP_SET "m","k","v"`,
		`MAP_FROMXML "m","x"`,
		`DT_CREATE "t"`,
		`DT_NOCASE "t",1`,
		`DT_CELL_GET "t",1,"c"`,
		`DT_CELL_GET "t",1,"c",1`,
		`GGETFONT 1`,
		`GGETFONTSIZE 1`,
		`GGETFONTSTYLE 1`,
		`GGETBRUSH 1`,
		`GGETPEN 1`,
		`GGETPENWIDTH 1`,
	} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(erabasic.EMEE))
		if len(r.Diagnostics) != 0 {
			t.Errorf("valid modern expanded method %q: diagnostics=%+v", line, r.Diagnostics)
		}
	}
	for _, line := range []string{`GCREATE "x",2,3`, `SPRITECREATED 1`, `MAP_SET 1,"k","v"`, `DT_CELL_GET "t","row","c"`, `HTML_SUBSTRING 1,2`} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, methodOptions(erabasic.EMEE))
		if len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "method") {
			t.Errorf("invalid expanded method %q: diagnostics=%+v", line, r.Diagnostics)
		}
	}
}
