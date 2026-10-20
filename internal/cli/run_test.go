package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestRunSingleFileAlwaysWritesSourceToStdout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unchanged.ERB")
	want := []byte("PRINTL plain text\n")
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if code := Run([]string{path}, strings.NewReader(""), &out, &diagnostics); code != 0 {
		t.Fatalf("Run() = %d, diagnostics %q", code, diagnostics.String())
	}
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatalf("stdout = %q, want %q", out.Bytes(), want)
	}
}

func TestRunCheckModes(t *testing.T) {
	changed := []byte("@F\nIF A+B\nENDIF\n")
	path := filepath.Join(t.TempDir(), "changed.ERB")
	if err := os.WriteFile(path, changed, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--check", path}, strings.NewReader(""), &out, &diagnostics); code != 1 {
		t.Fatalf("changed --check code = %d, diagnostics %q", code, diagnostics.String())
	}
	if out.String() != path+"\n" {
		t.Fatalf("changed --check output = %q", out.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, changed) {
		t.Fatalf("--check modified file: %q", got)
	}
	out.Reset()
	diagnostics.Reset()
	if code := Run([]string{"--stdin-kind=erb", path}, strings.NewReader(""), &out, &diagnostics); code != 2 {
		t.Fatalf("explicit --stdin-kind with file code = %d", code)
	}

	out.Reset()
	diagnostics.Reset()
	if code := Run([]string{"--check"}, strings.NewReader(string(changed)), &out, &diagnostics); code != 1 {
		t.Fatalf("stdin changed --check code = %d, diagnostics %q", code, diagnostics.String())
	}
	if out.String() != "<stdin>\n" {
		t.Fatalf("stdin changed --check output = %q", out.String())
	}

	out.Reset()
	diagnostics.Reset()
	want, formatDiagnostics := format.Format(changed, format.ERB)
	if len(formatDiagnostics) != 0 {
		t.Fatalf("test input diagnostics = %#v", formatDiagnostics)
	}
	if code := Run([]string{"--check"}, bytes.NewReader(want), &out, &diagnostics); code != 0 {
		t.Fatalf("stdin formatted --check code = %d, diagnostics %q", code, diagnostics.String())
	}
	if out.Len() != 0 {
		t.Fatalf("formatted stdin --check output = %q", out.String())
	}
}

func TestRunWriteModesAndUnsupportedFunctions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "write.ERB")
	input := []byte("@F\nIF A+B\nENDIF\n")
	if err := os.WriteFile(path, input, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"-w", path}, strings.NewReader(""), &out, &diagnostics); code != 0 {
		t.Fatalf("-w code = %d, diagnostics %q", code, diagnostics.String())
	}
	if out.Len() != 0 {
		t.Fatalf("-w stdout = %q", out.String())
	}
	want, formatDiagnostics := format.Format(input, format.ERB)
	if len(formatDiagnostics) != 0 {
		t.Fatalf("test input diagnostics = %#v", formatDiagnostics)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("-w file = %q, want %q", got, want)
	}

	path = filepath.Join(t.TempDir(), "write-list.ERB")
	if err := os.WriteFile(path, input, 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	diagnostics.Reset()
	if code := Run([]string{"-l", "-w", path}, strings.NewReader(""), &out, &diagnostics); code != 0 {
		t.Fatalf("-l -w code = %d, diagnostics %q", code, diagnostics.String())
	}
	if out.String() != path+"\n" {
		t.Fatalf("-l -w output = %q", out.String())
	}

	path = filepath.Join(t.TempDir(), "unsupported.ERB")
	input = []byte("@A\nIF B+C\nENDIF\n@B\n[[alias]]\n")
	if err := os.WriteFile(path, input, 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	diagnostics.Reset()
	if code := Run([]string{"-w", path}, strings.NewReader(""), &out, &diagnostics); code != 2 {
		t.Fatalf("unsupported -w code = %d, diagnostics %q", code, diagnostics.String())
	}
	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("IF B + C")) {
		t.Fatalf("safe function was not saved: %q", got)
	}
}

func TestRunErrorDoesNotWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "error.ERB")
	input := []byte("@F\nENDIF\n")
	if err := os.WriteFile(path, input, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"-w", path}, strings.NewReader(""), &out, &diagnostics); code != 2 {
		t.Fatalf("error -w code = %d, diagnostics %q", code, diagnostics.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, input) || out.Len() != 0 {
		t.Fatalf("error -w changed file or stdout: file=%q stdout=%q", got, out.String())
	}
}

