package format

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestContinuationHeaderSemantics(t *testing.T) {
	separator := " "
	src := []byte("  {\n@αβ(ARG:0,\nARGS:0=1)\n  }\nRETURN 1+2\n")
	r := FormatWithOptions(src, ERB, Options{ContinuationSeparator: &separator})
	want := []byte("  {\n@αβ(ARG:0,\nARGS:0=1)\n  }\n    RETURN 1 + 2\n")
	if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_syntax" || r.Diagnostics[0].Line != 3 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	if again := FormatWithOptions(r.Output, ERB, Options{ContinuationSeparator: &separator}); !bytes.Equal(again.Output, r.Output) {
		t.Fatalf("unstable: %q", again.Output)
	}
	if r := FormatWithOptions(src, ERB, Options{Select: []RuleID{InstructionIndent}, ContinuationSeparator: &separator}); len(r.Diagnostics) != 0 {
		t.Fatalf("selection: %+v", r.Diagnostics)
	}
	duplicate := []byte("{\n@αβ(ARG:0,\nARG:0)\n}\nRETURN 0\n")
	r = FormatWithOptions(duplicate, ERB, Options{ContinuationSeparator: &separator})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "duplicate_parameter" || r.Diagnostics[0].Line != 3 || r.Diagnostics[0].Column != 1 {
		t.Fatalf("duplicate mapping: %+v", r.Diagnostics)
	}
	separator = ""
	r = FormatWithOptions([]byte("{\n\t@αβ(ARG:0=\"x\",\nARG:1=0)\n}\n"), ERB, Options{ContinuationSeparator: &separator})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_syntax" || r.Diagnostics[0].Line != 2 || r.Diagnostics[0].Column != 12 {
		t.Fatalf("Unicode mapping: %+v", r.Diagnostics)
	}
}

func TestContinuationOpenerAllowsRuntimeTrailingWhitespace(t *testing.T) {
	for _, pad := range []string{"\u00a0", "\u0085", "\u2003", "\u3000", " \t\u3000 "} {
		src := []byte("@F\n\t{" + pad + "\nRETURN 1+2\n}\n")
		for _, options := range []Options{{Select: []RuleID{ContinuationIndent}}, {Select: []RuleID{}}} {
			r := FormatWithOptions(src, ERB, options)
			want := src
			if len(options.Select) != 0 {
				want = []byte("@F\n    {" + pad + "\nRETURN 1+2\n    }\n")
			}
			if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
				t.Fatalf("pad=%q output=%q diagnostics=%v", pad, r.Output, r.Diagnostics)
			}
			again := FormatWithOptions(r.Output, ERB, options)
			if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
				t.Fatalf("pad=%q is not idempotent", pad)
			}
		}
	}
}

func TestContinuationOpenerTrailingWhitespaceKeepsGrammarBoundaries(t *testing.T) {
	for _, opener := range []string{"\u00a0{", "{\u00a0code", "{\u00a0; comment"} {
		src := []byte("@F\n" + opener + "\nRETURN 0\n}\n")
		r := FormatWithOptions(src, ERB, Options{})
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) == 0 {
			t.Fatalf("opener=%q output=%q diagnostics=%v", opener, r.Output, r.Diagnostics)
		}
	}
}

func TestContinuationSourceUsesConfiguredSeparator(t *testing.T) {
	separator := "|"
	src := []byte("@F\n{\n  IF A>0\n  RETURN 1\n}\n")
	p := formatter{DocumentSession: erabasic.DocumentSession{Src: src, Lines: linesOf(src), ContinuationSeparator: &separator}}
	input, ok := p.collectContinuation(1)
	if !ok {
		t.Fatal("continuation not collected")
	}
	logical, segments := input.Logical, input.Segments
	if string(logical) != "  IF A>0|  RETURN 1|" {
		t.Fatalf("logical=%q", logical)
	}
	if len(segments) != 2 || segments[0].LogicalStart != 0 || segments[0].LogicalEnd != 8 || segments[1].LogicalStart != 9 || segments[1].LogicalEnd != 19 {
		t.Fatalf("segments=%+v", segments)
	}
}

