package cli

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestCorpusPreservationNulStructuredPrefixes(t *testing.T) {
	cases := []struct {
		body, formatted string
	}{
		{"PRINT_RECT 1+2\x00,4,5", "PRINT_RECT 1 + 2\x00,4,5"},
		{"PRINT_RECT 1+2,\x00,4,5", "PRINT_RECT 1 + 2,\x00,4,5"},
		{"SETCOLOR 1+2\x00,2", "SETCOLOR 1 + 2\x00,2"},
		{"LOADDATA 1+2\x00,ignored", "LOADDATA 1 + 2\x00,ignored"},
		{"PRINTBUTTON \"x\",1+2\x00,ignored", "PRINTBUTTON \"x\", 1 + 2\x00,ignored"},
	}
	for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
		for _, allowFullwidth := range []bool{false, true} {
			for _, tc := range cases {
				name := fmt.Sprintf("dialect=%d/fullwidth=%v/%s", dialect, allowFullwidth, tc.body[:bytes.IndexByte([]byte(tc.body), ' ')])
				t.Run(name, func(t *testing.T) {
					source := []byte("@F\n" + tc.body + "\n")
					options := format.Options{Dialect: dialect, AllowFullwidthSpace: allowFullwidth}
					result := format.FormatWithOptions(source, format.ERB, options)
					if len(result.Diagnostics) != 0 {
						t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
					}
					if !bytes.Contains(result.Output, []byte(tc.formatted)) {
						t.Fatalf("formatted prefix missing from output: want %q in %q", tc.formatted, result.Output)
					}
					proof := sourcePreservationOptions{dialect: dialect, allowFullwidthSpace: allowFullwidth}
					if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
						t.Fatalf("output=%q: %v", result.Output, err)
					}
				})
			}
		}
	}
}

func TestCorpusPreservationNulMutationInventory(t *testing.T) {
	cases := []struct {
		name, source, changed string
	}{
		{"call-target-padding", "CALL F\x00\n", "CALL F\x00 \n"},
		{"form-call-target-padding", "CALLFORM F\x00\n", "CALLFORM F\x00 \n"},
		{"raw-form-island", "PRINTFORM raw\x00{1+2}\n", "PRINTFORM raw\x00{ 1+2}\n"},
		{"inputs-form-island", "INPUTS prompt\x00{1+2}\n", "INPUTS prompt\x00{ 1+2}\n"},
		{"reuse-last-line-form-island", "REUSELASTLINE raw\x00{0+0}\n", "REUSELASTLINE raw\x00{0 +0}\n"},
		{"strlen-form-island", "STRLENFORM raw\x00{1+2}\n", "STRLENFORM raw\x00{1 +2}\n"},
		{"savevar-field", "SAVEVAR \"x\",\"y\",RESULT \x00,1+2\n", "SAVEVAR \"x\",\"y\",RESULT \x00,1+ 2\n"},
		{"abs-comment", "ABS 1\x00;\n", "ABS 1\x00; \n"},
		{"return-comment", "RETURN 1\x00; note  \n", "RETURN 1\x00; note   \n"},
		{"result-comment", "RESULT +=1\x00; note  \n", "RESULT +=1\x00;  note  \n"},
		{"loaddata-comment", "LOADDATA 1\x00;ote! \n", "LOADDATA 1\x00;ote!  \n"},
		{"setcolor-comment", "SETCOLOR 1\x00; 00000\n", "SETCOLOR 1\x00;  00000\n"},
		{"printbutton-comment", "PRINTBUTTON \"x\",1\x00;0\n", "PRINTBUTTON \"x\",1\x00; 0\n"},
		{"printv-numeric-tail", "PRINTV 1\x00\u3000\u30000\n", "PRINTV 1\x00\u3000\u30000 \n"},
		{"printv-comment-tail", "PRINTV 1\x00; n/te  \n", "PRINTV 1\x00; n/te   \n"},
		{"times-marker-tail", "TIMES RESULT,1.0\x00;!;;\n", "TIMES RESULT,1.0\x00;!;; \n"},
		{"plain-call-padding", "TRYCALL F\x00  \n", "TRYCALL F\x00\n"},
		{"plain-call-comma", "TRYCALL F\x00  ,1\n", "TRYCALL F\x00,1\n"},
		{"form-call-padding", "CALLFORM F\x00  \n", "CALLFORM F\x00\n"},
		{"expression-comment", "RETURN 1\x00;note\n", "RETURN 1\x00; note\n"},
		{"compound-comment", "RESULT +=1\x00;note\n", "RESULT +=1\x00; note\n"},
	}
	for _, tc := range cases {
		for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
			for _, allowFullwidth := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/dialect=%d/fullwidth=%v", tc.name, dialect, allowFullwidth), func(t *testing.T) {
					proof := sourcePreservationOptions{dialect: dialect, allowFullwidthSpace: allowFullwidth}
					if err := checkSourcePreservedOptions([]byte("@F\n"+tc.source), []byte("@F\n"+tc.changed), proof); err == nil {
						t.Fatalf("accepted ignored NUL suffix mutation: %q -> %q", tc.source, tc.changed)
					}
				})
			}
		}
	}
}

