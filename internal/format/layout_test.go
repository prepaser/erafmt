package format

import (
	"bytes"
	"strings"
	"testing"
)

func TestFinalNewlineAfterDeletingAllTrivia(t *testing.T) {
	for _, src := range []string{" \t", "\r\n@F"} {
		selected := []RuleID{BlankLineWhitespace, FileBoundaryBlankLines, FinalNewline}
		want := ""
		if src == "\r\n@F" {
			want = "@F\n"
		} else {
			selected = []RuleID{BlankLineWhitespace, FinalNewline}
		}
		r := FormatWithOptions([]byte(src), ERB, Options{Select: selected})
		if string(r.Output) != want || len(r.Diagnostics) != 0 || want == "" && r.Output != nil {
			t.Fatalf("source %q: output %q, diagnostics %+v", src, r.Output, r.Diagnostics)
		}
	}
}

func layoutOnly(t *testing.T, rule RuleID, input string) Result {
	t.Helper()
	return FormatWithOptions([]byte(input), ERB, Options{Select: []RuleID{rule}})
}

func TestLayoutRuleIsolation(t *testing.T) {
	tests := []struct {
		name        string
		rule        RuleID
		input, want string
	}{
		{"preprocessor indent", PreprocessorIndent, "  [IF FEATURE]\n[ENDIF]\n", "[IF FEATURE]\n[ENDIF]\n"},
		{"preprocessor fields", PreprocessorFieldSpacing, "[IF   FEATURE  ]\n[ENDIF]\n", "[IF FEATURE]\n[ENDIF]\n"},
		{"inline comment", InlineCommentSpacing, "IF A; note\nENDIF\n", "IF A ; note\nENDIF\n"},
		{"aligned inline comment", InlineCommentSpacing, "IF A    \t;note  \nENDIF\n", "IF A ;note  \nENDIF\n"},
		{"comment prefix", CommentPrefixSpacing, ";note\n", "; note\n"},
		{"blank body", BlankLineWhitespace, " \t\n", "\n"},
		{"code trailing", CodeTrailingWhitespace, "IF A   \nENDIF\n", "IF A\nENDIF\n"},
		{"comment trailing", CommentTrailingWhitespace, "; note   \n", "; note\n"},
		{"function separation", FunctionSeparation, "@A\nRETURN 0\n@B\nRETURN 1\n", "@A\nRETURN 0\n\n@B\nRETURN 1\n"},
		{"blank runs", BlankLineRuns, "RETURN 0\n\n\nRETURN 1\n", "RETURN 0\n\nRETURN 1\n"},
		{"file boundary blank lines", FileBoundaryBlankLines, "\n \t\nRETURN 0\n\n", "RETURN 0\n"},
		{"final newline", FinalNewline, "RETURN 0", "RETURN 0\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := layoutOnly(t, tt.rule, tt.input)
			if got.Output == nil || string(got.Output) != tt.want {
				t.Fatalf("output %q, want %q; diagnostics=%+v", got.Output, tt.want, got.Diagnostics)
			}
			for _, c := range got.Changes {
				if c.Rule != tt.rule {
					t.Fatalf("change %+v belongs to another rule", c)
				}
			}
			if len(got.Changes) == 0 {
				t.Fatal("expected a committed change")
			}
			again := layoutOnly(t, tt.rule, string(got.Output))
			if !bytes.Equal(again.Output, got.Output) || len(again.Changes) != 0 {
				t.Fatalf("not idempotent: %+v", again)
			}
		})
	}
}