func TestContinuationMappedEditsKeepPhysicalRows(t *testing.T) {
	segments := []erabasic.ContinuationSegment{{LogicalStart: 0, LogicalEnd: 8, Line: 1, SourceStart: 10, SourceEnd: 18, Indent: 2}, {LogicalStart: 9, LogicalEnd: 18, Line: 2, SourceStart: 20, SourceEnd: 29, Indent: 2}}
	edits := []edit{
		{start: 2, end: 3, text: []byte("X"), change: Change{Rule: BinarySpacing}},
		{start: 7, end: 10, text: []byte("Y"), change: Change{Rule: BinarySpacing}},
		{start: 12, end: 13, text: []byte("Z"), change: Change{Rule: BinarySpacing}},
	}
	got := continuationMappedEdits(edits, segments)
	if len(got) != 2 || got[0].start != 2 || got[0].end != 3 || got[1].start != 12 || got[1].end != 13 {
		t.Fatalf("mapped=%+v", got)
	}
}

func TestContinuationAddSortedMappedEditsMapsRowsAndBoundaries(t *testing.T) {
	src := []byte("prefix\nab\n\ncdef\nsuffix")
	lines := linesOf(src)
	segments := []erabasic.ContinuationSegment{
		{LogicalStart: 0, LogicalEnd: 2, Line: 1, SourceStart: lines[1].Start, SourceEnd: lines[1].BodyEnd},
		{LogicalStart: 3, LogicalEnd: 3, Line: 2, SourceStart: lines[2].Start, SourceEnd: lines[2].BodyEnd},
		{LogicalStart: 4, LogicalEnd: 8, Line: 3, SourceStart: lines[3].Start, SourceEnd: lines[3].BodyEnd},
	}
	edits := []edit{
		{start: 0, end: 1, text: []byte("Y"), change: Change{Rule: BinarySpacing}},
		{start: 2, end: 2, text: []byte("!"), change: Change{Rule: BinarySpacing}},
		{start: 5, end: 6, text: []byte("X"), change: Change{Rule: BinarySpacing}},
		{start: 8, end: 8, text: []byte("?"), change: Change{Rule: BinarySpacing}},
	}
	wantInput := append([]edit(nil), edits...)
	for i := range wantInput {
		wantInput[i].text = bytes.Clone(wantInput[i].text)
	}
	p := formatter{DocumentSession: erabasic.DocumentSession{Src: src, Lines: lines, Metadata: erabasic.NewMetadataBudget(0)}, rules: ruleSet{BinarySpacing: true}}
	continuationAddSortedMappedEdits(&p, edits, segments)
	if !reflect.DeepEqual(edits, wantInput) {
		t.Fatalf("input edits mutated: got=%+v want=%+v", edits, wantInput)
	}
	if len(p.edits) != 4 {
		t.Fatalf("mapped edits=%+v, want four edits", p.edits)
	}
	want := []edit{
		{start: lines[1].Start, end: lines[1].Start + 1, text: []byte("Y"), change: Change{Rule: BinarySpacing, Line: 2, Column: 1}},
		{start: lines[1].BodyEnd, end: lines[1].BodyEnd, text: []byte("!"), change: Change{Rule: BinarySpacing, Line: 2, Column: 3}},
		{start: lines[3].Start + 1, end: lines[3].Start + 2, text: []byte("X"), change: Change{Rule: BinarySpacing, Line: 4, Column: 2}},
		{start: lines[3].BodyEnd, end: lines[3].BodyEnd, text: []byte("?"), change: Change{Rule: BinarySpacing, Line: 4, Column: 5}},
	}
	if !reflect.DeepEqual(p.edits, want) {
		t.Fatalf("mapped edits=%+v want=%+v", p.edits, want)
	}
}

func TestContinuationAddSortedMappedEditsPreservesNearBudgetAbort(t *testing.T) {
	src := []byte("abc")
	lines := linesOf(src)
	segments := []erabasic.ContinuationSegment{{LogicalStart: 0, LogicalEnd: 3, Line: 0, SourceStart: lines[0].Start, SourceEnd: lines[0].BodyEnd}}
	edits := []edit{{start: 1, end: 1, text: []byte("!"), change: Change{Rule: BinarySpacing}}}
	p := formatter{DocumentSession: erabasic.DocumentSession{Src: src, Lines: lines, Metadata: erabasic.NewMetadataBudget(erabasic.MaxMetadataBytes - 1)}, rules: ruleSet{BinarySpacing: true}}
	continuationAddSortedMappedEdits(&p, edits, segments)
	if !p.Aborted || len(p.edits) != 0 || len(p.diagnostics) != 1 || p.diagnostics[0].Code != "resource_limit" || p.diagnostics[0].Severity != Error {
		t.Fatalf("aborted=%t edits=%+v diagnostics=%+v metadata=%d", p.Aborted, p.edits, p.diagnostics, p.Metadata.Used())
	}
}

