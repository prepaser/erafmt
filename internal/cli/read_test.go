package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReadSourceReturnsStableBytesAndState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.ERB")
	want := []byte("@F\nRETURN 1\n")
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}

	got, state, err := readSource(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("readSource() = %q, want %q", got, want)
	}
	wantState, err := captureState(path)
	if err != nil {
		t.Fatal(err)
	}
	if !state.same(wantState) {
		t.Fatalf("readSource() state = %#v, want %#v", state, wantState)
	}
}

func TestReadSourceRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated Windows privileges")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.ERB")
	link := filepath.Join(dir, "source.ERB")
	if err := os.WriteFile(target, []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readSource(link, nil); err == nil {
		t.Fatal("readSource accepted a symbolic link")
	}
}
