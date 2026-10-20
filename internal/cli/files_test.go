package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestDiscoveryPinsInitialState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.ERB")
	if err := os.WriteFile(path, []byte("@F\nRETURN 1+2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	targets, errs := discoverTargets([]string{path})
	if len(errs) != 0 || len(targets) != 1 {
		t.Fatalf("discoverTargets() = %#v, %v", targets, errs)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readSource(targets[0].path, &targets[0].state); err == nil || !strings.Contains(err.Error(), "after discovery") {
		t.Fatalf("readSource() error = %v, want discovery state mismatch", err)
	}
}

func TestDiscoveryAllowsStaticAncestorSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated Windows privileges")
	}
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(realDir, "source.ERB")
	if err := os.WriteFile(path, []byte("@F\nRETURN 1+2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(realDir, alias); err != nil {
		t.Fatal(err)
	}
	targets, errs := discoverTargets([]string{filepath.Join(alias, "source.ERB")})
	if len(errs) != 0 || len(targets) != 1 {
		t.Fatalf("discoverTargets() through ancestor symlink = %#v, %v", targets, errs)
	}
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"-w", targets[0].path}, strings.NewReader(""), &out, &diagnostics); code != 0 {
		t.Fatalf("format through ancestor symlink: code=%d diagnostics=%q", code, diagnostics.String())
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "@F\n    RETURN 1 + 2\n" {
		t.Fatalf("formatted alias target=%q error=%v", got, err)
	}
}

func TestRunSkipsDirectorySymlinkWithoutBatchUsageError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated Windows privileges")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source.ERB"), []byte("@F\nRETURN 1+2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(dir, alias); err != nil {
		t.Fatal(err)
	}

	var out, diagnostics bytes.Buffer
	if code := Run([]string{alias}, strings.NewReader(""), &out, &diagnostics); code != 2 {
		t.Fatalf("Run() = %d, diagnostics=%q", code, diagnostics.String())
	}
	if !strings.Contains(diagnostics.String(), "symbolic links are skipped") {
		t.Fatalf("diagnostics=%q, want skipped-link error", diagnostics.String())
	}
	if strings.Contains(diagnostics.String(), "multiple files and directories require") {
		t.Fatalf("diagnostics=%q, got a spurious batch-mode usage error", diagnostics.String())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout=%q, want empty output", out.String())
	}
}

func TestDiscoveryDeduplicatesAncestorAliases(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated Windows privileges")
	}
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(realDir, "source.ERB")
	if err := os.WriteFile(path, []byte("@F\nIF A+B\nENDIF\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	aliasDir := filepath.Join(dir, "alias")
	if err := os.Symlink(realDir, aliasDir); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(aliasDir, "source.ERB")
	targets, errs := discoverTargets([]string{alias, path})
	if len(errs) != 0 || len(targets) != 1 {
		t.Fatalf("discoverTargets() = %#v, %v", targets, errs)
	}
	if targets[0].path != filepath.Clean(alias) {
		t.Fatalf("retained path = %q, want first input %q", targets[0].path, alias)
	}

	var out, diagnostics bytes.Buffer
	if code := Run([]string{"-w", alias, path}, strings.NewReader(""), &out, &diagnostics); code != 0 {
		t.Fatalf("Run() = %d, stdout=%q diagnostics=%q", code, out.String(), diagnostics.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "@F\n    IF A + B\n    ENDIF\n" {
		t.Fatalf("formatted source = %q", got)
	}
}

func TestSymlinkParentPathsKeepTheirTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix symlink parent semantics")
	}
	for _, mode := range []string{"file", "directory"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			actual := filepath.Join(dir, "actual")
			root := filepath.Join(dir, "root")
			for _, path := range []string{filepath.Join(actual, "child"), root} {
				if err := os.MkdirAll(path, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			alias := filepath.Join(root, "alias")
			if err := os.Symlink(filepath.Join(actual, "child"), alias); err != nil {
				t.Fatal(err)
			}
			intended := filepath.Join(actual, "script.ERB")
			decoy := filepath.Join(root, "script.ERB")
			input, other := []byte("@INTENDED\nRETURN 1+2\n"), []byte("@DECOY\nRETURN 3+4\n")
			for path, data := range map[string][]byte{intended: input, decoy: other} {
				if err := os.WriteFile(path, data, 0o640); err != nil {
					t.Fatal(err)
				}
			}
			parent := alias + string(filepath.Separator) + ".."
			spelling := appendSourcePath(parent, "script.ERB")
			argument := spelling
			if mode == "directory" {
				argument = parent
			}
			targets, errs := discoverTargets([]string{argument, intended})
			if len(errs) != 0 || len(targets) != 1 || targets[0].path != spelling {
				t.Fatalf("targets=%+v errors=%v, want %q", targets, errs, spelling)
			}
			var out, diagnostics bytes.Buffer
			if code := Run([]string{"--check", argument, intended}, strings.NewReader(""), &out, &diagnostics); code != 1 || out.String() != spelling+"\n" {
				t.Fatalf("check code=%d output=%q diagnostics=%q", code, out.String(), diagnostics.String())
			}
			out.Reset()
			diagnostics.Reset()
			if code := Run([]string{"-w", argument, intended}, strings.NewReader(""), &out, &diagnostics); code != 0 {
				t.Fatalf("write code=%d diagnostics=%q", code, diagnostics.String())
			}
			want := []byte("@INTENDED\n    RETURN 1 + 2\n")
			for path, expected := range map[string][]byte{intended: want, decoy: other} {
				got, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(got, expected) {
					t.Fatalf("%q content=%q want=%q error=%v", path, got, expected, err)
				}
			}
			for _, path := range []string{actual, root} {
				entries, err := os.ReadDir(path)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if strings.HasPrefix(entry.Name(), ".erafmt-") {
						t.Fatalf("unexpected recovery file %q in %q", entry.Name(), path)
					}
				}
			}
		})
	}
}