func TestCorpusPreservationNulCallPrefixes(t *testing.T) {
	for _, tc := range []struct {
		body, formatted string
	}{
		{"TRYCALL F,1+2\x00ignored", "TRYCALL F, 1 + 2\x00ignored"},
		{"TRYCALL F(1+2)\x00ignored", "TRYCALL F(1 + 2)\x00ignored"},
		{"CALLFORM F,1+2\x00ignored", "CALLFORM F, 1 + 2\x00ignored"},
	} {
		for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
			for _, allowFullwidth := range []bool{false, true} {
				t.Run(fmt.Sprintf("dialect=%d/fullwidth=%v/%s", dialect, allowFullwidth, tc.body[:bytes.IndexByte([]byte(tc.body), ' ')]), func(t *testing.T) {
					source := []byte("@F\n" + tc.body + "\n")
					result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: dialect, AllowFullwidthSpace: allowFullwidth})
					if len(result.Diagnostics) != 0 {
						t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
					}
					if !bytes.Contains(result.Output, []byte(tc.formatted)) {
						t.Fatalf("formatted prefix missing from output: want %q in %q", tc.formatted, result.Output)
					}
					proof := sourcePreservationOptions{dialect: dialect, allowFullwidthSpace: allowFullwidth}
					if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
						t.Fatalf("output=%q: %v", result.Output, err)
					}
				})
			}
		}
	}
}

func TestCorpusPreservationNulCallOperatorSuffixes(t *testing.T) {
	cases := []struct {
		name, source, formatted, changed string
		options                          sourcePreservationOptions
		formatOptions                    format.Options
	}{
		{
			"plain-call-operator",
			"@F\nTRYCALL F,1\x00 &\n",
			"TRYCALL F, 1\x00 &",
			"@F\nTRYCALL F,1\x00  &\n",
			sourcePreservationOptions{dialect: format.EMEE},
			format.Options{Dialect: format.EMEE},
		},
		{
			"form-call-operator",
			"@F\nCALLFORM F,1\x00 &\n",
			"CALLFORM F, 1\x00 &",
			"@F\nCALLFORM F,1\x00  &\n",
			sourcePreservationOptions{dialect: format.EMEE},
			format.Options{Dialect: format.EMEE},
		},
		{
			"plain-call-fullwidth-argument",
			"@F\nTRYCALL F,A\u3000\x00 &\n",
			"TRYCALL F, A\u3000\x00 &",
			"@F\nTRYCALL F,A\u3000\x00  &\n",
			sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true},
			format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true},
		},
		{
			"form-call-fullwidth-argument",
			"@F\nCALLFORM F,A\u3000\x00 &\n",
			"CALLFORM F, A\u3000\x00 &",
			"@F\nCALLFORM F,A\u3000\x00  &\n",
			sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true},
			format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte(tc.source)
			result := format.FormatWithOptions(source, format.ERB, tc.formatOptions)
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
			}
			if !bytes.Contains(result.Output, []byte(tc.formatted)) {
				t.Fatalf("formatted prefix missing: want %q in %q", tc.formatted, result.Output)
			}
			if err := checkSourcePreservedOptions(source, result.Output, tc.options); err != nil {
				t.Fatalf("safe prefix rejected: output=%q: %v", result.Output, err)
			}
			if err := checkSourcePreservedOptions(source, []byte(tc.changed), tc.options); err == nil {
				t.Fatalf("accepted ignored operator suffix mutation: %q", tc.changed)
			}
		})
	}
}

