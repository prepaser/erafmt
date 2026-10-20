package format

import (
	"bytes"
	"strings"
	"testing"
)

func TestProvenOrdinaryExpressionShape(t *testing.T) {
	for _, source := range []string{
		"RESULT=1",
		"RESULT 1",
		"RESULT:0 1",
		"RESULT:0(1)",
		"RESULT:0[1]",
		"RESULT:0..1",
		"RESULT@FUNC:0",
		"RESULTS@FUNC:0",
		"CFLAG@FUNC:0",
		"NAME@FUNC:0",
		"IS",
		"RESULT+TO",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
}

func TestProvenOrdinaryExpressionShapeDefersUnknowns(t *testing.T) {
	for _, source := range []string{
		"UNKNOWN=1",
		"RESULT+UNKNOWN=1",
		"RESULT+UNKNOWN_FN(1)",
		"ARG@FUNC:0",
		"RESULT:INDEX(1)",
		"RESULT()",
		"F(1)",
		"A[0]",
		`RESULT+[[runtime]]`,
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was not deferred: %v", source, err)
		}
	}
}

func TestProvenExpressionShapeRejectsKnownBoundarySymbols(t *testing.T) {
	for _, source := range []string{
		":RESULT",
		".RESULT",
		"[RESULT]",
		"(:RESULT)",
		"(RESULT:)",
		"A#.",
		"A+.",
		"A?.",
		"1:RESULT",
		"RESULT:0+1:RESULT",
		"(RESULT):0",
		`"x":RESULT`,
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
}

func TestProvenExpressionShapeKeepsKnownAndUnknownBoundaries(t *testing.T) {
	for _, source := range []string{
		"RESULT:RESULT",
		"FLAG:(RESULT:0)",
		"CFLAG:0:1",
		"CFLAG:(0):1",
		"CDFLAG:0:(0):1",
		"RESULT:(0)",
		"A?1#2",
		"UNKNOWN+.",
		"RESULT:INDEX(1)",
		"F(RESULT)",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was rejected: %v", source, err)
		}
	}
}

func TestProvenExpressionShapeDefersScopedAndDeepProofs(t *testing.T) {
	for _, source := range []string{"LOCAL@TO + .", "LOCAL@RESULT + .", "ARGS@TO + ."} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was rejected: %v", source, err)
		}
	}
	deep := strings.Repeat("(", maxBuiltinProofDepth+1) + "RESULT." + strings.Repeat(")", maxBuiltinProofDepth+1)
	tokens, err := lexExpression([]byte(deep), 0)
	if err != nil {
		t.Fatalf("deep expression: %v", err)
	}
	if err := provenExpressionShape(tokens, CommonDialect, false); err != nil {
		t.Fatalf("deep expression was rejected: %v", err)
	}
}

func TestPrintVKnownFieldsBeforeMacros(t *testing.T) {
	for _, body := range []string{"A#.,UNKNOWN", "1,A#.,UNKNOWN", "=,a"} {
		result := FormatWithOptions([]byte("@F\nPRINTV "+body+"\n"), ERB, Options{})
		if len(result.Diagnostics) == 0 || !bytes.Contains(result.Output, []byte(body)) {
			t.Fatalf("body=%q output=%q diagnostics=%+v", body, result.Output, result.Diagnostics)
		}
	}
	for _, body := range []string{"UNKNOWN,A#.", "A,UNKNOWN,A#.", "=a"} {
		result := FormatWithOptions([]byte("@F\nPRINTV "+body+"\n"), ERB, Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("body=%q diagnostics=%+v", body, result.Diagnostics)
		}
	}
}

