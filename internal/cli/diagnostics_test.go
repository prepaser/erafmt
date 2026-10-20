package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestDiagnosticFormatJSONPreservesInvalidPathBytes(t *testing.T) {
	path := string([]byte{'d', 0xff, 'g'})
	var out bytes.Buffer
	reporter := diagnosticReporter{out: &out, format: diagnosticJSON}
	if err := reporter.diagnostic(path, format.Diagnostic{Severity: format.Error, Code: "io", Message: "failure"}); err != nil {
		t.Fatal(err)
	}
	var event struct {
		Path      string `json:"path"`
		PathBytes string `json:"path_bytes"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &event); err != nil {
		t.Fatal(err)
	}
	if event.Path != "d\ufffdg" {
		t.Fatalf("path = %q, want JSON replacement form", event.Path)
	}
	wantBytes := base64.StdEncoding.EncodeToString([]byte(path))
	if event.PathBytes != wantBytes {
		t.Fatalf("path_bytes = %q, want %q", event.PathBytes, wantBytes)
	}
}

func TestChangeFormatJSONPreservesInvalidPathBytes(t *testing.T) {
	path := string([]byte{'c', 0xfe, 'g'})
	var out bytes.Buffer
	reporter := diagnosticReporter{out: &out, format: diagnosticJSON}
	if err := reporter.change(path, format.Change{Rule: format.BinarySpacing, Line: 1, Column: 2}, "binary spacing"); err != nil {
		t.Fatal(err)
	}
	var event struct {
		Path      string `json:"path"`
		PathBytes string `json:"path_bytes"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &event); err != nil {
		t.Fatal(err)
	}
	if event.Path != "c\ufffdg" {
		t.Fatalf("path = %q, want JSON replacement form", event.Path)
	}
	wantBytes := base64.StdEncoding.EncodeToString([]byte(path))
	if event.PathBytes != wantBytes {
		t.Fatalf("path_bytes = %q, want %q", event.PathBytes, wantBytes)
	}
}

func decodeDiagnosticLines(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(data), []byte{'\n'})
	if len(lines) == 1 && len(lines[0]) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var event map[string]any
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatalf("invalid JSON diagnostic %q: %v", line, err)
		}
		result = append(result, event)
	}
	return result
}

func assertEventKeys(t *testing.T, event map[string]any, keys ...string) {
	t.Helper()
	want := make(map[string]bool, len(keys))
	for _, key := range keys {
		want[key] = true
	}
	for key := range event {
		if !want[key] {
			t.Errorf("unexpected JSON event key %q in %#v", key, event)
		}
	}
	for key := range want {
		if _, ok := event[key]; !ok {
			t.Errorf("missing JSON event key %q in %#v", key, event)
		}
	}
}

func TestDiagnosticFormatJSONUnsupportedStdin(t *testing.T) {
	var out, diagnostics bytes.Buffer
	code := Run([]string{"--diagnostic-format=JSON", "--check"}, strings.NewReader("@F\nFUTURECOMMAND A+B\n"), &out, &diagnostics)
	if code != 2 || out.Len() != 0 {
		t.Fatalf("code=%d stdout=%q diagnostics=%q", code, out.String(), diagnostics.String())
	}
	events := decodeDiagnosticLines(t, diagnostics.Bytes())
	if len(events) != 1 {
		t.Fatalf("events=%#v", events)
	}
	event := events[0]
	assertEventKeys(t, event, "type", "path", "line", "column", "severity", "code", "message")
	if event["type"] != "diagnostic" || event["path"] != "<stdin>" || event["severity"] != "unsupported" || event["code"] != "unsupported_command" {
		t.Fatalf("event=%#v", event)
	}
	if event["line"] != float64(2) || event["column"] != float64(1) {
		t.Fatalf("event location=%#v", event)
	}
}

func TestDiagnosticFormatJSONExplainsChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "explain.ERB")
	input := []byte("@F\nIF A+B\nENDIF\n")
	if err := os.WriteFile(path, input, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	code := Run([]string{"--diagnostic-format=json", "--explain", path}, strings.NewReader(""), &out, &diagnostics)
	if code != 0 || bytes.Equal(out.Bytes(), input) {
		t.Fatalf("code=%d stdout=%q diagnostics=%q", code, out.String(), diagnostics.String())
	}
	events := decodeDiagnosticLines(t, diagnostics.Bytes())
	if len(events) == 0 {
		t.Fatal("JSON explain produced no events")
	}
	for _, event := range events {
		assertEventKeys(t, event, "type", "path", "line", "column", "rule", "name")
		if event["type"] != "change" || event["path"] != path || event["rule"] == "" || event["name"] == "" {
			t.Fatalf("event=%#v", event)
		}
	}
}

