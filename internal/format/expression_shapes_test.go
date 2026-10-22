package format

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

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
	for _, source := range []string{"FLAG:TO-1", "ARG:TO-1", "RESULT:TO-1", "DA:TO-1", "A+.", "1+."} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := erabasic.ProvenExpressionShape(tokens, erabasic.CommonDialect, true); err == nil {
			t.Fatalf("%q was accepted", source)
		}
		input := "@F\nSELECTCASE 1\nCASE " + source + "\nENDSELECT\n"
		result := FormatWithOptions([]byte(input), ERB, Options{})
		if len(result.Diagnostics) == 0 || !bytes.Contains(result.Output, []byte("CASE "+source)) {
			t.Fatalf("%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
	}
	for _, source := range []string{"1 TO 2", "1 TO (0)", "LOCAL@TO", "UNKNOWN TO 1"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := erabasic.ProvenExpressionShape(tokens, erabasic.CommonDialect, true); err != nil {
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
	for _, source := range []string{"@F\nRETURN CFLAG:0:1\n", "@F\nRETURN CFLAG:(0):1\n"} {
		result := FormatWithOptions([]byte(source), ERB, Options{Dialect: CommonDialect})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
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

func TestCaseKeywordsAreASCIIOnly(t *testing.T) {
	for _, keyword := range []string{"IS", "is", "TO", "to"} {
		if !erabasic.IsCaseKeyword([]byte(keyword)) {
			t.Fatalf("%q was not recognized as a CASE keyword", keyword)
		}
		tokens, err := lexExpression([]byte(keyword), 0)
		if err != nil {
			t.Fatalf("%q: %v", keyword, err)
		}
		if err := erabasic.ProvenExpressionShape(tokens, erabasic.CommonDialect, true); err == nil {
			t.Fatalf("%q was accepted as a CASE clause", keyword)
		}
	}
	for _, identifier := range []string{"Iſ", "iſ", "ıS", "KK"} {
		if erabasic.IsCaseKeyword([]byte(identifier)) {
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
		for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
			result := FormatWithOptions([]byte(source), ERB, Options{Dialect: Dialect(dialect)})
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == Unsupported || diagnostic.Severity == Error {
					t.Fatalf("source=%q dialect=%d false positive: %+v", source, dialect, diagnostic)
				}
			}
			again := FormatWithOptions(result.Output, ERB, Options{Dialect: Dialect(dialect)})
			if !bytes.Equal(result.Output, again.Output) || len(again.Changes) != 0 {
				t.Fatalf("source=%q dialect=%d is not idempotent: %+v", source, dialect, again)
			}
		}
	}
}

func TestExpressionShapeIntegrationCoversCallsAndForms(t *testing.T) {
	for _, source := range []string{"@F\nIF (1,2)\nENDIF\n", "@F\nCALL F((1,2))\n", "@F\nPRINTV RESULT=1\n", "@F\nPRINTFORM {(1,2)}\n", "@F\nSELECTCASE 1\nCASE IS >(1,2)\nENDSELECT\n"} {
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
	for _, source := range []string{"@F\nRETURN ABS(1)(2)\n", "@F\nRETURN ABS(1)[2]\n", "@F\nRETURN ABS(1):2\n", "@F\nRETURN ABS(1)@\"x {2+3}\"\n", "PRINT_SPACE ABS(1)(2)\n"} {
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
	for _, source := range []string{"@F\nHTML_PRINT CFLAG:0:0+1\n", "@F\nSETCOLOR NAME:0*2\n", "@F\nSETCOLOR NAME:0+\"x\"\n"} {
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
	for _, source := range []string{"@F\nRETURN 1A\n", "@F\nRETURN 1RESULT\n", "@F\nRETURN 1@\"x\"\n", "@F\nPRINTFORM {1A}\n"} {
		result := FormatWithOptions([]byte(source), ERB, Options{Select: []RuleID{BinarySpacing, TokenSpacing, FormPadding}})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_expression" {
			t.Fatalf("source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
		if !bytes.Equal(result.Output, []byte(source)) {
			t.Fatalf("source=%q changed to %q", source, result.Output)
		}
	}
	for _, source := range []string{"@F\nRETURN UNKNOWN 1\n", "@F\nRETURN UNKNOWN(1)\n"} {
		result := FormatWithOptions([]byte(source), ERB, Options{Select: []RuleID{BinarySpacing, TokenSpacing}})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("unknown source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
	}
}

func TestParenthesizedIndexShapeIntegrationPreservesRejectedBodies(t *testing.T) {
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		source := []byte("@F\nRETURN RESULT:(1+2)[0]\nRETURN 1+2\n")
		result := FormatWithOptions(source, ERB, Options{Dialect: Dialect(dialect), Select: []RuleID{BinarySpacing}})
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

func TestScalarCharacterBuiltinIndexShapeIntegration(t *testing.T) {
	source := []byte("@F\nPRINTV NAME:(0)[0]\nPRINTV NAME:0[0]\n")
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		result := FormatWithOptions(source, ERB, Options{Dialect: Dialect(dialect), Select: []RuleID{BinarySpacing}})
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