func TestPaddedSkipEndUsesPreprocessorLayout(t *testing.T) {
	input := []byte("@F\n [ SKIPSTART ]\nPRINT raw\n \t[ SKIPEND ]\nRETURN 0\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{PreprocessorIndent, PreprocessorFieldSpacing}})
	want := []byte("@F\n[SKIPSTART]\nPRINT raw\n[SKIPEND]\nRETURN 0\n")
	if !bytes.Equal(result.Output, want) || len(result.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
	if len(result.Changes) != 4 {
		t.Fatalf("changes=%+v", result.Changes)
	}
	for _, change := range result.Changes {
		if change.Rule != PreprocessorIndent && change.Rule != PreprocessorFieldSpacing {
			t.Fatalf("unexpected change=%+v", change)
		}
	}

	result = FormatWithOptions(input, ERB, Options{Select: []RuleID{}})
	if !bytes.Equal(result.Output, input) || len(result.Changes) != 0 || len(result.Diagnostics) != 0 {
		t.Fatalf("select none output=%q changes=%+v diagnostics=%+v", result.Output, result.Changes, result.Diagnostics)
	}
}

func TestFileBoundaryBlankLinesOwnership(t *testing.T) {
	input := []byte("\n \t\nRETURN 0\n\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{BlankLineWhitespace, BlankLineRuns}})
	if string(r.Output) != "\nRETURN 0\n\n" {
		t.Fatalf("L006 disabled output %q", r.Output)
	}
	for _, c := range r.Changes {
		if c.Rule == FileBoundaryBlankLines {
			t.Fatalf("disabled L006 emitted change %+v", c)
		}
	}

	r = FormatWithOptions(input, ERB, Options{Select: []RuleID{FileBoundaryBlankLines, BlankLineWhitespace, BlankLineRuns}})
	if string(r.Output) != "RETURN 0\n" {
		t.Fatalf("boundary output %q, changes=%+v", r.Output, r.Changes)
	}
	var sawBoundary bool
	for _, c := range r.Changes {
		sawBoundary = sawBoundary || c.Rule == FileBoundaryBlankLines
		if c.Rule == BlankLineRuns {
			t.Fatalf("L005 overlapped an L006-owned boundary: %+v", c)
		}
	}
	if !sawBoundary {
		t.Fatal("expected L006 change")
	}
	if again := FormatWithOptions(r.Output, ERB, Options{Select: []RuleID{FileBoundaryBlankLines, BlankLineWhitespace, BlankLineRuns}}); len(again.Changes) != 0 || string(again.Output) != "RETURN 0\n" {
		t.Fatalf("not idempotent: %+v", again)
	}
}

func TestFileBoundaryBlankLinesEOLAndFinalNewline(t *testing.T) {
	r := FormatWithOptions([]byte("\r\n\r\n@F"), ERB, Options{Select: []RuleID{FileBoundaryBlankLines, FinalNewline}})
	if string(r.Output) != "@F\n" {
		t.Fatalf("default EOL did not use surviving terminator fallback: %q", r.Output)
	}

	r = FormatWithOptions([]byte("\r\n\r\n@F"), ERB, Options{Select: []RuleID{FileBoundaryBlankLines, FinalNewline}, EOL: CRLF})
	if string(r.Output) != "@F\r\n" {
		t.Fatalf("explicit EOL was ignored: %q", r.Output)
	}

	r = FormatWithOptions([]byte("\n\n"), ERB, Options{Select: []RuleID{FileBoundaryBlankLines}})
	if len(r.Output) != 0 || len(r.Changes) != 1 || r.Changes[0].Rule != FileBoundaryBlankLines {
		t.Fatalf("L006-only all-blank output=%q changes=%+v", r.Output, r.Changes)
	}
	for _, src := range []string{"\n\n", "\r\n\r\n  \r\n", " \t"} {
		for _, eol := range []EOL{DefaultEOL, LF, CRLF} {
			r = FormatWithOptions([]byte(src), ERB, Options{Select: []RuleID{FileBoundaryBlankLines, FinalNewline}, EOL: eol})
			if len(r.Output) != 0 || len(r.Changes) != 1 || r.Changes[0].Rule != FileBoundaryBlankLines {
				t.Fatalf("L006+F001 source=%q EOL=%v result=%+v", src, eol, r)
			}
		}
	}
	for _, options := range []Options{{}, {Select: []RuleID{FileBoundaryBlankLines, FinalNewline}}} {
		again := FormatWithOptions(r.Output, ERB, options)
		if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
			t.Fatalf("canonical blank file emitted changes: %+v", again)
		}
	}
	r = FormatWithOptions(nil, ERB, Options{Select: []RuleID{FileBoundaryBlankLines, FinalNewline}})
	if len(r.Output) != 0 || len(r.Changes) != 0 {
		t.Fatalf("empty input changed: output=%q changes=%+v", r.Output, r.Changes)
	}
}

func TestFileBoundaryBlankLinesProtectsOpaqueBoundaries(t *testing.T) {
	input := []byte("[SKIP]\n\n[SKIPEND]\nRETURN 0\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{FileBoundaryBlankLines}})
	if string(r.Output) != string(input) {
		t.Fatalf("SKIP boundary changed: %q", r.Output)
	}

	input = []byte("{\n\n}\nRETURN 0\n")
	r = FormatWithOptions(input, ERB, Options{Select: []RuleID{FileBoundaryBlankLines}})
	if string(r.Output) != string(input) {
		t.Fatalf("continuation boundary changed: %q", r.Output)
	}
}

func TestFileBoundaryBlankLinesRetainsContentBOM(t *testing.T) {
	for _, prefix := range []string{"\n", "\r", "\r\n", "\n \t\n"} {
		for _, body := range []string{"\ufeff", "\ufeff@F\nRETURN 0\n", " \ufeff", "\t\ufeff@F\nRETURN 0\n"} {
			for _, options := range []Options{{}, {EOL: CRLF}, {Select: []RuleID{FileBoundaryBlankLines}}, {Select: []RuleID{}}} {
				src := []byte(prefix + body)
				first := FormatWithOptions(src, ERB, options)
				second := FormatWithOptions(first.Output, ERB, options)
				if bytes.HasPrefix(first.Output, []byte("\ufeff")) || bytes.Count(first.Output, []byte("\ufeff")) != 1 {
					t.Fatalf("source=%q options=%+v output=%q", src, options, first.Output)
				}
				if hasFatalDiagnostic(first.Diagnostics) || hasFatalDiagnostic(second.Diagnostics) || !bytes.Equal(first.Output, second.Output) || len(second.Changes) != 0 {
					t.Fatalf("source=%q options=%+v first=%+v second=%+v", src, options, first, second)
				}
			}
		}
	}
}

func TestInstructionIndentRetainsLeadingContentBOM(t *testing.T) {
	for _, test := range []struct {
		name, src, want string
		options         Options
	}{
		{
			name: "assignment",
			src:  "\t\ufeffX+=1+2\n",
			want: "\t\ufeffX += 1 + 2\n",
			options: Options{Select: []RuleID{
				InstructionIndent, AssignmentLeftSpacing, AssignmentRightSpacing, BinarySpacing,
			}},
		},
		{
			name: "postfix increment",
			src:  " \ufeffX++\n",
			want: " \ufeffX++\n",
			options: Options{Select: []RuleID{
				InstructionIndent, AssignmentLeftSpacing, AssignmentRightSpacing, BinarySpacing,
			}},
		},
		{
			name: "leading blank with boundary rule",
			src:  "\n \ufeffX=1\n",
			want: " \ufeffX = 1\n",
			options: Options{Select: []RuleID{
				InstructionIndent, AssignmentLeftSpacing, AssignmentRightSpacing, BinarySpacing, FileBoundaryBlankLines,
			}},
		},
		{
			name: "leading blank without boundary rule",
			src:  "\n \ufeffX=1\n",
			want: "\n\ufeffX = 1\n",
			options: Options{Select: []RuleID{
				InstructionIndent, AssignmentLeftSpacing, AssignmentRightSpacing, BinarySpacing,
			}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			first := FormatWithOptions([]byte(test.src), ERB, test.options)
			second := FormatWithOptions(first.Output, ERB, test.options)
			if string(first.Output) != test.want || hasFatalDiagnostic(first.Diagnostics) {
				t.Fatalf("source=%q output=%q diagnostics=%+v", test.src, first.Output, first.Diagnostics)
			}
			if !bytes.Equal(first.Output, second.Output) || len(second.Changes) != 0 || hasFatalDiagnostic(second.Diagnostics) {
				t.Fatalf("source=%q first=%+v second=%+v", test.src, first, second)
			}
		})
	}
}

func TestInstructionIndentStillNormalizesInteriorContentBOM(t *testing.T) {
	src := []byte("A=1\n \ufeffB=2\n")
	options := Options{Select: []RuleID{InstructionIndent, AssignmentLeftSpacing, AssignmentRightSpacing, BinarySpacing, FileBoundaryBlankLines}}
	r := FormatWithOptions(src, ERB, options)
	if !bytes.HasPrefix(r.Output, []byte("A = 1\n\ufeffB = 2\n")) || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	if again := FormatWithOptions(r.Output, ERB, options); !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 || len(again.Diagnostics) != 0 {
		t.Fatalf("not idempotent: first=%q second=%+v", r.Output, again)
	}
}

func TestDeclarationBodySeparation(t *testing.T) {
	input := []byte("@F\n#DIM USR_X\nRETURN 0\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{DeclarationBodySeparation}})
	if string(r.Output) != "@F\n#DIM USR_X\n\nRETURN 0\n" || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	if len(r.Changes) != 1 || r.Changes[0].Rule != DeclarationBodySeparation || r.Changes[0].Line != 3 || r.Changes[0].Column != 1 {
		t.Fatalf("changes=%+v", r.Changes)
	}
	again := FormatWithOptions(r.Output, ERB, Options{Select: []RuleID{DeclarationBodySeparation}})
	if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
		t.Fatalf("not idempotent: %+v", again)
	}

	r = FormatWithOptions(input, ERB, Options{Select: []RuleID{DeclarationWordSpacing}})
	if string(r.Output) != string(input) || len(r.Changes) != 0 {
		t.Fatalf("disabled L007 changed input: %+v", r)
	}
	input = []byte("@F\n#DIMS USR_S\nS = raw  ; literal  \n")
	r = FormatWithOptions(input, ERB, Options{Select: []RuleID{DeclarationBodySeparation}})
	if string(r.Output) != "@F\n#DIMS USR_S\n\nS = raw  ; literal  \n" || len(r.Diagnostics) != 0 {
		t.Fatalf("plain assignment boundary output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestDeclarationBodySeparationBlankRunsAndComments(t *testing.T) {
	input := []byte("@F\n#FUNCTIONS\n#DIMS USR_S\n; declaration docs\nRETURN 0\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{DeclarationBodySeparation}})
	want := string(input)
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("comment attachment output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}

	input = []byte("@F\n#DIM USR_X\n\n\nRETURN 0\n")
	r = FormatWithOptions(input, ERB, Options{Select: []RuleID{DeclarationBodySeparation}})
	if string(r.Output) != string(input) || len(r.Changes) != 0 {
		t.Fatalf("L007 changed existing blank run: %+v", r)
	}
	r = FormatWithOptions(input, ERB, Options{Select: []RuleID{DeclarationBodySeparation, BlankLineRuns}})
	if string(r.Output) != "@F\n#DIM USR_X\n\nRETURN 0\n" || len(r.Changes) != 1 || r.Changes[0].Rule != BlankLineRuns {
		t.Fatalf("L005 ownership output=%q changes=%+v", r.Output, r.Changes)
	}
}

func TestDeclarationBodySeparationAllowsLeadingCommentsAndLabels(t *testing.T) {
	for _, input := range []string{
		"@F\n; declaration docs\n#DIM USR_X\nRETURN 0\n",
		"@F\n#DIM USR_X\n$LABEL\nRETURN 0\n",
	} {
		r := FormatWithOptions([]byte(input), ERB, Options{Select: []RuleID{DeclarationBodySeparation}})
		if len(r.Diagnostics) != 0 {
			t.Fatalf("input %q: diagnostics=%+v", input, r.Diagnostics)
		}
		want := strings.Replace(input, "#DIM USR_X\n", "#DIM USR_X\n\n", 1)
		if string(r.Output) != want || len(r.Changes) != 1 || r.Changes[0].Rule != DeclarationBodySeparation {
			t.Fatalf("input %q: output=%q changes=%+v", input, r.Output, r.Changes)
		}
		again := FormatWithOptions(r.Output, ERB, Options{Select: []RuleID{DeclarationBodySeparation}})
		if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
			t.Fatalf("input %q is not idempotent: %+v", input, again)
		}
	}
}

func TestDeclarationBodySeparationGuards(t *testing.T) {
	tests := []string{
		"@F\n#DIM [[ALIAS]]\nRETURN 0\n",
		"@123\n#DIM USR_X\nRETURN 0\n",
		"@F\n{\n#DIM USR_X\n}\nRETURN 0\n",
		"@F\n#DIM USR_X\n",
		"@F\n#FUNCTIONS\nRETURN 0\n",
		"@F\n#DIM USR_X\n; declaration docs\nRETURN 0\n",
		"@F\n#DIM USR_X\n;---\nRETURN 0\n",
		"#DIM USR_X\nRETURN 0\n",
	}
	for _, input := range tests {
		r := FormatWithOptions([]byte(input), ERB, Options{Select: []RuleID{DeclarationBodySeparation}})
		if string(r.Output) != input || len(r.Changes) != 0 {
			t.Fatalf("guard changed %q: output=%q changes=%+v diagnostics=%+v", input, r.Output, r.Changes, r.Diagnostics)
		}
	}

	r := FormatWithOptions([]byte("@F\r\n#DIM USR_X\r\nRETURN 0\r\n"), ERB, Options{Select: []RuleID{DeclarationBodySeparation}})
	if string(r.Output) != "@F\r\n#DIM USR_X\r\n\r\nRETURN 0\r\n" {
		t.Fatalf("EOL output=%q", r.Output)
	}
	if r = FormatWithOptions([]byte("@F\r\n#DIM USR_X\r\nRETURN 0\r\n"), ERB, Options{Select: []RuleID{DeclarationBodySeparation}, EOL: LF}); string(r.Output) != "@F\r\n#DIM USR_X\r\n\nRETURN 0\r\n" {
		t.Fatalf("explicit EOL output=%q", r.Output)
	}
}

func TestDeclarationBodyPayloadBoundaries(t *testing.T) {
	for _, body := range []string{
		"PRINTL  raw ; literal  ",
		"DEBUGPRINT raw  ",
		"PRINTFORM text { A+1 } ;literal",
		"PRINTV 1+2, 'raw ;literal  ",
		"RETURNFORM raw { A+1 }",
		"CALLFORM F{ A+1 }, 2+3",
		"INPUTS prompt { A+1 } ;literal",
	} {
		input := "@F\n#DIM USR_X\n" + body + "\n"
		options := Options{Select: []RuleID{DeclarationBodySeparation}}
		r := FormatWithOptions([]byte(input), ERB, options)
		want := "@F\n#DIM USR_X\n\n" + body + "\n"
		if string(r.Output) != want || len(r.Diagnostics) != 0 || len(r.Changes) != 1 || r.Changes[0].Rule != DeclarationBodySeparation {
			t.Fatalf("body %q: output %q, changes %+v, diagnostics %+v", body, r.Output, r.Changes, r.Diagnostics)
		}
		if again := FormatWithOptions(r.Output, ERB, options); !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
			t.Fatalf("body %q is not idempotent: %+v", body, again)
		}
	}
	for _, body := range []string{"FUTURECOMMAND 1+2", "PRINTVN 1+2", "PRINTFORM [[ALIAS]]", ";!;PRINTL raw"} {
		input := []byte("@F\n#DIM USR_X\n" + body + "\n")
		r := FormatWithOptions(input, ERB, Options{Select: []RuleID{DeclarationBodySeparation}})
		if !bytes.Equal(r.Output, input) || len(r.Changes) != 0 {
			t.Fatalf("guarded body %q changed: %+v", body, r)
		}
	}
}

func TestLayoutBatchedEdits(t *testing.T) {
	input := []byte("\r\n \t\r\n@A\r\n#DIM USR_X\r\nPRINTL raw  ;literal  \r\n \t\r\n\r\n  ; docs\r\n@B\r\n#DIM USR_Y\r\nRETURN 0\r\n\r\n \t")
	options := Options{Select: []RuleID{
		InstructionIndent, DeclarationIndent, CommentIndent, FunctionSeparation, DeclarationBodySeparation,
		BlankLineWhitespace, BlankLineRuns, FileBoundaryBlankLines, LineEndingNormalization, FinalNewline,
	}}
	r := FormatWithOptions(input, ERB, options)
	want := "@A\n    #DIM USR_X\n\n    PRINTL raw  ;literal  \n\n; docs\n@B\n    #DIM USR_Y\n\n    RETURN 0\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output %q, changes %+v, diagnostics %+v", r.Output, r.Changes, r.Diagnostics)
	}
	for _, change := range r.Changes {
		if change.Line <= 2 || change.Line >= 12 {
			if change.Rule != FileBoundaryBlankLines {
				t.Fatalf("deleted boundary retained an edit: %+v", change)
			}
		}
		if change.Line == 7 && change.Rule != FunctionSeparation {
			t.Fatalf("deleted function gap retained an edit: %+v", change)
		}
	}
	if again := FormatWithOptions(r.Output, ERB, options); !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
		t.Fatalf("not idempotent: %+v", again)
	}
}

func TestInlineCommentPrefixSpacingOwnership(t *testing.T) {
	input := []byte("@F\nRETURN 1;note\nRETURN 2;  note  \nRETURN 3;   \nPRINTFORM raw;note\nA=  raw;note\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{CommentPrefixSpacing}})
	want := "@F\nRETURN 1; note\nRETURN 2; note  \nRETURN 3;   \nPRINTFORM raw;note\nA=  raw;note\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("S015 output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	for _, c := range r.Changes {
		if c.Rule != CommentPrefixSpacing {
			t.Fatalf("unexpected rule change %+v", c)
		}
	}

	r = FormatWithOptions([]byte("@F\nRETURN 1;note  \n"), ERB, Options{Select: []RuleID{InlineCommentSpacing, CommentPrefixSpacing, CommentTrailingWhitespace}})
	if string(r.Output) != "@F\nRETURN 1 ; note\n" {
		t.Fatalf("combined comment output=%q changes=%+v", r.Output, r.Changes)
	}
	for _, c := range r.Changes {
		if c.Rule != InlineCommentSpacing && c.Rule != CommentPrefixSpacing && c.Rule != CommentTrailingWhitespace {
			t.Fatalf("unexpected ownership change %+v", c)
		}
	}
}

func TestBlankLineWhitespacePreservesMixedEOLBoundaries(t *testing.T) {
	for _, test := range []struct {
		src, want string
		options   Options
	}{
		{"@F\rRETURN 0\r \t\nRETURN 1\n", "@F\rRETURN 0\r \t\nRETURN 1\n", Options{Select: []RuleID{BlankLineWhitespace}}},
		{"@F\rRETURN 0\r \n@G\nRETURN 1\n", "@F\rRETURN 0\r \n@G\nRETURN 1\n", Options{Select: []RuleID{BlankLineWhitespace, FunctionSeparation}}},
		{"@F\r#DIM USR_X\r \nRETURN 0\n", "@F\r#DIM USR_X\r \nRETURN 0\n", Options{Select: []RuleID{BlankLineWhitespace, DeclarationBodySeparation}}},
		{"@F\rRETURN 0\r \n \t\nRETURN 1\n", "@F\rRETURN 0\r \nRETURN 1\n", Options{Select: []RuleID{BlankLineWhitespace, BlankLineRuns}}},
		{"@F\rRETURN 0\r \n", "@F\rRETURN 0\r", Options{Select: []RuleID{BlankLineWhitespace, FileBoundaryBlankLines}}},
		{"@F\rRETURN 0\r \nRETURN 1", "@F\rRETURN 0\r \nRETURN 1\n", Options{Select: []RuleID{BlankLineWhitespace, FinalNewline}}},
		{"@F\rRETURN 0\r　\t\nRETURN 1\n", "@F\rRETURN 0\r　\t\nRETURN 1\n", Options{Select: []RuleID{BlankLineWhitespace}, AllowFullwidthSpace: true}},
		{"@F\r#DIM USR_X\r \nRETURN 0\n", "@F\n#DIM USR_X\n\nRETURN 0\n", Options{Select: []RuleID{BlankLineWhitespace, DeclarationBodySeparation, LineEndingNormalization}}},
		{"@F\r#DIM USR_X\r \nRETURN 0\n", "@F\r\n#DIM USR_X\r\n\r\nRETURN 0\r\n", Options{Select: []RuleID{BlankLineWhitespace, DeclarationBodySeparation, LineEndingNormalization}, EOL: CRLF}},
	} {
		r := FormatWithOptions([]byte(test.src), ERB, test.options)
		if string(r.Output) != test.want || len(r.Diagnostics) != 0 {
			t.Fatalf("source=%q output=%q diagnostics=%+v", test.src, r.Output, r.Diagnostics)
		}
		again := FormatWithOptions(r.Output, ERB, test.options)
		if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 || len(again.Diagnostics) != 0 {
			t.Fatalf("unstable output=%q second=%+v", r.Output, again)
		}
	}
}

func TestInsertedBlankLinesAfterLoneCR(t *testing.T) {
	for _, test := range []struct {
		src, want string
		options   Options
	}{
		{"@A\rRETURN 1\r@B\rRETURN 2", "@A\rRETURN 1\r\n\n@B\rRETURN 2", Options{Select: []RuleID{FunctionSeparation}, EOL: LF}},
		{"@A\r#DIM USR_X,1\rRETURN 1", "@A\r#DIM USR_X,1\r\n\nRETURN 1", Options{Select: []RuleID{DeclarationBodySeparation}, EOL: LF}},
		{"@A\r\nRETURN 1\r\n@B", "@A\r\nRETURN 1\r\n\n@B", Options{Select: []RuleID{FunctionSeparation}, EOL: LF}},
		{"@A\nRETURN 1\n@B", "@A\nRETURN 1\n\n@B", Options{Select: []RuleID{FunctionSeparation}, EOL: LF}},
		{"@A\rRETURN 1\r@B", "@A\rRETURN 1\r\r\n@B", Options{Select: []RuleID{FunctionSeparation}, EOL: CRLF}},
		{"@A\rRETURN 1\r@B", "@A\nRETURN 1\n\n@B", Options{Select: []RuleID{FunctionSeparation, LineEndingNormalization}, EOL: LF}},
	} {
		r := FormatWithOptions([]byte(test.src), ERB, test.options)
		if string(r.Output) != test.want || len(r.Diagnostics) != 0 {
			t.Fatalf("source=%q output=%q diagnostics=%+v", test.src, r.Output, r.Diagnostics)
		}
		again := FormatWithOptions(r.Output, ERB, test.options)
		if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
			t.Fatalf("unstable output=%q second=%+v", r.Output, again)
		}
	}
}

func TestLayoutEOLAndFinalNewline(t *testing.T) {
	r := FormatWithOptions([]byte(" \t"), ERB, Options{})
	if len(r.Output) != 0 {
		t.Fatalf("blank final line output %q", r.Output)
	}
	r = FormatWithOptions([]byte("@F\r\n\r\n\nRETURN 0"), ERB, Options{Select: []RuleID{BlankLineRuns, FinalNewline}})
	if string(r.Output) != "@F\r\n\r\nRETURN 0\r\n" {
		t.Fatalf("surviving EOL output %q", r.Output)
	}
	input := []byte("@A\r\nRETURN 0\n")
	r = FormatWithOptions(input, ERB, Options{Select: []RuleID{LineEndingNormalization}, EOL: LF})
	if string(r.Output) != "@A\nRETURN 0\n" {
		t.Fatalf("LF output %q", r.Output)
	}
	for _, c := range r.Changes {
		if c.Rule != LineEndingNormalization || c.Line < 1 || c.Column < 1 {
			t.Fatalf("invalid EOL change %+v", c)
		}
	}

	r = FormatWithOptions([]byte("RETURN 0"), ERB, Options{Select: []RuleID{FinalNewline}, EOL: CRLF})
	if string(r.Output) != "RETURN 0\r\n" {
		t.Fatalf("CRLF final output %q", r.Output)
	}
	if len(r.Changes) != 1 || r.Changes[0].Rule != FinalNewline || r.Changes[0].Column != 9 {
		t.Fatalf("final newline change=%+v", r.Changes)
	}

	r = FormatWithOptions([]byte("RETURN 0\n\n   "), ERB, Options{Select: []RuleID{BlankLineRuns, FinalNewline}})
	if string(r.Output) != "RETURN 0\n\n" || len(r.Changes) != 1 || r.Changes[0].Rule != BlankLineRuns {
		t.Fatalf("blank EOF output=%q changes=%+v", r.Output, r.Changes)
	}
}

func TestLayoutDeletesContainedEdits(t *testing.T) {
	input := []byte("RETURN 0\r\n \t\r\n\r\nRETURN 1\r\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{BlankLineWhitespace, BlankLineRuns, LineEndingNormalization}, EOL: LF})
	if string(r.Output) != "RETURN 0\n\nRETURN 1\n" {
		t.Fatalf("output %q", r.Output)
	}
	for _, c := range r.Changes {
		if c.Rule == LineEndingNormalization && c.Line == 3 {
			t.Fatalf("deleted line retained an EOL change: %+v", c)
		}
	}
}

func TestBlankRunDeletionKeepsFollowingIndent(t *testing.T) {
	input := []byte("@F\n\n\nRETURN 0\n")
	r := FormatWithOptions(input, ERB, Options{})
	want := "@F\n\n    RETURN 0\n"
	if string(r.Output) != want {
		t.Fatalf("output %q, want %q; changes=%+v", r.Output, want, r.Changes)
	}
	again := FormatWithOptions(r.Output, ERB, Options{})
	if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
		t.Fatalf("not idempotent: %+v", again)
	}
}

func TestAttachedCommentsAndOpaqueFunction(t *testing.T) {
	input := []byte("@A\nRETURN 0\n  ; docs\n@B\nRETURN 1\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{CommentIndent, FunctionSeparation}})
	if string(r.Output) != "@A\nRETURN 0\n\n; docs\n@B\nRETURN 1\n" {
		t.Fatalf("attached comments output %q", r.Output)
	}

	input = []byte("@BAD\n[[alias]]\n@GOOD\nRETURN 1\n")
	r = FormatWithOptions(input, ERB, Options{Select: []RuleID{FunctionSeparation}})
	if string(r.Output) != string(input) {
		t.Fatalf("opaque predecessor got separated: %q", r.Output)
	}

	input = []byte("@A\nRETURN 0\n;note   \n;---   \n;tail   \n@B\nRETURN 1\n")
	r = FormatWithOptions(input, ERB, Options{Select: []RuleID{CommentIndent, CommentPrefixSpacing, CommentTrailingWhitespace}})
	if string(r.Output) != "@A\nRETURN 0\n;note   \n;---   \n;tail   \n@B\nRETURN 1\n" {
		t.Fatalf("decorative comment block changed: %q", r.Output)
	}
}

func TestFatalStructureSkipsGlobalLayout(t *testing.T) {
	input := []byte("@F\r\nIF A\r\n")
	r := FormatWithOptions(input, ERB, Options{})
	if !bytes.Equal(r.Output, input) || len(r.Changes) != 0 {
		t.Fatalf("fatal input changed: output=%q changes=%+v diagnostics=%+v", r.Output, r.Changes, r.Diagnostics)
	}
}

func TestCommentTableProtectsContiguousBlock(t *testing.T) {
	input := []byte("@F\n;Title  \n;a|b  \n;Footer  \n\n;plain  \n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{CommentIndent, CommentPrefixSpacing, CommentTrailingWhitespace}})
	want := "@F\n    ;Title  \n    ;a|b  \n    ;Footer  \n\n    ; plain\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output %q, diagnostics %+v", r.Output, r.Diagnostics)
	}
}

func TestFullwidthDecorativeCommentPrefixIsProtected(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"standalone", ";　----------------\n"},
		{"inline", "@F\nRETURN 1;　[IF USR_X]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := FormatWithOptions([]byte(tt.input), ERB, Options{Select: []RuleID{InlineCommentSpacing, CommentPrefixSpacing, CommentTrailingWhitespace}})
			if string(r.Output) != tt.input || len(r.Changes) != 0 || len(r.Diagnostics) != 0 {
				t.Fatalf("output=%q changes=%+v diagnostics=%+v", r.Output, r.Changes, r.Diagnostics)
			}
		})
	}
}

func TestFullwidthMarkerPrefixRemainsAnOrdinaryComment(t *testing.T) {
	for _, marker := range []string{"!;", "#;", "^;"} {
		input := ";　" + marker + "\n"
		r := FormatWithOptions([]byte(input), ERB, Options{Select: []RuleID{CommentPrefixSpacing}})
		if string(r.Output) != "; 　"+marker+"\n" || len(r.Changes) != 1 || r.Changes[0].Rule != CommentPrefixSpacing {
			t.Fatalf("input=%q output=%q changes=%+v diagnostics=%+v", input, r.Output, r.Changes, r.Diagnostics)
		}
	}
}
