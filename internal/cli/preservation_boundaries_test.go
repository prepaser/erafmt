package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestSourcePreservationRejectsNonGrammarUnicodeBoundaryRemoval(t *testing.T) {
	tests := []struct {
		name, original, output string
		dialect                format.Dialect
	}{
		{name: "blank-fullwidth", original: "@F\n　\n", output: "@F\n\n"},
		{name: "blank-nbsp", original: "@F\n\u00a0\n", output: "@F\n\n"},
		{name: "close-fullwidth", original: "@F\n{\nRETURN 1\n　}　\n", output: "@F\n{\nRETURN 1\n}\n"},
		{name: "varsize-fullwidth", original: "@F\nVARSIZE 　NAME\n", output: "@F\nVARSIZE NAME\n"},
		{name: "data-fullwidth", original: "@F\nPRINTDATA 　\nENDDATA\n", output: "@F\nPRINTDATA\nENDDATA\n"},
		{name: "times-fullwidth", original: "@F\nTIMES A,　1\n", output: "@F\nTIMES A,1\n"},
		{name: "dt-fullwidth", original: "@F\nDT_COLUMN_OPTIONS \"db\",　\"col\",DEFAULT,1\n", output: "@F\nDT_COLUMN_OPTIONS \"db\",\"col\",DEFAULT,1\n", dialect: format.EMEE},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkSourcePreservedOptions([]byte(tt.original), []byte(tt.output), sourcePreservationOptions{dialect: tt.dialect}); err == nil {
				t.Fatal("non-grammar Unicode removal passed preservation audit")
			}
		})
	}
}

func TestSourcePreservationAcceptsFullwidthGrammarFormatting(t *testing.T) {
	separator := " "
	tests := []struct {
		name, source string
		kind         format.Kind
		dialect      format.Dialect
		continuation bool
	}{
		{name: "blank", source: "@F\n　\n", kind: format.ERB},
		{name: "close", source: "@F\n{\nRETURN 1+2\n　}\n", kind: format.ERB, continuation: true},
		{name: "varsize", source: "@F\nVARSIZE 　NAME\n", kind: format.ERB},
		{name: "data", source: "@F\nPRINTDATA 　\nENDDATA\n", kind: format.ERB},
		{name: "times", source: "@F\nTIMES A,　1\n", kind: format.ERB},
		{name: "dt", source: "@F\nDT_COLUMN_OPTIONS \"db\",　\"col\",DEFAULT,1\n", kind: format.ERB, dialect: format.EMEE},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := format.Options{Dialect: tt.dialect, AllowFullwidthSpace: true}
			verification := sourcePreservationOptions{dialect: tt.dialect, allowFullwidthSpace: true}
			if tt.continuation {
				options.ContinuationSeparator = &separator
				verification.continuationSeparator = &separator
			}
			result := format.FormatWithOptions([]byte(tt.source), tt.kind, options)
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == format.Error {
					t.Fatalf("unexpected fatal diagnostic: %+v", diagnostic)
				}
			}
			if err := checkSourcePreservedOptions([]byte(tt.source), result.Output, verification); err != nil {
				t.Fatalf("output=%q: %v", result.Output, err)
			}
		})
	}
	if err := checkSourcePreservedOptions([]byte("@F\n\u00a0\n"), []byte("@F\n\n"), sourcePreservationOptions{allowFullwidthSpace: true}); err == nil {
		t.Fatal("non-grammar NBSP removal passed fullwidth-enabled audit")
	}
}

func TestSourcePreservationAcceptsFullwidthFormAnyFormatting(t *testing.T) {
	source := []byte("@F\nSETBGIMAGE　\"x　literal\",{ A+1 }, raw　text;keep　literal\nREMOVEBGIMAGE　name,{ C+3 }, \"literal　text\"\n")
	result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true})
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity == format.Error {
			t.Fatalf("unexpected fatal diagnostic: %+v\noutput=%q", diagnostic, result.Output)
		}
	}
	if bytes.Contains(result.Output, []byte("SETBGIMAGE　")) || bytes.Contains(result.Output, []byte("REMOVEBGIMAGE　")) {
		t.Fatalf("formatter retained a command gap: %q", result.Output)
	}
	for _, literal := range []string{"x　literal", "raw　text;keep　literal", "literal　text"} {
		if !bytes.Contains(result.Output, []byte(literal)) {
			t.Fatalf("formatter changed FORM literal %q: %q", literal, result.Output)
		}
	}
	options := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true}
	if err := checkSourcePreservedOptions(source, result.Output, options); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
	changed := bytes.Replace(result.Output, []byte("x　literal"), []byte("changed　literal"), 1)
	if err := checkSourcePreservedOptions(source, changed, options); err == nil {
		t.Fatal("FORM literal mutation passed preservation audit")
	}
	changed = bytes.Replace(result.Output, []byte("keep　literal"), []byte("changed　literal"), 1)
	if err := checkSourcePreservedOptions(source, changed, options); err == nil {
		t.Fatal("FORM semicolon literal mutation passed preservation audit")
	}
}