func TestRunUnicodeUTF16AndDiscoveryBoundaries(t *testing.T) {
	root := t.TempDir()
	pack := filepath.Join(root, "#αβ")
	if err := os.Mkdir(pack, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(pack, "sample-αβ.ERB")
	text := []byte("@F\r\nIF A+B\r\nENDIF\r\n")
	encoded, err := encodeSource(sourceDocument{text: text, encoding: encodingUTF16LE, bom: []byte{0xFF, 0xFE}}, text)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	vcs := filepath.Join(root, ".git")
	if err := os.Mkdir(vcs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vcs, "ignored.ERB"), encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"-w", root}, strings.NewReader(""), &out, &diagnostics); code != 0 {
		t.Fatalf("UTF-16 directory write code = %d, diagnostics %q", code, diagnostics.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := decodeSource(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(doc.text, []byte("IF A + B")) {
		t.Fatalf("UTF-16 source was not formatted: %q", doc.text)
	}
	if _, err := os.Stat(filepath.Join(vcs, "ignored.ERB")); err != nil {
		t.Fatal(err)
	}

	if runtime.GOOS != "windows" {
		link := filepath.Join(root, "link.ERB")
		if err := os.Symlink(path, link); err != nil {
			t.Fatal(err)
		}
		out.Reset()
		diagnostics.Reset()
		if code := Run([]string{"-w", root}, strings.NewReader(""), &out, &diagnostics); code != 0 {
			t.Fatalf("symlink discovery code = %d, diagnostics %q", code, diagnostics.String())
		}
	}
}

func TestRunDeduplicatesAbsoluteAndRelativePaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dedup.ERB")
	input := []byte("@F\nIF A+B\nENDIF\n")
	if err := os.WriteFile(path, input, 0o644); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, path)
	if err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--check", relative, path}, strings.NewReader(""), &out, &diagnostics); code != 1 {
		t.Fatalf("dedup --check code = %d, diagnostics %q", code, diagnostics.String())
	}
	if out.String() != filepath.Clean(relative)+"\n" {
		t.Fatalf("dedup output = %q", out.String())
	}
}

func TestReplaceFileRejectsStateChangesAndHardlinks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "race.ERB")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := captureState(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(path, before, []byte("a\n"), []byte("b\n")); err == nil {
		t.Fatal("replaceFile accepted a changed source")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "changed\n" {
		t.Fatalf("state-change failure modified source: %q", got)
	}

	hardlink := filepath.Join(dir, "hardlink.ERB")
	if err := os.Link(path, hardlink); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("filesystem does not support hard links: %v", err)
		}
		t.Fatal(err)
	}
	state, err := captureState(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(path, state, []byte("changed\n"), []byte("new\n")); err == nil {
		t.Fatal("replaceFile accepted a hard link")
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }

func TestCheckOutputFailure(t *testing.T) {
	var diagnostics bytes.Buffer
	if code := Run([]string{"--check"}, strings.NewReader("@F\nRETURN 1\n"), failedWriter{}, &diagnostics); code != 2 {
		t.Fatalf("failed check output returned %d", code)
	}
}

func TestRunDirectoryCheckReportsReadErrors(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.ERB")
	bad := filepath.Join(dir, "bad.ERB")
	if err := os.WriteFile(good, []byte("@F\nIF A+B\nENDIF\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte{0xff, 0xfe, 0xfd}, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--check", dir}, strings.NewReader(""), &out, &diagnostics); code != 2 {
		t.Fatalf("directory error --check code = %d, diagnostics %q", code, diagnostics.String())
	}
	if !strings.Contains(out.String(), good) {
		t.Fatalf("valid changed file missing from check output: %q", out.String())
	}
	if diagnostics.Len() == 0 {
		t.Fatal("directory error produced no diagnostics")
	}
}

func TestReplaceFileRestoresPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not portable")
	}
	path := filepath.Join(t.TempDir(), "permissions.ERB")
	if err := os.WriteFile(path, []byte("a"), 0o640); err != nil {
		t.Fatal(err)
	}
	state, err := captureState(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(path, state, []byte("a"), []byte("b")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o, want 640", info.Mode().Perm())
	}
}

func TestReplaceFileRejectsPermissionChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.ERB")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := captureState(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(path, 0o644)
	if err := replaceFile(path, before, []byte("original"), []byte("formatted")); err == nil {
		t.Fatal("replacement overwrote a permission change")
	}
}

