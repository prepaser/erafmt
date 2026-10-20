package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunSkipsExplicitDirectorySymlinkSuffixes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix symlink path semantics")
	}
	for _, mode := range []string{"--check", "-l", "-w"} {
		for _, suffix := range []string{"alias/", "alias/.", "alias//", "alias/./"} {
			t.Run(mode+"/"+strings.ReplaceAll(suffix, "/", "_"), func(t *testing.T) {
				dir := t.TempDir()
				realDir := filepath.Join(dir, "real")
				if err := os.Mkdir(realDir, 0o755); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(realDir, "source.ERB")
				original := []byte("@F\nRETURN 1+2\n")
				if err := os.WriteFile(path, original, 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(realDir, filepath.Join(dir, "alias")); err != nil {
					t.Fatal(err)
				}

				argument := dir + string(filepath.Separator) + suffix
				var out, diagnostics bytes.Buffer
				if code := Run([]string{mode, argument}, strings.NewReader(""), &out, &diagnostics); code != 2 {
					t.Fatalf("Run() = %d, stdout=%q diagnostics=%q", code, out.String(), diagnostics.String())
				}
				if out.Len() != 0 || !strings.Contains(diagnostics.String(), "symbolic links are skipped") {
					t.Fatalf("stdout=%q diagnostics=%q", out.String(), diagnostics.String())
				}
				got, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(got, original) {
					t.Fatalf("target=%q err=%v, want unchanged source", got, err)
				}
			})
		}
	}
}

func TestRunKeepsDirectoryAndAncestorSymlinkControls(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix symlink path semantics")
	}
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(realDir, "source.ERB")
	original := []byte("@F\nRETURN 1+2\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(realDir, alias); err != nil {
		t.Fatal(err)
	}

	for _, argument := range []string{realDir + string(filepath.Separator), filepath.Join(alias, "source.ERB")} {
		var out, diagnostics bytes.Buffer
		if code := Run([]string{"--check", argument}, strings.NewReader(""), &out, &diagnostics); code != 1 {
			t.Fatalf("argument=%q code=%d stdout=%q diagnostics=%q", argument, code, out.String(), diagnostics.String())
		}
		if !strings.Contains(out.String(), argument) || diagnostics.Len() != 0 {
			t.Fatalf("argument=%q stdout=%q diagnostics=%q", argument, out.String(), diagnostics.String())
		}
	}
}

func TestSymlinkInspectionPathPreservesRootAndParentSemantics(t *testing.T) {
	sep := string(filepath.Separator)
	root := sep
	for _, test := range []struct {
		path string
		want string
	}{
		{path: root + ".", want: root},
		{path: root + "." + sep + ".", want: root},
		{path: "." + sep, want: "."},
		{path: "." + sep + ".", want: "."},
		{path: "alias" + sep, want: "alias"},
		{path: "alias" + sep + ".", want: "alias"},
		{path: "alias" + sep + "." + sep + ".", want: "alias"},
		{path: "alias" + sep + ".." + sep, want: "alias" + sep + ".."},
		{path: "alias" + sep + "..", want: "alias" + sep + ".."},
		{path: "alias" + sep + ".." + sep + ".", want: "alias" + sep + ".."},
	} {
		if got := symlinkInspectionPath(test.path); got != test.want {
			t.Fatalf("symlinkInspectionPath(%q) = %q, want %q", test.path, got, test.want)
		}
	}
	if filepath.VolumeName("C:") != "" {
		if got := symlinkInspectionPath("C:"); got != "C:" {
			t.Fatalf("bare drive path changed to %q", got)
		}
		root := "C:" + sep
		if got := symlinkInspectionPath(root); got != root {
			t.Fatalf("drive root changed to %q", got)
		}
		for _, path := range []string{root + ".", root + "." + sep + "."} {
			if got := symlinkInspectionPath(path); got != root {
				t.Fatalf("drive-root dot path %q changed to %q", path, got)
			}
		}
	}
}
