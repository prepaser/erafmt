package format

import (
	"bytes"
	"testing"
)

func TestDeclarationAndHeaderSpacing(t *testing.T) {
	input := []byte("@F( ARG:0=1+2,ARG:1=3 )\n#DIM  CONST USER_X,1+2=1,2,3\n#DIMS strings\n")
	options := Options{Select: []RuleID{
		DeclarationWordSpacing,
		DeclarationDefaultSpacing,
		CommandSpacing,
		BinarySpacing,
		CommaSpacing,
		DelimiterSpacing,
	}}
	result := FormatWithOptions(input, ERB, options)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	want := []byte("@F(ARG:0 = 1 + 2, ARG:1 = 3)\n#DIM CONST USER_X, 1 + 2 = 1, 2, 3\n#DIMS strings\n")
	if !bytes.Equal(result.Output, want) {
		t.Fatalf("output = %q, want %q", result.Output, want)
	}
}

func TestReferenceDimensionsMayBeOmitted(t *testing.T) {
	input := []byte("@F(ARG:0,)\n#DIM REF REF_X, ,\n#DIMS DYNAMIC REF REF_S,0,\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{CommaSpacing, DeclarationWordSpacing}})
	if string(result.Output) != "@F(ARG:0,)\n#DIM REF REF_X,,\n#DIMS DYNAMIC REF REF_S, 0,\n" || len(result.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}

	invalid := []byte("@F\n#DIM DIM_X, ,\n#DIM REF REF_Y,1,\n#DIM REF STATIC REF_Z,,\n#DIM REF REF REF_R,,\n#DIM REF REF_W,,,,\n#DIM REF REF_Q,, = 1\n")
	result = FormatWithOptions(invalid, ERB, Options{Select: []RuleID{CommaSpacing, DeclarationWordSpacing}})
	if string(result.Output) != string(invalid) || len(result.Diagnostics) != 4 || result.Diagnostics[0].Line != 3 {
		t.Fatalf("invalid output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}

	erh := []byte("#DIM REF REF_X, ,\n")
	result = FormatWithOptions(erh, ERH, Options{Select: []RuleID{CommaSpacing}})
	if string(result.Output) != string(erh) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_declaration" {
		t.Fatalf("ERH output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
	erh = []byte("#DIM REF REF_X,0\n#DIM REF REF_Y,1\n")
	result = FormatWithOptions(erh, ERH, Options{})
	if string(result.Output) != string(erh) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_declaration" {
		t.Fatalf("ERH global REF output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
}

func TestDeclarationDimensionsAcceptNativeZeroSpellings(t *testing.T) {
	input := []byte("@F\n#DIM REF REF_X,0x0,\n#DIM REF REF_Y,0b0,,\n#DIM REF REF_Z,+0,\n#DIM REF REF_Q,-0,\n#DIMS DYNAMIC REF REF_S,(0),\n#DIM REF REF_T,0p1,\nRETURN 0\n")
	result := FormatWithOptions(input, ERB, Options{})
	want := []byte("@F\n    #DIM REF REF_X, 0x0,\n    #DIM REF REF_Y, 0b0,,\n    #DIM REF REF_Z, +0,\n    #DIM REF REF_Q, -0,\n    #DIMS DYNAMIC REF REF_S, (0),\n    #DIM REF REF_T, 0p1,\n\n    RETURN 0\n")
	if !bytes.Equal(result.Output, want) || len(result.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
}

func TestDeclarationDimensionsRejectDirectInvalidValues(t *testing.T) {
	input := []byte("@F\n#DIM DIM_X,\"x\"\n#DIM DIM_Y,0\n#DIM DIM_Z,-1\n#DIM DIM_W,1000001\n#DIM REF REF_R,1\nRETURN 1+2\n")
	result := FormatWithOptions(input, ERB, Options{})
	if len(result.Diagnostics) != 5 || !bytes.Contains(result.Output, []byte("#DIM DIM_X,\"x\"")) || !bytes.Contains(result.Output, []byte("#DIM REF REF_R,1")) || !bytes.Contains(result.Output, []byte("RETURN 1 + 2")) {
		t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Line == 6 {
			if diagnostic.Code != "unsupported_syntax" || diagnostic.Severity != Unsupported {
				t.Fatalf("reference diagnostic=%+v", diagnostic)
			}
		} else if diagnostic.Code != "suspicious_dimension" || diagnostic.Severity != Warning {
			t.Fatalf("diagnostics=%+v", result.Diagnostics)
		}
	}

	for _, value := range []string{"UNKNOWN", "1+2", "UNKNOWN+1", "EFAUDIT_DIM,0"} {
		r := FormatWithOptions([]byte("@F\n#DIM DIM_X,"+value+"\n"), ERB, Options{})
		if len(r.Diagnostics) != 0 {
			t.Fatalf("value=%q diagnostics=%+v", value, r.Diagnostics)
		}
	}
}

func TestOrdinaryDeclarationRejectsProvenOmittedDimensions(t *testing.T) {
	input := []byte("@F\n#DIM VALUE,2,\n#DIM CONST CONSTANT,, = 1\n#DIM VALUE,1+1,\nRETURN 0\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{BinarySpacing}})
	if !bytes.Equal(result.Output, input) || len(result.Diagnostics) != 3 {
		t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code != "unsupported_syntax" || diagnostic.Severity != Unsupported || diagnostic.Message != "ordinary declaration dimensions cannot be omitted" {
			t.Fatalf("diagnostics=%+v", result.Diagnostics)
		}
	}
}

func TestOmittedDimensionsDeferMacroCapableRoles(t *testing.T) {
	macro := FormatWithOptions([]byte("#DEFINE HEAD REF VALUE\n#DIM HEAD,,\n"), ERH, Options{})
	if len(macro.Diagnostics) != 0 {
		t.Fatalf("macro-capable declaration diagnostics=%+v output=%q", macro.Diagnostics, macro.Output)
	}
	for _, source := range []string{"@F\n#DIM HEAD,UNKNOWN,2,\n", "@F\n#DIM HEAD,2,UNKNOWN,\n", "@F\n#DIM CONST HEAD,UNKNOWN,, = 1\n"} {
		result := FormatWithOptions([]byte(source), ERB, Options{Select: []RuleID{BinarySpacing}})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("uncertain declaration source=%q diagnostics=%+v", source, result.Diagnostics)
		}
	}
	input := []byte("@F\n#DIM DYNAMIC HEAD,,\n#DIM HEAD,UNKNOWN,\n#DIM HEAD,0,\nRETURN 0\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{BinarySpacing}})
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "suspicious_dimension" || result.Diagnostics[0].Line != 4 {
		t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
}

func TestDeclarationDimensionsWarnOnShape(t *testing.T) {
	input := []byte("@F\n#DIM OK,1000,1000\n#DIM OVER,2000,501\n#DIMS FOUR,1,1,1,1\n#DIM COMPLEX,1+2,3,4,5\n#DIM MACRO,UNKNOWN,2000,501\nRETURN 0\n")
	result := FormatWithOptions(input, ERB, Options{})
	if !bytes.Contains(result.Output, []byte("#DIM OK, 1000, 1000")) || !bytes.Contains(result.Output, []byte("#DIM OVER,2000,501")) || !bytes.Contains(result.Output, []byte("#DIMS FOUR,1,1,1,1")) {
		t.Fatalf("output=%q", result.Output)
	}
	if len(result.Diagnostics) != 3 {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code != "suspicious_dimension" || diagnostic.Severity != Warning {
			t.Fatalf("diagnostics=%+v", result.Diagnostics)
		}
	}
	if !bytes.Contains(result.Output, []byte("#DIM MACRO, UNKNOWN, 2000, 501")) {
		t.Fatalf("unresolved dimension was unexpectedly preserved: %q", result.Output)
	}
}

func TestDeclarationInitializerAllowsOneTerminalComma(t *testing.T) {
	input := []byte("@F\n#DIM VALUE,2=1,2,\n#DIMS LABELS=\"a\",\"b\",\nRETURN 1+2\n")
	result := FormatWithOptions(input, ERB, Options{})
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "suspicious_initializer" {
			t.Fatalf("terminal initializer comma was rejected: %+v output=%q", result.Diagnostics, result.Output)
		}
	}
	if !bytes.Contains(result.Output, []byte("#DIM VALUE, 2 = 1, 2,")) || !bytes.Contains(result.Output, []byte("#DIMS LABELS = \"a\", \"b\",")) {
		t.Fatalf("terminal initializer comma was not preserved: %q", result.Output)
	}

	for _, source := range []string{
		"@F\n#DIM VALUE,2=1,,\n",
		"@F\n#DIM VALUE,2=,1\n",
	} {
		result = FormatWithOptions([]byte(source), ERB, Options{})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "suspicious_initializer" {
			t.Fatalf("invalid initializer omission was accepted: source=%q diagnostics=%+v output=%q", source, result.Diagnostics, result.Output)
		}
	}
}

func TestObjectDefineBoundarySpacingPreservesBody(t *testing.T) {
	input := []byte("#DEFINE  TEXT   \"a  b\" ; comment  \n#DEFINE EMPTY   ; comment\n")
	result := FormatWithOptions(input, ERH, Options{Select: []RuleID{DeclarationWordSpacing}})
	want := []byte("#DEFINE TEXT \"a  b\" ; comment  \n#DEFINE EMPTY   ; comment\n")
	if !bytes.Equal(result.Output, want) || len(result.Diagnostics) != 0 {
		t.Fatalf("object define spacing changed its body: output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
	again := FormatWithOptions(result.Output, ERH, Options{Select: []RuleID{DeclarationWordSpacing}})
	if !bytes.Equal(again.Output, result.Output) {
		t.Fatalf("object define spacing is not idempotent: first=%q second=%q", result.Output, again.Output)
	}

	for _, source := range []string{
		"#DEFINE FOO(x) 1\n",
		"#DEFINE [[FOO]] 1\n",
		"#DEFINE FOO 1 ;!; RETURN 2\n",
		"#define  FOO  1\n",
	} {
		result = FormatWithOptions([]byte(source), ERH, Options{Select: []RuleID{DeclarationWordSpacing}})
		if !bytes.Equal(result.Output, []byte(source)) {
			t.Fatalf("uncertain define was changed: source=%q output=%q diagnostics=%+v", source, result.Output, result.Diagnostics)
		}
	}
	result = FormatWithOptions([]byte("#DEFINE  FOO\x00 1\n"), ERH, Options{Select: []RuleID{DeclarationWordSpacing}})
	if !bytes.Equal(result.Output, []byte("#DEFINE  FOO\x00 1\n")) {
		t.Fatalf("NUL define was changed: output=%q", result.Output)
	}
}

func TestERHCharadataRejectsProvenThirdDimension(t *testing.T) {
	input := []byte("#DIM CHARADATA CHARA2,2,2\n#DIM CHARADATA CHARA3,2,2,2\n#DIM CHARADATA UNKNOWN,2,UNKNOWN,2\n")
	result := FormatWithOptions(input, ERH, Options{})
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "suspicious_dimension" || result.Diagnostics[0].Severity != Warning || result.Diagnostics[0].Line != 2 {
		t.Fatalf("CHARADATA shape diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
	if !bytes.Contains(result.Output, []byte("#DIM CHARADATA CHARA2, 2, 2")) || !bytes.Contains(result.Output, []byte("#DIM CHARADATA CHARA3,2,2,2")) {
		t.Fatalf("CHARADATA shape formatting=%q", result.Output)
	}

	local := FormatWithOptions([]byte("@F\n#DIM CHARADATA CHARA3,2,2,2\n"), ERB, Options{})
	if len(local.Diagnostics) != 1 || local.Diagnostics[0].Code != "unsupported_syntax" {
		t.Fatalf("local CHARADATA diagnostic=%+v output=%q", local.Diagnostics, local.Output)
	}
}

func TestUnsupportedDeclarationKeepsEarlierFunctionEditsRolledBack(t *testing.T) {
	input := []byte("@F\n#DIM USER_VAR,1+2\n#DIM RESULT\nRETURN 1\n@G\nRETURN 1+2\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{
		FunctionIndent,
		DeclarationIndent,
		InstructionIndent,
		BinarySpacing,
	}})
	wantPrefix := []byte("@F\n#DIM USER_VAR,1+2\n#DIM RESULT\nRETURN 1\n")
	if !bytes.HasPrefix(result.Output, wantPrefix) {
		t.Fatalf("preserved function was changed: output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
	if !bytes.Contains(result.Output, []byte("@G\n    RETURN 1 + 2\n")) {
		t.Fatalf("later function was not formatted: output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_syntax" {
		t.Fatalf("diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
}

func TestLocalSizeWarnsOnIgnoredDirectLiteral(t *testing.T) {
	input := []byte("@F\n#LOCALSIZE 0\n#LOCALSSIZE -1\n#LOCALSIZE 2147483646\n#LOCALSSIZE 2147483647 ; note\n#LOCALSIZE UNKNOWN\nRETURN 0\n")
	result := FormatWithOptions(input, ERB, Options{})
	if len(result.Diagnostics) != 3 {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity != Warning || diagnostic.Code != "suspicious_local_size" {
			t.Fatalf("diagnostics=%+v", result.Diagnostics)
		}
	}
	if !bytes.Contains(result.Output, []byte("#LOCALSIZE 0")) || !bytes.Contains(result.Output, []byte("#LOCALSSIZE -1")) || !bytes.Contains(result.Output, []byte("#LOCALSSIZE 2147483647")) {
		t.Fatalf("output=%q", result.Output)
	}
}

func TestLocalSizeSkipsEventFunctions(t *testing.T) {
	input := []byte("@EVENTTRAIN\n#LOCALSIZE \"invalid\"\n#LOCALSSIZE 0\nRETURN 0\n@F\n#LOCALSIZE 0\nRETURN 0\n")
	result := FormatWithOptions(input, ERB, Options{})
	if len(result.Diagnostics) != 3 || result.Diagnostics[0].Code != "event_local_size" || result.Diagnostics[1].Code != "event_local_size" || result.Diagnostics[2].Code != "suspicious_local_size" || result.Diagnostics[2].Line != 6 {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
}

func TestDeclarationDimensionValidationFollowsExpressionSelection(t *testing.T) {
	input := []byte("@F\n#DIM DIM_X,0\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{}})
	if !bytes.Equal(result.Output, input) || len(result.Diagnostics) != 0 || len(result.Changes) != 0 {
		t.Fatalf("result=%+v", result)
	}
}

func TestDeclarationModifierBoundaries(t *testing.T) {
	input := []byte("@F\n#DIM CONST1,2=1\n#DIM REF0,2=1\n#DIM STATIC2,2=1\n#DIM PUBLIC,2=1\n#DIM CONST USER_X,1=1\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{
		DeclarationWordSpacing,
		DeclarationDefaultSpacing,
		CommaSpacing,
		BinarySpacing,
	}})
	want := "@F\n#DIM CONST1, 2 = 1\n#DIM REF0, 2 = 1\n#DIM STATIC2, 2 = 1\n#DIM PUBLIC, 2 = 1\n#DIM CONST USER_X, 1 = 1\n"
	if string(result.Output) != want || len(result.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
}

func TestHeaderDelimiterSpacing(t *testing.T) {
	input := []byte("@F( )\n@F ( X )\n@G ( ARGS:0 )\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{DelimiterSpacing}})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	want := []byte("@F()\n@F(X)\n@G(ARGS:0)\n")
	if !bytes.Equal(result.Output, want) {
		t.Fatalf("output = %q, want %q", result.Output, want)
	}
}

func TestHeaderCommaAndDelimiterOwnership(t *testing.T) {
	input := []byte("@F , ARG\n@G ( ARG )\n")
	for _, tc := range []struct {
		rules []RuleID
		want  string
	}{
		{[]RuleID{DelimiterSpacing}, "@F , ARG\n@G(ARG)\n"},
		{[]RuleID{CommaSpacing}, "@F, ARG\n@G ( ARG )\n"},
		{[]RuleID{DelimiterSpacing, CommaSpacing}, "@F, ARG\n@G(ARG)\n"},
	} {
		r := FormatWithOptions(input, ERB, Options{Select: tc.rules})
		if string(r.Output) != tc.want || len(r.Diagnostics) != 0 {
			t.Fatalf("rules %v: output %q, diagnostics %+v", tc.rules, r.Output, r.Diagnostics)
		}
		for _, c := range r.Changes {
			if c.Rule == DelimiterSpacing && c.Line == 1 {
				t.Fatalf("delimiter rule claimed header comma: %+v", c)
			}
		}
		again := FormatWithOptions(r.Output, ERB, Options{Select: tc.rules})
		if !bytes.Equal(again.Output, r.Output) {
			t.Fatalf("rules %v are not idempotent", tc.rules)
		}
	}
}

func TestDeclarationOpaqueRegions(t *testing.T) {
	input := []byte("#DEFINE X  A+B\n#DIM [[ALIAS]] = A+B\n")
	options := Options{Select: []RuleID{
		DeclarationWordSpacing,
		DeclarationDefaultSpacing,
		BinarySpacing,
	}}
	result := FormatWithOptions(input, ERB, options)
	if !bytes.Equal(result.Output, input) {
		t.Fatalf("opaque declaration changed: %q", result.Output)
	}
}

func TestLocalSizeUsesExpressionRules(t *testing.T) {
	input := []byte("@F\n#LOCALSIZE 1+2*3\n#LOCALSSIZE 4+5\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{BinarySpacing}})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	want := []byte("@F\n#LOCALSIZE 1 + 2 * 3\n#LOCALSSIZE 4 + 5\n")
	if !bytes.Equal(result.Output, want) {
		t.Fatalf("output = %q, want %q", result.Output, want)
	}
}

func TestUncertainDeclarationsRollbackTheirEdits(t *testing.T) {
	input := []byte("@F(A=)\n#DIM CONST USER_X,\n")
	options := Options{Select: []RuleID{
		DeclarationWordSpacing,
		DeclarationDefaultSpacing,
		BinarySpacing,
		CommaSpacing,
	}}
	result := FormatWithOptions(input, ERB, options)
	if !bytes.Equal(result.Output, input) {
		t.Fatalf("uncertain declaration changed: %q", result.Output)
	}
}

func TestEmptyHeaderFieldsStayOpaque(t *testing.T) {
	input := []byte("@F(X, )\n@F(ARG= )\n#DIM CONST USER_X, \n#DIM CONST USER_Y = \n")
	options := Options{Select: []RuleID{
		DeclarationWordSpacing,
		DeclarationDefaultSpacing,
		CommaSpacing,
		DelimiterSpacing,
	}}
	result := FormatWithOptions(input, ERB, options)
	if !bytes.Equal(result.Output, input) {
		t.Fatalf("empty field changed: %q", result.Output)
	}
}

func TestDeclarationCommentsRemainOpaque(t *testing.T) {
	input := []byte("@F ,  ARG:0=1 ; note  , x=2  \n#DIM USER_X,2=1 ; note  , x=2  \n#LOCALSIZE 1+2 ; note  , x=2  \n")
	options := Options{Select: []RuleID{CommaSpacing, BinarySpacing, DeclarationDefaultSpacing}}
	r := FormatWithOptions(input, ERB, options)
	want := "@F, ARG:0 = 1 ; note  , x=2  \n#DIM USER_X, 2 = 1 ; note  , x=2  \n#LOCALSIZE 1 + 2 ; note  , x=2  \n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output %q, diagnostics %+v", r.Output, r.Diagnostics)
	}
}
