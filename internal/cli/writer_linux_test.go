//go:build linux

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestReplaceFileCompletesAtomicReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.ERB")
	if err := os.WriteFile(path, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	before, err := captureState(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(path, before, []byte("original"), []byte("formatted")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "formatted" {
		t.Fatalf("replacement = %q, want formatted", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatalf("directory entries = %v, want only %q", entries, filepath.Base(path))
	}
}

func TestPreparedReplacementFallsBackToRename(t *testing.T) {
	for _, exchangeErr := range []error{syscall.ENOSYS, syscall.EOPNOTSUPP, syscall.EINVAL, syscall.EACCES} {
		t.Run(exchangeErr.Error(), func(t *testing.T) {
			path, tempPath, before, original, output, snapshot := preparedReplacementFixture(t, false)
			retained, err := replacePreparedFileWithOps(tempPath, path, before, original, snapshot, syncDirectory, func(*os.File, string, *os.File, string) error {
				return exchangeErr
			})
			want := output
			if exchangeErr == syscall.EACCES {
				want = original
				if err == nil || retained {
					t.Fatalf("permission error was bypassed: retained=%v err=%v", retained, err)
				}
			} else {
				if err != nil || !retained {
					t.Fatalf("rename fallback failed: retained=%v err=%v", retained, err)
				}
				entries, err := os.ReadDir(filepath.Dir(path))
				if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
					t.Fatalf("staging directory remains: entries=%v err=%v", entries, err)
				}
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("contents=%q want=%q err=%v", got, want, err)
			}
		})
	}
}

func TestPreparedReplacementRejectsCreatorSnapshotMismatch(t *testing.T) {
	for _, mutate := range []struct {
		name string
		fn   func(t *testing.T, tempPath string)
	}{
		{name: "content", fn: func(t *testing.T, tempPath string) {
			if err := os.WriteFile(tempPath, []byte("tamperedd"), 0o640); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "inode", fn: func(t *testing.T, tempPath string) {
			replacement := filepath.Join(filepath.Dir(tempPath), "replacement")
			if err := os.WriteFile(replacement, []byte("formatted"), 0o640); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(replacement, tempPath); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "mode", fn: func(t *testing.T, tempPath string) {
			if err := os.Chmod(tempPath, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "xattr", fn: func(t *testing.T, tempPath string) {
			if err := syscall.Setxattr(tempPath, "user.erafmt-snapshot", []byte("tampered"), 0); err != nil {
				if unsupportedXattr(err) {
					t.Skipf("xattrs unavailable: %v", err)
				}
				t.Fatal(err)
			}
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			path, tempPath, before, original, _, snapshot := preparedReplacementFixture(t, false)
			mutate.fn(t, tempPath)
			retained, err := replacePreparedFile(tempPath, path, before, original, snapshot)
			if retained || err == nil {
				t.Fatalf("retained=%v err=%v, want preflight rejection", retained, err)
			}
			got, readErr := os.ReadFile(path)
			if readErr != nil || !bytes.Equal(got, original) {
				t.Fatalf("target=%q readErr=%v, want original", got, readErr)
			}
		})
	}
}

func TestReplacementTempUsesPrivateStageAndSafeCleanup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.ERB")
	victim := filepath.Join(dir, "victim")
	if err := os.WriteFile(victim, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	temp, cleanup, err := createReplacementTemp(path)
	if err != nil {
		t.Fatal(err)
	}
	tempPath := temp.Name()
	stagePath := filepath.Dir(tempPath)
	resolvedStage, err := filepath.EvalSymlinks(stagePath)
	if err != nil {
		t.Fatal(err)
	}
	actualTempPath := filepath.Join(resolvedStage, filepath.Base(tempPath))
	info, err := os.Stat(stagePath)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("stage mode=%v err=%v", info.Mode(), err)
	}
	if err := temp.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(tempPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, tempPath); err != nil {
		t.Fatal(err)
	}
	if err := cleanup(false); err == nil {
		t.Fatal("cleanup followed a replaced temporary path")
	}
	info, err = os.Stat(victim)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("victim mode=%v err=%v", info.Mode(), err)
	}
	if info, err := os.Lstat(actualTempPath); err != nil {
		t.Fatalf("replacement path was removed: %v", err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("replacement path mode=%v, want symlink", info.Mode())
	}
}

func preparedReplacementFixture(t *testing.T, concurrent bool) (path, tempPath string, before fileState, original, output []byte, snapshot preparedFileSnapshot) {
	t.Helper()
	dir := t.TempDir()
	path = filepath.Join(dir, "source.ERB")
	original, output = []byte("original"), []byte("formatted")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	var err error
	if before, err = captureState(path); err != nil {
		t.Fatal(err)
	}
	temp, cleanup, err := createReplacementTemp(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cleanup(true) })
	tempPath = temp.Name()
	if _, err := temp.Write(output); err != nil {
		t.Fatal(err)
	}
	if err := preserveMetadata(path, temp, before); err != nil {
		t.Fatal(err)
	}
	if err := temp.Sync(); err != nil {
		t.Fatal(err)
	}
	state, err := stateFromFile(temp)
	if err != nil {
		t.Fatal(err)
	}
	attrs, err := capturePreparedAttributes(temp)
	if err != nil {
		t.Fatal(err)
	}
	snapshot = preparedFileSnapshot{state: state, output: append([]byte(nil), output...), attrs: attrs}
	if err := temp.Close(); err != nil {
		t.Fatal(err)
	}
	if concurrent {
		if err := os.WriteFile(path, []byte("concurrent"), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	return path, tempPath, before, original, output, snapshot
}

func TestPreparedReplacementSyncFailureRetainsRecoveryPaths(t *testing.T) {
	path, tempPath, before, original, output, snapshot := preparedReplacementFixture(t, false)
	syncCalls := 0
	retain, err := replacePreparedFileWithOps(tempPath, path, before, original, snapshot, func(*os.File) error {
		syncCalls++
		if syncCalls == 3 {
			return syscall.EIO
		}
		return nil
	}, exchangeFiles)
	if !retain || err == nil || syncCalls != 3 || !strings.Contains(err.Error(), "original backup retained") {
		t.Fatalf("replacement: retain=%v syncCalls=%d err=%v", retain, syncCalls, err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(output) {
		t.Fatalf("target=%q err=%v", got, err)
	}
	got, err = os.ReadFile(tempPath)
	if err != nil || string(got) != string(original) {
		t.Fatalf("backup=%q err=%v", got, err)
	}
}

func TestPreparedReplacementSyncFailureUsesRelocatedRecoveryPath(t *testing.T) {
	path, tempPath, before, original, output, snapshot := preparedReplacementFixture(t, false)
	oldDir, newDir := filepath.Dir(path), filepath.Join(t.TempDir(), "moved")
	syncCalls := 0
	retained, err := replacePreparedFileWithOps(tempPath, path, before, original, snapshot, func(*os.File) error {
		syncCalls++
		if syncCalls == 3 {
			if err := os.Rename(oldDir, newDir); err != nil {
				return err
			}
			return syscall.EIO
		}
		return nil
	}, exchangeFiles)
	if !retained || err == nil {
		t.Fatalf("retained=%v error=%v", retained, err)
	}
	actualBackup := filepath.Join(newDir, filepath.Base(filepath.Dir(tempPath)), filepath.Base(tempPath))
	if !strings.Contains(err.Error(), actualBackup) {
		t.Fatalf("error=%v, want relocated backup path %q", err, actualBackup)
	}
	if strings.Contains(err.Error(), "/proc/self/fd/") || strings.Contains(err.Error(), tempPath) {
		t.Fatalf("error contains stale recovery path: %v", err)
	}
	got, err := os.ReadFile(actualBackup)
	if err != nil || string(got) != string(original) {
		t.Fatalf("backup=%q error=%v", got, err)
	}
	got, err = os.ReadFile(filepath.Join(newDir, filepath.Base(path)))
	if err != nil || string(got) != string(output) {
		t.Fatalf("target=%q error=%v", got, err)
	}
}

func TestRecoveryLocationUsesDirectoryIdentityWhenUnlinked(t *testing.T) {
	root := t.TempDir()
	dirPath := filepath.Join(root, "parent")
	if err := os.Mkdir(dirPath, 0o700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.Open(dirPath)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	if err := os.Remove(dirPath); err != nil {
		t.Fatal(err)
	}
	got := recoveryLocation(dir, ".erafmt-prepared")
	if !strings.Contains(got, "directory identity") || !strings.Contains(got, "entry basename") {
		t.Fatalf("location=%q, want directory identity and basename", got)
	}
	if strings.Contains(got, "/proc/self/fd/") || strings.Contains(got, dirPath) {
		t.Fatalf("location exposes unusable path: %q", got)
	}
}

func TestRecoveryLocationKeepsLiteralDeletedSuffix(t *testing.T) {
	root := t.TempDir()
	dirPath := filepath.Join(root, "parent (deleted)")
	if err := os.Mkdir(dirPath, 0o700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.Open(dirPath)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	got := recoveryLocation(dir, ".erafmt-prepared")
	want := filepath.Join(dirPath, ".erafmt-prepared")
	if !strings.Contains(got, want) {
		t.Fatalf("location=%q, want literal suffix path %q", got, want)
	}
}

func TestPreparedReplacementCleanupSyncFailureReportsUnconfirmedDurability(t *testing.T) {
	path, tempPath, before, original, output, snapshot := preparedReplacementFixture(t, false)
	syncCalls := 0
	retain, err := replacePreparedFileWithOps(tempPath, path, before, original, snapshot, func(*os.File) error {
		syncCalls++
		if syncCalls == 5 {
			return syscall.EIO
		}
		return nil
	}, exchangeFiles)
	if !retain || err == nil || syncCalls != 5 || !strings.Contains(err.Error(), "durability is unconfirmed") {
		t.Fatalf("replacement: retain=%v syncCalls=%d err=%v", retain, syncCalls, err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(output) {
		t.Fatalf("target=%q err=%v", got, err)
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("backup still exists: err=%v", err)
	}
}

func TestPreparedReplacementRestorationSyncFailureKeepsConcurrentTarget(t *testing.T) {
	path, tempPath, before, original, output, snapshot := preparedReplacementFixture(t, true)
	syncCalls := 0
	retain, err := replacePreparedFileWithOps(tempPath, path, before, original, snapshot, func(*os.File) error {
		syncCalls++
		if syncCalls == 3 {
			return syscall.EIO
		}
		return nil
	}, exchangeFiles)
	if !retain || err == nil || syncCalls != 3 || !strings.Contains(err.Error(), "original recovery is uncertain") {
		t.Fatalf("replacement: retain=%v syncCalls=%d err=%v", retain, syncCalls, err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "concurrent" {
		t.Fatalf("target=%q err=%v", got, err)
	}
	got, err = os.ReadFile(tempPath)
	if err != nil || string(got) != string(output) {
		t.Fatalf("retained output=%q err=%v", got, err)
	}
}

func TestPreparedReplacementRecoversConcurrentChanges(t *testing.T) {
	for _, change := range []string{"content", "inode", "xattr", "hardlink", "symlink", "fifo"} {
		t.Run(change, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "source.ERB")
			original, output := []byte("original"), []byte("formatted")
			if err := os.WriteFile(path, original, 0o640); err != nil {
				t.Fatal(err)
			}
			before, err := captureState(path)
			if err != nil {
				t.Fatal(err)
			}
			temp, cleanup, err := createReplacementTemp(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cleanup(true) })
			tempPath := temp.Name()
			if _, err := temp.Write(output); err != nil {
				t.Fatal(err)
			}
			if err := preserveMetadata(path, temp, before); err != nil {
				t.Fatal(err)
			}
			if err := temp.Sync(); err != nil {
				t.Fatal(err)
			}
			preparedState, err := stateFromFile(temp)
			if err != nil {
				t.Fatal(err)
			}
			preparedAttrs, err := capturePreparedAttributes(temp)
			if err != nil {
				t.Fatal(err)
			}
			snapshot := preparedFileSnapshot{state: preparedState, output: append([]byte(nil), output...), attrs: preparedAttrs}
			if err := temp.Close(); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "content":
				if err := os.WriteFile(path, []byte("concurrent"), 0o640); err != nil {
					t.Fatal(err)
				}
			case "inode":
				newPath := filepath.Join(dir, "editor-temp")
				if err := os.WriteFile(newPath, []byte("concurrent"), 0o640); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(newPath, path); err != nil {
					t.Fatal(err)
				}
			case "xattr":
				if err := syscall.Setxattr(path, "user.erafmt-race", []byte("concurrent"), 0); err != nil {
					if unsupportedXattr(err) {
						t.Skip(err)
					}
					t.Fatal(err)
				}
			case "hardlink":
				if err := os.Link(path, filepath.Join(dir, "editor-link")); err != nil {
					t.Fatal(err)
				}
			case "symlink", "fifo":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if change == "symlink" {
					if err := os.Symlink("missing-target", path); err != nil {
						t.Fatal(err)
					}
				} else if err := syscall.Mkfifo(path, 0o640); err != nil {
					t.Fatal(err)
				}
			}
			retain, err := replacePreparedFile(tempPath, path, before, original, snapshot)
			if err == nil || !retain || !strings.Contains(err.Error(), filepath.Base(tempPath)) || !strings.Contains(err.Error(), "retained output at") {
				t.Fatalf("replacement: retain=%v err=%v", retain, err)
			}
			got, err := os.ReadFile(tempPath)
			if err != nil || string(got) != string(output) {
				t.Fatalf("retained output=%q err=%v", got, err)
			}
			info, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if change == "symlink" || change == "fifo" {
				mode := os.ModeSymlink
				if change == "fifo" {
					mode = os.ModeNamedPipe
				}
				if info.Mode()&mode == 0 {
					t.Fatalf("restored mode=%v", info.Mode())
				}
				return
			}
			got, err = os.ReadFile(path)
			want := string(original)
			if change == "content" || change == "inode" {
				want = "concurrent"
			}
			if err != nil || string(got) != want {
				t.Fatalf("restored source=%q err=%v", got, err)
			}
			if change == "xattr" {
				value, err := readXattr(path, "user.erafmt-race")
				if err != nil || string(value) != "concurrent" {
					t.Fatalf("restored xattr=%q err=%v", value, err)
				}
			}
		})
	}
}