func TestSymlinkParentDirectoryStillSkipsVCS(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix symlink parent semantics")
	}
	dir := t.TempDir()
	vcs := filepath.Join(dir, ".git")
	if err := os.MkdirAll(filepath.Join(vcs, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vcs, "source.ERB"), []byte("RETURN 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(filepath.Join(vcs, "child"), alias); err != nil {
		t.Fatal(err)
	}
	targets, errs := discoverTargets([]string{alias + string(filepath.Separator) + ".."})
	if len(targets) != 0 || len(errs) != 0 {
		t.Fatalf("VCS discovery targets=%+v errors=%v", targets, errs)
	}
}

func TestDiscoveryKeepsDistinctHardlinkNames(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.ERB")
	second := filepath.Join(dir, "second.ERB")
	if err := os.WriteFile(first, []byte("@F\nIF A+B\nENDIF\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(first, second); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("filesystem does not support hard links: %v", err)
		}
		t.Fatal(err)
	}
	targets, errs := discoverTargets([]string{first, second})
	if len(errs) != 0 || len(targets) != 2 {
		t.Fatalf("discoverTargets() = %#v, %v", targets, errs)
	}
}

func TestProcessRejectsAncestorSwapAfterDiscovery(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated Windows privileges")
	}
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	otherDir := filepath.Join(dir, "other")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	input := []byte("@F\nIF A+B\nENDIF\n")
	if err := os.WriteFile(filepath.Join(realDir, "source.ERB"), input, 0o644); err != nil {
		t.Fatal(err)
	}
	otherInput := []byte("@F\nIF C+D\nENDIF\n")
	if err := os.WriteFile(filepath.Join(otherDir, "source.ERB"), otherInput, 0o644); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(realDir, alias); err != nil {
		t.Fatal(err)
	}
	targets, errs := discoverTargets([]string{filepath.Join(alias, "source.ERB")})
	if len(errs) != 0 || len(targets) != 1 {
		t.Fatalf("discoverTargets() = %#v, %v", targets, errs)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(otherDir, alias); err != nil {
		t.Fatal(err)
	}
	var diagnostics bytes.Buffer
	r := runner{reporter: diagnosticReporter{out: &diagnostics, format: diagnosticText}}
	if _, ok := r.process(targets[0], format.Options{}); ok || r.code != 2 {
		t.Fatalf("process() = ok:%t code:%d, want a discovery state error", ok, r.code)
	}
	got, err := os.ReadFile(filepath.Join(realDir, "source.ERB"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, input) {
		t.Fatalf("original file changed after ancestor swap: %q", got)
	}
	got, err = os.ReadFile(filepath.Join(otherDir, "source.ERB"))
	if err != nil || !bytes.Equal(got, otherInput) {
		t.Fatalf("substituted file changed: %q error=%v", got, err)
	}
}

func TestPathOutputFormats(t *testing.T) {
	dir := t.TempDir()
	name := "line\nbreak.ERB"
	if runtime.GOOS == "windows" {
		name = "line break.ERB"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("@F\nIF A+B\nENDIF\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	expectedJSON, err := json.Marshal(path)
	if err != nil {
		t.Fatal(err)
	}
	expectedJSON = append(expectedJSON, '\n')
	for _, test := range []struct {
		name   string
		format string
		want   []byte
	}{
		{name: "text", format: "text", want: []byte(path + "\n")},
		{name: "nul", format: "nul", want: append([]byte(path), 0)},
		{name: "json", format: "json", want: expectedJSON},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out, diagnostics bytes.Buffer
			code := Run([]string{"--check", "--path-format=" + test.format, path}, strings.NewReader(""), &out, &diagnostics)
			if code != 1 {
				t.Fatalf("Run() = %d, diagnostics %q", code, diagnostics.String())
			}
			if !bytes.Equal(out.Bytes(), test.want) {
				t.Fatalf("stdout = %q, want %q", out.Bytes(), test.want)
			}
		})
	}
}

func TestPathOutputFormatValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.ERB")
	if err := os.WriteFile(path, []byte("@F\nIF A+B\nENDIF\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--path-format=json"},
		{"--path-format=json", "--check"},
		{"--path-format=json", path},
		{"--path-format=json", "-w", path},
		{"--rules", "--path-format=json"},
		{"--check", "--path-format=invalid", path},
	} {
		var out, diagnostics bytes.Buffer
		if code := Run(args, strings.NewReader(""), &out, &diagnostics); code != 2 {
			t.Fatalf("Run(%v) = %d, diagnostics %q", args, code, diagnostics.String())
		}
	}
}

