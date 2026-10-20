package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/prepaser/erafmt/internal/format"
)

type diagnosticFormat uint8

const (
	diagnosticText diagnosticFormat = iota
	diagnosticJSON
)

func parseDiagnosticFormat(value string) (diagnosticFormat, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "text":
		return diagnosticText, nil
	case "json":
		return diagnosticJSON, nil
	default:
		return diagnosticText, fmt.Errorf("--diagnostic-format must be text or json")
	}
}

type diagnosticReporter struct {
	out     io.Writer
	format  diagnosticFormat
	encoder *json.Encoder
}

var diagnosticRuleNames = func() map[format.RuleID]string {
	rules := format.Rules()
	names := make(map[format.RuleID]string, len(rules))
	for _, rule := range rules {
		names[rule.ID] = rule.Name
	}
	return names
}()

type diagnosticEvent struct {
	Type      string `json:"type"`
	Path      string `json:"path"`
	PathBytes string `json:"path_bytes,omitempty"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Severity  string `json:"severity"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

type changeEvent struct {
	Type      string `json:"type"`
	Path      string `json:"path"`
	PathBytes string `json:"path_bytes,omitempty"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Rule      string `json:"rule"`
	Name      string `json:"name"`
}

func jsonPathBytes(path string) string {
	if utf8.ValidString(path) {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(path))
}

func (r *diagnosticReporter) diagnostic(path string, diagnostic format.Diagnostic) error {
	if path == "" {
		path = "<unknown>"
	}
	severity := string(diagnostic.Severity)
	if severity == "" {
		severity = string(format.Error)
	}
	if r.format == diagnosticJSON {
		return r.writeJSON(diagnosticEvent{
			Type:      "diagnostic",
			Path:      path,
			PathBytes: jsonPathBytes(path),
			Line:      diagnostic.Line,
			Column:    diagnostic.Column,
			Severity:  severity,
			Code:      diagnostic.Code,
			Message:   diagnostic.Message,
		})
	}
	location := "0:0"
	if diagnostic.Line > 0 || diagnostic.Column > 0 {
		location = fmt.Sprintf("%d:%d", diagnostic.Line, diagnostic.Column)
	}
	_, err := fmt.Fprintf(r.out, "%s:%s %s/%s: %s\n", path, location, severity, diagnostic.Code, diagnostic.Message)
	return err
}

func (r *diagnosticReporter) usage(message string) error {
	if r.format == diagnosticJSON {
		return r.diagnostic("", format.Diagnostic{Severity: format.Error, Code: "usage", Message: message})
	}
	_, err := fmt.Fprintf(r.out, "erafmt: %s\n", message)
	return err
}

func (r *diagnosticReporter) rawFlagError(message string) error {
	if r.format == diagnosticJSON {
		return r.diagnostic("", format.Diagnostic{Severity: format.Error, Code: "usage", Message: strings.TrimSpace(message)})
	}
	_, err := io.WriteString(r.out, message)
	return err
}

func (r *diagnosticReporter) change(path string, change format.Change, name string) error {
	if r.format == diagnosticJSON {
		return r.writeJSON(changeEvent{
			Type:      "change",
			Path:      path,
			PathBytes: jsonPathBytes(path),
			Line:      change.Line,
			Column:    change.Column,
			Rule:      string(change.Rule),
			Name:      name,
		})
	}
	_, err := fmt.Fprintf(r.out, "%s:%d:%d rule/%s: %s\n", path, change.Line, change.Column, change.Rule, name)
	return err
}

func (r *diagnosticReporter) writeJSON(value any) error {
	if r.encoder == nil {
		r.encoder = json.NewEncoder(r.out)
	}
	return r.encoder.Encode(value)
}