func TestCorpusPreservationNulCallCommentsAndFullwidthPadding(t *testing.T) {
	for _, tc := range []struct {
		name, source, changed string
		options               sourcePreservationOptions
	}{
		{"plain-empty-comment", "@F\nTRYCALL F,\u3000;note\n", "@F\nTRYCALL F, ; note\n", sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}},
		{"form-empty-comment", "@F\nCALLFORM F,\u3000;note\n", "@F\nCALLFORM F, ;note\n", sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}},
		{"nonempty-comment", "@F\nCALL F,1;A 1\n", "@F\nCALL F, 1 ; A 1\n", sourcePreservationOptions{dialect: format.CommonDialect}},
		{"form-nonempty-comment", "@F\nCALLFORM F,1;A 1\n", "@F\nCALLFORM F, 1;A 1\n", sourcePreservationOptions{dialect: format.EMEE}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkSourcePreservedOptions([]byte(tc.source), []byte(tc.changed), tc.options); err != nil {
				t.Fatalf("safe comment formatting rejected: %v", err)
			}
		})
	}
	for _, tc := range []struct {
		name, source, changed string
		options               sourcePreservationOptions
	}{
		{"comment-payload", "@F\nCALL F,1;note\n", "@F\nCALL F, 1 ; changed\n", sourcePreservationOptions{dialect: format.CommonDialect}},
		{"fullwidth-comment-payload", "@F\nTRYCALL F,\u3000;note\n", "@F\nTRYCALL F, ; changed\n", sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}},
		{"fullwidth-literal-comment", "@F\nCALL F,1;note\u3000payload\n", "@F\nCALL F, 1 ; note payload\n", sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}},
		{"fullwidth-mandatory-marker", "@F\nTRYCALL F,1;!;\u3000payload\n", "@F\nTRYCALL F, 1 ;!; payload\n", sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}},
		{"fullwidth-protected-marker", "@F\nTRYCALL F,1;#;\u3000payload\n", "@F\nTRYCALL F, 1 ;#; payload\n", sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkSourcePreservedOptions([]byte(tc.source), []byte(tc.changed), tc.options); err == nil {
				t.Fatal("accepted comment payload mutation")
			}
		})
	}
}

func TestCorpusPreservationNulPrintVTerminalComma(t *testing.T) {
	source := []byte("@F\nPRINTV 'x,0,\x00\n")
	for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
		for _, allowFullwidth := range []bool{false, true} {
			options := format.Options{Dialect: dialect, AllowFullwidthSpace: allowFullwidth}
			result := format.FormatWithOptions(source, format.ERB, options)
			if len(result.Diagnostics) != 0 {
				t.Fatalf("dialect=%d fullwidth=%v diagnostics=%v output=%q", dialect, allowFullwidth, result.Diagnostics, result.Output)
			}
			if !bytes.Contains(result.Output, []byte("PRINTV 'x, 0,\x00")) {
				t.Fatalf("dialect=%d fullwidth=%v formatter did not add terminal comma spacing", dialect, allowFullwidth)
			}
			proof := sourcePreservationOptions{dialect: dialect, allowFullwidthSpace: allowFullwidth}
			if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
				t.Fatalf("dialect=%d fullwidth=%v output=%q: %v", dialect, allowFullwidth, result.Output, err)
			}
		}
	}
	for _, changed := range []string{
		"@F\nPRINTV 'x,,0,\x00\n",
		"@F\nPRINTV 'x,0,,\x00\n",
		"@F\nPRINTV , 'x,0,\x00\n",
	} {
		if err := checkSourcePreserved(source, []byte(changed)); err == nil {
			t.Fatalf("accepted malformed PRINTV omission: %q", changed)
		}
	}
}

func TestCorpusPreservationNulFormBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, source, changed string
		wantErr               bool
	}{
		{"raw-suffix-island", "@F\nPRINTFORM raw\x00{1+2}\n", "@F\nPRINTFORM raw\x00{1 + 2}\n", true},
		{"pre-nul-island", "@F\nPRINTFORM {1+2}\x00{3+4}\n", "@F\nPRINTFORM {1 + 2}\x00{3+4}\n", false},
		{"quoted-nul-payload", "@F\nPRINTFORM @\"x\x00{1+2}\"\n", "@F\nPRINTFORM @\"x\x00{1 + 2}\"\n", true},
		{"identifier-nul", "@F\nPRINTV foo\x00bar\u3000baz\n", "@F\nPRINTV foo\x00bar baz\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkSourcePreservedOptions([]byte(tc.source), []byte(tc.changed), sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true})
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v got=%v: %v", tc.wantErr, err != nil, err)
			}
		})
	}
}

func TestCorpusPreservationNulLexicalControls(t *testing.T) {
	cases := []struct {
		name, source, formatted string
	}{
		{"quoted-literal", "@F\nPRINTFORM @\"x\x00raw\"\n", "PRINTFORM @\"x\x00raw\""},
		{"form-island-identifier", "@F\nPRINTFORM {A\x00B+1}\n", "PRINTFORM {A\x00B + 1}"},
		{"conditional-identifier", "@F\nPRINTFORM \\@ A\x00B+1?yes#no\\@\n", "PRINTFORM \\@ A\x00B + 1 ?yes#no\\@"},
	}
	for _, tc := range cases {
		for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
			for _, allowFullwidth := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/dialect=%d/fullwidth=%v", tc.name, dialect, allowFullwidth), func(t *testing.T) {
					source := []byte(tc.source)
					result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: dialect, AllowFullwidthSpace: allowFullwidth})
					if len(result.Diagnostics) != 0 {
						t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
					}
					if !bytes.Contains(result.Output, []byte(tc.formatted)) {
						t.Fatalf("formatted payload missing: want %q in %q", tc.formatted, result.Output)
					}
					proof := sourcePreservationOptions{dialect: dialect, allowFullwidthSpace: allowFullwidth}
					if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
						t.Fatalf("output=%q: %v", result.Output, err)
					}
				})
			}
		}
	}
}

func TestCorpusPreservationNulInputsBoundaries(t *testing.T) {
	options := format.Options{Dialect: format.EMEE}
	for _, tc := range []struct {
		name, source, formatted string
	}{
		{"embedded-identifier", "@F\nINPUTS prompt,foo\x00bar+1\n", "INPUTS prompt, foo\x00bar + 1"},
		{"raw-first-field", "@F\nINPUTS prompt\x00{1+2}\n", "INPUTS prompt\x00{1+2}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte(tc.source)
			result := format.FormatWithOptions(source, format.ERB, options)
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics=%v output=%q", result.Diagnostics, result.Output)
			}
			if !bytes.Contains(result.Output, []byte(tc.formatted)) {
				t.Fatalf("formatted payload missing: want %q in %q", tc.formatted, result.Output)
			}
			if err := checkSourcePreservedDialect(source, result.Output, format.EMEE); err != nil {
				t.Fatalf("output=%q: %v", result.Output, err)
			}
		})
	}
}
