package format

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestLabelMarkerSpacing(t *testing.T) {
	input := []byte("@  F\n$\tLOOP\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{LabelMarkerSpacing}})
	if string(r.Output) != "@F\n$LOOP\n" || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	for _, c := range r.Changes {
		if c.Rule != LabelMarkerSpacing {
			t.Fatalf("unexpected change %+v", c)
		}
	}
	again := FormatWithOptions(r.Output, ERB, Options{Select: []RuleID{LabelMarkerSpacing}})
	if !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
		t.Fatalf("not idempotent: %+v", again)
	}
}

func TestLabelMarkerSpacingOwnsOnlyMarkerGap(t *testing.T) {
	input := []byte("@  F ( X )\n$  LOOP ; note\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{LabelMarkerSpacing}})
	want := "@F ( X )\n$LOOP ; note\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	for _, c := range r.Changes {
		if c.Rule != LabelMarkerSpacing {
			t.Fatalf("unexpected change %+v", c)
		}
	}
}

func TestLabelTailsUseExistingLayoutRules(t *testing.T) {
	input := []byte("$LOOP   ; note   \n$NEXT   \n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{InlineCommentSpacing, CommentTrailingWhitespace, CodeTrailingWhitespace}})
	want := "$LOOP ; note\n$NEXT\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
}

func TestLabelMarkerSpacingPreservesUnprovenLabels(t *testing.T) {
	input := []byte("@  [[F]]\n$  LOOP ARG\n@  F+BAD\n@　F\n@  123\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{LabelMarkerSpacing}})
	if !bytes.Equal(r.Output, input) {
		t.Fatalf("unproven label changed: %q", r.Output)
	}
}

func TestLabelMarkerSpacingKeepsIdentifierAndLiteralText(t *testing.T) {
	input := []byte("@  F\u0301(ARGS:0=\"　\")\n$  L\u0301\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{LabelMarkerSpacing}})
	want := "@F\u0301(ARGS:0=\"　\")\n$L\u0301\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output %q, diagnostics %+v", r.Output, r.Diagnostics)
	}
}

func TestLabelMarkerSpacingAcceptsRuntimeUnicodeDigitNames(t *testing.T) {
	input := []byte("@ １２３\n@\t١ABC(ARG:0)\n$ １２３\n$ 123\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{LabelMarkerSpacing}})
	want := "@１２３\n@١ABC(ARG:0)\n$１２３\n$ 123\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output %q, diagnostics %+v", r.Output, r.Diagnostics)
	}
}

func TestMarkerOnlyHeaderTailRemainsTrusted(t *testing.T) {
	input := []byte("@F;!;\nRETURN 1\n@G;#;\nRETURN 2\n@H;^;\nRETURN 3\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{FunctionIndent, FunctionSeparation}})
	want := "@F;!;\nRETURN 1\n\n@G;#;\nRETURN 2\n\n@H;^;\nRETURN 3\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output %q, diagnostics %+v", r.Output, r.Diagnostics)
	}
}

func TestBareSharpFunctionsOwnMetadataTails(t *testing.T) {
	input := []byte("@F\n#FUNCTION   ; note   \n#FUNCTIONS  \n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{CodeTrailingWhitespace, InlineCommentSpacing, CommentTrailingWhitespace}})
	want := "@F\n#FUNCTION ; note\n#FUNCTIONS\n"
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	for _, c := range r.Changes {
		if c.Rule != CodeTrailingWhitespace && c.Rule != InlineCommentSpacing && c.Rule != CommentTrailingWhitespace {
			t.Fatalf("unexpected change %+v", c)
		}
	}
}

func TestSharpFunctionPayloadAndERHStayOpaque(t *testing.T) {
	for _, kind := range []erabasic.Kind{erabasic.ERB, erabasic.ERH} {
		input := []byte("#FUNCTION  value   ; note   \n")
		r := FormatWithOptions(input, Kind(kind), Options{Select: []RuleID{CodeTrailingWhitespace, InlineCommentSpacing, CommentTrailingWhitespace}})
		if !bytes.Equal(r.Output, input) {
			t.Fatalf("kind=%v changed opaque declaration: %q", kind, r.Output)
		}
	}
}
