//go:build linux

package cli

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestPreserveMetadataCopiesOwnerModeAndXattrs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.ERB")
	if err := os.WriteFile(path, []byte("source\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Setxattr(path, "user.erafmt-binary", []byte{0, 1, 255}, 0); err != nil {
		if unsupportedXattr(err) {
			t.Skipf("xattrs unavailable: %v", err)
		}
		t.Fatal(err)
	}
	if err := syscall.Setxattr(path, "user.erafmt-empty", nil, 0); err != nil {
		t.Fatal(err)
	}

	before, err := captureState(path)
	if err != nil {
		t.Fatal(err)
	}
	temp, err := os.CreateTemp(dir, ".erafmt-test-*")
	if err != nil {
		t.Fatal(err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()
	if err := syscall.Setxattr(tempPath, "user.erafmt-inherited", []byte("remove"), 0); err != nil {
		t.Fatal(err)
	}
	if err := preserveMetadata(path, temp, before); err != nil {
		t.Fatal(err)
	}

	sourceInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	tempInfo, err := temp.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if tempInfo.Mode() != modeForWrite(sourceInfo.Mode()) {
		t.Fatalf("mode = %v, want %v", tempInfo.Mode(), modeForWrite(sourceInfo.Mode()))
	}
	sourceStat := sourceInfo.Sys().(*syscall.Stat_t)
	tempStat := tempInfo.Sys().(*syscall.Stat_t)
	if sourceStat.Uid != tempStat.Uid || sourceStat.Gid != tempStat.Gid {
		t.Fatalf("owner = %d:%d, want %d:%d", tempStat.Uid, tempStat.Gid, sourceStat.Uid, sourceStat.Gid)
	}
	source, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	want, err := readXattrsStable(procFDPath(source.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	got, err := readXattrsStable(procFDPath(temp.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if !sameXattrs(want, got) {
		t.Fatalf("xattrs = %#v, want %#v", got, want)
	}
}

func TestPreserveMetadataRejectsSourceChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.ERB")
	if err := os.WriteFile(path, []byte("source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := captureState(path)
	if err != nil {
		t.Fatal(err)
	}
	temp, err := os.CreateTemp(dir, ".erafmt-test-*")
	if err != nil {
		t.Fatal(err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := preserveMetadata(path, temp, before); err == nil {
		t.Fatal("preserveMetadata accepted changed source")
	}
}

func TestReplaceFileRejectsXattrChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.ERB")
	if err := os.WriteFile(path, []byte("source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Setxattr(path, "user.erafmt-change", []byte("before"), 0); err != nil {
		if unsupportedXattr(err) {
			t.Skipf("xattrs unavailable: %v", err)
		}
		t.Fatal(err)
	}
	before, err := captureState(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Setxattr(path, "user.erafmt-change", []byte("after"), 0); err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(path, before, []byte("source\n"), []byte("formatted\n")); err == nil {
		t.Fatal("replaceFile accepted a changed source xattr")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "source\n" {
		t.Fatalf("source changed after rejected replacement: %q", data)
	}
	value := make([]byte, len("after"))
	n, err := syscall.Getxattr(path, "user.erafmt-change", value)
	if err != nil || n != len(value) || string(value) != "after" {
		t.Fatalf("changed source xattr was not preserved: value=%q size=%d err=%v", value, n, err)
	}
}

func TestPreserveMetadataCopiesPOSIXACL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.ERB")
	if err := os.WriteFile(path, []byte("source\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	acl := testACL(uint32(os.Getuid()))
	if err := syscall.Setxattr(path, "system.posix_acl_access", acl, 0); err != nil {
		if unsupportedXattr(err) {
			t.Skipf("POSIX ACLs unavailable: %v", err)
		}
		t.Fatal(err)
	}
	before, err := captureState(path)
	if err != nil {
		t.Fatal(err)
	}
	temp, err := os.CreateTemp(dir, ".erafmt-test-*")
	if err != nil {
		t.Fatal(err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()
	if err := preserveMetadata(path, temp, before); err != nil {
		t.Fatal(err)
	}
	got, err := readXattr(procFDPath(temp.Fd()), "system.posix_acl_access")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(acl) {
		t.Fatalf("POSIX ACL changed: got %x, want %x", got, acl)
	}
}

func testACL(uid uint32) []byte {
	entries := [][3]uint32{
		{0x1, 0x6, 0xffffffff},
		{0x2, 0x4, uid},
		{0x4, 0x4, 0xffffffff},
		{0x10, 0x4, 0xffffffff},
		{0x20, 0x0, 0xffffffff},
	}
	data := make([]byte, 4+len(entries)*8)
	binary.LittleEndian.PutUint32(data, 0x0002)
	for i, entry := range entries {
		offset := 4 + i*8
		binary.LittleEndian.PutUint16(data[offset:], uint16(entry[0]))
		binary.LittleEndian.PutUint16(data[offset+2:], uint16(entry[1]))
		binary.LittleEndian.PutUint32(data[offset+4:], entry[2])
	}
	return data
}

func unsupportedXattr(err error) bool {
	return errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.EPERM)
}