func TestProvenCaseShapeRejectsInvalidKnownTokens(t *testing.T) {
	for _, source := range []string{
		"FLAG:TO-1",
		"ARG:TO-1",
		"RESULT:TO-1",
		"DA:TO-1",
		"A+.",
		"1+.",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, true); err == nil {
			t.Fatalf("%q was accepted", source)
		}
		input := "@F\nSELECTCASE 1\nCASE " + source + "\nENDSELECT\n"
		result := FormatWithOptions([]byte(input), ERB, Options{})
		if len(result.Diagnostics) == 0 || !bytes.Contains(result.Output, []byte("CASE "+source)) {
			t.Fatalf("%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
	}
	for _, source := range []string{
		"1 TO 2",
		"1 TO (0)",
		"LOCAL@TO",
		"UNKNOWN TO 1",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, true); err != nil {
			t.Fatalf("%q was rejected: %v", source, err)
		}
	}
}

func TestKnownBoundaryShapeReachesDirectCallers(t *testing.T) {
	for _, test := range []struct {
		source string
		body   string
	}{
		{"@F\nRETURN 1 : RESULT\n", "1 : RESULT"},
		{"@F\nRETURN A#.\n", "A#."},
		{"@F\n#FUNCTION\nRETURNF A+.\n", "A+."},
		{"@F\nPRINTV A?.\n", "A?."},
		{"@F\nABS A#.\n", "A#."},
		{"@F\nPRINT_SPACE A+.\n", "A+."},
		{"@F\nSPLIT A#.,\",\",RESULT\n", "A#."},
		{"@F\nSAVEVAR A#.,\"y\",RESULT\n", "A#."},
		{"@F\nPRINTFORM {A?.}\n", "A?."},
	} {
		result := FormatWithOptions([]byte(test.source), ERB, Options{Dialect: CommonDialect})
		if len(result.Diagnostics) == 0 || !bytes.Contains(result.Output, []byte(test.body)) {
			t.Fatalf("source=%q diagnostics=%+v output=%q", test.source, result.Diagnostics, result.Output)
		}
	}
}

func TestExpressionShapeKeepsBuiltinIndexColonChains(t *testing.T) {
	for _, source := range []string{
		"@F\nRETURN CFLAG:0:1\n",
		"@F\nRETURN CFLAG:(0):1\n",
	} {
		result := FormatWithOptions([]byte(source), ERB, Options{Dialect: CommonDialect})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
	}
}

func TestProvenExpressionShapeRejectsAdjacentAfterClosedBuiltin(t *testing.T) {
	for _, source := range []string{
		"ABS(1)2",
		"STRLENS(\"x\")1",
		"MIN(1,2)3",
		"(1)2",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
	for _, source := range []string{
		"ABS(1)+2",
		"STRLENS(\"x\")",
		"UNKNOWN(1)2",
		"ABS(UNKNOWN)2",
		"(1)UNKNOWN",
		"(1)ABS(UNKNOWN)",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was not deferred: %v", source, err)
		}
	}
}

func TestProvenExpressionShapeRejectsKnownClosedOperators(t *testing.T) {
	for _, source := range []string{
		"ABS(1)++",
		"ABS(RESULT)++",
		"STRLENS(RESULTS)++",
		"ABS(1)!",
		"ABS(1)+",
		"(RESULT+1)++",
		"++ABS(1)",
		"++(RESULT+1)",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}

	for _, source := range []string{
		"(RESULT)++",
		"++(RESULT)",
		"RESULT:0++",
		"(RESULT:0)++",
		"ABS(1)+2",
		"UNKNOWN(1)++",
		"ABS(UNKNOWN)++",
		"++UNKNOWN(1)",
		"++ABS(UNKNOWN)",
		"(F(1))++",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was not accepted or deferred: %v", source, err)
		}
	}
}

