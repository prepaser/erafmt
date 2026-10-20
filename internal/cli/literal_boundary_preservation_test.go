package cli

import (
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestSourcePreservationRejectsLiteralBoundaryMutations(t *testing.T) {
	tests := []struct {
		name, original, output string
		dialect                format.Dialect
		allowFullwidth         bool
	}{
		{
			name:           "assignment-leading-fullwidth",
			original:       "@F\nRESULTS=\u3000hello\n",
			output:         "@F\nRESULTS= hello\n",
			allowFullwidth: true,
		},
		{
			name:     "form-any-tab-payload",
			original: "@F\nSETBGIMAGE \timage\n",
			output:   "@F\nSETBGIMAGE image\n",
			dialect:  format.EMEE,
		},
		{
			name:           "form-any-leading-fullwidth-payload",
			original:       "@F\nSETBGIMAGE \u3000image\n",
			output:         "@F\nSETBGIMAGE image\n",
			dialect:        format.EMEE,
			allowFullwidth: true,
		},
		{
			name:           "call-leading-fullwidth",
			original:       "@F\nTRYCALL \u3000F\n",
			output:         "@F\nTRYCALL F\n",
			allowFullwidth: true,
		},
		{
			name:           "call-leading-fullwidth-before-unknown-arguments",
			original:       "@F\nTRYCALL \u3000F(UNKNOWN+1)\n",
			output:         "@F\nTRYCALL F(UNKNOWN + 1)\n",
			allowFullwidth: true,
		},
		{
			name:           "call-leading-fullwidth-before-marked-arguments",
			original:       "@F\nTRYCALL \u3000F(;!;1+2)\n",
			output:         "@F\nTRYCALL F(;!;1 + 2)\n",
			allowFullwidth: true,
		},
		{
			name:           "call-leading-fullwidth-interior-ascii",
			original:       "@F\nTRYCALL \u3000 F\n",
			output:         "@F\nTRYCALL \u3000F\n",
			allowFullwidth: true,
		},
		{
			name:           "form-call-leading-fullwidth",
			original:       "@F\nTRYCALLFORM \u3000F\n",
			output:         "@F\nTRYCALLFORM F\n",
			allowFullwidth: true,
		},
		{
			name:           "call-comma-fullwidth",
			original:       "@F\nTRYCALL F\u3000,1\n",
			output:         "@F\nTRYCALL F, 1\n",
			allowFullwidth: true,
		},
		{
			name:           "call-interior-fullwidth",
			original:       "@F\nTRYCALL F \u3000 G\n",
			output:         "@F\nTRYCALL F G\n",
			allowFullwidth: true,
		},
		{
			name:           "call-trailing-fullwidth-interior-ascii",
			original:       "@F\nTRYCALL F \u3000\n",
			output:         "@F\nTRYCALL F\u3000\n",
			allowFullwidth: true,
		},
		{
			name:     "call-interior-ascii-without-fullwidth",
			original: "@F\nTRYCALL F G\n",
			output:   "@F\nTRYCALL FG\n",
		},
		{
			name:           "form-call-delimiter-fullwidth",
			original:       "@F\nTRYCALLFORM F\u3000(1)\n",
			output:         "@F\nTRYCALLFORM F(1)\n",
			allowFullwidth: true,
		},
		{
			name:           "call-end-fullwidth",
			original:       "@F\nTRYCALL F\u3000\n",
			output:         "@F\nTRYCALL F\n",
			allowFullwidth: true,
		},
		{
			name:           "call-comment-fullwidth",
			original:       "@F\nTRYCALL F\u3000 ;note\n",
			output:         "@F\nTRYCALL F ;note\n",
			allowFullwidth: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkSourcePreservedOptions([]byte(tc.original), []byte(tc.output), sourcePreservationOptions{dialect: tc.dialect, allowFullwidthSpace: tc.allowFullwidth}); err == nil {
				t.Fatalf("accepted literal mutation: %q -> %q", tc.original, tc.output)
			}
		})
	}
}

func TestSourcePreservationAcceptsGrammarBoundaryFormatting(t *testing.T) {
	tests := []struct {
		name, original, output string
		dialect                format.Dialect
		allowFullwidth         bool
	}{
		{
			name:           "assignment-ascii-padding",
			original:       "@F\nRESULTS=  hello\n",
			output:         "@F\nRESULTS= hello\n",
			allowFullwidth: true,
		},
		{
			name:     "form-any-tab-separator",
			original: "@F\nSETBGIMAGE\timage\n",
			output:   "@F\nSETBGIMAGE image\n",
			dialect:  format.EMEE,
		},
		{
			name:     "form-any-semicolon-separator",
			original: "@F\nSETBGIMAGE;  image { A+1 }\n",
			output:   "@F\nSETBGIMAGE;  image {A + 1}\n",
			dialect:  format.EMEE,
		},
		{
			name:           "call-fullwidth-command-separator",
			original:       "@F\nTRYCALL\u3000F\n",
			output:         "@F\nTRYCALL F\n",
			allowFullwidth: true,
		},
		{
			name:           "call-expression-fullwidth-spacing",
			original:       "@F\nTRYCALL F(1\u3000+\u30002)\n",
			output:         "@F\nTRYCALL F(1 + 2)\n",
			allowFullwidth: true,
		},
		{
			name:           "form-call-expression-fullwidth-spacing",
			original:       "@F\nTRYCALLFORM F(1\u3000+\u30002)\n",
			output:         "@F\nTRYCALLFORM F(1 + 2)\n",
			allowFullwidth: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkSourcePreservedOptions([]byte(tc.original), []byte(tc.output), sourcePreservationOptions{dialect: tc.dialect, allowFullwidthSpace: tc.allowFullwidth}); err != nil {
				t.Fatalf("rejected safe formatting: %q -> %q: %v", tc.original, tc.output, err)
			}
		})
	}
}

func TestSourcePreservationRejectsContinuedLiteralBoundaryMutations(t *testing.T) {
	tests := []struct {
		name, line, output string
		dialect            format.Dialect
		fullwidth          bool
	}{
		{name: "assignment", line: "RESULTS=\u3000hello", output: "RESULTS= hello", fullwidth: true},
		{name: "call-prefix", line: "TRYCALL \u3000F", output: "TRYCALL F", fullwidth: true},
		{name: "form-call-delimiter", line: "TRYCALLFORM F\u3000(1)", output: "TRYCALLFORM F(1)", fullwidth: true},
		{name: "call-comma", line: "TRYCALL F\u3000,1", output: "TRYCALL F, 1", fullwidth: true},
		{name: "call-end", line: "TRYCALL F\u3000", output: "TRYCALL F", fullwidth: true},
		{name: "call-comment", line: "TRYCALL F\u3000 ;note", output: "TRYCALL F ;note", fullwidth: true},
		{name: "form-any-tab", line: "SETBGIMAGE \timage", output: "SETBGIMAGE image", dialect: format.EMEE},
	}
	for _, separator := range []string{"", " "} {
		for _, tc := range tests {
			t.Run(tc.name+"/separator="+separator, func(t *testing.T) {
				original := []byte("@F\n{\n" + tc.line + "\n}\n")
				output := []byte("@F\n{\n" + tc.output + "\n}\n")
				options := sourcePreservationOptions{dialect: tc.dialect, allowFullwidthSpace: tc.fullwidth, continuationSeparator: &separator}
				if err := checkSourcePreservedOptions(original, output, options); err == nil {
					t.Fatalf("accepted continued literal mutation: %q -> %q", original, output)
				}
			})
		}
	}
}
