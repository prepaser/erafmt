//go:build linux

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenSourceDoesNotFollowSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.ERB")
	link := filepath.Join(dir, "source.ERB")
	if err := os.WriteFile(target, []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	file, err := openSource(link)
	if err == nil {
		_ = file.Close()
		t.Fatal("openSource followed a symbolic link")
	}
}