func TestProvenExpressionShapeRejectsDirectIncrementStateErrors(t *testing.T) {
	for _, source := range []string{
		"RESULT++1",
		"RESULT++--",
		"++RESULT++",
		"RESULT++!1",
		"RESULT!++1",
		"RESULT + * RESULT++",
		"GAMEBASE_VERSION++1",
		"++GAMEBASE_VERSION+1",
		"++1+RESULT",
		"1++RESULT",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
	for _, source := range []string{
		"RESULT++ + 1",
		"RESULT+++1",
		"++RESULT + 1",
		"RESULT++ ? 1 # 2",
		"RESULT++ + RESULT++",
		"RESULT ? RESULT++ # 1",
		"++RESULT + RESULT++",
		"UNKNOWN++1",
		"++UNKNOWN+1",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was rejected: %v", source, err)
		}
	}
}

func TestProvenExpressionShapeAllowsScopedCaseKeywords(t *testing.T) {
	for _, source := range []string{"LOCAL@IS", "LOCAL@TO", "ARGS@IS+1", "ARGS@TO+1"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was rejected: %v", source, err)
		}
	}
}

func TestExpressionShapeIntegrationRejectsDirectIncrementStateErrors(t *testing.T) {
	options := Options{Dialect: EMEE, Select: []RuleID{BinarySpacing, TokenSpacing}}
	for _, source := range []string{
		"@F\nRETURN RESULT++1\n",
		"@F\nRETURN RESULT++--\n",
		"@F\nRETURN ++RESULT++\n",
		"@F\nRETURN RESULT!++1\n",
		"@F\nRETURN RESULT++ + * RESULT\n",
		"@F\nRETURN RESULT++ * * RESULT\n",
		"@F\nRETURN RESULT++ ? 1 + * 2 # 3\n",
		"@F\nRETURN RESULT++ + (RESULT + *RESULT)\n",
		"@F\nRETURN RESULT++ + (RESULT * *RESULT)\n",
		"@F\nRETURN RESULT++ + (RESULT ? *1 # 2)\n",
		"@F\nRETURN RESULT++ + (!*RESULT)\n",
		"@F\nRETURN ++(RESULT++ + 1)\n",
		"@F\nRETURN (RESULT++ + 1)++\n",
		"@F\nRETURN GAMEBASE_VERSION++1\n",
		"@F\nRETURN ++GAMEBASE_VERSION+1\n",
		"@F\nPRINTFORM {RESULT++1}\n",
		"@F\nPRINTFORM {++GAMEBASE_VERSION+1}\n",
		"@F\nSELECTCASE 1\nCASE IS > RESULT++1\nENDSELECT\n",
	} {
		result := FormatWithOptions([]byte(source), ERB, options)
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_expression" || !bytes.Equal(result.Output, []byte(source)) {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
	}
	for _, source := range []string{
		"@F\nRETURN RESULT++ + 1\n",
		"@F\nRETURN RESULT+++1\n",
		"@F\nRETURN ++RESULT + 1\n",
		"@F\nRETURN RESULT++ + RESULT++\n",
		"@F\nRETURN RESULT ? RESULT++ # 1\n",
		"@F\nRETURN ++RESULT + RESULT++\n",
		"@F\nRETURN (RESULT:(RESULT++))++\n",
		"@F\nRETURN ++(RESULT:(RESULT++))\n",
		"@F\nRETURN RESULT:(RESULT++)++\n",
		"@F\nPRINTFORM {RESULT+++1}\n",
		"@F\nRETURN LOCAL@IS\n",
		"@F\nRETURN ARGS@TO+1\n",
		"@F\nSELECTCASE 1\nCASE IS > ++A\nCASE A++ TO 2\nENDSELECT\n",
		"@F\nSELECTCASE 1\nCASE IS > UNKNOWN++1\nENDSELECT\n",
	} {
		result := FormatWithOptions([]byte(source), ERB, options)
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
	}
}