func TestContinuationProxyFormatsOnlyExpressionSyntax(t *testing.T) {
	rules, err := resolveRules(Options{})
	if err != nil {
		t.Fatal(err)
	}
	separator := " "
	p := formatter{DocumentSession: erabasic.DocumentSession{Kind: erabasic.ERB, State: erabasic.State{Declarations: false}, Dialect: erabasic.CommonDialect, ContinuationSeparator: &separator}, rules: rules}
	src := []byte("IF A>0")
	q := p.continuationProxy(src)
	continuationProxySyntax(q)
	got := applyEdits(src, q.edits)
	if !bytes.Equal(got, []byte("IF A > 0")) {
		t.Fatalf("got=%q diagnostics=%+v", got, q.diagnostics)
	}
}

func TestContinuedDeclarationTracksFunctionState(t *testing.T) {
	separator := ""
	src := []byte("@F\n{\n#DIM UserVar,1+\n2\n}\n#DIM UserVar\nRETURN 0\n")
	options := Options{Select: []RuleID{BinarySpacing}, ContinuationSeparator: &separator}
	r := FormatWithOptions(src, ERB, options)
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "duplicate_declaration" || !bytes.Contains(r.Output, []byte("#DIM UserVar,1 +\n2")) {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	again := FormatWithOptions(r.Output, ERB, options)
	if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
		t.Fatalf("not idempotent: %+v", again)
	}
}

func TestContinuationSyntaxMapsEditsToPhysicalRow(t *testing.T) {
	separator := " "
	r := FormatWithOptions([]byte("@F\n{\nIF A>\n0\n}\nENDIF\n"), ERB, Options{Select: []RuleID{BinarySpacing, ContinuationIndent}, ContinuationSeparator: &separator})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", r.Diagnostics)
	}
	want := "@F\n    {\nIF A >\n0\n    }\nENDIF\n"
	if string(r.Output) != want {
		t.Fatalf("got=%q want=%q changes=%+v", r.Output, want, r.Changes)
	}
	again := FormatWithOptions(r.Output, ERB, Options{Select: []RuleID{BinarySpacing, ContinuationIndent}, ContinuationSeparator: &separator})
	if string(again.Output) != want || len(again.Diagnostics) != 0 {
		t.Fatalf("not idempotent: output=%q diagnostics=%+v", again.Output, again.Diagnostics)
	}
}

func TestContinuationPreservesRawPayload(t *testing.T) {
	separator := " "
	src := []byte("@F\n{\nBEGIN  raw  {A+1};!; keep\n}\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{ContinuationIndent}, ContinuationSeparator: &separator})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", r.Diagnostics)
	}
	want := "@F\n    {\nBEGIN  raw  {A+1};!; keep\n    }\n"
	if string(r.Output) != want {
		t.Fatalf("got=%q want=%q", r.Output, want)
	}
}

func TestContinuationCustomSeparatorsKeepPhysicalRows(t *testing.T) {
	for _, separator := range []string{"", "+"} {
		t.Run(separator, func(t *testing.T) {
			src := []byte("@F\n{\nBEGIN \nraw\n}\n")
			r := FormatWithOptions(src, ERB, Options{Select: []RuleID{ContinuationIndent}, ContinuationSeparator: &separator})
			if len(r.Diagnostics) != 0 {
				t.Fatalf("separator=%q diagnostics=%+v", separator, r.Diagnostics)
			}
			want := "@F\n    {\nBEGIN \nraw\n    }\n"
			if string(r.Output) != want {
				t.Fatalf("separator=%q got=%q want=%q", separator, r.Output, want)
			}
		})
	}
}

func TestContinuationDisabledExpressionRulesLeaveInteriorUntouched(t *testing.T) {
	separator := " "
	src := []byte("@F\n{\nIF A>\n0\n}\nENDIF\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{ContinuationIndent}, ContinuationSeparator: &separator})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", r.Diagnostics)
	}
	want := "@F\n    {\nIF A>\n0\n    }\nENDIF\n"
	if string(r.Output) != want {
		t.Fatalf("got=%q want=%q", r.Output, want)
	}
}

