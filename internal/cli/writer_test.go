package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceFileWritesAndSkipsUnchangedFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.ERB")
	original, output := []byte("original"), []byte("formatted")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	before, err := captureState(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(path, before, original, original); err != nil {
		t.Fatal(err)
	}
	after, err := captureState(path)
	if err != nil || !before.same(after) {
		t.Fatalf("unchanged file was replaced: state=%+v err=%v", after, err)
	}
	if err := replaceFile(path, before, original, output); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, output) {
		t.Fatalf("contents=%q err=%v", got, err)
	}
	after, err = captureState(path)
	if err != nil || after.mode != before.mode {
		t.Fatalf("mode changed: before=%v after=%v err=%v", before.mode, after.mode, err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatalf("temporary files remain: entries=%v err=%v", entries, err)
	}
}

func TestRenamePreparedFile(t *testing.T) {
	for _, change := range []string{"none", "original", "prepared", "expected-output"} {
		t.Run(change, func(t *testing.T) {
			dir := t.TempDir()
			path, temp := filepath.Join(dir, "source.ERB"), filepath.Join(dir, ".erafmt-prepared")
			original, output := []byte("original"), []byte("formatted")
			for name, data := range map[string][]byte{path: original, temp: output} {
				if err := os.WriteFile(name, data, 0o640); err != nil {
					t.Fatal(err)
				}
			}
			before, err := captureState(path)
			if err != nil {
				t.Fatal(err)
			}
			prepared, err := captureState(temp)
			if err != nil {
				t.Fatal(err)
			}
			snapshot := preparedFileSnapshot{state: prepared, output: output}
			want := output
			if change != "none" {
				want = original
				switch change {
				case "original":
					want = []byte("concurrent")
					if err := os.WriteFile(path, want, 0o640); err != nil {
						t.Fatal(err)
					}
				case "prepared":
					if err := os.WriteFile(temp, []byte("changed"), 0o640); err != nil {
						t.Fatal(err)
					}
				case "expected-output":
					snapshot.output = []byte("different")
				}
			}
			retained, err := renamePreparedFile(temp, path, before, original, snapshot)
			if retained || (err != nil) != (change != "none") {
				t.Fatalf("retained=%v err=%v", retained, err)
			}
			got, readErr := os.ReadFile(path)
			if readErr != nil || !bytes.Equal(got, want) {
				t.Fatalf("contents=%q want=%q err=%v", got, want, readErr)
			}
		})
	}
}