func TestProvenExpressionShapeRejectsMalformedClosedOperatorChains(t *testing.T) {
	for _, source := range []string{
		"ABS(1)+?2#3",
		"ABS(1)=1",
		"ABS(1)<>1",
		"ABS(1)+*1",
		"ABS(1)+!",
		"ABS(1)+~",
		"ABS(1)+-",
		"ABS(1)+--",
		"ABS(1)+:0",
		"((ABS(1)))++",
		"++(ABS(1))",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}

	for _, source := range []string{
		"ABS(1)+-2",
		"ABS(1)?2#3",
		"(RESULT)++",
		"A++",
		"UNKNOWN(1)+?2#3",
		"UNKNOWN(1)+*1",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was rejected: %v", source, err)
		}
	}
}

func TestProvenExpressionShapeRejectsKnownNonLocalBuiltin(t *testing.T) {
	for _, source := range []string{"RESULT@FUNC", "CFLAG@FUNC", "NAME@FUNC", "RESULT@UNKNOWN"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
	for _, source := range []string{"ARG@FUNC", "ARGS@FUNC", "LOCAL@FUNC", "LOCALS@FUNC", "UNKNOWN@FUNC"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was not deferred: %v", source, err)
		}
	}
}

func TestProvenExpressionShapeRejectsKnownClosedPostfix(t *testing.T) {
	for _, source := range []string{
		"ABS(1)(2)",
		"ABS(1)[2]",
		"ABS(1):2",
		"ABS(1).2",
		`ABS(1)@"x {2+3}"`,
		"(1)(2)",
		"(1)[2]",
		"(1):2",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}

	for _, source := range []string{
		"UNKNOWN(1)(2)",
		"UNKNOWN(1)[2]",
		"UNKNOWN(1):2",
		`UNKNOWN(1)@"x {2+3}"`,
		"ABS(UNKNOWN)(2)",
		"(F(1))[2]",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was not deferred: %v", source, err)
		}
	}

	for _, source := range []string{"CFLAG:(0):1", "NAME:(0):1"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was rejected: %v", source, err)
		}
	}
}

func TestProvenCaseShape(t *testing.T) {
	for _, source := range []string{
		"IS",
		"IS = 1",
		"IS ? 1",
		"IS # 1",
		"IS >",
		"IS > RESULT=1",
		"TO 1",
		"1 TO",
		"1 TO RESULT=1",
		"1 TO 2 +",
		"1 TO 2 TO 3",
		"1 IS = 2",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, true); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
	for _, source := range []string{
		"IS + 1",
		"IS > 1",
		"IS > ++A",
		"IS > --A",
		"IS > A++",
		"IS > A--",
		"IS > ++(A)",
		"IS > (A)++",
		"IS > ++(A)+1",
		"IS > 1+A++",
		"A++ TO 2",
		"--A TO 2",
		"1 TO 2",
		"1, 2 TO 3",
		"1,",
		"UNKNOWN TO",
		"1 TO UNKNOWN",
		"UNKNOWN_FN()",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, true); err != nil {
			t.Fatalf("%q was rejected: %v", source, err)
		}
	}
}

func TestCaseKeywordsAreASCIIOnly(t *testing.T) {
	for _, keyword := range []string{"IS", "is", "TO", "to"} {
		if !isCaseKeyword([]byte(keyword)) {
			t.Fatalf("%q was not recognized as a CASE keyword", keyword)
		}
		tokens, err := lexExpression([]byte(keyword), 0)
		if err != nil {
			t.Fatalf("%q: %v", keyword, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, true); err == nil {
			t.Fatalf("%q was accepted as a CASE clause", keyword)
		}
	}
	for _, identifier := range []string{"Iſ", "iſ", "ıS", "KK"} {
		if isCaseKeyword([]byte(identifier)) {
			t.Fatalf("%q was misclassified as a CASE keyword", identifier)
		}
	}

	for _, source := range []string{
		"@F\n#DIM Iſ,1\nRETURN Iſ:0+1\n",
		"@F\n#DIM Iſ,1\nIſ:0=1+2\nRETURN 0\n",
		"@F\n#DIM Iſ,1\nIſ:0++\nRETURN 0\n",
		"@F\n#DIM Iſ\nRETURN Iſ+1\n",
		"@F\nSELECTCASE Iſ\nCASE 1\nRETURN 0\nENDSELECT\n",
		"@F\nSELECTCASE 1\nCASE Iſ\nRETURN 0\nENDSELECT\n",
		"@F\n#DIM KK,1\nRETURN KK+1\n",
		"@F\n#DIM ıS,1\nRETURN ıS+1\n",
	} {
		for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
			result := FormatWithOptions([]byte(source), ERB, Options{Dialect: dialect})
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == Unsupported || diagnostic.Severity == Error {
					t.Fatalf("source=%q dialect=%d false positive: %+v", source, dialect, diagnostic)
				}
			}
			again := FormatWithOptions(result.Output, ERB, Options{Dialect: dialect})
			if !bytes.Equal(result.Output, again.Output) || len(again.Changes) != 0 {
				t.Fatalf("source=%q dialect=%d is not idempotent: %+v", source, dialect, again)
			}
		}
	}
}

