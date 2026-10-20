package format

import (
	"bytes"
	"testing"
)

func TestCallTargetEscapes(t *testing.T) {
	for _, tc := range []struct {
		rule        RuleID
		input, want string
	}{
		{CommaSpacing, `TRYCALL F\ ,1+2`, `TRYCALL F\ , 1+2`},
		{CommaSpacing, "TRYCALL F\\\t,1", "TRYCALL F\\\t, 1"},
		{CommaSpacing, `TRYCALL F\,1`, `TRYCALL F\,1`},
		{CommaSpacing, `TRYCALL F\\ ,1`, `TRYCALL F\\, 1`},
		{CommaSpacing, `TRYCALLFORM F\ ,1`, `TRYCALLFORM F\ , 1`},
		{DelimiterSpacing, `TRYCALL F\ ( 1 )`, `TRYCALL F\ (1)`},
		{DelimiterSpacing, `TRYCALL F\ [ 1 ] ( 2 )`, `TRYCALL F\ [1](2)`},
		{DelimiterSpacing, `TRYCALLFORM F\ ( 1 )`, `TRYCALLFORM F\ (1)`},
		{BinarySpacing, `TRYCALL F\(1+2)`, `TRYCALL F\(1+2)`},
		{BinarySpacing, `TRYCALL F\[1+2]`, `TRYCALL F\[1+2]`},
		{BinarySpacing, `TRYCALL F\;tail(1+2)`, `TRYCALL F\;tail(1 + 2)`},
		{BinarySpacing, `TRYCALL F"(1+2)`, `TRYCALL F"(1 + 2)`},
		{BinarySpacing, `TRYCALL F\"(1+2)`, `TRYCALL F\"(1 + 2)`},
		{InlineCommentSpacing, `TRYCALL F\;tail`, `TRYCALL F\;tail`},
		{InlineCommentSpacing, `TRYCALL F\ ;note`, `TRYCALL F\  ;note`},
		{CommentPrefixSpacing, `TRYCALL F\;tail`, `TRYCALL F\;tail`},
		{CommentPrefixSpacing, `TRYCALL F\ ;note`, `TRYCALL F\ ; note`},
		{CommentTrailingWhitespace, `TRYCALL F\;tail  `, `TRYCALL F\;tail  `},
		{CodeTrailingWhitespace, "TRYCALL F\\ ", "TRYCALL F\\ "},
		{CodeTrailingWhitespace, "TRYCALL F\\\t", "TRYCALL F\\\t"},
		{CodeTrailingWhitespace, "TRYCALL F\\ \t  ", "TRYCALL F\\ "},
		{CodeTrailingWhitespace, "TRYCALL F\\\\  ", "TRYCALL F\\\\"},
		{CodeTrailingWhitespace, "TRYCALL F\\　  ", "TRYCALL F\\　"},
	} {
		t.Run(string(tc.rule)+"/"+tc.input, func(t *testing.T) {
			for _, fullwidth := range []bool{false, true} {
				for _, separator := range []*string{nil, new(string), new(" ")} {
					source, want := "@F\n"+tc.input+"\n", "@F\n"+tc.want+"\n"
					if separator != nil {
						source, want = "@F\n{\n"+tc.input+"\n}\n", "@F\n{\n"+tc.want+"\n}\n"
						if *separator != "" && tc.rule == CodeTrailingWhitespace {
							want = source
						}
					}
					options := Options{Select: []RuleID{tc.rule}, AllowFullwidthSpace: fullwidth, ContinuationSeparator: separator}
					result := FormatWithOptions([]byte(source), ERB, options)
					if string(result.Output) != want || len(result.Diagnostics) != 0 {
						t.Fatalf("fullwidth=%v separator=%v output=%q diagnostics=%+v want=%q", fullwidth, separator, result.Output, result.Diagnostics, want)
					}
					again := FormatWithOptions(result.Output, ERB, options)
					if !bytes.Equal(again.Output, result.Output) || len(again.Changes) != 0 || len(again.Diagnostics) != 0 {
						t.Fatalf("not idempotent: %+v", again)
					}
				}
			}
		})
	}
}