type unreadableInput struct{}

func (unreadableInput) Read([]byte) (int, error) { return 0, os.ErrClosed }

func TestRuleCatalogDoesNotReadInput(t *testing.T) {
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--rules"}, unreadableInput{}, &out, &diagnostics); code != 0 {
		t.Fatalf("--rules code = %d, diagnostics %q", code, diagnostics.String())
	}
	if out.Len() == 0 {
		t.Fatal("--rules produced an empty catalog")
	}
	if rules := format.Rules(); len(rules) == 0 || !strings.Contains(out.String(), string(rules[0].ID)) {
		t.Fatalf("--rules output does not contain the catalog: %q", out.String())
	}
	if code := Run([]string{"--rules", "--check"}, unreadableInput{}, &out, &diagnostics); code != 2 {
		t.Fatalf("combined --rules code = %d", code)
	}
}

func TestRuleOptionsValidateBeforeFilesystem(t *testing.T) {
	for _, arg := range []string{"--select=I001,,S001", "--select=unknown", "--ignore="} {
		var out, diagnostics bytes.Buffer
		if code := Run([]string{arg, "/path/that/does/not/exist.ERB"}, strings.NewReader(""), &out, &diagnostics); code != 2 {
			t.Fatalf("%s code = %d", arg, code)
		}
		if strings.Contains(diagnostics.String(), "does/not/exist") {
			t.Fatalf("%s touched the filesystem: %q", arg, diagnostics.String())
		}
	}
}

func TestEOLOptionValidatesBeforeInputAndRules(t *testing.T) {
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--eol=invalid"}, unreadableInput{}, &out, &diagnostics); code != 2 {
		t.Fatalf("invalid --eol code = %d", code)
	}
	if strings.Contains(diagnostics.String(), "closed") {
		t.Fatalf("invalid --eol read stdin: %q", diagnostics.String())
	}
	out.Reset()
	diagnostics.Reset()
	if code := Run([]string{"--eol=invalid", "/path/that/does/not/exist.ERB"}, strings.NewReader(""), &out, &diagnostics); code != 2 {
		t.Fatalf("invalid --eol with file code = %d", code)
	}
	if strings.Contains(diagnostics.String(), "does/not/exist") {
		t.Fatalf("invalid --eol touched the filesystem: %q", diagnostics.String())
	}
	out.Reset()
	diagnostics.Reset()
	if code := Run([]string{"--rules", "--eol=lf"}, unreadableInput{}, &out, &diagnostics); code != 2 {
		t.Fatalf("--rules --eol code = %d", code)
	}
	out.Reset()
	diagnostics.Reset()
	if code := Run([]string{"-h"}, unreadableInput{}, &out, &diagnostics); code != 0 || !strings.Contains(diagnostics.String(), "-eol") {
		t.Fatalf("help missing --eol: code=%d output=%q", code, diagnostics.String())
	}
}

func TestDialectOptionValidatesBeforeInputAndRules(t *testing.T) {
	for _, value := range []string{"invalid", ""} {
		var out, diagnostics bytes.Buffer
		if code := Run([]string{"--dialect=" + value}, unreadableInput{}, &out, &diagnostics); code != 2 {
			t.Fatalf("invalid --dialect=%q code = %d", value, code)
		}
		if strings.Contains(diagnostics.String(), "closed") {
			t.Fatalf("invalid --dialect=%q read stdin: %q", value, diagnostics.String())
		}
		out.Reset()
		diagnostics.Reset()
		if code := Run([]string{"--dialect=" + value, "/path/that/does/not/exist.ERB"}, strings.NewReader(""), &out, &diagnostics); code != 2 {
			t.Fatalf("invalid --dialect=%q with file code = %d", value, code)
		}
		if strings.Contains(diagnostics.String(), "does/not/exist") {
			t.Fatalf("invalid --dialect=%q touched the filesystem: %q", value, diagnostics.String())
		}
	}

	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--rules", "--dialect=common"}, unreadableInput{}, &out, &diagnostics); code != 2 {
		t.Fatalf("--rules --dialect code = %d", code)
	}
	out.Reset()
	diagnostics.Reset()
	if code := Run([]string{"-h"}, unreadableInput{}, &out, &diagnostics); code != 0 || !strings.Contains(diagnostics.String(), "-dialect") {
		t.Fatalf("help missing --dialect: code=%d output=%q", code, diagnostics.String())
	}
}