func TestProvenCaseShapeRejectsInvalidIncrements(t *testing.T) {
	for _, source := range []string{
		"IS > ++1",
		"IS > 1++",
		"IS > ++NAME",
		"IS > NAME++",
		"IS > ++ITEMPRICE",
		"IS > ITEMPRICE++",
		"IS > ++NAME:0",
		"IS > NAME:0++",
		"IS > ++A++",
		"IS > ++(A+1)",
		"IS > (A+1)++",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, true); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
}

func TestProvenExpressionShapeNestedContainers(t *testing.T) {
	for _, source := range []string{"(1,2)", "RESULT+(1,2)", "1 TO (2,3)", "IS >(1,2)"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, bytes.Contains([]byte(source), []byte("TO")) || bytes.HasPrefix([]byte(source), []byte("IS"))); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
	for _, source := range []string{"F(1,2)", "RESULT+F(1,2)"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was rejected: %v", source, err)
		}
	}
}

func TestProvenExpressionShapeStringIncrement(t *testing.T) {
	for _, source := range []string{"RESULTS++", "--RESULTS", "NAME:0++", "++CSTR:0"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
	for _, source := range []string{"RESULT++", "CFLAG:0++"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was rejected: %v", source, err)
		}
	}
}

func TestKnownExpressionTypeForDialectPropagatesIndexedCompound(t *testing.T) {
	for _, test := range []struct {
		source string
		want   knownValueType
	}{
		{"CFLAG:0:0+1", knownValueNumeric},
		{"NAME:0*2", knownValueString},
		{"NAME:0+\"x\"", knownValueString},
	} {
		tokens, err := lexExpression([]byte(test.source), 0)
		if err != nil {
			t.Fatalf("%q: %v", test.source, err)
		}
		if got := knownExpressionTypeForDialect(tokens, CommonDialect); got != test.want {
			t.Fatalf("%q type=%v, want %v", test.source, got, test.want)
		}
	}
}

func TestExpressionShapeIntegrationCoversCallsAndForms(t *testing.T) {
	for _, source := range []string{
		"@F\nIF (1,2)\nENDIF\n",
		"@F\nCALL F((1,2))\n",
		"@F\nPRINTV RESULT=1\n",
		"@F\nPRINTFORM {(1,2)}\n",
		"@F\nSELECTCASE 1\nCASE IS >(1,2)\nENDSELECT\n",
	} {
		r := FormatWithOptions([]byte(source), ERB, Options{Select: []RuleID{BinarySpacing, CommaSpacing, DelimiterSpacing, CaseSpacing, FormPadding}})
		if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_expression" {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, r.Diagnostics, r.Output)
		}
		if !bytes.Equal(r.Output, []byte(source)) {
			t.Fatalf("source=%q changed to %q", source, r.Output)
		}
	}
}

