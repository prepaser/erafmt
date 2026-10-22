package erabasic

import (
	"bytes"
	"strings"
	"testing"
)

func TestProvenOrdinaryExpressionShape(t *testing.T) {
	for _, source := range []string{"RESULT=1", "RESULT 1", "RESULT:0 1", "RESULT:0(1)", "RESULT:0[1]", "RESULT:0..1", "RESULT@FUNC:0", "RESULTS@FUNC:0", "CFLAG@FUNC:0", "NAME@FUNC:0", "IS", "RESULT+TO"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
}

func TestProvenOrdinaryExpressionShapeDefersUnknowns(t *testing.T) {
	for _, source := range []string{"UNKNOWN=1", "RESULT+UNKNOWN=1", "RESULT+UNKNOWN_FN(1)", "ARG@FUNC:0", "RESULT:INDEX(1)", "RESULT()", "F(1)", "A[0]", `RESULT+[[runtime]]`} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was not deferred: %v", source, err)
		}
	}
}

func TestProvenExpressionShapeRejectsKnownBoundarySymbols(t *testing.T) {
	for _, source := range []string{":RESULT", ".RESULT", "[RESULT]", "(:RESULT)", "(RESULT:)", "A#.", "A+.", "A?.", "1:RESULT", "RESULT:0+1:RESULT", "(RESULT):0", `"x":RESULT`} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
}

func TestProvenExpressionShapeKeepsKnownAndUnknownBoundaries(t *testing.T) {
	for _, source := range []string{"RESULT:RESULT", "FLAG:(RESULT:0)", "CFLAG:0:1", "CFLAG:(0):1", "CDFLAG:0:(0):1", "RESULT:(0)", "A?1#2", "UNKNOWN+.", "RESULT:INDEX(1)", "F(RESULT)"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err != nil {
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
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was rejected: %v", source, err)
		}
	}
	deep := strings.Repeat("(", maxBuiltinProofDepth+1) + "RESULT." + strings.Repeat(")", maxBuiltinProofDepth+1)
	tokens, err := lexExpression([]byte(deep), 0)
	if err != nil {
		t.Fatalf("deep expression: %v", err)
	}
	if err := ProvenExpressionShape(tokens, CommonDialect, false); err != nil {
		t.Fatalf("deep expression was rejected: %v", err)
	}
}

func TestProvenExpressionShapeRejectsAdjacentAfterClosedBuiltin(t *testing.T) {
	for _, source := range []string{"ABS(1)2", "STRLENS(\"x\")1", "MIN(1,2)3", "(1)2"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
	for _, source := range []string{"ABS(1)+2", "STRLENS(\"x\")", "UNKNOWN(1)2", "ABS(UNKNOWN)2", "(1)UNKNOWN", "(1)ABS(UNKNOWN)"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was not deferred: %v", source, err)
		}
	}
}

func TestProvenExpressionShapeRejectsKnownClosedOperators(t *testing.T) {
	for _, source := range []string{"ABS(1)++", "ABS(RESULT)++", "STRLENS(RESULTS)++", "ABS(1)!", "ABS(1)+", "(RESULT+1)++", "++ABS(1)", "++(RESULT+1)"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
	for _, source := range []string{"(RESULT)++", "++(RESULT)", "RESULT:0++", "(RESULT:0)++", "ABS(1)+2", "UNKNOWN(1)++", "ABS(UNKNOWN)++", "++UNKNOWN(1)", "++ABS(UNKNOWN)", "(F(1))++"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was not accepted or deferred: %v", source, err)
		}
	}
}

