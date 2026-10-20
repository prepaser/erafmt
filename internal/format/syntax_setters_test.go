package format

import (
	"bytes"
	"testing"
)

func setterOptions() Options {
	return Options{Select: []RuleID{
		InstructionIndent, CommandSpacing, BinarySpacing, CommaSpacing,
		DelimiterSpacing, FormPadding,
	}}
}

func TestSetterExpressionFormatting(t *testing.T) {
	input := []byte("@F\nVARSET  RESULT,1+2,\nCVARSET CFLAG,2+3\nPOWER  RESULT,2+3,4+5\n")
	want := []byte("@F\n    VARSET RESULT, 1 + 2,\n    CVARSET CFLAG, 2 + 3\n    POWER RESULT, 2 + 3, 4 + 5\n")
	r := FormatWithOptions(input, ERB, setterOptions())
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%v want=%q", r.Output, r.Diagnostics, want)
	}
	again := FormatWithOptions(r.Output, ERB, setterOptions())
	if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 || len(again.Diagnostics) != 0 {
		t.Fatalf("not idempotent: %+v", again)
	}
}

func TestSetterLiteralAndEmptyArgumentGuards(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing, FormPadding}}
	for _, line := range []string{
		"VARSET 1,2", "VARSET (-1),2", "VARSET \"RESULT\",1", "VARSET @\"RESULT\",1",
		"VARSET ,1", "VARSET RESULT,,1", "CVARSET CFLAG,,2", "POWER RESULT,2,,3",
	} {
		input := []byte("@F\n" + line + "\n")
		r := FormatWithOptions(input, ERB, options)
		if !bytes.Equal(r.Output, input) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q output=%q diagnostics=%v", line, r.Output, r.Diagnostics)
		}
		quiet := FormatWithOptions(input, ERB, Options{Select: []RuleID{}})
		if !bytes.Equal(quiet.Output, input) || len(quiet.Diagnostics) != 0 || len(quiet.Changes) != 0 {
			t.Fatalf("select-none %q: %+v", line, quiet)
		}
	}
}

func TestSetterOptionalFieldsAndMacroShapedFields(t *testing.T) {
	input := []byte("@F\n#DEFINE EFAUDIT_VARSET RESULT,1\nVARSET EFAUDIT_VARSET\nVARSET RESULT,\nCVARSET CFLAG,1,\nPOWER RESULT,2,3\n")
	r := FormatWithOptions(input, ERB, setterOptions())
	want := []byte("@F\n#DEFINE EFAUDIT_VARSET RESULT,1\n    VARSET EFAUDIT_VARSET\n    VARSET RESULT,\n    CVARSET CFLAG, 1,\n    POWER RESULT, 2, 3\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%v want=%q", r.Output, r.Diagnostics, want)
	}
}

func TestSetterOptionalTrailingFieldsAreNotSurplus(t *testing.T) {
	for _, line := range []string{
		"VARSET RESULT,",
		"VARSET RESULT,1,2,",
		"CVARSET CFLAG,",
		"CVARSET CFLAG,0,1,2,",
		"POWER RESULT,2,3,",
	} {
		r := FormatWithOptions([]byte("@F\n    "+line+"\n"), ERB, setterOptions())
		if len(r.Diagnostics) != 0 {
			t.Fatalf("%q diagnostics=%+v output=%q", line, r.Diagnostics, r.Output)
		}
	}
}

func TestSetterSurplusPreservesBodyAndWarns(t *testing.T) {
	options := setterOptions()
	for _, line := range []string{
		`VARSET RESULT,1,2,3,RESULTS+"x"`,
		`CVARSET CFLAG,0,1,2,3,RESULTS+"x"`,
		`POWER RESULT,2,3,RESULTS+"x"`,
		`VARSET RESULT,1,2,3,,4`,
		`CVARSET CFLAG,0,1,2,3,,4`,
		`POWER RESULT,2,3,,4`,
		`VARSET RESULT,1,2,3,,`,
		`VARSET RESULT,1,2,3,1+`,
	} {
		src := []byte("@F\n    " + line + "\n")
		r := FormatWithOptions(src, ERB, options)
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Warning || r.Diagnostics[0].Code != "ignored_arguments" {
			t.Fatalf("%q output=%q diagnostics=%+v", line, r.Output, r.Diagnostics)
		}
	}
}

func TestSetterSurplusGuards(t *testing.T) {
	options := setterOptions()
	for _, line := range []string{
		`VARSET EFAUDIT_SETTER,1,2,3,4,5`,
		`VARSET RESULT,1,2,3,UNKNOWN_TAIL_MACRO`,
		`VARSET ;!; RESULT, 1, 2, 3, 4`,
		`VARSET [[ALIAS]],1,2,3,4`,
	} {
		src := []byte("@F\n    " + line + "\n")
		r := FormatWithOptions(src, ERB, options)
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 0 {
			t.Fatalf("%q changed uncertain body: output=%q diagnostics=%+v", line, r.Output, r.Diagnostics)
		}
	}
}

func TestSetterMacroPrefixWithOmittedRawFieldIsOpaque(t *testing.T) {
	input := []byte("@F\n#DEFINE EFAUDIT_POWER RESULT,2,3\nPOWER EFAUDIT_POWER,,4\n")
	r := FormatWithOptions(input, ERB, setterOptions())
	want := []byte("@F\n#DEFINE EFAUDIT_POWER RESULT,2,3\n    POWER EFAUDIT_POWER,,4\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%v want=%q", r.Output, r.Diagnostics, want)
	}
}