func TestExpressionShapeIntegrationRejectsKnownClosedPostfix(t *testing.T) {
	for _, source := range []string{
		"@F\nRETURN ABS(1)(2)\n",
		"@F\nRETURN ABS(1)[2]\n",
		"@F\nRETURN ABS(1):2\n",
		"@F\nRETURN ABS(1)@\"x {2+3}\"\n",
		"PRINT_SPACE ABS(1)(2)\n",
	} {
		result := FormatWithOptions([]byte(source), ERB, Options{Dialect: EMEE, Select: []RuleID{BinarySpacing, DelimiterSpacing, IndexSpacing, FormPadding}})
		if len(result.Diagnostics) != 1 || !bytes.Equal(result.Output, []byte(source)) {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
	}
}

func TestExpressionShapeIntegrationRejectsKnownClosedOperators(t *testing.T) {
	for _, source := range []string{
		"@F\nRETURN ABS(1)++\n",
		"@F\nRETURN ABS(RESULT)++\n",
		"@F\nRETURN STRLENS(RESULTS)++\n",
		"@F\nRETURN ABS(1)!\n",
		"@F\nRETURN ABS(1)+\n",
		"@F\nRETURN (RESULT+1)++\n",
		"@F\nRETURN ++ABS(1)\n",
		"@F\nRETURN ++(RESULT+1)\n",
		"@F\nRETURN RESULT@FUNC\n",
		"@F\nRETURN CFLAG@FUNC\n",
		"@F\nRETURNF NAME@FUNC\n",
		"@F\nRETURN ABS(1)+?2#3\n",
		"@F\nRETURN ABS(1)=1\n",
		"@F\nRETURN ABS(1)<>1\n",
		"@F\nRETURN ABS(1)+*1\n",
		"@F\nRETURN ABS(1)+!\n",
		"@F\nRETURN ABS(1)+~\n",
		"@F\nRETURN ABS(1)+-\n",
		"@F\nRETURN ABS(1)+--\n",
		"@F\nRETURN ABS(1)+:0\n",
		"@F\nRETURN (ABS(1))++\n",
		"@F\nRETURN ++(ABS(1))\n",
	} {
		result := FormatWithOptions([]byte(source), ERB, Options{Dialect: EMEE, Select: []RuleID{BinarySpacing, TokenSpacing}})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_expression" || !bytes.Equal(result.Output, []byte(source)) {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
	}

	for _, source := range []string{
		"@F\nRETURN (RESULT)++\n",
		"@F\nRETURN ++(RESULT)\n",
		"@F\nRETURN RESULT:0++\n",
		"@F\nRETURN (RESULT:0)++\n",
		"@F\nRETURN ABS(1)+2\n",
		"@F\nRETURN UNKNOWN(1)++\n",
		"@F\nRETURN ABS(UNKNOWN)++\n",
		"@F\nRETURN ++UNKNOWN(1)\n",
		"@F\nRETURN ++ABS(UNKNOWN)\n",
		"@F\nRETURN ARG@FUNC\n",
		"@F\nRETURN UNKNOWN@FUNC\n",
	} {
		result := FormatWithOptions([]byte(source), ERB, Options{Dialect: EMEE, Select: []RuleID{BinarySpacing, TokenSpacing}})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
	}
}

func TestIndexedCompoundTypeIntegration(t *testing.T) {
	for _, source := range []string{
		"@F\nHTML_PRINT CFLAG:0:0+1\n",
		"@F\nSETCOLOR NAME:0*2\n",
		"@F\nSETCOLOR NAME:0+\"x\"\n",
	} {
		r := FormatWithOptions([]byte(source), ERB, Options{Dialect: EMEE, Select: []RuleID{BinarySpacing, CommaSpacing}})
		if len(r.Diagnostics) != 1 {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, r.Diagnostics, r.Output)
		}
		if !bytes.Equal(r.Output, []byte(source)) {
			t.Fatalf("source=%q changed to %q", source, r.Output)
		}
	}
}

