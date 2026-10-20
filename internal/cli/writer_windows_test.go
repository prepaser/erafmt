package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

func TestDeleteFileByHandleKeepsReplacementPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup")
	moved := filepath.Join(dir, "moved")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	handle, err := openDeleteHandle(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, moved); err != nil {
		_ = handle.Close()
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement"), 0o644); err != nil {
		_ = handle.Close()
		t.Fatal(err)
	}
	if err := deleteFileByHandle(handle); err != nil {
		_ = handle.Close()
		t.Fatal(err)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "replacement" {
		t.Fatalf("replacement path = %q, err=%v", data, err)
	}
	if _, err := os.Stat(moved); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("original remains after handle deletion: err=%v", err)
	}
}

func TestReplaceFailureDidNotConsumeTemp(t *testing.T) {
	for _, test := range []struct {
		name string
		err  syscall.Errno
		want bool
	}{
		{name: "unable to remove replaced", err: windowsErrorUnableToRemoveReplaced, want: true},
		{name: "unable to move replacement", err: windowsErrorUnableToMoveReplacement, want: true},
		{name: "unable to move replacement with backup", err: windowsErrorUnableToMoveReplacement2, want: true},
		{name: "unknown", err: syscall.Errno(9999), want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := replaceFailureDidNotConsumeTemp(test.err); got != test.want {
				t.Fatalf("replaceFailureDidNotConsumeTemp(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}

func TestOriginalBackupInspectionSharesDeleteHandle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup")
	original := []byte("original")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := captureState(path)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := openVerifiedOriginalBackup(path, before, original)
	if err != nil {
		t.Fatalf("backup inspection with an open DELETE handle failed: %v", err)
	}
	if err := backup.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFileDispositionInfoUsesBooleanWidth(t *testing.T) {
	if size := unsafe.Sizeof(fileDispositionInfo{}); size != 1 {
		t.Fatalf("FILE_DISPOSITION_INFO size = %d, want 1", size)
	}
}

func TestReplacementTempCleanupRejectsIdentityChange(t *testing.T) {
	temp, cleanup, err := createReplacementTemp(filepath.Join(t.TempDir(), "source.ERB"))
	if err != nil {
		t.Fatal(err)
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		t.Fatal(err)
	}
	attacker := tempPath + ".replacement"
	if err := os.WriteFile(attacker, []byte("attacker"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(tempPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(attacker, tempPath); err != nil {
		t.Fatal(err)
	}
	if err := cleanup(false); err == nil || !strings.Contains(err.Error(), strconv.Quote(tempPath)) {
		t.Fatalf("cleanup error=%v, want the replacement path", err)
	}
	data, err := os.ReadFile(tempPath)
	if err != nil || string(data) != "attacker" {
		t.Fatalf("replacement temp = %q, err=%v", data, err)
	}
}

func TestReplacementTempCleanupReportsBlockedPath(t *testing.T) {
	temp, cleanup, err := createReplacementTemp(filepath.Join(t.TempDir(), "source.ERB"))
	if err != nil {
		t.Fatal(err)
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		t.Fatal(err)
	}
	name, err := syscall.UTF16PtrFromString(tempPath)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	lock := os.NewFile(uintptr(handle), tempPath)
	if lock == nil {
		_ = syscall.CloseHandle(handle)
		t.Fatal("invalid blocking handle")
	}
	defer lock.Close()
	err = cleanup(false)
	if !errors.Is(err, syscall.Errno(32)) || !strings.Contains(err.Error(), strconv.Quote(tempPath)) {
		t.Fatalf("cleanup error=%v, want sharing violation and temporary path", err)
	}
	if _, err := os.Stat(tempPath); err != nil {
		t.Fatalf("retained temporary file: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cleanup(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file after cleanup: %v", err)
	}
}

func TestExpectedReplacementStateAllowsReplaceFileAttributeMerges(t *testing.T) {
	for _, test := range []struct {
		name     string
		before   uint32
		replace  uint32
		expected uint32
	}{
		{
			name:     "plain original clears replacement compression and encryption",
			before:   syscall.FILE_ATTRIBUTE_NORMAL,
			replace:  syscall.FILE_ATTRIBUTE_ARCHIVE | 0x00000800 | 0x00004000,
			expected: syscall.FILE_ATTRIBUTE_ARCHIVE,
		},
		{
			name:     "compressed original survives normal replacement",
			before:   0x00000800,
			replace:  syscall.FILE_ATTRIBUTE_NORMAL,
			expected: 0x00000800,
		},
		{
			name:     "normal remains alone",
			before:   syscall.FILE_ATTRIBUTE_NORMAL,
			replace:  syscall.FILE_ATTRIBUTE_NORMAL,
			expected: syscall.FILE_ATTRIBUTE_NORMAL,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := fileState{attrs: test.before}
			snapshot := preparedFileSnapshot{state: fileState{attrs: test.replace}}
			if expected := expectedReplacementState(before, snapshot); expected.attrs != test.expected {
				t.Fatalf("expected replacement attributes=%#x, want %#x", expected.attrs, test.expected)
			}
		})
	}
}

func TestReplaceFileLockedTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locked.ERB")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := captureState(path)
	if err != nil {
		t.Fatal(err)
	}
	locked, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer locked.Close()
	if err := replaceFile(path, before, []byte("original"), []byte("formatted")); err == nil {
		t.Fatal("replacement succeeded while target disallowed deletion")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "original" {
		t.Fatal("locked target was changed")
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".erafmt-*"))
	if err != nil || len(files) != 0 {
		t.Fatal("failed replacement left temporary files")
	}
}

func TestReplaceFileReadonlyTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "readonly.ERB")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(path, 0o644)
	before, err := captureState(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(path, before, []byte("original"), []byte("formatted")); err == nil {
		t.Fatal("replacement overwrote a read-only target")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "original" {
		t.Fatal("read-only target was changed")
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".erafmt-*"))
	if err != nil || len(files) != 0 {
		t.Fatal("failed replacement left read-only temporary files")
	}
}

func TestReplaceFilePreservesAlternateDataStream(t *testing.T) {
	path := filepath.Join(t.TempDir(), "streams.ERB")
	streamPath := path + ":erafmt.metadata"
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(streamPath, []byte("metadata"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := captureState(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(path, before, []byte("original"), []byte("formatted")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(streamPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "metadata" {
		t.Fatalf("alternate data stream changed: %q", data)
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".erafmt-backup-*"))
	if err != nil || len(files) != 0 {
		t.Fatal("successful replacement left a backup file")
	}
}

func TestReplaceFilePreservesFileAttributes(t *testing.T) {
	const notContentIndexed = 0x00002000
	const preserved = syscall.FILE_ATTRIBUTE_HIDDEN | syscall.FILE_ATTRIBUTE_SYSTEM | syscall.FILE_ATTRIBUTE_ARCHIVE | notContentIndexed
	for _, want := range []uint32{0, preserved} {
		path := filepath.Join(t.TempDir(), "attributes.ERB")
		if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}
		pathPtr, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			t.Fatal(err)
		}
		attributes, err := syscall.GetFileAttributes(pathPtr)
		if err != nil {
			t.Fatal(err)
		}
		attributes = attributes&^preserved | want
		if attributes == 0 {
			attributes = syscall.FILE_ATTRIBUTE_NORMAL
		}
		if err := syscall.SetFileAttributes(pathPtr, attributes); err != nil {
			t.Fatal(err)
		}
		before, err := captureState(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := replaceFile(path, before, []byte("original"), []byte("formatted")); err != nil {
			t.Fatal(err)
		}
		pathPtr, err = syscall.UTF16PtrFromString(path)
		if err != nil {
			t.Fatal(err)
		}
		after, err := syscall.GetFileAttributes(pathPtr)
		if err != nil {
			t.Fatal(err)
		}
		if after&preserved != want {
			t.Fatalf("file attributes changed: before %#x after %#x", attributes, after)
		}
	}
}