func TestDiagnosticFormatJSONUsageAndFlagErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "usage", args: []string{"--diagnostic-format=json", "--eol=bad"}},
		{name: "rules", args: []string{"--diagnostic-format=json", "--rules", "--check"}},
		{name: "flag", args: []string{"--diagnostic-format=json", "--unknown"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, diagnostics bytes.Buffer
			if code := Run(tt.args, strings.NewReader(""), &out, &diagnostics); code != 2 || out.Len() != 0 {
				t.Fatalf("code=%d stdout=%q diagnostics=%q", code, out.String(), diagnostics.String())
			}
			events := decodeDiagnosticLines(t, diagnostics.Bytes())
			if len(events) != 1 {
				t.Fatalf("events=%#v", events)
			}
			assertEventKeys(t, events[0], "type", "path", "line", "column", "severity", "code", "message")
			if events[0]["type"] != "diagnostic" || events[0]["severity"] != "error" || events[0]["code"] != "usage" {
				t.Fatalf("event=%#v", events[0])
			}
		})
	}
}

func TestDiagnosticFormatJSONRulesKeepsCatalogText(t *testing.T) {
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--rules", "--diagnostic-format=JSON"}, unreadableInput{}, &out, &diagnostics); code != 0 {
		t.Fatalf("code=%d stdout=%q diagnostics=%q", code, out.String(), diagnostics.String())
	}
	if diagnostics.Len() != 0 || !bytes.Contains(out.Bytes(), []byte("I001\t")) {
		t.Fatalf("stdout=%q diagnostics=%q", out.String(), diagnostics.String())
	}
}

func TestDiagnosticFormatJSONRulesWriterFailure(t *testing.T) {
	var diagnostics bytes.Buffer
	if code := Run([]string{"--rules", "--diagnostic-format=json"}, unreadableInput{}, failedWriter{}, &diagnostics); code != 2 {
		t.Fatalf("code=%d diagnostics=%q", code, diagnostics.String())
	}
	events := decodeDiagnosticLines(t, diagnostics.Bytes())
	if len(events) != 1 || events[0]["path"] != "<stdout>" || events[0]["code"] != "io" {
		t.Fatalf("events=%#v", events)
	}
}

func TestDiagnosticFormatJSONCodecError(t *testing.T) {
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--diagnostic-format=json"}, bytes.NewReader([]byte{0xff, 0xfe, 0xfd}), &out, &diagnostics); code != 2 || out.Len() != 0 {
		t.Fatalf("code=%d stdout=%q diagnostics=%q", code, out.String(), diagnostics.String())
	}
	events := decodeDiagnosticLines(t, diagnostics.Bytes())
	if len(events) != 1 {
		t.Fatalf("events=%#v", events)
	}
	assertEventKeys(t, events[0], "type", "path", "line", "column", "severity", "code", "message")
	if events[0]["path"] != "<stdin>" || events[0]["line"] != float64(0) || events[0]["column"] != float64(0) {
		t.Fatalf("event=%#v", events[0])
	}
}

func TestDiagnosticFormatJSONWriterFailureDoesNotRecurse(t *testing.T) {
	var diagnostics bytes.Buffer
	if code := Run([]string{"--diagnostic-format=json", "--check"}, strings.NewReader("@F\nIF A+B\nENDIF\n"), failedWriter{}, &diagnostics); code != 2 {
		t.Fatalf("code=%d diagnostics=%q", code, diagnostics.String())
	}
	events := decodeDiagnosticLines(t, diagnostics.Bytes())
	if len(events) != 1 || events[0]["path"] != "<stdout>" || events[0]["code"] != "io" {
		t.Fatalf("events=%#v", events)
	}
}

func TestDiagnosticFormatJSONHelpRemainsHuman(t *testing.T) {
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--diagnostic-format=json", "-h"}, unreadableInput{}, &out, &diagnostics); code != 0 {
		t.Fatalf("code=%d diagnostics=%q", code, diagnostics.String())
	}
	if diagnostics.Len() == 0 || bytes.HasPrefix(bytes.TrimSpace(diagnostics.Bytes()), []byte("{")) {
		t.Fatalf("help was not human-readable: %q", diagnostics.String())
	}
	if !strings.Contains(diagnostics.String(), "opt into continuation analysis with this literal separator\n") {
		t.Fatalf("help omitted opt-in continuation behavior or advertised a default: %q", diagnostics.String())
	}
}
