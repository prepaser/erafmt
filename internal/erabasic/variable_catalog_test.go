package erabasic

import "testing"

func TestReadonlyBuiltinVariablesAreDialectGated(t *testing.T) {
	for _, test := range []struct {
		name    string
		dialect Dialect
		value   KnownValueType
		rank    uint8
	}{{"ITEMPRICE", CommonDialect, KnownValueNumeric, 1}, {"ABLNAME", CommonDialect, KnownValueString, 1}, {"GAMEBASE_AUTHOR", CommonDialect, KnownValueString, 0}, {"DAYNAME", EMEE, KnownValueString, 1}} {
		tokens, err := lexExpression([]byte(test.name), 0)
		if err != nil {
			t.Fatalf("%q: %v", test.name, err)
		}
		info, ok := BuiltinVariableInfoForDialect(tokens, test.dialect)
		if !ok || !info.ReadOnly || info.Value != test.value || info.Rank != test.rank {
			t.Fatalf("%q: %+v, %v", test.name, info, ok)
		}
		if !BuiltinVariableReadOnly(tokens, test.dialect) {
			t.Fatalf("%q was not read-only", test.name)
		}
	}
	for _, dialect := range []Dialect{CommonDialect, Emuera1824} {
		tokens, err := lexExpression([]byte("DAYNAME"), 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := BuiltinVariableInfoForDialect(tokens, dialect); ok {
			t.Fatalf("DAYNAME unexpectedly known in dialect %d", dialect)
		}
		if BuiltinVariableReadOnly(tokens, dialect) {
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
		if got := BuiltinVariableReadOnly(tokens, CommonDialect); got != test.want {
			t.Fatalf("%q: got %v want %v", test.expr, got, test.want)
		}
		if _, got := BuiltinVariableIdentity(tokens, CommonDialect); got != test.want {
			t.Fatalf("%q identity: got %v want %v", test.expr, got, test.want)
		}
	}
}

func TestIndexedBuiltinSubset(t *testing.T) {
	tests := []struct {
		expr    string
		dialect Dialect
		value   KnownValueType
		valid   bool
		proven  bool
	}{
		{expr: "RESULTS:0", value: KnownValueString, valid: true, proven: true},
		{expr: "RESULT:0", value: KnownValueNumeric, valid: true, proven: true},
		{expr: "NAME:0", value: KnownValueString, valid: true, proven: true},
		{expr: "CFLAG:0:0", value: KnownValueNumeric, valid: true, proven: true},
		{expr: "CFLAG:0:\"csv-name\"", value: KnownValueNumeric, valid: true, proven: true},
		{expr: "CDFLAG:0:0:0", value: KnownValueNumeric, valid: true, proven: true},
		{expr: "RESULT:0:1", valid: false, proven: true},
		{expr: "GAMEBASE_AUTHOR:0", valid: false, proven: true},
		{expr: "NAME:0:1", valid: false, proven: true},
		{expr: "CDFLAG:0:0", valid: false, proven: true},
		{expr: "RESULTS:INDEX", valid: false, proven: false},
		{expr: "RESULTS:0:INDEX", valid: false, proven: false},
		{expr: "RESULT@FUNC:0", valid: false, proven: false},
		{expr: "RESULT:0+1", valid: false, proven: false},
		{expr: "RESULT:(0)", valid: false, proven: false},
		{expr: "DAYNAME:0", dialect: EMEE, value: KnownValueString, valid: true, proven: true},
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
		if test.valid && KnownExpressionTypeForDialect(tokens, test.dialect) != test.value {
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
	for _, expression := range []string{`RESULTS:0+1`, `RESULT:0+"x"`, `NAME:0+1`, `GAMEBASE_AUTHOR:0`, `RESULT:0:1`, `DA:0:"x"`, `CFLAG:"x":0`, `CSTR:"x":0`, `CDFLAG:"x":0:0`} {
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