func TestProvenExpressionShapeRejectsDirectIncrementStateErrors(t *testing.T) {
	for _, source := range []string{"RESULT++1", "RESULT++--", "++RESULT++", "RESULT++!1", "RESULT!++1", "RESULT + * RESULT++", "GAMEBASE_VERSION++1", "++GAMEBASE_VERSION+1", "++1+RESULT", "1++RESULT"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
	for _, source := range []string{"RESULT++ + 1", "RESULT+++1", "++RESULT + 1", "RESULT++ ? 1 # 2", "RESULT++ + RESULT++", "RESULT ? RESULT++ # 1", "++RESULT + RESULT++", "UNKNOWN++1", "++UNKNOWN+1"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err != nil {
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
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was rejected: %v", source, err)
		}
	}
}

func TestProvenExpressionShapeRejectsMalformedClosedOperatorChains(t *testing.T) {
	for _, source := range []string{"ABS(1)+?2#3", "ABS(1)=1", "ABS(1)<>1", "ABS(1)+*1", "ABS(1)+!", "ABS(1)+~", "ABS(1)+-", "ABS(1)+--", "ABS(1)+:0", "((ABS(1)))++", "++(ABS(1))"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
	for _, source := range []string{"ABS(1)+-2", "ABS(1)?2#3", "(RESULT)++", "A++", "UNKNOWN(1)+?2#3", "UNKNOWN(1)+*1"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err != nil {
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
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
	for _, source := range []string{"ARG@FUNC", "ARGS@FUNC", "LOCAL@FUNC", "LOCALS@FUNC", "UNKNOWN@FUNC"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was not deferred: %v", source, err)
		}
	}
}

func TestProvenExpressionShapeRejectsKnownClosedPostfix(t *testing.T) {
	for _, source := range []string{"ABS(1)(2)", "ABS(1)[2]", "ABS(1):2", "ABS(1).2", `ABS(1)@"x {2+3}"`, "(1)(2)", "(1)[2]", "(1):2"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
	for _, source := range []string{"UNKNOWN(1)(2)", "UNKNOWN(1)[2]", "UNKNOWN(1):2", `UNKNOWN(1)@"x {2+3}"`, "ABS(UNKNOWN)(2)", "(F(1))[2]"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was not deferred: %v", source, err)
		}
	}
	for _, source := range []string{"CFLAG:(0):1", "NAME:(0):1"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was rejected: %v", source, err)
		}
	}
}

func TestProvenCaseShape(t *testing.T) {
	for _, source := range []string{"IS", "IS = 1", "IS ? 1", "IS # 1", "IS >", "IS > RESULT=1", "TO 1", "1 TO", "1 TO RESULT=1", "1 TO 2 +", "1 TO 2 TO 3", "1 IS = 2"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, true); err == nil {
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
		if err := ProvenExpressionShape(tokens, CommonDialect, true); err != nil {
			t.Fatalf("%q was rejected: %v", source, err)
		}
	}
}

func TestProvenCaseShapeRejectsInvalidIncrements(t *testing.T) {
	for _, source := range []string{"IS > ++1", "IS > 1++", "IS > ++NAME", "IS > NAME++", "IS > ++ITEMPRICE", "IS > ITEMPRICE++", "IS > ++NAME:0", "IS > NAME:0++", "IS > ++A++", "IS > ++(A+1)", "IS > (A+1)++"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, true); err == nil {
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
		if err := ProvenExpressionShape(tokens, CommonDialect, bytes.Contains([]byte(source), []byte("TO")) || bytes.HasPrefix([]byte(source), []byte("IS"))); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
	for _, source := range []string{"F(1,2)", "RESULT+F(1,2)"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err != nil {
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
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err == nil {
			t.Fatalf("%q was accepted", source)
		}
	}
	for _, source := range []string{"RESULT++", "CFLAG:0++"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was rejected: %v", source, err)
		}
	}
}

func TestKnownExpressionTypeForDialectPropagatesIndexedCompound(t *testing.T) {
	for _, test := range []struct {
		source string
		want   KnownValueType
	}{{"CFLAG:0:0+1", KnownValueNumeric}, {"NAME:0*2", KnownValueString}, {"NAME:0+\"x\"", KnownValueString}} {
		tokens, err := lexExpression([]byte(test.source), 0)
		if err != nil {
			t.Fatalf("%q: %v", test.source, err)
		}
		if got := KnownExpressionTypeForDialect(tokens, CommonDialect); got != test.want {
			t.Fatalf("%q type=%v, want %v", test.source, got, test.want)
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
			if err := ProvenExpressionShape(tokens, dialect, false); err != nil {
				t.Fatalf("dialect=%v valid source=%q: %v", dialect, source, err)
			}
		}
		for _, source := range []string{"RESULT:(1+2)[0]", "RESULT:(1+2).0", "CFLAG:(0):1[0]"} {
			tokens, err := lexExpression([]byte(source), 0)
			if err != nil {
				t.Fatalf("dialect=%v source=%q: %v", dialect, source, err)
			}
			if err := ProvenExpressionShape(tokens, dialect, false); err == nil {
				t.Fatalf("dialect=%v source=%q was accepted", dialect, source)
			}
		}
	}
}

func TestParenthesizedIndexShapeDefersDynamicFields(t *testing.T) {
	for _, source := range []string{"RESULT:(UNKNOWN)[0]", "RESULT:(F(1))[0]", "RESULT:(@\"x\")[0]", "RESULT[0]", "A[0]", "F(A,B[0])"} {
		tokens, err := lexExpression([]byte(source), 0)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if err := ProvenExpressionShape(tokens, CommonDialect, false); err != nil {
			t.Fatalf("%q was not deferred: %v", source, err)
		}
	}
}

func TestScalarCharacterBuiltinIndexShape(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		for _, source := range []string{"NAME:0", "NAME:(0)", "ISASSI:0", "NO:(0)"} {
			tokens, err := lexExpression([]byte(source), 0)
			if err != nil {
				t.Fatalf("dialect=%v source=%q: %v", dialect, source, err)
			}
			if err := ProvenExpressionShape(tokens, dialect, false); err != nil {
				t.Fatalf("dialect=%v valid source=%q: %v", dialect, source, err)
			}
		}
		for _, source := range []string{"NAME:0[0]", "NAME:(0)[0]", "NAME:(0).0", "ISASSI:0[0]", "NO:(0)[0]"} {
			tokens, err := lexExpression([]byte(source), 0)
			if err != nil {
				t.Fatalf("dialect=%v source=%q: %v", dialect, source, err)
			}
			if err := ProvenExpressionShape(tokens, dialect, false); err == nil {
				t.Fatalf("dialect=%v source=%q was accepted", dialect, source)
			}
		}
	}
}