func TestDialectValueIsTrimmedAndDoesNotEnableRules(t *testing.T) {
	input := []byte("@F\nvalue= 1+2\n")
	want := format.FormatWithOptions(input, format.ERB, format.Options{Dialect: format.EMEE, Select: []format.RuleID{}})
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--dialect= EMEE ", "--select=none"}, bytes.NewReader(input), &out, &diagnostics); code != 0 {
		t.Fatalf("trimmed --dialect code = %d, diagnostics %q", code, diagnostics.String())
	}
	if !bytes.Equal(out.Bytes(), want.Output) {
		t.Fatalf("trimmed --dialect output = %q, want %q", out.Bytes(), want.Output)
	}
}

func TestSyntaxOptionsFlowThroughStdin(t *testing.T) {
	input := []byte("@F\nIF A　+　B\nENDIF\n")
	want := format.FormatWithOptions(input, format.ERB, format.Options{AllowFullwidthSpace: true})
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--allow-fullwidth-space"}, bytes.NewReader(input), &out, &diagnostics); code != 0 {
		t.Fatalf("--allow-fullwidth-space code = %d, diagnostics %q", code, diagnostics.String())
	}
	if !bytes.Equal(out.Bytes(), want.Output) {
		t.Fatalf("--allow-fullwidth-space output = %q, want %q", out.Bytes(), want.Output)
	}
	defaultOutput, _ := format.Format(input, format.ERB)
	if bytes.Equal(defaultOutput, want.Output) {
		t.Fatal("fullwidth-space option did not change the grammar-sensitive input")
	}

	input = []byte("@F\n{\nRETURN 1+2\n}\n")
	separator := ""
	want = format.FormatWithOptions(input, format.ERB, format.Options{ContinuationSeparator: &separator})
	out.Reset()
	diagnostics.Reset()
	if code := Run([]string{"--continuation-separator="}, bytes.NewReader(input), &out, &diagnostics); code != 0 {
		t.Fatalf("empty --continuation-separator code = %d, diagnostics %q", code, diagnostics.String())
	}
	if !bytes.Equal(out.Bytes(), want.Output) {
		t.Fatalf("empty --continuation-separator output = %q, want %q", out.Bytes(), want.Output)
	}
	if got := out.String(); got != "@F\n    {\nRETURN 1 + 2\n    }\n" {
		t.Fatalf("continuation syntax output = %q", got)
	}
}

func TestContinuationSeparatorValidatesBeforeInput(t *testing.T) {
	for _, value := range []string{"\n", "\r", "\x00", string([]byte{0xff})} {
		var out, diagnostics bytes.Buffer
		if code := Run([]string{"--continuation-separator=" + value}, unreadableInput{}, &out, &diagnostics); code != 2 {
			t.Fatalf("invalid continuation separator %q code = %d", value, code)
		}
		if strings.Contains(diagnostics.String(), "closed") {
			t.Fatalf("invalid continuation separator %q read stdin: %q", value, diagnostics.String())
		}
	}
}

func TestRulesRejectsExplicitOptions(t *testing.T) {
	for _, args := range [][]string{
		{"--rules", "-l=false"},
		{"--rules", "-w=false"},
		{"--rules", "--check=false"},
		{"--rules", "--utf8=false"},
		{"--rules", "--explain=false"},
		{"--rules", "--check", "--check=false"},
		{"--rules", "--allow-fullwidth-space=false"},
		{"--rules", "--continuation-separator="},
	} {
		var out, diagnostics bytes.Buffer
		if code := Run(args, unreadableInput{}, &out, &diagnostics); code != 2 || out.Len() != 0 || !strings.Contains(diagnostics.String(), "cannot be combined") {
			t.Fatalf("%v code = %d, diagnostics %q", args, code, diagnostics.String())
		}
	}
}