func TestSetterRuleOwnership(t *testing.T) {
	input := []byte("@F\nVARSET  RESULT,1+2\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{CommandSpacing}})
	want := []byte("@F\nVARSET RESULT,1+2\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("command spacing output=%q diagnostics=%v", r.Output, r.Diagnostics)
	}
	r = FormatWithOptions(input, ERB, Options{Select: []RuleID{BinarySpacing}})
	want = []byte("@F\nVARSET  RESULT,1 + 2\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
		t.Fatalf("binary spacing output=%q diagnostics=%v", r.Output, r.Diagnostics)
	}
}

func TestSetterProvenExpressionShapes(t *testing.T) {
	for _, line := range []string{
		"VARSET RESULT,RESULT 1",
		"CVARSET CFLAG,0,RESULT 1",
		"POWER RESULT,RESULT 1,2",
		"VARSET RESULT:(1,2),1",
		"VARSET RESULT,RESULT:(1,2)",
		"POWER RESULT,RESULT:(RESULT 1)+1,2",
	} {
		src := []byte("@F\n    " + line + "\n")
		r := FormatWithOptions(src, ERB, setterOptions())
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q output=%q diagnostics=%+v", line, r.Output, r.Diagnostics)
		}
		quiet := FormatWithOptions(src, ERB, Options{Select: []RuleID{}})
		if !bytes.Equal(quiet.Output, src) || len(quiet.Diagnostics) != 0 {
			t.Fatalf("select-none %q: %+v", line, quiet)
		}
	}
	for _, line := range []string{
		"VARSET RESULT,RESULT+1",
		"POWER RESULT,(RESULT:0)+1,2",
		"VARSET RESULT,UNKNOWN 1",
		"VARSET RESULT,IS",
		"CVARSET CFLAG,IS,1",
		"VARSET EFAUDIT_SETTER,RESULT 1",
		"VARSET RESULT,;!;RESULT 1",
	} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, setterOptions())
		if len(r.Diagnostics) != 0 {
			t.Fatalf("valid/deferred %q: %+v", line, r.Diagnostics)
		}
	}
}

func TestSetterOmissionsPreserveMacroPrefixes(t *testing.T) {
	for _, line := range []string{
		"POWER EFAUDIT_POWER,,4", "POWER RESULT,EFAUDIT_VALUES,,4",
		"POWER (result),2,,4", "VARSET CFLAG:INDEX,,1", "VARSET RESULT,F(1),,4",
	} {
		input := []byte("@F\n" + line + "\n")
		r := FormatWithOptions(input, ERB, Options{Select: []RuleID{BinarySpacing, CommaSpacing, DelimiterSpacing}})
		if !bytes.Equal(r.Output, input) || len(r.Diagnostics) != 0 || len(r.Changes) != 0 {
			t.Fatalf("%q output=%q diagnostics=%v", line, r.Output, r.Diagnostics)
		}
	}
}

func TestSetterDestinationShapeGuardsKnownExpressions(t *testing.T) {
	options := Options{Select: []RuleID{BinarySpacing, CommaSpacing, DelimiterSpacing}}
	for _, line := range []string{
		"VARSET RESULT+1,1", "VARSET RESULT*1,1", "VARSET -RESULT,1",
		"VARSET -RESULT+SOME_MACRO,1", "VARSET -(RESULT)+SOME_MACRO,1",
		"VARSET STRLENS(\"x\"),1", "VARSET (RESULT)+1,1",
	} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, options)
		if string(r.Output) != "@F\n"+line+"\n" || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q output=%q diagnostics=%v", line, r.Output, r.Diagnostics)
		}
	}
	for _, line := range []string{
		"VARSET +RESULT,1", "VARSET (RESULT),1", "VARSET ((RESULT)),1",
		"VARSET RESULT:1,1", "VARSET F(1),1", "VARSET UNKNOWN+1,1",
		"VARSET -SOME_MACRO+1,1",
	} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, options)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("%q diagnostics=%v", line, r.Diagnostics)
		}
	}
}

func TestSetterRejectsReadonlySystemConstants(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing}}
	for _, line := range []string{
		"VARSET ITEMPRICE,1",
		"VARSET ITEMPRICE:0,1",
		"CVARSET ABLNAME:0,1",
		"POWER GAMEBASE_VERSION,1,2",
		"VARSET ;!; ITEMPRICE,1",
	} {
		src := []byte("@F\n" + line + "\n")
		r := FormatWithOptions(src, ERB, options)
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%q output=%q diagnostics=%+v", line, r.Output, r.Diagnostics)
		}
	}
	if r := FormatWithOptions([]byte("@F\nVARSET ITEMPRICE:EFA_IDX,1\n"), ERB, options); len(r.Diagnostics) != 0 {
		t.Fatalf("dynamic readonly index should defer: %+v", r.Diagnostics)
	}
	for _, dialect := range []Dialect{CommonDialect, Emuera1824} {
		src := []byte("@F\nVARSET DAYNAME,1\n")
		r := FormatWithOptions(src, ERB, Options{Dialect: dialect, Select: options.Select})
		if len(r.Diagnostics) != 0 {
			t.Fatalf("DAYNAME should remain project-defined in dialect %d: %+v", dialect, r.Diagnostics)
		}
	}
	src := []byte("@F\nVARSET DAYNAME,1\n")
	r := FormatWithOptions(src, ERB, Options{Dialect: EMEE, Select: options.Select})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("modern DAYNAME output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}