func TestCallEscapeAcrossContinuationRows(t *testing.T) {
	for _, tail := range []string{",1+2", "(1+2)", "[1+2]", ";tail"} {
		source := "@F\n{\nTRYCALL F\\\n" + tail + "\n}\n"
		options := Options{Select: []RuleID{BinarySpacing, CommaSpacing, DelimiterSpacing, InlineCommentSpacing, CommentPrefixSpacing}, ContinuationSeparator: new(string)}
		result := FormatWithOptions([]byte(source), ERB, options)
		if string(result.Output) != source || len(result.Diagnostics) != 0 || len(result.Changes) != 0 {
			t.Fatalf("tail=%q output=%q diagnostics=%+v", tail, result.Output, result.Diagnostics)
		}
	}
	source := "@F\n{\nTRYCALL F\\\n,1+2\n}\n"
	options := Options{Select: []RuleID{BinarySpacing}, ContinuationSeparator: new(" ")}
	result := FormatWithOptions([]byte(source), ERB, options)
	if string(result.Output) != "@F\n{\nTRYCALL F\\\n,1 + 2\n}\n" || len(result.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
}

func TestCallTargetIncompleteEscapes(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		for _, command := range []string{"CALL", "CALLF", "TRYCALL", "JUMP", "GOTO", "CALLSHARP"} {
			if commandUnavailable(command, dialect) {
				continue
			}
			for _, tail := range []string{"\\", "\\\x00(1+2)", "\\\x00,1+2", "\\\x00[1+2](3+4)"} {
				prefix := "    " + command + " F"
				source := []byte("@F\n" + prefix + tail + "\n")
				options := Options{Dialect: dialect}
				result := FormatWithOptions(source, ERB, options)
				if !bytes.Equal(result.Output, source) || len(result.Changes) != 0 || len(result.Diagnostics) != 1 {
					t.Fatalf("dialect=%d source=%q result=%+v", dialect, source, result)
				}
				d := result.Diagnostics[0]
				if d.Code != "unsupported_syntax" || d.Severity != Unsupported || d.Line != 2 || d.Column != len(prefix)+1 {
					t.Fatalf("source=%q diagnostic=%+v", source, d)
				}
				again := FormatWithOptions(result.Output, ERB, options)
				if !bytes.Equal(again.Output, result.Output) || len(again.Changes) != 0 || len(again.Diagnostics) != 1 || again.Diagnostics[0] != d {
					t.Fatalf("unstable result: %+v", again)
				}
				options.Select = []RuleID{}
				disabled := FormatWithOptions(source, ERB, options)
				if !bytes.Equal(disabled.Output, source) || len(disabled.Diagnostics) != 0 {
					t.Fatalf("disabled parsing: %+v", disabled)
				}
			}
		}
	}
}

func TestCallIncompleteEscapeContinuation(t *testing.T) {
	for _, body := range []string{"CALL F\\", "CALL F\\\x00(1+2)", "CALL F\\\n\x00,1+2"} {
		source := []byte("@F\n{\n" + body + "\n}\n")
		options := Options{Select: []RuleID{BinarySpacing, CommaSpacing, DelimiterSpacing}, ContinuationSeparator: new(string)}
		result := FormatWithOptions(source, ERB, options)
		if !bytes.Equal(result.Output, source) || len(result.Changes) != 0 || len(result.Diagnostics) != 1 {
			t.Fatalf("source=%q result=%+v", source, result)
		}
		d := result.Diagnostics[0]
		if d.Code != "unsupported_syntax" || d.Severity != Unsupported || d.Line != 3 || d.Column != len("CALL F")+1 {
			t.Fatalf("source=%q diagnostic=%+v", source, d)
		}
		options.ContinuationSeparator = nil
		opaque := FormatWithOptions(source, ERB, options)
		if !bytes.Equal(opaque.Output, source) || len(opaque.Diagnostics) != 0 {
			t.Fatalf("unknown separator: %+v", opaque)
		}
		if body == "CALL F\\" || body == "CALL F\\\n\x00,1+2" {
			options.ContinuationSeparator = new(" ")
			valid := FormatWithOptions(source, ERB, options)
			if !bytes.Equal(valid.Output, source) || len(valid.Diagnostics) != 0 {
				t.Fatalf("valid separator escape: %+v", valid)
			}
		}
	}
}

func TestInlineSemicolonCommentProtection(t *testing.T) {
	for _, command := range []string{"RETURN 1", "TRYCALL F", "SETCOLOR 1", "PRINTV"} {
		for _, tail := range []string{";;;note  ", ";;;; note  ", "; ;;note  "} {
			for _, selected := range [][]RuleID{{InlineCommentSpacing}, {CommentPrefixSpacing}, {CommentTrailingWhitespace}, {InlineCommentSpacing, CommentPrefixSpacing, CommentTrailingWhitespace}} {
				source := "@F\n" + command + "\t" + tail + "\n"
				result := FormatWithOptions([]byte(source), ERB, Options{Select: selected})
				if string(result.Output) != source || len(result.Diagnostics) != 0 || len(result.Changes) != 0 {
					t.Fatalf("source=%q selected=%v output=%q diagnostics=%+v", source, selected, result.Output, result.Diagnostics)
				}
			}
		}
	}
	source := []byte("@F\nRETURN 1 ;;note  \n")
	result := FormatWithOptions(source, ERB, Options{Select: []RuleID{CommentPrefixSpacing, CommentTrailingWhitespace}})
	if string(result.Output) != "@F\nRETURN 1 ; ;note\n" || len(result.Diagnostics) != 0 {
		t.Fatalf("ordinary double semicolon output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
}