func TestEOLFlowsThroughStdinAndSelection(t *testing.T) {
	input := []byte("@F\r\nIF A+B\nENDIF\r\n")
	all := format.FormatWithOptions(input, format.ERB, format.Options{EOL: format.LF})
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--eol=lf"}, bytes.NewReader(input), &out, &diagnostics); code != 0 {
		t.Fatalf("--eol=lf code = %d, diagnostics %q", code, diagnostics.String())
	}
	if !bytes.Equal(out.Bytes(), all.Output) {
		t.Fatalf("--eol=lf output = %q, want %q", out.Bytes(), all.Output)
	}
	if bytes.Contains(out.Bytes(), []byte("\r\n")) {
		t.Fatalf("--eol=lf retained CRLF: %q", out.Bytes())
	}

	out.Reset()
	diagnostics.Reset()
	noRules := format.FormatWithOptions(input, format.ERB, format.Options{Select: []format.RuleID{}, EOL: format.LF})
	if code := Run([]string{"--select=none", "--eol=lf"}, bytes.NewReader(input), &out, &diagnostics); code != 0 {
		t.Fatalf("--select=none --eol=lf code = %d, diagnostics %q", code, diagnostics.String())
	}
	if !bytes.Equal(out.Bytes(), noRules.Output) {
		t.Fatalf("style flag enabled a rule: %q, want %q", out.Bytes(), noRules.Output)
	}

	var selected format.RuleID
	for _, change := range all.Changes {
		selected = change.Rule
		break
	}
	if selected == "" {
		t.Fatal("test input produced no selected rule change")
	}
	out.Reset()
	diagnostics.Reset()
	want := format.FormatWithOptions(input, format.ERB, format.Options{Select: []format.RuleID{selected}, EOL: format.CRLF})
	if code := Run([]string{"--select=" + string(selected), "--eol=crlf"}, bytes.NewReader(input), &out, &diagnostics); code != 0 {
		t.Fatalf("selected --eol=crlf code = %d, diagnostics %q", code, diagnostics.String())
	}
	if !bytes.Equal(out.Bytes(), want.Output) {
		t.Fatalf("selected --eol=crlf output = %q, want %q", out.Bytes(), want.Output)
	}
}

func TestRuleSelectionIgnoreAndExplain(t *testing.T) {
	input := []byte("@F\nIF A+B\nENDIF\n")
	all := format.FormatWithOptions(input, format.ERB, format.Options{})
	if len(all.Changes) == 0 {
		t.Fatal("test input produced no rule changes")
	}
	id := all.Changes[0].Rule
	name := string(id)
	for _, rule := range format.Rules() {
		if rule.ID == id {
			name = rule.Name
			break
		}
	}
	path := filepath.Join(t.TempDir(), "rules.ERB")
	if err := os.WriteFile(path, input, 0o644); err != nil {
		t.Fatal(err)
	}

	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--select=" + string(id), path}, strings.NewReader(""), &out, &diagnostics); code != 0 {
		t.Fatalf("selected rule code = %d, diagnostics %q", code, diagnostics.String())
	}
	selected := format.FormatWithOptions(input, format.ERB, format.Options{Select: []format.RuleID{id}})
	if !bytes.Equal(out.Bytes(), selected.Output) {
		t.Fatalf("selected output = %q, want %q", out.Bytes(), selected.Output)
	}

	out.Reset()
	diagnostics.Reset()
	if code := Run([]string{"--ignore=" + string(id), path}, strings.NewReader(""), &out, &diagnostics); code != 0 {
		t.Fatalf("ignored rule code = %d, diagnostics %q", code, diagnostics.String())
	}
	ignored := format.FormatWithOptions(input, format.ERB, format.Options{Ignore: []format.RuleID{id}})
	if !bytes.Equal(out.Bytes(), ignored.Output) {
		t.Fatalf("ignored output = %q, want %q", out.Bytes(), ignored.Output)
	}

	out.Reset()
	diagnostics.Reset()
	if code := Run([]string{"--explain", path}, strings.NewReader(""), &out, &diagnostics); code != 0 {
		t.Fatalf("explain code = %d, diagnostics %q", code, diagnostics.String())
	}
	if !bytes.Equal(out.Bytes(), all.Output) {
		t.Fatalf("explain polluted stdout: %q, want %q", out.Bytes(), all.Output)
	}
	explanation := diagnostics.String()
	if !strings.Contains(explanation, path+":") || !strings.Contains(explanation, "rule/"+string(id)+": "+name) {
		t.Fatalf("explanation = %q", explanation)
	}
	if got := strings.Count(explanation, "rule/"); got != len(all.Changes) {
		t.Fatalf("explanation lines = %d, want %d: %q", got, len(all.Changes), explanation)
	}

	out.Reset()
	diagnostics.Reset()
	if code := Run([]string{"--select=" + string(id)}, bytes.NewReader(input), &out, &diagnostics); code != 0 {
		t.Fatalf("stdin selected rule code = %d, diagnostics %q", code, diagnostics.String())
	}
	if !bytes.Equal(out.Bytes(), selected.Output) {
		t.Fatalf("stdin selected output = %q, want %q", out.Bytes(), selected.Output)
	}
}