func TestExpressionShapeIntegrationKeepsContexts(t *testing.T) {
	options := Options{Select: []RuleID{BinarySpacing, TernarySpacing, CaseSpacing, TokenSpacing}}
	for _, test := range []struct {
		name string
		src  string
	}{
		{"assignment", "@F\nIF RESULT=1\nENDIF\n"},
		{"adjacent", "@F\nIF RESULT:0 1\nENDIF\n"},
		{"call", "@F\nIF RESULT:0(1)\nENDIF\n"},
		{"bracket", "@F\nIF RESULT:0[1]\nENDIF\n"},
		{"namespace", "@F\nIF RESULT:0..1\nENDIF\n"},
		{"reserved", "@F\nIF IS\nENDIF\n"},
		{"scoped-reference", "@F\nIF NAME@FUNC\nENDIF\n"},
		{"case-is-missing", "@F\nSELECTCASE 1\nCASE IS >\nENDSELECT\n"},
		{"case-to-assignment", "@F\nSELECTCASE 1\nCASE 1 TO RESULT=1\nENDSELECT\n"},
		{"case-duplicate-to", "@F\nSELECTCASE 1\nCASE 1 TO 2 TO 3\nENDSELECT\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := FormatWithOptions([]byte(test.src), ERB, options)
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_expression" {
				t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
			}
			if !bytes.Equal(result.Output, []byte(test.src)) {
				t.Fatalf("invalid expression changed: %q", result.Output)
			}
		})
	}

	for _, test := range []struct {
		name string
		src  string
	}{
		{"case-compatibility", "@F\nSELECTCASE 1\nCASE IS +1\nCASE 1 TO 2\nENDSELECT\n"},
		{"bare-call", "@F\nIF RESULT()\nENDIF\n"},
		{"header-default", "@F(ARG=1+2)\nRETURN 0\n"},
		{"assignment-rhs", "@F\nRESULT = RESULT=1\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := FormatWithOptions([]byte(test.src), ERB, options)
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
			}
		})
	}
}

func TestExpressionShapeRejectsKnownAdjacentOperands(t *testing.T) {
	for _, source := range []string{
		"@F\nRETURN 1A\n",
		"@F\nRETURN 1RESULT\n",
		"@F\nRETURN 1@\"x\"\n",
		"@F\nPRINTFORM {1A}\n",
	} {
		result := FormatWithOptions([]byte(source), ERB, Options{Select: []RuleID{BinarySpacing, TokenSpacing, FormPadding}})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_expression" {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
		if !bytes.Equal(result.Output, []byte(source)) {
			t.Fatalf("source=%q changed to %q", source, result.Output)
		}
	}

	for _, source := range []string{
		"@F\nRETURN UNKNOWN 1\n",
		"@F\nRETURN UNKNOWN(1)\n",
	} {
		result := FormatWithOptions([]byte(source), ERB, Options{Select: []RuleID{BinarySpacing, TokenSpacing}})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("unknown source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
	}
}

func TestProvenExpressionShapeRejectsPostfixAfterParenthesizedIndex(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		for _, source := range []string{"RESULT:(1+2)", "CFLAG:(0):1"} {
			tokens, err := lexExpression([]byte(source), 0)
			if err != nil {
				t.Fatalf("dialect=%v source=%q: %v", dialect, source, err)
			}
			if err := provenExpressionShape(tokens, dialect, false); err != nil {
				t.Fatalf("dialect=%v valid source=%q: %v", dialect, source, err)
			}
		}
		for _, source := range []string{
			"RESULT:(1+2)[0]",
			"RESULT:(1+2).0",
			"CFLAG:(0):1[0]",
		} {
			tokens, err := lexExpression([]byte(source), 0)
			if err != nil {
				t.Fatalf("dialect=%v source=%q: %v", dialect, source, err)
			}
			if err := provenExpressionShape(tokens, dialect, false); err == nil {
				t.Fatalf("dialect=%v source=%q was accepted", dialect, source)
			}
		}
	}
}