func TestSourcePreservationAcceptsContinuedFullwidthFormAnyFormatting(t *testing.T) {
	separator := " "
	source := []byte("@F\n{\nSETBGIMAGE　\"x　literal\",{ A+\n1 }, raw　text;keep　literal\n}\n@G\n{\nREMOVEBGIMAGE　name,{ C+\n3 }, \"literal　text\"\n}\n")
	result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true, ContinuationSeparator: &separator})
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity == format.Error {
			t.Fatalf("unexpected fatal diagnostic: %+v\noutput=%q", diagnostic, result.Output)
		}
	}
	if bytes.Contains(result.Output, []byte("SETBGIMAGE　")) || bytes.Contains(result.Output, []byte("REMOVEBGIMAGE　")) {
		t.Fatalf("formatter retained a continued command gap: %q", result.Output)
	}
	for _, literal := range []string{"x　literal", "raw　text;keep　literal", "literal　text"} {
		if !bytes.Contains(result.Output, []byte(literal)) {
			t.Fatalf("formatter changed continued FORM literal %q: %q", literal, result.Output)
		}
	}
	options := sourcePreservationOptions{dialect: format.EMEE, allowFullwidthSpace: true, continuationSeparator: &separator}
	if err := checkSourcePreservedOptions(source, result.Output, options); err != nil {
		t.Fatalf("output=%q: %v", result.Output, err)
	}
	changed := bytes.Replace(result.Output, []byte("x　literal"), []byte("changed　literal"), 1)
	if err := checkSourcePreservedOptions(source, changed, options); err == nil {
		t.Fatal("continued FORM literal mutation passed preservation audit")
	}
	changed = bytes.Replace(result.Output, []byte("keep　literal"), []byte("changed　literal"), 1)
	if err := checkSourcePreservedOptions(source, changed, options); err == nil {
		t.Fatal("continued FORM semicolon literal mutation passed preservation audit")
	}
	rowChanged := bytes.Replace(result.Output, []byte("raw　text"), []byte("raw\ntext"), 1)
	if err := checkSourcePreservedOptions(source, rowChanged, options); err == nil {
		t.Fatal("continued FORM row-boundary mutation passed preservation audit")
	}
}

func TestFileStdoutFailureAttribution(t *testing.T) {
	for _, mode := range []string{"unchanged-source", "changed-source", "path-list"} {
		t.Run(mode, func(t *testing.T) {
			source := []byte("@F\n    RETURN 1\n")
			if mode != "unchanged-source" {
				source = []byte("@F\nRETURN 1+2\n")
			}
			path := filepath.Join(t.TempDir(), "sample.ERB")
			if err := os.WriteFile(path, source, 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"--diagnostic-format=json"}
			if mode == "path-list" {
				args = append(args, "--check")
			}
			args = append(args, path)
			var diagnostics bytes.Buffer
			if code := Run(args, strings.NewReader(""), failedWriter{}, &diagnostics); code != 2 {
				t.Fatalf("code=%d diagnostics=%q", code, diagnostics.String())
			}
			var event diagnosticEvent
			if err := json.Unmarshal(diagnostics.Bytes(), &event); err != nil {
				t.Fatal(err)
			}
			if event.Path != "<stdout>" || event.Code != "io" {
				t.Fatalf("stdout failure attributed to %q: %+v", event.Path, event)
			}
		})
	}
}

func TestSourcePreservationDTColumnOptionsTerminalComma(t *testing.T) {
	options := sourcePreservationOptions{dialect: format.EMEE}
	for _, suffix := range []string{",", ", ; kept", ", DEFAULT, 3+4,"} {
		src := []byte("@F\nDT_COLUMN_OPTIONS \"db\",\"col\",DEFAULT,1+2" + suffix + "\n")
		result := format.FormatWithOptions(src, format.ERB, format.Options{Dialect: format.EMEE})
		if err := checkSourcePreservedOptions(src, result.Output, options); err != nil {
			t.Fatalf("output=%q: %v", result.Output, err)
		}
		changed := bytes.Replace(result.Output, []byte("1 + 2"), []byte("1 + 3"), 1)
		if err := checkSourcePreservedOptions(src, changed, options); err == nil {
			t.Fatal("changed option value passed preservation audit")
		}
	}
}

func TestSourcePreservationKeepsUnicodeContinuationDelimiters(t *testing.T) {
	separator := " "
	for _, trivia := range []string{"\u00a0", "\u3000"} {
		for _, allow := range []bool{false, true} {
			for _, join := range []*string{nil, &separator} {
				src := []byte("@F\n{" + trivia + "\nRETURN 1+2\n" + trivia + "}" + trivia + "\nRETURN 3+4\n")
				options := sourcePreservationOptions{allowFullwidthSpace: allow, continuationSeparator: join}
				result := format.FormatWithOptions(src, format.ERB, format.Options{AllowFullwidthSpace: allow, ContinuationSeparator: join})
				if err := checkSourcePreservedOptions(src, result.Output, options); err != nil {
					t.Fatalf("allow=%t trivia=%q joined=%t output=%q: %v", allow, trivia, join != nil, result.Output, err)
				}
				if !allow || trivia != "\u3000" {
					changed := bytes.ReplaceAll(result.Output, []byte(trivia), nil)
					if err := checkSourcePreservedOptions(src, changed, options); err == nil {
						t.Fatal("Unicode delimiter payload removal passed preservation audit")
					}
				}
			}
		}
	}
}
