package format

import (
	"bytes"
	"strings"
	"testing"
)

func TestConstantExpressionsPreserveInvalidIslands(t *testing.T) {
	for _, expression := range []string{"1+", "(1+)", "1(2)", "1:2", "1[2]", "++1", "1++", "!!1", "(1?2)#3", "1?(2#3)", `1+"x"`} {
		input := []byte("@F\nPRINTFORM {" + expression + "} {2+3}\n")
		result := FormatWithOptions(input, ERB, Options{})
		if len(result.Diagnostics) != 1 || !bytes.Contains(result.Output, []byte("{"+expression+"} {2 + 3}")) {
			t.Fatalf("expression=%q output=%q diagnostics=%+v", expression, result.Output, result.Diagnostics)
		}
	}
}

func TestCompoundLiteralTypes(t *testing.T) {
	for _, source := range []string{
		"@F\nIF \"a\"+\"b\"\nENDIF\n",
		"@F\nPRINTS \"a\"==\"b\"\n",
		"@F\nPRINTFORM {\"a\"*2}\n",
		"@F\nIF 1?\"a\"#\"b\"\nENDIF\n",
	} {
		result := FormatWithOptions([]byte(source), ERB, Options{})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("source=%q diagnostics=%+v", source, result.Diagnostics)
		}
	}
	for _, source := range []string{
		"@F\nIF \"a\"==\"b\"\nENDIF\n",
		"@F\nPRINTS 1?\"a\"#\"b\"\n",
		"@F\nPRINTFORM %\"a\"*2%\n",
		"@F\nRETURN !(1)+2<<3\n",
		"@F\nPRINTS \"a\"*(-1)\n",
	} {
		result := FormatWithOptions([]byte(source), ERB, Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%+v", source, result.Diagnostics)
		}
	}
}

func TestConstantProofDefersOpaqueSyntax(t *testing.T) {
	for _, source := range []string{
		"@F\nRETURN UNKNOWN+\n",
		"@F\nIF \"x\";!;1\nENDIF\n",
		"@F\nDEBUGPRINTFORM {1+} {\"x\"}\n",
		"@F\nVALUE=1+\n",
		"@F\nSKIPLOG =3+4\n",
		"@F\nARRAYSORT RESULTS,FORWARD,1+\n",
		"@F\nSORTCHARA FORWARD,1+\n",
		"@F\nRETURN " + strings.Repeat("(", 257) + "1+" + strings.Repeat(")", 257) + "\n",
	} {
		result := FormatWithOptions([]byte(source), ERB, Options{Dialect: EMEE})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%+v", source, result.Diagnostics)
		}
	}
	input := []byte("@F\nRETURN 1+\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{}})
	if !bytes.Equal(input, result.Output) || len(result.Diagnostics) != 0 {
		t.Fatalf("disabled parsing result=%+v", result)
	}
}

func TestBuiltinExpressionProof(t *testing.T) {
	for _, expression := range []string{`RESULT<>1`, `RESULT#1`, `RESULT?1`, `RESULT+"x"`, `RESULTS+1`, `!RESULTS`, `RESULT?1#"x"`, `RESULT=="x"`, `ARG+"x"`, `RESULT@FUNC+"x"`} {
		src := "@F\nIF " + expression + "\nENDIF\n"
		r := FormatWithOptions([]byte(src), ERB, Options{})
		if len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported || !bytes.Contains(r.Output, []byte("IF "+expression+"\n")) {
			t.Fatalf("expression=%q output=%q diagnostics=%+v", expression, r.Output, r.Diagnostics)
		}
	}
	for _, expression := range []string{`RESULT+1`, `RESULTS=="x"`, `USER+"x"`, `RESULT()`, `RESULT:INDEX+"x"`, `RESULT+"x";!;`} {
		src := "@F\nIF " + expression + "\nENDIF\n"
		r := FormatWithOptions([]byte(src), ERB, Options{})
		if len(r.Diagnostics) != 0 {
			t.Fatalf("expression=%q diagnostics=%+v", expression, r.Diagnostics)
		}
	}
	r := FormatWithOptions([]byte("@F\nPRINTFORM {RESULT+\"x\"} { 1+2 }\n"), ERB, Options{})
	if len(r.Diagnostics) != 1 || !bytes.Contains(r.Output, []byte("{RESULT+\"x\"} {1 + 2}")) {
		t.Fatalf("FORM output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}