func TestParenthesizedIndexShapeDefersDynamicFields(t *testing.T) {
	for _, source := range []string{
		"RESULT:(UNKNOWN)[0]",
		"RESULT:(F(1))[0]",
		"RESULT:(@\"x\")[0]",
		"RESULT[0]",
		"A[0]",
		"F(A,B[0])",
	} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := provenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was not deferred: %v", source, err)
		}
	}
}

func TestParenthesizedIndexShapeIntegrationPreservesRejectedBodies(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		source := []byte("@F\nRETURN RESULT:(1+2)[0]\nRETURN 1+2\n")
		result := FormatWithOptions(source, ERB, Options{Dialect: dialect, Select: []RuleID{BinarySpacing}})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_expression" || result.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("dialect=%v diagnostics=%+v output=%q", dialect, result.Diagnostics, result.Output)
		}
		want := []byte("@F\nRETURN RESULT:(1+2)[0]\nRETURN 1 + 2\n")
		if !bytes.Equal(result.Output, want) {
			t.Fatalf("dialect=%v rejected body changed to %q", dialect, result.Output)
		}
	}

	continuation := []byte("@F\n{\nRETURN RESULT:(1+2)[0]\n}\n")
	separator := " "
	result := FormatWithOptions(continuation, ERB, Options{ContinuationSeparator: &separator, Select: []RuleID{BinarySpacing}})
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_expression" || !bytes.Equal(result.Output, continuation) {
		t.Fatalf("continuation diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
}

func TestParenthesizedIndexShapeSelectNonePreservesInput(t *testing.T) {
	source := []byte("@F\nRETURN RESULT:(1+2)[0]\n")
	result := FormatWithOptions(source, ERB, Options{Select: []RuleID{}})
	if len(result.Diagnostics) != 0 || !bytes.Equal(result.Output, source) || len(result.Changes) != 0 {
		t.Fatalf("select-none diagnostics=%+v output=%q changes=%v", result.Diagnostics, result.Output, result.Changes)
	}
}

func TestScalarCharacterBuiltinIndexShape(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		for _, source := range []string{"NAME:0", "NAME:(0)", "ISASSI:0", "NO:(0)"} {
			tokens, err := lexExpression([]byte(source), 0)
			if err != nil {
				t.Fatalf("dialect=%v source=%q: %v", dialect, source, err)
			}
			if err := provenExpressionShape(tokens, dialect, false); err != nil {
				t.Fatalf("dialect=%v valid source=%q: %v", dialect, source, err)
			}
		}
		for _, source := range []string{
			"NAME:0[0]",
			"NAME:(0)[0]",
			"NAME:(0).0",
			"ISASSI:0[0]",
			"NO:(0)[0]",
		} {
			tokens, err := lexExpression([]byte(source), 0)
			if err != nil {
				t.Fatalf("dialect=%v source=%q: %v", dialect, source, err)
			}
			if err := provenExpressionShape(tokens, dialect, false); err == nil {
				t.Fatalf("dialect=%v source=%q was accepted", dialect, source)
			}
		}
	}
}

func TestScalarCharacterBuiltinIndexShapeIntegration(t *testing.T) {
	source := []byte("@F\nPRINTV NAME:(0)[0]\nPRINTV NAME:0[0]\n")
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		result := FormatWithOptions(source, ERB, Options{Dialect: dialect, Select: []RuleID{BinarySpacing}})
		if len(result.Diagnostics) != 2 || !bytes.Equal(result.Output, source) {
			t.Fatalf("dialect=%v diagnostics=%+v output=%q", dialect, result.Diagnostics, result.Output)
		}
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.Code != "unsupported_expression" || diagnostic.Severity != Unsupported {
				t.Fatalf("dialect=%v diagnostic=%+v", dialect, diagnostic)
			}
		}
	}
}
