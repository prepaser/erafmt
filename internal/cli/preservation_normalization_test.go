package cli

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestPreservationOracleAllowsProvenBoundaryNormalizations(t *testing.T) {
	join := ""
	joinSpace := " "
	cases := []struct {
		name       string
		source     string
		options    format.Options
		preserve   sourcePreservationOptions
		mutateFrom string
		mutateTo   string
	}{
		{
			name:       "empty-sharp-declaration",
			source:     "@A\n# ",
			mutateFrom: "#",
			mutateTo:   "#X",
		},
		{
			name:       "mandatory-marker-comma",
			source:     "@F\nRETURN 0;!;0,0\n",
			mutateFrom: "0, 0",
			mutateTo:   "1, 0",
		},
		{
			name:       "mandatory-marker-hash",
			source:     "@F\nRETURN 0;!;#0\n",
			mutateFrom: "# 0",
			mutateTo:   "# 1",
		},
		{
			name:       "mandatory-marker-numeric-boundary",
			source:     "@F\nRETURN ;!;0A\n",
			mutateFrom: "0 A",
			mutateTo:   "0 B",
		},
		{
			name:       "printv-nul-boundary",
			source:     "@F\nPRINTV 0%0 \x00\n",
			mutateFrom: "0 % 0",
			mutateTo:   "0 % 1",
		},
		{
			name:       "if-nul-boundary",
			source:     "@F\nIF 0%0\x00!\nENDIF\n",
			mutateFrom: "0 % 0",
			mutateTo:   "0 % 1",
		},
		{
			name:       "if-nul-comma-boundary",
			source:     "@F\nIF 0%0\x00,!\nENDIF\n",
			mutateFrom: "0 % 0",
			mutateTo:   "0 % 1",
		},
		{
			name:       "return-nul-boundary",
			source:     "@F\nRETURN 0A\x00,\x000\n",
			mutateFrom: "0 A",
			mutateTo:   "0 B",
		},
		{
			name:       "deferred-postfix-gap",
			source:     "@F\nRETURN 0!--a000-- 0\n",
			mutateFrom: "a000",
			mutateTo:   "b000",
		},
		{
			name:       "printv-empty-field-command-gap",
			source:     "@F\nPRINTV  ,000!\n",
			mutateFrom: "000",
			mutateTo:   "001",
		},
		{
			name:       "empty-continuation-return-gap",
			source:     "@F\n{\nRETURN  \n\n}\n",
			options:    format.Options{ContinuationSeparator: &join},
			preserve:   sourcePreservationOptions{continuationSeparator: &join},
			mutateFrom: "RETURN\n",
			mutateTo:   "RETRUN\n",
		},
		{
			name:       "continued-numeric-boundary",
			source:     "@F\n{\nRETURN 0\n0A000\n}\n",
			options:    format.Options{ContinuationSeparator: &join},
			preserve:   sourcePreservationOptions{continuationSeparator: &join},
			mutateFrom: "0 A000",
			mutateTo:   "0 B000",
		},
		{
			name:       "continued-printv-numeric-boundary",
			source:     "@F\n{\nPRINTV 0\n0A000\n}\n",
			options:    format.Options{ContinuationSeparator: &join},
			preserve:   sourcePreservationOptions{continuationSeparator: &join},
			mutateFrom: "A000",
			mutateTo:   "B000",
		},
		{
			name:       "continued-printv-numeric-boundary-with-space",
			source:     "@F\n{\nPRINTV 0\n0A000\n}\n",
			options:    format.Options{ContinuationSeparator: &joinSpace},
			preserve:   sourcePreservationOptions{continuationSeparator: &joinSpace},
			mutateFrom: "A000",
			mutateTo:   "B000",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := format.FormatWithOptions([]byte(tc.source), format.ERB, tc.options)
			if err := checkSourcePreservedOptions([]byte(tc.source), result.Output, tc.preserve); err != nil {
				t.Fatalf("output=%q diagnostics=%+v: %v", result.Output, result.Diagnostics, err)
			}
			changed := bytes.Replace(result.Output, []byte(tc.mutateFrom), []byte(tc.mutateTo), 1)
			if bytes.Equal(changed, result.Output) {
				t.Fatalf("mutation did not apply to output=%q", result.Output)
			}
			if err := checkSourcePreservedOptions([]byte(tc.source), changed, tc.preserve); err == nil {
				t.Fatalf("accepted semantic mutation output=%q", changed)
			}
		})
	}
}