func TestContinuationSpacingPreservesUnindentedPhysicalRows(t *testing.T) {
	separator := ""
	for _, body := range []string{"RETURN 1+\n2", "RETURN 1,\n2", `PRINTFORM {1+` + "\n2}", `RETURNF \@` + "\n1?yes#no\\@"} {
		src := []byte("@F\n{\n" + body + "\n}\n")
		r := FormatWithOptions(src, ERB, Options{ContinuationSeparator: &separator})
		before, after := bytes.Split(src, []byte{'\n'}), bytes.Split(r.Output, []byte{'\n'})
		if len(before) != len(after) || len(r.Diagnostics) != 0 {
			t.Fatalf("%q: %+v", body, r)
		}
		for i := 2; i < len(before)-2; i++ {
			if len(after[i]) == 0 || erabasic.HalfSpace(after[i][0]) {
				t.Fatalf("row indentation changed: %q -> %q", before[i], after[i])
			}
		}
		again := FormatWithOptions(r.Output, ERB, Options{ContinuationSeparator: &separator})
		if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
			t.Fatalf("unstable continuation: %+v", again)
		}
	}
}

func TestContinuationCaretCommentsAllowProvenSyntax(t *testing.T) {
	separator := " "
	src := []byte("@F\n{\nRETURN 1+2;^; tail\n}\n")
	want := []byte("@F\n    {\nRETURN 1 + 2;^; tail\n    }\n")
	for _, dialect := range []erabasic.Dialect{erabasic.CommonDialect, erabasic.Emuera1824, erabasic.EMEE} {
		r := FormatWithOptions(src, ERB, Options{Dialect: Dialect(dialect), ContinuationSeparator: &separator})
		if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
			t.Fatalf("dialect=%v: %+v", dialect, r)
		}
	}
	bad := []byte("@F\n{\nRETURNF @\"{1+2;^;bad}\"\n}\n")
	r := FormatWithOptions(bad, ERB, Options{ContinuationSeparator: &separator})
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_expression" || r.Diagnostics[0].Line != 3 {
		t.Fatalf("caret diagnostic: %+v", r)
	}
}

func TestContinuationSeparatorValidationInCore(t *testing.T) {
	src := []byte("@F\nRETURN 0\n")
	for _, separator := range []string{"\xff", "\r", "\n", "\x00"} {
		r := FormatWithOptions(src, ERB, Options{ContinuationSeparator: &separator})
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "invalid_continuation_separator" || r.Diagnostics[0].Severity != Error {
			t.Fatalf("separator=%q result=%+v", separator, r)
		}
	}
}

func TestContinuationFormPayloadSpansRows(t *testing.T) {
	separator := " "
	src := []byte("@F\n{\nPRINTFORM  hello {A+1}\nraw  text\n}\nRETURN 0\n")
	r := FormatWithOptions(src, ERB, Options{Select: []RuleID{BinarySpacing, FormPadding, ContinuationIndent}, ContinuationSeparator: &separator})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v output=%q", r.Diagnostics, r.Output)
	}
	if !bytes.Contains(r.Output, []byte("PRINTFORM  hello {A + 1}")) || !bytes.Contains(r.Output, []byte("raw  text")) {
		t.Fatalf("output=%q", r.Output)
	}
}

func TestContinuationMalformedFormReportsPhysicalRow(t *testing.T) {
	separator := " "
	r := FormatWithOptions([]byte("@F\n{\nPRINTFORM  hello {A+1\nraw  text\n}\nRETURN 0\n"), ERB, Options{Select: []RuleID{BinarySpacing, FormPadding, ContinuationIndent}, ContinuationSeparator: &separator})
	want := "@F\n    {\nPRINTFORM  hello {A+1\nraw  text\n    }\nRETURN 0\n"
	if string(r.Output) != want {
		t.Fatalf("output=%q want=%q", r.Output, want)
	}
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_expression" || r.Diagnostics[0].Line != 3 || r.Diagnostics[0].Column != 19 {
		t.Fatalf("diagnostics=%+v", r.Diagnostics)
	}
}
