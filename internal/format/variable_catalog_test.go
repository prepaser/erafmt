package format

import (
	"bytes"
	"testing"
)

func TestBuiltinVariableRoles(t *testing.T) {
	for _, line := range []string{
		"IF NAME\nENDIF", "RETURN SAVEDATA_TEXT", "PRINTS NO", "FOR BASE,0,1\nNEXT",
		"CVARSET DAY,0,1", "CVARSET CDFLAG,0,1", "ARRAYSHIFT NO,1,2",
		"PRINTS ARG", "PRINTS LOCAL", "IF ARGS\nENDIF", "IF LOCALS\nENDIF",
	} {
		result := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, Options{Select: []RuleID{BinarySpacing}})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q: %+v", line, result)
		}
	}
	for _, line := range []string{
		"IF name\nENDIF", "IF NAME:INDEX\nENDIF", "IF LOCAL@FUNC\nENDIF", "PRINTS USER_VALUE",
		"CVARSET USER_VALUE,0,1", "CVARSET DAY:INDEX,0,1", "FOR LOCAL,0,1\nNEXT",
		"PRINTS ARGS", "PRINTS LOCALS", "IF ARG\nENDIF", "IF LOCAL\nENDIF",
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
		want knownValueType
	}{
		{`RESULTS=="x"`, knownValueNumeric},
		{`RESULTS+CSTR`, knownValueString},
		{`NO+COUNT`, knownValueNumeric},
		{`RESULT?RESULTS#NAME`, knownValueString},
		{`RESULTS:INDEX=="x"`, knownValueUnknown},
		{`NAME:0`, knownValueUnknown},
		{`NAME@FUNC`, knownValueUnknown},
		{`NAME+USER_VALUE`, knownValueUnknown},
	} {
		tokens, err := lexExpression([]byte(test.expr), 0)
		if err != nil || knownExpressionType(tokens) != test.want {
			t.Fatalf("%q: type=%d error=%v", test.expr, knownExpressionType(tokens), err)
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

func TestReadonlyBuiltinVariablesAreDialectGated(t *testing.T) {
	for _, test := range []struct {
		name    string
		dialect Dialect
		value   knownValueType
		rank    uint8
	}{
		{"ITEMPRICE", CommonDialect, knownValueNumeric, 1},
		{"ABLNAME", CommonDialect, knownValueString, 1},
		{"GAMEBASE_AUTHOR", CommonDialect, knownValueString, 0},
		{"DAYNAME", EMEE, knownValueString, 1},
	} {
		tokens, err := lexExpression([]byte(test.name), 0)
		if err != nil {
			t.Fatalf("%q: %v", test.name, err)
		}
		info, ok := builtinVariableInfoForDialect(tokens, test.dialect)
		if !ok || !info.readOnly || info.value != test.value || info.rank != test.rank {
			t.Fatalf("%q: %+v, %v", test.name, info, ok)
		}
		if !builtinVariableReadOnly(tokens, test.dialect) {
			t.Fatalf("%q was not read-only", test.name)
		}
	}
	for _, dialect := range []Dialect{CommonDialect, Emuera1824} {
		tokens, err := lexExpression([]byte("DAYNAME"), 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := builtinVariableInfoForDialect(tokens, dialect); ok {
			t.Fatalf("DAYNAME unexpectedly known in dialect %d", dialect)
		}
		if builtinVariableReadOnly(tokens, dialect) {
			t.Fatalf("DAYNAME unexpectedly read-only in dialect %d", dialect)
		}
	}
}

func TestReadonlyBuiltinVariableIndexesAndDeferredShapes(t *testing.T) {
	for _, test := range []struct {
		expr string
		want bool
	}{
		{"ITEMPRICE", true},
		{"ITEMPRICE:0", true},
		{"ABLNAME:0", true},
		{"ABLNAME:INDEX", false},
		{"ITEMPRICE:EFA_MACRO", false},
		{"ITEMPRICE:0:1", false},
		{"GAMEBASE_AUTHOR:0", false},
		{"ITEMPRICE@FUNC", false},
		{"ITEMPRICE:", false},
		{"UNKNOWN_CONST", false},
	} {
		tokens, err := lexExpression([]byte(test.expr), 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := builtinVariableReadOnly(tokens, CommonDialect); got != test.want {
			t.Fatalf("%q: got %v want %v", test.expr, got, test.want)
		}
		if _, got := builtinVariableIdentity(tokens, CommonDialect); got != test.want {
			t.Fatalf("%q identity: got %v want %v", test.expr, got, test.want)
		}
	}
}

func TestReadonlyBuiltinVariablesAreRejectedInMutableRoles(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing}}
	for _, test := range []struct {
		src string
	}{
		{"@F\nARRAYSHIFT ITEMPRICE:0,1,2\n"},
		{"@F\nARRAYREMOVE ABLNAME,1,2\n"},
		{"@F\nFOR ;!; ITEMPRICE,0,1\nNEXT\n"},
	} {
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
		}{
			{`ITEMNAME:(0+0)`, true},
			{`ITEMNAME:(CHARANUM+0)`, true},
			{`RESULTS:(0+0)`, false},
			{`ITEMNAME:(USR_INDEX)`, false},
			{`ITEMNAME:(F(1))`, false},
			{`ITEMNAME@FUNC:(0)`, false},
		} {
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

func TestIndexedBuiltinSubset(t *testing.T) {
	tests := []struct {
		expr    string
		dialect Dialect
		value   knownValueType
		valid   bool
		proven  bool
	}{
		{expr: "RESULTS:0", value: knownValueString, valid: true, proven: true},
		{expr: "RESULT:0", value: knownValueNumeric, valid: true, proven: true},
		{expr: "NAME:0", value: knownValueString, valid: true, proven: true},
		{expr: "CFLAG:0:0", value: knownValueNumeric, valid: true, proven: true},
		{expr: "CFLAG:0:\"csv-name\"", value: knownValueNumeric, valid: true, proven: true},
		{expr: "CDFLAG:0:0:0", value: knownValueNumeric, valid: true, proven: true},
		{expr: "RESULT:0:1", valid: false, proven: true},
		{expr: "GAMEBASE_AUTHOR:0", valid: false, proven: true},
		{expr: "NAME:0:1", valid: false, proven: true},
		{expr: "CDFLAG:0:0", valid: false, proven: true},
		{expr: "RESULTS:INDEX", valid: false, proven: false},
		{expr: "RESULTS:0:INDEX", valid: false, proven: false},
		{expr: "RESULT@FUNC:0", valid: false, proven: false},
		{expr: "RESULT:0+1", valid: false, proven: false},
		{expr: "RESULT:(0)", valid: false, proven: false},
		{expr: "DAYNAME:0", dialect: EMEE, value: knownValueString, valid: true, proven: true},
		{expr: "DAYNAME:0", dialect: CommonDialect, valid: false, proven: false},
	}
	for _, test := range tests {
		tokens, err := lexExpression([]byte(test.expr), 0)
		if err != nil {
			t.Fatalf("%q: %v", test.expr, err)
		}
		value, valid, proven := indexedBuiltinType(tokens, test.dialect)
		if value != test.value || valid != test.valid || proven != test.proven {
			t.Fatalf("%q: got (%d,%v,%v), want (%d,%v,%v)", test.expr, value, valid, proven, test.value, test.valid, test.proven)
		}
		if test.valid && knownExpressionTypeForDialect(tokens, test.dialect) != test.value {
			t.Fatalf("%q: type proof did not propagate", test.expr)
		}
	}
}

func TestIndexedBuiltinStringIndexPositions(t *testing.T) {
	tests := []struct {
		expr    string
		dialect Dialect
		valid   bool
		proven  bool
	}{
		{expr: `CFLAG:"key"`, valid: true, proven: true},
		{expr: `CFLAG:0:"key"`, valid: true, proven: true},
		{expr: `CFLAG:"key":0`, valid: false, proven: true},
		{expr: `CSTR:"key"`, valid: true, proven: true},
		{expr: `CSTR:"key":0`, valid: false, proven: true},
		{expr: `CDFLAGNAME1:"key"`, valid: true, proven: true},
		{expr: `CDFLAGNAME2:"key"`, valid: true, proven: true},
		{expr: `CDFLAG:0:"key":0`, valid: true, proven: true},
		{expr: `CDFLAG:0:0:"key"`, valid: true, proven: true},
		{expr: `CDFLAG:"key":0:0`, valid: false, proven: true},
		{expr: `DA:0:"key"`, valid: false, proven: true},
		{expr: `DA:"key":0`, valid: false, proven: true},
		{expr: `ABL:0:"key"`, valid: true, proven: true},
		{expr: `ABL:"key":0`, valid: false, proven: true},
		{expr: `NAME:"key"`, valid: false, proven: true},
		{expr: `ABLNAME:"key"`, valid: false, proven: true},
		{expr: `DAY:"key"`, dialect: EMEE, valid: true, proven: true},
		{expr: `DAY:"key"`, dialect: CommonDialect, valid: false, proven: true},
	}
	for _, test := range tests {
		tokens, err := lexExpression([]byte(test.expr), 0)
		if err != nil {
			t.Fatalf("%q: %v", test.expr, err)
		}
		_, valid, proven := indexedBuiltinType(tokens, test.dialect)
		if valid != test.valid || proven != test.proven {
			t.Fatalf("%q: got (%v,%v), want (%v,%v)", test.expr, valid, proven, test.valid, test.proven)
		}
	}
}

func TestIndexedBuiltinExpressionProof(t *testing.T) {
	for _, expression := range []string{
		`RESULTS:0+1`, `RESULT:0+"x"`, `NAME:0+1`, `GAMEBASE_AUTHOR:0`, `RESULT:0:1`,
		`DA:0:"x"`, `CFLAG:"x":0`, `CSTR:"x":0`, `CDFLAG:"x":0:0`,
	} {
		tokens, err := lexExpression([]byte(expression), 0)
		if err != nil {
			t.Fatalf("%q: %v", expression, err)
		}
		if err := provenExpressionsValid(tokens, true, CommonDialect); err == nil {
			t.Fatalf("%q was accepted", expression)
		}
	}
	for _, test := range []struct {
		expression string
		dialect    Dialect
	}{
		{`RESULT:0+1`, CommonDialect},
		{`RESULTS:0+"x"`, CommonDialect},
		{`CFLAG:0:"csv-name"+1`, CommonDialect},
		{`CDFLAG:0:0:0+1`, CommonDialect},
		{`CFLAG:"x"+1`, CommonDialect},
		{`CFLAG:0:"x"+1`, CommonDialect},
		{`DAYNAME:0+"x"`, EMEE},
		{`DAYNAME:0+"x"`, CommonDialect},
		{`RESULTS:INDEX+1`, CommonDialect},
		{`RESULT:0+UNKNOWN`, CommonDialect},
		{`RESULT:0+F(1)`, CommonDialect},
	} {
		tokens, err := lexExpression([]byte(test.expression), 0)
		if err != nil {
			t.Fatalf("%q: %v", test.expression, err)
		}
		if err := provenExpressionsValid(tokens, true, test.dialect); err != nil {
			t.Fatalf("%q: %v", test.expression, err)
		}
	}
}
