package format

import (
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestWrappedBuiltinExpressionProof(t *testing.T) {
	for _, test := range []struct {
		source string
		valid  bool
	}{
		{"@F\nRETURN (RESULT:0)+1\n", true},
		{"@F\nRETURN ++(RESULT:0)+1\n", true},
		{"@F\nRETURN (RESULT:0)++ + 1\n", true},
		{"@F\nRETURN ++(RESULT:(LOCAL:2))\n", true},
		{"@F\nRETURN ++(ITEMPRICE:(LOCAL:2))\n", true},
		{"@F\nRETURN ++(NAME:0)+1\n", true},
		{"@F\nHTML_PRINT (NAME:0)++\n", false},
		{"@F\nRETURN (NAME:0:0)++\n", false},
	} {
		result := FormatWithOptions([]byte(test.source), ERB, Options{})
		if got := len(result.Diagnostics) == 0; got != test.valid {
			t.Fatalf("source=%q valid=%v diagnostics=%+v", test.source, test.valid, result.Diagnostics)
		}
	}
	for _, expression := range []string{"++(ITEMPRICE:(LOCAL:2))", "(ITEMPRICE:(LOCAL:2))++"} {
		tokens, err := lexExpression([]byte(expression), 0)
		if err != nil {
			t.Fatalf("expression=%q lex: %v", expression, err)
		}
		if _, _, proven := erabasic.BuiltinExpressionType(tokens, erabasic.CommonDialect); proven {
			t.Fatalf("expression=%q incorrectly proved dynamic index mutability", expression)
		}
	}
}

func TestDebugFormPayloadsAreOpaque(t *testing.T) {
	for _, command := range []string{"DEBUGPRINTFORM", "DEBUGPRINTFORML"} {
		for _, payload := range []string{`{(1,2)}`, `{1,2,3,4}`, `{RESULT=1}`, `%1,2,3,4%`, `\@(1,2)?yes#no\@`, `{(1`, `{"unterminated}`, `{1\`, `\@(1?yes#no`, `{ 1+2 }`} {
			source := []byte("@F\n" + command + " " + payload + "\n")
			result := FormatWithOptions(source, ERB, Options{})
			want := []byte("@F\n    " + command + " " + payload + "\n")
			if len(result.Diagnostics) != 0 || string(result.Output) != string(want) {
				t.Fatalf("%s payload=%q output=%q diagnostics=%+v", command, payload, result.Output, result.Diagnostics)
			}
		}
	}
	ordinary := FormatWithOptions([]byte("@F\nPRINTFORM {(1,2)}\n"), ERB, Options{})
	if len(ordinary.Diagnostics) == 0 {
		t.Fatal("ordinary PRINTFORM accepted malformed payload")
	}
}

func TestDirectIndexedBuiltinValueTypes(t *testing.T) {
	for _, tc := range []struct {
		line    string
		dialect erabasic.Dialect
		reject  bool
	}{
		{`RETURN NAME:(1+2)`, erabasic.CommonDialect, true},
		{`RETURN NAME:RESULT`, erabasic.CommonDialect, true},
		{`HTML_PRINT ITEMPRICE:(RESULT+1)`, erabasic.CommonDialect, true},
		{`HTML_PRINT NAME:(RESULT+1)`, erabasic.CommonDialect, false},
		{`RETURN NAME:USER_IDX`, erabasic.CommonDialect, false},
		{`RETURN NAME:ABS(1)`, erabasic.CommonDialect, false},
		{`RETURN NAME:(LOCAL@F:0)`, erabasic.CommonDialect, false},
		{`RETURN DAYNAME:(1+2)`, erabasic.CommonDialect, false},
		{`RETURN DAYNAME:(1+2)`, erabasic.EMEE, true},
	} {
		src := []byte("@F\n" + tc.line + "\n")
		r := FormatWithOptions(src, ERB, Options{Dialect: Dialect(tc.dialect)})
		if (len(r.Diagnostics) != 0) != tc.reject {
			t.Fatalf("%q dialect=%v diagnostics=%+v", tc.line, tc.dialect, r.Diagnostics)
		}
		if tc.reject && !strings.Contains(string(r.Output), tc.line) {
			t.Fatalf("rejected body changed: %q", r.Output)
		}
	}
}

func TestCompoundIndexedBuiltinProof(t *testing.T) {
	for _, tc := range []struct {
		line   string
		reject bool
	}{
		{`RETURN NAME:(0)+1`, true},
		{`HTML_PRINT ITEMPRICE:(RESULT+1)+"x"`, true},
		{`RETURN NAME:(RESULT:(0))+1`, true},
		{`RETURN RESULT:(0):0+1`, true},
		{`RETURN GAMEBASE_AUTHOR:(0)=="x"`, true},
		{`RETURN RESULT:("x")+1`, true},
		{`RETURN ITEMPRICE:(RESULT:0:0)+1`, true},
		{`RETURN ITEMPRICE:(CFLAG:0:0:0)+1`, true},
		{`RETURN ITEMPRICE:(NAME:0:0)+1`, true},
		{`RETURN ITEMPRICE:(RESULT:0:0+1)+1`, true},
		{`RETURN ITEMPRICE:((RESULT:0:0)+1)+1`, true},
		{`RETURN ITEMPRICE:(NAME:0:0=="x")+1`, true},
		{`RETURN RESULT:(1+"x")+1`, true},
		{`RETURN RESULT:(RESULT+"x")+1`, true},
		{`RETURN RESULT:(1 2)+1`, true},
		{`RETURN RESULT:(RESULT 1)+1`, true},
		{`RETURN RESULT:(1?RESULT:0:0#1)+1`, true},
		{`RETURN CSTR:(0)++`, true},
		{`RETURN --(CSTR:(RESULT+1))`, true},
		{`RETURN NAME:(0)=="x"`, false},
		{`RETURN ITEMPRICE:(RESULT+1)+1`, false},
		{`RETURN ITEMPRICE:(NAME:0=="x")+1`, false},
		{`RETURN ITEMPRICE:(RESULT:0:0+UNKNOWN)+1`, false},
		{`RETURN CFLAG:(0):("key")+1`, false},
		{`RETURN RESULT:(0)++`, false},
		{`HTML_PRINT NAME:(0)+"x"`, false},
		{`RETURN NAME:(USR_INDEX)+1`, false},
		{`RETURN NAME:ABS(1)+1`, false},
		{`RETURN NAME:(LOCAL@F:0)+1`, false},
		{`RETURN NAME:(RESULT+1)+F(1)`, false},
		{`RETURN NAME:(RESULT+1)+[[ALIAS]]`, false},
		{`RETURN NAME:(RESULT+1)+;!;1`, false},
	} {
		src := []byte("@F\n" + tc.line + "\n")
		r := FormatWithOptions(src, ERB, Options{})
		if (len(r.Diagnostics) != 0) != tc.reject {
			t.Fatalf("%q diagnostics=%+v", tc.line, r.Diagnostics)
		}
		if tc.reject && !strings.Contains(string(r.Output), tc.line) {
			t.Fatalf("rejected body changed: %q", r.Output)
		}
		if second := FormatWithOptions(r.Output, ERB, Options{}); string(second.Output) != string(r.Output) {
			t.Fatalf("%q unstable output=%q", tc.line, second.Output)
		}
	}
}
