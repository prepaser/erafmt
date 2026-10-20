package format

import (
	"bytes"
	"testing"
)

func TestFormAnyPreservesLiteralWhitespaceAfterSeparator(t *testing.T) {
	options := Options{Dialect: EMEE, Select: []RuleID{CommandSpacing, BinarySpacing, FormPadding}}
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{name: "tab", body: "SETBGIMAGE \timage { A+1 }", want: "SETBGIMAGE \timage {A + 1}"},
		{name: "wide", body: "REMOVEBGIMAGE \u3000image { A+1 }", want: "REMOVEBGIMAGE \u3000image {A + 1}"},
		{name: "tab-only", body: "SETBGIMAGE \t", want: "SETBGIMAGE \t"},
		{name: "wide-only", body: "REMOVEBGIMAGE \u3000", want: "REMOVEBGIMAGE \u3000"},
	} {
		t.Run(test.name, func(t *testing.T) {
			src := []byte("@F\n" + test.body + "\n")
			result := FormatWithOptions(src, ERB, options)
			want := []byte("@F\n" + test.want + "\n")
			if !bytes.Equal(result.Output, want) || len(result.Diagnostics) != 0 {
				t.Fatalf("output=%q diagnostics=%+v want=%q", result.Output, result.Diagnostics, want)
			}
		})
	}
	for _, body := range []string{"SETBGIMAGE \u3000image", "REMOVEBGIMAGE \u3000"} {
		src := []byte("@F\n" + body + "\n")
		wideOptions := options
		wideOptions.AllowFullwidthSpace = true
		result := FormatWithOptions(src, ERB, wideOptions)
		if !bytes.Equal(result.Output, src) || len(result.Diagnostics) != 0 {
			t.Fatalf("allow-fullwidth body=%q output=%q diagnostics=%+v", body, result.Output, result.Diagnostics)
		}
	}

	for _, body := range []string{"SETBGIMAGE    image", "REMOVEBGIMAGE\timage", "SETBGIMAGE \u3000image"} {
		src := []byte("@F\n" + body + "\n")
		result := FormatWithOptions(src, ERB, options)
		if len(result.Diagnostics) != 0 {
			t.Fatalf("body=%q diagnostics=%+v", body, result.Diagnostics)
		}
		if !bytes.Contains(result.Output, []byte("SETBGIMAGE ")) && !bytes.Contains(result.Output, []byte("REMOVEBGIMAGE ")) {
			t.Fatalf("body=%q output=%q", body, result.Output)
		}
	}
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{name: "physical", body: "SETBGIMAGE;image { A+1 }", want: "SETBGIMAGE;image {A + 1}"},
		{name: "physical-spaces", body: "REMOVEBGIMAGE;  image { A+1 }", want: "REMOVEBGIMAGE;  image {A + 1}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := FormatWithOptions([]byte("@F\n"+test.body+"\n"), ERB, options)
			want := []byte("@F\n" + test.want + "\n")
			if !bytes.Equal(result.Output, want) || len(result.Diagnostics) != 0 {
				t.Fatalf("output=%q diagnostics=%+v want=%q", result.Output, result.Diagnostics, want)
			}
		})
	}

	for _, body := range []string{"SETBGIMAGE    ", "REMOVEBGIMAGE   "} {
		src := []byte("@F\n" + body + "\n")
		result := FormatWithOptions(src, ERB, options)
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_syntax" || !bytes.Equal(result.Output, src) {
			t.Fatalf("body=%q output=%q diagnostics=%+v", body, result.Output, result.Diagnostics)
		}
	}
}

func TestFormAnyPreservesLiteralWhitespaceInExplicitContinuation(t *testing.T) {
	separator := " "
	options := Options{Dialect: EMEE, Select: []RuleID{CommandSpacing, BinarySpacing, FormPadding}, ContinuationSeparator: &separator}
	for _, test := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "tab",
			src:  "@F\n{\nSETBGIMAGE\n\timage { A+1 }\n}\n",
			want: "@F\n{\nSETBGIMAGE\n\timage {A + 1}\n}\n",
		},
		{
			name: "wide",
			src:  "@F\n{\nREMOVEBGIMAGE\n\u3000image { A+1 }\n}\n",
			want: "@F\n{\nREMOVEBGIMAGE\n\u3000image {A + 1}\n}\n",
		},
		{
			name: "semicolon",
			src:  "@F\n{\nSETBGIMAGE\n;image { A+1 }\n}\n",
			want: "@F\n{\nSETBGIMAGE\n;image {A + 1}\n}\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := FormatWithOptions([]byte(test.src), ERB, options)
			if !bytes.Equal(result.Output, []byte(test.want)) || len(result.Diagnostics) != 0 {
				t.Fatalf("output=%q diagnostics=%+v want=%q", result.Output, result.Diagnostics, test.want)
			}
		})
	}
}

func TestSAVEVARAcceptsOneTrailingComma(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing, FormPadding}}
	for _, src := range []string{
		"@F\nSAVEVAR \"file\",\"message\",RESULT,\n",
		"@F\nSAVEVAR \"file\",\"message\",RESULT,   ; comment\n",
	} {
		result := FormatWithOptions([]byte(src), ERB, options)
		if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte("RESULT,")) {
			t.Fatalf("source=%q output=%q diagnostics=%+v", src, result.Output, result.Diagnostics)
		}
	}
	src := []byte("@F\nSAVEVAR \"file\",\"message\",RESULT \t, ; comment\n")
	want := []byte("@F\nSAVEVAR \"file\", \"message\", RESULT, ; comment\n")
	result := FormatWithOptions(src, ERB, options)
	if !bytes.Equal(result.Output, want) || len(result.Diagnostics) != 0 {
		t.Fatalf("terminal comma spacing: output=%q diagnostics=%+v want=%q", result.Output, result.Diagnostics, want)
	}
}

func TestSAVEVARRejectsDoubleTrailingComma(t *testing.T) {
	options := Options{Select: []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing, FormPadding}}
	for _, src := range []string{
		"@F\nSAVEVAR \"file\",\"message\",RESULT,,\n",
		"@F\nSAVEVAR \"file\",\"message\",RESULT,,   ; comment\n",
	} {
		result := FormatWithOptions([]byte(src), ERB, options)
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_syntax" || !bytes.Equal(result.Output, []byte(src)) {
			t.Fatalf("source=%q output=%q diagnostics=%+v", src, result.Output, result.Diagnostics)
		}
	}
}

func TestSAVEVARAcceptsOneTrailingCommaInExplicitContinuation(t *testing.T) {
	separator := " "
	src := "@F\n{\nSAVEVAR \"file\",\"message\",RESULT,\n\n}\n"
	result := FormatWithOptions([]byte(src), ERB, Options{
		Select:                []RuleID{CommandSpacing, BinarySpacing, CommaSpacing, DelimiterSpacing, FormPadding},
		ContinuationSeparator: &separator,
	})
	if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte("RESULT,")) {
		t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
}
