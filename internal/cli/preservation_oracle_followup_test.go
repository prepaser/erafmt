package cli

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestPreservationOraclePercentIndexedStringCall(t *testing.T) {
	source := []byte("@F\nPRINTFORM %STRCOUNT(CSTR:PLAYER:12,CALLNAME:TARGET)%\n")
	for index, dialect := range []format.Dialect{format.CommonDialect, format.EMEE} {
		t.Run([]string{"common", "emee"}[index], func(t *testing.T) {
			options := format.Options{Dialect: dialect}
			result := format.FormatWithOptions(source, format.ERB, options)
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
			}
			proof := sourcePreservationOptions{dialect: dialect}
			if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
				t.Fatalf("output=%q: %v", result.Output, err)
			}
			changed := bytes.Replace(result.Output, []byte("TARGET"), []byte("OTHER"), 1)
			if bytes.Equal(changed, result.Output) {
				t.Fatal("mutation did not apply")
			}
			if err := checkSourcePreservedOptions(source, changed, proof); err == nil {
				t.Fatalf("accepted indexed call mutation: %q", changed)
			}
		})
	}
}

func TestPreservationOracleTimesMarkerAndOverflowCoefficient(t *testing.T) {
	proof := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}
	marker := []byte("@F\nTIMES　RESULT:0,1　;!;　+2\n")
	result := format.FormatWithOptions(marker, format.ERB, format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("marker diagnostics=%v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreservedOptions(marker, result.Output, proof); err != nil {
		t.Fatalf("marker output=%q: %v", result.Output, err)
	}
	markerMutation := bytes.Replace(result.Output, []byte("　;!;"), []byte(" ;!;"), 1)
	if bytes.Equal(markerMutation, result.Output) {
		t.Fatal("marker gap mutation did not apply")
	}
	if err := checkSourcePreservedOptions(marker, markerMutation, proof); err == nil {
		t.Fatalf("accepted marker gap mutation: %q", markerMutation)
	}

	overflow := []byte("@F\nTIMES RESULT:0,1e999\n")
	result = format.FormatWithOptions(overflow, format.ERB, format.Options{Dialect: format.EMEE})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("overflow diagnostics=%v output=%q", result.Diagnostics, result.Output)
	}
	if err := checkSourcePreservedOptions(overflow, result.Output, sourcePreservationOptions{dialect: format.EMEE}); err != nil {
		t.Fatalf("overflow output=%q: %v", result.Output, err)
	}
	changed := bytes.Replace(result.Output, []byte("1e999"), []byte("1e998"), 1)
	if bytes.Equal(changed, result.Output) {
		t.Fatal("coefficient mutation did not apply")
	}
	if err := checkSourcePreservedOptions(overflow, changed, sourcePreservationOptions{dialect: format.EMEE}); err == nil {
		t.Fatalf("accepted coefficient mutation: %q", changed)
	}
}

func TestPreservationOracleTimesPartialAndSpecialCoefficients(t *testing.T) {
	for _, tc := range []struct {
		name, source, changed string
	}{
		{"partial", "@F\nTIMES RESULT,1e+2\n", "@F\nTIMES RESULT,1e +2\n"},
		{"nan", "@F\nTIMES RESULT,NaN\n", "@F\nTIMES RESULT, NaN\n"},
		{"infinity", "@F\nTIMES RESULT,Infinity\n", "@F\nTIMES RESULT, Infinity\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkSourcePreservedOptions([]byte(tc.source), []byte(tc.changed), sourcePreservationOptions{dialect: format.EMEE}); err == nil {
				t.Fatalf("accepted protected coefficient mutation: %q", tc.changed)
			}
		})
	}
}

func TestPreservationOracleFormNumericIdentifierBoundary(t *testing.T) {
	cases := []struct {
		name, source string
		options      format.Options
		proof        sourcePreservationOptions
	}{
		{
			name:    "callform",
			source:  "@F\nCALLFORM FUNC_{1?2#3,0,LEFT},0A0\n",
			options: format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true},
			proof:   sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true},
		},
		{
			name:    "percent-form",
			source:  "@F\nPRINTFORM %USR+1,0A0%\n",
			options: format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true},
			proof:   sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true},
		},
		{
			name:    "embedded-nul-identifier",
			source:  "@F\nPRINTFORM %A\x00B+1,0A0%\n",
			options: format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true},
			proof:   sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte(tc.source)
			result := format.FormatWithOptions(source, format.ERB, tc.options)
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
			}
			if err := checkSourcePreservedOptions(source, result.Output, tc.proof); err != nil {
				t.Fatalf("output=%q: %v", result.Output, err)
			}
			changed := bytes.Replace(result.Output, []byte("0 A0"), []byte("0 A1"), 1)
			if bytes.Equal(changed, result.Output) {
				t.Fatal("numeric identifier mutation did not apply")
			}
			if err := checkSourcePreservedOptions(source, changed, tc.proof); err == nil {
				t.Fatalf("accepted numeric identifier mutation: %q", changed)
			}
		})
	}
}

func TestPreservationOracleFormNumericIdentifierControls(t *testing.T) {
	for _, tc := range []struct {
		name, source, changed string
	}{
		{"numeric-spelling", "@F\nPRINTFORM %USR+1,0A0%\n", "@F\nPRINTFORM %USR+1,00A0%\n"},
		{"indexed-string", "@F\nPRINTFORM %CSTR:0A0,1%\n", "@F\nPRINTFORM %CSTR:0A1,1%\n"},
		{"marker", "@F\nPRINTFORM %USR+1,0A0;!;%\n", "@F\nPRINTFORM %USR+1,0 A0;!;%\n"},
		{"nested-call", "@F\nCALLFORM F(FOO(BAR(0A0)))\n", "@F\nCALLFORM F(FOO(BAR(0 A0)))\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkSourcePreservedOptions([]byte(tc.source), []byte(tc.changed), sourcePreservationOptions{dialect: format.EMEE}); err == nil {
				t.Fatalf("accepted protected FORM mutation: %q", tc.changed)
			}
		})
	}
}
