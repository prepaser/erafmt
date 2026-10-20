package cli

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestIndexedCallProducerExamples(t *testing.T) {
	tests := []struct {
		name, source string
		dialect      format.Dialect
		full         bool
	}{
		{"isnumeric-index", "@F\nIF LOCALS:2 != \"\" && ( ISNUMERIC(LOCALS:2) )\nENDIF\n", format.EMEE, false},
		{"strcount-index", "@F\nIF LOCAL:1 && ARG:1 == 1 && STRCOUNT(CSTR:PLAYER:12,CALLNAME:TARGET)\nENDIF\n", format.EMEE, false},
		{"groupmatch-index", "@F\nSIF !GROUPMATCH(LOCAL, 50, 75) && TRAINNAME:LOCAL != \"\"\nPRINTV 0\n", format.EMEE, false},
		{"match-index", "@F\nIF TALENTNAME:(LOCAL:1) != \"\" && +MATCH(LOCALS , TALENTNAME:(LOCAL:1))\nENDIF\n", format.EMEE, false},
		{"getnum-index", "@F\nCALL MAGIC_BOX , LOCAL , GETNUM(ITEM, LOCALS:1)\n", format.EMEE, false},
		{"form-index-multiplication", "@F\nPRINTFORML %ARGS:1*(138-STRLENS(ARGS))%\n", format.EMEE, true},
		{"form-call-index", "@F\nCALLFORM SKILL_NAME_{GETNUM(ITEM, ARGS:(LOCAL+1)) - 10000}\n", format.EMEE, true},
		{"form-call-percent", "@F\nTRYCALLLIST\nFUNC %\"LUNA_KOJO_CHARA\"+TOSTR(ARG:0)+\"_EVENT_\"+ARGS:0%\nENDFUNC\n", format.EMEE, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			options := format.Options{Dialect: tc.dialect, AllowFullwidthSpace: tc.full}
			result := format.FormatWithOptions([]byte(tc.source), format.ERB, options)
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
			}
			proof := sourcePreservationOptions{dialect: tc.dialect, allowFullwidthSpace: tc.full}
			if err := checkSourcePreservedOptions([]byte(tc.source), result.Output, proof); err != nil {
				t.Fatalf("output=%q: %v", result.Output, err)
			}
			second := format.FormatWithOptions(result.Output, format.ERB, options)
			if !bytes.Equal(second.Output, result.Output) || len(second.Diagnostics) != 0 {
				t.Fatalf("second pass output=%q diagnostics=%v", second.Output, second.Diagnostics)
			}
		})
	}
}

func TestIndexedCallOracleMutationControls(t *testing.T) {
	const condition = `LOCALS:2 != "" && ( ISNUMERIC(LOCALS:2) )`
	for _, tc := range []struct {
		name, source, changed string
		wantErr               bool
	}{
		{"condition-trivia", "@F\nIF " + condition + "\nENDIF\n", "@F\nIF LOCALS:2 != \"\" && (ISNUMERIC( LOCALS:2 ))\nENDIF\n", false},
		{"condition-index", "@F\nIF " + condition + "\nENDIF\n", "@F\nIF LOCALS:3 != \"\" && (ISNUMERIC(LOCALS:2))\nENDIF\n", true},
		{"condition-call", "@F\nIF " + condition + "\nENDIF\n", "@F\nIF LOCALS:2 != \"\" && (ISNUMERIC2(LOCALS:2))\nENDIF\n", true},
		{"condition-literal", "@F\nIF " + condition + "\nENDIF\n", "@F\nIF LOCALS:2 != \"x\" && (ISNUMERIC(LOCALS:2))\nENDIF\n", true},
		{"condition-operator", "@F\nIF " + condition + "\nENDIF\n", "@F\nIF LOCALS:2 == \"\" && (ISNUMERIC(LOCALS:2))\nENDIF\n", true},
		{"condition-no-indexed-call", "@F\nIF LOCALS:2 != \"\" && ( ISNUMERIC(1) )\nENDIF\n", "@F\nIF LOCALS:2 != \"\" && (ISNUMERIC( 1 ))\nENDIF\n", true},
		{"condition-scoped", "@F\nIF LOCALS:2 != \"\" && ( ISNUMERIC(LOCAL@F:2) )\nENDIF\n", "@F\nIF LOCALS:2 != \"\" && (ISNUMERIC(LOCAL @ F:2))\nENDIF\n", true},
		{"condition-raw", "@F\nIF LOCALS:2 != \"\" && ( ISNUMERIC('raw') )\nENDIF\n", "@F\nIF LOCALS:2 != \"\" && (ISNUMERIC('raw '))\nENDIF\n", true},
		{"condition-marker", "@F\nIF LOCALS:2 != \"\" && ( ISNUMERIC(LOCALS:2) ) ;!;keep\nENDIF\n", "@F\nIF LOCALS:2 != \"\" && (ISNUMERIC(LOCALS:2));!;keep\nENDIF\n", true},
		{"condition-nul", "@F\nIF LOCALS:2 != \"\" && ( ISNUMERIC(LOCALS:2\x00) )\nENDIF\n", "@F\nIF LOCALS:2 != \"\" && (ISNUMERIC(LOCALS:2\x00 ))\nENDIF\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkSourcePreservedOptions([]byte(tc.source), []byte(tc.changed), sourcePreservationOptions{dialect: format.EMEE})
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v got=%v: %v", tc.wantErr, err != nil, err)
			}
		})
	}
}

func TestIndexedStringMultiplicationOracleBoundaries(t *testing.T) {
	positive := []string{
		`ARGS:1*(138-STRLENS(ARGS))`,
		`ARGS:1*(FOO(ARGS))`,
	}
	for _, source := range positive {
		if !corpusIndexedStringMultiplicationExpression([]byte(source), format.EMEE) {
			t.Fatalf("accepted shape was rejected: %q", source)
		}
	}
	for _, source := range []string{
		`A*(1-STRLENS(A))`,
		`ARGS:1*(OTHER:2)`,
		`ARGS:1*("x")`,
		`ARGS:1*('raw')`,
		"ARGS:1*(1\x00+2)",
		`ARGS:1*(1,2)`,
		`ARGS:1*(1,)`,
		`ARGS:1*(1=2)`,
		`ARGS:1*(1+)`,
		`ARGS:1*(FOO(BAR(ARGS)))`,
		`ARGS:(LOCAL+1)*(1+2)`,
		`ARGS:1*(1e-2g00)`,
		`ARGS:9223372036854775808*(138-STRLENS(ARGS))`,
		`ARGS:1*(LOCAL@F)`,
		`ARGS:1*(\@"A"?1#2\@)`,
		`ARGS:1*(1+2);!;payload`,
	} {
		if corpusIndexedStringMultiplicationExpression([]byte(source), format.EMEE) {
			t.Fatalf("accepted unsafe shape: %q", source)
		}
	}
	source := []byte("@F\nPRINTFORML %ARGS:9223372036854775808*(138-STRLENS(ARGS))%\n")
	changed := bytes.Replace(source, []byte("*(138-STRLENS(ARGS))"), []byte(" * (138 - STRLENS(ARGS))"), 1)
	if err := checkSourcePreservedOptions(source, changed, sourcePreservationOptions{dialect: format.EMEE}); err == nil {
		t.Fatal("accepted trivia changes in an invalid string index")
	}
}
