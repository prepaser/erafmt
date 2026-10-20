package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestRunWriteMixedEOLIsStable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.ERB")
	if err := os.WriteFile(path, []byte("@F\r#DIM USR_X\r \nRETURN 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"-w", "--ignore=F002", path}, unreadableInput{}, &out, &diagnostics); code != 0 || out.Len() != 0 || diagnostics.Len() != 0 {
		t.Fatalf("write code=%d stdout=%q diagnostics=%q", code, out.String(), diagnostics.String())
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "@F\r    #DIM USR_X\r \n    RETURN 0\n" {
		t.Fatalf("written source=%q err=%v", got, err)
	}
	if code := Run([]string{"--check", "--ignore=F002", path}, unreadableInput{}, &out, &diagnostics); code != 0 || out.Len() != 0 || diagnostics.Len() != 0 {
		t.Fatalf("check code=%d stdout=%q diagnostics=%q", code, out.String(), diagnostics.String())
	}
}

func TestRunCheckReportsUnsupportedCommands(t *testing.T) {
	for _, command := range []string{"FUTURECOMMAND A+B", "PRINTVN A+B", "PRINTN raw"} {
		input := []byte("@F\n    " + command + "\n")
		var out, diagnostics bytes.Buffer
		if code := Run([]string{"--check"}, bytes.NewReader(input), &out, &diagnostics); code != 2 || out.Len() != 0 || !bytes.Contains(diagnostics.Bytes(), []byte("unsupported/unsupported_command")) {
			t.Fatalf("command=%q code=%d stdout=%q diagnostics=%q", command, code, out.String(), diagnostics.String())
		}
		out.Reset()
		diagnostics.Reset()
		if code := Run([]string{"--select=none", "--check"}, bytes.NewReader(input), &out, &diagnostics); code != 0 || out.Len() != 0 || diagnostics.Len() != 0 {
			t.Fatalf("no selected rules: code=%d stdout=%q diagnostics=%q", code, out.String(), diagnostics.String())
		}
	}
}

func TestRunCheckReportsInvalidCallTail(t *testing.T) {
	for _, dialect := range []string{"common", "emuera1824", "emee"} {
		for _, tail := range []string{",2", "(2)", " 2"} {
			var out, diagnostics bytes.Buffer
			source := "@F\n    CALL TARGET(1 + 2)" + tail + "\n"
			code := Run([]string{"--check", "--dialect=" + dialect, "--diagnostic-format=json"}, strings.NewReader(source), &out, &diagnostics)
			if code != 2 || out.Len() != 0 {
				t.Fatalf("dialect=%s tail=%q code=%d stdout=%q diagnostics=%q", dialect, tail, code, out.String(), diagnostics.String())
			}
			events := decodeDiagnosticLines(t, diagnostics.Bytes())
			if len(events) != 1 || events[0]["code"] != "unsupported_syntax" || events[0]["line"] != float64(2) {
				t.Fatalf("dialect=%s tail=%q events=%v", dialect, tail, events)
			}
		}
	}
}

func TestRunResourceLimitDoesNotConvertOrWrite(t *testing.T) {
	var source strings.Builder
	source.WriteString("@F\n")
	for i := 0; i < 513; i++ {
		source.WriteString("IF 1\n")
	}
	text := []byte(source.String())
	input, err := encodeSource(sourceDocument{encoding: encodingUTF16LE, bom: []byte{0xff, 0xfe}}, text)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "deep.ERB")
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--utf8", "-w", path}, strings.NewReader(""), &out, &diagnostics); code != 2 || !bytes.Contains(diagnostics.Bytes(), []byte("error/resource_limit")) {
		t.Fatalf("code=%d stdout=%q diagnostics=%q", code, out.String(), diagnostics.String())
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, input) {
		t.Fatalf("resource limit changed encoded input: bytes=%x err=%v", got, err)
	}
	result := format.FormatWithOptions(text, format.ERB, format.Options{})
	if !bytes.Equal(result.Output, text) || len(result.Changes) != 0 {
		t.Fatalf("core resource limit changed input: %+v", result)
	}
}

func TestLoopControlWarningsDoNotChangeExitCode(t *testing.T) {
	var out, diagnostics bytes.Buffer
	code := Run([]string{"--check"}, strings.NewReader("@F\n    BREAK\n"), &out, &diagnostics)
	if code != 0 || out.Len() != 0 || !bytes.Contains(diagnostics.Bytes(), []byte("warning/invalid_loop_control")) {
		t.Fatalf("code=%d stdout=%q diagnostics=%q", code, out.String(), diagnostics.String())
	}
}

func TestDimensionWarningDoesNotRejectMacroSensitiveSource(t *testing.T) {
	var out, diagnostics bytes.Buffer
	source := "@F\n#DIM EFAUDIT_DECL,0\n"
	code := Run([]string{"--check", "--select=S005"}, strings.NewReader(source), &out, &diagnostics)
	if code != 0 || out.Len() != 0 || !strings.Contains(diagnostics.String(), "warning/suspicious_dimension") {
		t.Fatalf("code=%d output=%q diagnostics=%q", code, out.String(), diagnostics.String())
	}
}

func TestNewWarningsDoNotChangeExitCode(t *testing.T) {
	for _, test := range []struct {
		source, code string
		selectRules  string
	}{
		{"@F\nIF 1\nELSE\nELSEIF 2\nENDIF\n", "branch_after_final", "none"},
		{"@F\nREPEAT 2\nREPEAT 1\nREND\nREND\n", "nested_repeat", "none"},
		{"@F\n#LOCALSIZE 0\n", "suspicious_local_size", "S005"},
	} {
		var out, diagnostics bytes.Buffer
		code := Run([]string{"--check", "--select=" + test.selectRules}, strings.NewReader(test.source), &out, &diagnostics)
		if code != 0 || out.Len() != 0 || !strings.Contains(diagnostics.String(), "warning/"+test.code) {
			t.Fatalf("warning=%s code=%d output=%q diagnostics=%q", test.code, code, out.String(), diagnostics.String())
		}
	}
}

func TestRunMalformedHeaderDoesNotConvertOrWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.ERB")
	text := []byte("@\nRETURN 0\n")
	input, err := encodeSource(sourceDocument{encoding: encodingUTF16LE, bom: []byte{0xFF, 0xFE}}, text)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, input, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--utf8", "--select=none", "-w", path}, bytes.NewReader(nil), &out, &diagnostics); code != 2 || !bytes.Contains(diagnostics.Bytes(), []byte("error/invalid_header")) {
		t.Fatalf("code=%d diagnostics=%q", code, diagnostics.String())
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, input) || out.Len() != 0 {
		t.Fatalf("malformed header write: bytes=%x stdout=%q err=%v", got, out.String(), err)
	}
}