func TestPreservationOracleProtectsOpaqueMalformedPayloads(t *testing.T) {
	controls := []struct {
		name, source, changed string
		options               sourcePreservationOptions
	}{
		{
			name:    "protected-triple-semicolon-label",
			source:  "@F\n$A00000;;;0\n",
			changed: "@F\n$A00000 ; ;;1\n",
		},
		{
			name:    "unknown-sharp-protected-comment",
			source:  "@F\n#00000000;;00\n",
			changed: "@F\n#00000000 ; ;01\n",
		},
		{
			name:    "unknown-return-hash",
			source:  "@F\nRETURN _0#\n",
			changed: "@F\nRETURN _0 #1\n",
		},
	}
	for _, tc := range controls {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkSourcePreservedOptions([]byte(tc.source), []byte(tc.changed), tc.options); err == nil {
				t.Fatalf("accepted opaque payload mutation: %q", tc.changed)
			}
		})
	}
}

func TestPreservationOracleProtectsContinuedPrintVPayloads(t *testing.T) {
	join := ""
	cases := []struct {
		name, source, changed string
	}{
		{
			name:    "raw",
			source:  "@F\n{\nPRINTV '0\nA000\n}\n",
			changed: "@F\n{\nPRINTV '0\nB000\n}\n",
		},
		{
			name:    "quoted",
			source:  "@F\n{\nPRINTV \"0\nA000\"\n}\n",
			changed: "@F\n{\nPRINTV \"0\nB000\"\n}\n",
		},
		{
			name:    "nul-suffix",
			source:  "@F\n{\nPRINTV 0\n0A000\x00tail\n}\n",
			changed: "@F\n{\nPRINTV 0\n0A000\x00fail\n}\n",
		},
		{
			name:    "comment",
			source:  "@F\n{\nPRINTV 0\n0A000 ;tail\n}\n",
			changed: "@F\n{\nPRINTV 0\n0A000 ;fail\n}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			options := sourcePreservationOptions{continuationSeparator: &join}
			if err := checkSourcePreservedOptions([]byte(tc.source), []byte(tc.source), options); err != nil {
				t.Fatalf("rejected unchanged continued PRINTV: %v", err)
			}
			if err := checkSourcePreservedOptions([]byte(tc.source), []byte(tc.changed), options); err == nil {
				t.Fatalf("accepted continued PRINTV payload mutation: %q", tc.changed)
			}
		})
	}
}

func TestPreservationOracleAllowsConsumedInlineCommentCanonicalization(t *testing.T) {
	source := []byte("@F\nRETURN 0; ;0 ")
	result := format.FormatWithOptions(source, format.ERB, format.Options{})
	if err := checkSourcePreserved(source, result.Output); err != nil {
		t.Fatalf("output=%q diagnostics=%+v: %v", result.Output, result.Diagnostics, err)
	}
	changed := bytes.Replace(result.Output, []byte("; ;0"), []byte("; ;1"), 1)
	if bytes.Equal(changed, result.Output) {
		t.Fatalf("mutation did not apply to output=%q", result.Output)
	}
	if err := checkSourcePreserved(source, changed); err == nil {
		t.Fatalf("accepted consumed comment payload mutation: %q", changed)
	}
	standalone := []byte("@F\n;;standalone\n")
	standaloneChanged := []byte("@F\n; ;standalone\n")
	if err := checkSourcePreserved(standalone, standaloneChanged); err == nil {
		t.Fatalf("accepted standalone protected comment mutation: %q", standaloneChanged)
	}
}