func TestJSONPathOutputRejectsInvalidUTF8(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows paths are Unicode")
	}
	path := filepath.Join(t.TempDir(), string([]byte{0xff})+".ERB")
	if err := os.WriteFile(path, []byte("@F\nIF A+B\nENDIF\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--check", "--path-format=json", path}, strings.NewReader(""), &out, &diagnostics); code != 2 {
		t.Fatalf("Run() = %d, diagnostics %q", code, diagnostics.String())
	}
	if out.Len() != 0 || !strings.Contains(diagnostics.String(), "use --path-format=nul") {
		t.Fatalf("invalid UTF-8 path output = %q, diagnostics %q", out.Bytes(), diagnostics.String())
	}
}

func TestJSONPathOutputPreflightPreventsWrites(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows paths are Unicode")
	}
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "A.ERB"), filepath.Join(dir, string([]byte{0xff})+".ERB")}
	src := []byte("@F\nRETURN 1+2\n")
	for _, path := range paths {
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, diagnostics bytes.Buffer
	code := Run([]string{"-l", "-w", "--path-format=json", "--diagnostic-format=json", dir}, strings.NewReader(""), &out, &diagnostics)
	if code != 2 || out.Len() != 0 {
		t.Fatalf("code=%d output=%q diagnostics=%q", code, out.String(), diagnostics.String())
	}
	for _, path := range paths {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, src) {
			t.Fatalf("preflight changed %q: %q (%v)", path, got, err)
		}
	}
	events := decodeDiagnosticLines(t, diagnostics.Bytes())
	if len(events) != 1 || events[0]["code"] != "io" || events[0]["path_bytes"] == nil {
		t.Fatalf("events=%+v", events)
	}
}

func TestPathOutputFailureIsFatal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.ERB")
	if err := os.WriteFile(path, []byte("@F\nIF A+B\nENDIF\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var diagnostics bytes.Buffer
	if code := Run([]string{"--check", "--path-format=nul", path}, strings.NewReader(""), failedWriter{}, &diagnostics); code != 2 {
		t.Fatalf("Run() = %d, diagnostics %q", code, diagnostics.String())
	}
}

func discoverTargets(args []string) ([]discoveredTarget, []error) {
	result := discoverTargetsWithMetadata(args)
	return result.targets, result.errors
}
