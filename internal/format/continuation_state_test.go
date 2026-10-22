package format

import (
	"bytes"
	"maps"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestContinuationProxyKeepsParentNames(t *testing.T) {
	for _, source := range []string{"#DIM UserNew,1", "$LabelNew", "#PRI"} {
		p := formatter{
			DocumentSession: erabasic.DocumentSession{
				Kind:            erabasic.ERB,
				Base:            1,
				State:           erabasic.State{Declarations: true},
				FunctionContext: erabasic.FunctionContext{PrivateDeclarations: map[string]bool{"UserOld": true}, DollarLabels: map[string]bool{"LabelOld": true}},
			},
			rules: ruleSet{CommaSpacing: true, LabelMarkerSpacing: true},
		}
		if source == "#PRI" {
			p.FunctionName = "EVENTTRAIN"
			p.EventFunctionKnown, p.EventFunctionExact = true, true
			p.SharpAttributes = erabasic.SharpAttributeSingle
		}
		declarations := maps.Clone(p.PrivateDeclarations)
		labels := maps.Clone(p.DollarLabels)
		attributes := p.SharpAttributes
		proxy := p.continuationProxy([]byte(source))
		continuationProxySyntax(proxy)
		if len(proxy.diagnostics) != 0 || len(proxy.Opaque) != 1 || proxy.Opaque[0] {
			t.Fatalf("proxy did not prove %q: %+v", source, proxy.diagnostics)
		}
		if !maps.Equal(p.PrivateDeclarations, declarations) || !maps.Equal(p.DollarLabels, labels) {
			t.Fatalf("uncommitted proxy changed parent names for %q", source)
		}
		if p.SharpAttributes != attributes {
			t.Fatalf("uncommitted proxy changed parent attributes for %q", source)
		}
	}
}

func TestContinuedLabelState(t *testing.T) {
	separator := " "
	options := Options{ContinuationSeparator: &separator}
	for _, tc := range []struct {
		src  string
		line int
	}{
		{"@F\n{\n$L\n}\n$L\nRETURN 0\n", 5},
		{"@F\n$L\n{\n$L\n}\nRETURN 0\n", 4},
		{"@F\n{\n$L\n}\n{\n$L\n}\nRETURN 0\n", 6},
		{"@F\n{\n$L\n}\n$M\nRETURN 0\n", 0},
		{"@F\n{\n$L [[ALIAS]]\n}\n$L\nRETURN 0\n", 0},
		{"@F\n{\n;#;$L\n}\n$L\nRETURN 0\n", 0},
		{"@F\n[IF X]\n{\n$L\n}\n[ENDIF]\n$L\nRETURN 0\n", 0},
		{"@F\n{\n$L +\n}\n$L\nRETURN 0\n", 0},
	} {
		r := FormatWithOptions([]byte(tc.src), ERB, options)
		line := 0
		for _, d := range r.Diagnostics {
			if d.Code == "duplicate_label" {
				line = d.Line
			}
		}
		if line != tc.line {
			t.Fatalf("source=%q diagnostics=%+v want duplicate at %d", tc.src, r.Diagnostics, tc.line)
		}
		again := FormatWithOptions(r.Output, ERB, options)
		if !bytes.Equal(r.Output, again.Output) || len(again.Changes) != 0 {
			t.Fatalf("source=%q not idempotent: %+v", tc.src, again)
		}
	}
}

func TestContinuedFullwidthState(t *testing.T) {
	separator := " "
	options := Options{AllowFullwidthSpace: true, ContinuationSeparator: &separator}
	for _, tc := range []struct {
		head string
		code string
	}{{"$L", "duplicate_label"}, {"#DIM USR_X", "duplicate_declaration"}} {
		src := "@F\n{\n　" + tc.head + "\n}\n" + tc.head + "\nRETURN 0\n"
		r := FormatWithOptions([]byte(src), ERB, options)
		line := 0
		for _, d := range r.Diagnostics {
			if d.Code == tc.code {
				line = d.Line
			}
		}
		if line != 5 {
			t.Fatalf("head=%q diagnostics=%+v want duplicate at 5", tc.head, r.Diagnostics)
		}
		if again := FormatWithOptions(r.Output, ERB, options); !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
			t.Fatalf("head=%q not idempotent: %+v", tc.head, again)
		}
	}
}

func TestContinuedFunctionSeparation(t *testing.T) {
	separator := " "
	options := Options{Select: []RuleID{FunctionSeparation}, ContinuationSeparator: &separator}
	for _, tc := range []struct {
		src  string
		want string
	}{
		{"@A\nRETURN 0\n{\n@B\n}\nRETURN 0\n", "@A\nRETURN 0\n\n{\n@B\n}\nRETURN 0\n"},
		{"@A\nRETURN 0\n; docs\n{\n@B\n}\nRETURN 0\n", "@A\nRETURN 0\n\n; docs\n{\n@B\n}\nRETURN 0\n"},
		{"@A\nRETURN 0\n{\n@B(ARG:0,\nARGS:0)\n}\nRETURN 0\n", "@A\nRETURN 0\n\n{\n@B(ARG:0,\nARGS:0)\n}\nRETURN 0\n"},
		{"@A\nRETURN 0\n{\n@B\n}\nRETURN 0\n@C\nRETURN 0\n", "@A\nRETURN 0\n\n{\n@B\n}\nRETURN 0\n\n@C\nRETURN 0\n"},
	} {
		r := FormatWithOptions([]byte(tc.src), ERB, options)
		if string(r.Output) != tc.want || len(r.Diagnostics) != 0 {
			t.Fatalf("source=%q output=%q diagnostics=%+v", tc.src, r.Output, r.Diagnostics)
		}
		if again := FormatWithOptions(r.Output, ERB, options); !bytes.Equal(again.Output, r.Output) || len(again.Changes) != 0 {
			t.Fatalf("not idempotent: %+v", again)
		}
	}
	options.Select = append(options.Select, BinarySpacing)
	for _, head := range []string{"@B(1)", ";!;@B", "@B[[ALIAS]]", "@B\n@C"} {
		src := "@A\nRETURN 0\n{\n" + head + "\n}\nRETURN 0\n"
		if r := FormatWithOptions([]byte(src), ERB, options); strings.Contains(string(r.Output), "RETURN 0\n\n{") {
			t.Fatalf("untrusted header=%q became a layout boundary: %+v", head, r)
		}
	}
	options.ContinuationSeparator = nil
	src := []byte("@A\nRETURN 0\n{\n@B\n}\nRETURN 0\n")
	if r := FormatWithOptions(src, ERB, options); !bytes.Equal(r.Output, src) {
		t.Fatalf("opaque continuation changed: %+v", r)
	}
}
