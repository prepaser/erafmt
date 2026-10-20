package format

import (
	"bytes"
	"testing"
)

func TestScalarRolesRequireMultidimensionalIndexes(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		for _, body := range []string{
			"SETBIT DA,1", "CLEARBIT TA,1", "INVERTBIT (CDFLAG),1",
			"POWER DA,2,3", "POWER RESULT,DA,3", "TIMES +DA,2.0",
			"FOR DA,0,1\nNEXT", "FOR RESULT,0,TA\nNEXT",
			"RETURN DA", "RETURN (DA)+1", "IF TA\nENDIF", "WHILE DA\nWEND",
			"SETCOLOR CDFLAG", "BAR DA,1,1", "SAVEDATA DA,\"x\"",
			"PRINTDATA DA\nDATA x\nENDDATA", "SAVENOS DA", "PRINTCPERLINE TA",
			"SWAP DA,TA", "SPLIT \"a\",\",\",RESULTS,DA",
			"ABS DA", "SQRT CDFLAG", "MAX DA", "MIN 1,TA", "GETBIT DA,1",
		} {
			src := []byte("@F\n" + body + "\n")
			options := destinationRoleOptions()
			options.Dialect = dialect
			result := FormatWithOptions(src, ERB, options)
			if !bytes.Equal(result.Output, src) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Severity != Unsupported {
				t.Fatalf("dialect=%v body=%q output=%q diagnostics=%+v", dialect, body, result.Output, result.Diagnostics)
			}
			options.Select = []RuleID{}
			result = FormatWithOptions(src, ERB, options)
			if !bytes.Equal(result.Output, src) || len(result.Diagnostics) != 0 {
				t.Fatalf("disabled rules: body=%q result=%+v", body, result)
			}
		}
	}
}

func TestScalarRolesKeepDefaultIndexesAndArrayReferences(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		for _, body := range []string{
			"SETBIT RESULT,1", "SETBIT DA:0:0,1", "POWER DA:0:0,2,3", "TIMES (TA:0:0:0),2.0",
			"FOR DA:0:0,0,1\nNEXT", "RETURN DA:0:0", "RETURN RESULT+1",
			"PRINTDATA DA:0:0\nDATA x\nENDDATA", "STRDATA RESULTS\nDATA x\nENDDATA",
			"SAVENOS RESULT", "PRINTCPERLINE DA:0:0", "ABS DA:0:0", "MAX RESULT,TA:0:0:0",
			"VARSET DA,1", "VARSIZE DA", "GDRAWSPRITE 0,\"x\",0,0,1,1,DA", "GDRAWG 0,1,0,0,1,1,0,0,1,1,TA",
			"SWAP DA:0:0,TA:0:0:0", "SPLIT \"a\",\",\",RESULTS,DA:0:0",
			"RETURN UNKNOWN+DA", "RETURN LOCAL@DA:0", "SETBIT DA:UNKNOWN,1", "MAX UNKNOWN,DA",
		} {
			src := []byte("@F\n" + body + "\n")
			options := destinationRoleOptions()
			options.Dialect = dialect
			result := FormatWithOptions(src, ERB, options)
			if len(result.Diagnostics) != 0 {
				t.Fatalf("dialect=%v body=%q output=%q diagnostics=%+v", dialect, body, result.Output, result.Diagnostics)
			}
			again := FormatWithOptions(result.Output, ERB, options)
			if !bytes.Equal(again.Output, result.Output) || len(again.Changes) != 0 || len(again.Diagnostics) != 0 {
				t.Fatalf("unstable body=%q result=%+v", body, again)
			}
		}
	}
}
