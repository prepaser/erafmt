package format

import (
	"bytes"
	"testing"
)

func TestCallTargetLiteralWhitespace(t *testing.T) {
	for _, tc := range []struct {
		rule        RuleID
		input, want string
	}{
		{CommandSpacing, "TRYCALL \u3000F", "TRYCALL \u3000F"},
		{CommandSpacing, "TRYCALL\t\t\u3000F", "TRYCALL \u3000F"},
		{CommandSpacing, "TRYCALLFORM\u3000\u3000F", "TRYCALLFORM \u3000F"},
		{CommandSpacing, "TRYGOTO \u3000LABEL", "TRYGOTO \u3000LABEL"},
		{CommandSpacing, "TRYCALL \u3000", "TRYCALL \u3000"},
		{CommaSpacing, "TRYCALL F\u3000,1", "TRYCALL F\u3000, 1"},
		{CommaSpacing, "TRYCALLFORM F\u3000,1", "TRYCALLFORM F\u3000, 1"},
		{DelimiterSpacing, "TRYCALLFORM F\u3000(\u30001\u3000)", "TRYCALLFORM F\u3000(1)"},
		{DelimiterSpacing, "TRYCALL F\u3000[\u30001\u3000](\u30002\u3000)", "TRYCALL F\u3000[1](2)"},
		{CodeTrailingWhitespace, "TRYCALL F\u3000\t  ", "TRYCALL F\u3000"},
		{CodeTrailingWhitespace, "TRYCALL F(1)\u3000\t  ", "TRYCALL F(1)"},
		{InlineCommentSpacing, "TRYCALL F\u3000\t ;note", "TRYCALL F\u3000 ;note"},
		{InlineCommentSpacing, "TRYCALL F(1)\u3000 ;note", "TRYCALL F(1) ;note"},
	} {
		t.Run(string(tc.rule)+"/"+tc.input, func(t *testing.T) {
			for _, separator := range []*string{nil, new(string), new(" ")} {
				source, want := "@F\n"+tc.input+"\n", "@F\n"+tc.want+"\n"
				if separator != nil {
					source, want = "@F\n{\n"+tc.input+"\n}\n", "@F\n{\n"+tc.want+"\n}\n"
					if *separator != "" && tc.rule == CodeTrailingWhitespace {
						want = source
					}
				}
				options := Options{Select: []RuleID{tc.rule}, AllowFullwidthSpace: true, ContinuationSeparator: separator}
				result := FormatWithOptions([]byte(source), ERB, options)
				if string(result.Output) != want || len(result.Diagnostics) != 0 {
					t.Fatalf("separator=%v output=%q diagnostics=%+v, want %q", separator, result.Output, result.Diagnostics, want)
				}
				again := FormatWithOptions(result.Output, ERB, options)
				if !bytes.Equal(result.Output, again.Output) || len(again.Changes) != 0 || len(again.Diagnostics) != 0 {
					t.Fatalf("not idempotent: %+v", again)
				}
			}
		})
	}
}

func TestPlainAssignmentLiteralWhitespace(t *testing.T) {
	for _, destination := range []string{"RESULTS", "UserVar"} {
		for _, operator := range []string{"=", "=="} {
			for _, payload := range []string{"\u3000hello", "\u3000\thello", "\u3000"} {
				for _, separator := range []*string{nil, new(string), new(" ")} {
					line := destination + operator + " \t" + payload
					want := destination + operator + " " + payload
					source, expected := "@F\n"+line+"\n", "@F\n"+want+"\n"
					if separator != nil {
						source, expected = "@F\n{\n"+line+"\n}\n", "@F\n{\n"+want+"\n}\n"
					}
					options := Options{Select: []RuleID{AssignmentRightSpacing}, AllowFullwidthSpace: true, ContinuationSeparator: separator}
					result := FormatWithOptions([]byte(source), ERB, options)
					if string(result.Output) != expected || len(result.Diagnostics) != 0 {
						t.Fatalf("input=%q output=%q diagnostics=%+v want=%q", source, result.Output, result.Diagnostics, expected)
					}
					assertFullwidthIdempotent(t, result.Output, options)
				}
			}
		}
	}
}
