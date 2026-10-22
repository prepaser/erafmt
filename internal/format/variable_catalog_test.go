package format

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestBuiltinVariableRoles(t *testing.T) {
	for _, line := range []string{
		"IF NAME\nENDIF",
		"RETURN SAVEDATA_TEXT",
		"PRINTS NO",
		"FOR BASE,0,1\nNEXT",
		"CVARSET DAY,0,1",
		"CVARSET CDFLAG,0,1",
		"ARRAYSHIFT NO,1,2",
		"PRINTS ARG",
		"PRINTS LOCAL",
		"IF ARGS\nENDIF",
		"IF LOCALS\nENDIF",
	} {
		result := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, Options{Select: []RuleID{BinarySpacing}})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q: %+v", line, result)
		}
	}
	for _, line := range []string{
		"IF name\nENDIF",
		"IF NAME:INDEX\nENDIF",
		"IF LOCAL@FUNC\nENDIF",
		"PRINTS USER_VALUE",
		"CVARSET USER_VALUE,0,1",
		"CVARSET DAY:INDEX,0,1",
		"FOR LOCAL,0,1\nNEXT",
		"PRINTS ARGS",
		"PRINTS LOCALS",
		"IF ARG\nENDIF",
		"IF LOCAL\nENDIF",
	} {
		result := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, Options{Select: []RuleID{BinarySpacing}})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("deferred %q: %+v", line, result)
		}
	}
}

func TestKnownVariableCompoundTypes(t *testing.T) {
	for _, test := range []struct {
		expr string
		want erabasic.KnownValueType
	}{
		{`RESULTS=="x"`, erabasic.KnownValueNumeric},
		{`RESULTS+CSTR`, erabasic.KnownValueString},
		{`NO+COUNT`, erabasic.KnownValueNumeric},
		{`RESULT?RESULTS#NAME`, erabasic.KnownValueString},
		{`RESULTS:INDEX=="x"`, erabasic.KnownValueUnknown},
		{`NAME:0`, erabasic.KnownValueUnknown},
		{`NAME@FUNC`, erabasic.KnownValueUnknown},
		{`NAME+USER_VALUE`, erabasic.KnownValueUnknown},
	} {
		tokens, err := lexExpression([]byte(test.expr), 0)
		if err != nil || erabasic.KnownExpressionType(tokens) != test.want {
			t.Fatalf("%q: type=%d error=%v", test.expr, erabasic.KnownExpressionType(tokens), err)
		}
	}
	input := []byte("@F\nPRINTS NO\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{}})
	if len(result.Diagnostics) != 0 || !bytes.Equal(input, result.Output) {
		t.Fatalf("selection-none: %+v", result)
	}
}

func TestPowerRequiredFields(t *testing.T) {
	for _, line := range []string{"POWER RESULT", "POWER RESULT,1", "POWER RESULT,1,"} {
		result := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, Options{})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q: %+v", line, result)
		}
	}
	for _, line := range []string{"POWER RESULT,1,2,", "POWER EFAUDIT_FIELDS", "POWER RESULT,EFAUDIT_FIELDS", "POWER RESULT,1,;!;"} {
		result := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("deferred/valid %q: %+v", line, result)
		}
	}
}

func TestReadonlyBuiltinVariablesAreRejectedInMutableRoles(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing}}
	for _, test := range []struct{ src string }{{"@F\nARRAYSHIFT ITEMPRICE:0,1,2\n"}, {"@F\nARRAYREMOVE ABLNAME,1,2\n"}, {"@F\nFOR ;!; ITEMPRICE,0,1\nNEXT\n"}} {
		r := FormatWithOptions([]byte(test.src), ERB, options)
		if len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q: output=%q diagnostics=%+v", test.src, r.Output, r.Diagnostics)
		}
	}
}

func TestIndexedReadonlyOutputProof(t *testing.T) {
	for _, command := range []string{`SPLIT "x",",",`, `HTML_TAGSPLIT "x",`} {
		for _, tc := range []struct {
			output string
			reject bool
		}{{`ITEMNAME:(0+0)`, true}, {`ITEMNAME:(CHARANUM+0)`, true}, {`RESULTS:(0+0)`, false}, {`ITEMNAME:(USR_INDEX)`, false}, {`ITEMNAME:(F(1))`, false}, {`ITEMNAME@FUNC:(0)`, false}} {
			line := command + tc.output + ",RESULT"
			r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, Options{})
			if (len(r.Diagnostics) != 0) != tc.reject {
				t.Fatalf("%q diagnostics=%+v", line, r.Diagnostics)
			}
			if tc.reject && !bytes.Contains(r.Output, []byte(line)) {
				t.Fatalf("rejected body changed: %q", r.Output)
			}
		}
	}
}
