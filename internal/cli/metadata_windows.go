//go:build windows

package cli

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var replaceFileProc = syscall.NewLazyDLL("kernel32.dll").NewProc("ReplaceFileW")
var setFileInformationByHandleProc = syscall.NewLazyDLL("kernel32.dll").NewProc("SetFileInformationByHandle")

const (
	fileDeleteAccess                         = 0x00010000
	fileDispositionInformation               = 4
	fileDispositionDelete      byte          = 1
	fileFlagDeleteOnClose                    = 0x04000000
	windowsErrorInvalidHandle  syscall.Errno = 6
)

type fileDispositionInfo struct {
	DeleteFile byte
}

func preserveMetadata(_ string, temp *os.File, before fileState) error {
	if before.mode.Perm()&0o222 == 0 {
		return fmt.Errorf("target is read-only")
	}
	if err := temp.Chmod(modeForWrite(before.mode)); err != nil {
		return err
	}
	return preserveFileAttributes(before.attrs, temp)
}

func capturePreparedAttributes(*os.File) (map[string][]byte, error) { return nil, nil }

func replacePreparedFile(tempPath, path string, before fileState, original []byte, snapshot preparedFileSnapshot) (bool, error) {
	if err := verifyPreparedFile(tempPath, snapshot, snapshot.state); err != nil {
		return false, fmt.Errorf("verify prepared replacement: %w", err)
	}
	backupPath, err := newBackupPath(path)
	if err != nil {
		return false, err
	}
	replaced, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false, fmt.Errorf("prepare ReplaceFileW paths: %w (backup %q)", err, backupPath)
	}
	replacement, err := syscall.UTF16PtrFromString(tempPath)
	if err != nil {
		return false, fmt.Errorf("prepare ReplaceFileW paths: %w (backup %q)", err, backupPath)
	}
	backup, err := syscall.UTF16PtrFromString(backupPath)
	if err != nil {
		return false, fmt.Errorf("prepare ReplaceFileW paths: %w (backup %q)", err, backupPath)
	}
	ret, _, callErr := replaceFileProc.Call(
		uintptr(unsafe.Pointer(replaced)),
		uintptr(unsafe.Pointer(replacement)),
		uintptr(unsafe.Pointer(backup)),
		0,
		0,
		0,
	)
	if ret == 0 {
		return replaceFailure(tempPath, path, backupPath, before, original, callErr)
	}
	backupFile, err := openVerifiedOriginalBackup(backupPath, before, original)
	if err != nil {
		return true, fmt.Errorf("ReplaceFileW replaced target; original backup retained at %q and replacement target remains at %q: %w", backupPath, path, err)
	}
	if err := verifyPreparedFile(path, snapshot, expectedReplacementState(before, snapshot)); err != nil {
		_ = backupFile.Close()
		return true, fmt.Errorf("ReplaceFileW replaced target; original backup retained at %q and replacement target remains at %q: %w", backupPath, path, err)
	}
	if err := deleteFileByHandle(backupFile); err != nil {
		_ = backupFile.Close()
		return true, fmt.Errorf("ReplaceFileW succeeded and target %q was replaced; original backup retained at %q: remove backup by handle: %w", path, backupPath, err)
	}
	if err := backupFile.Close(); err != nil {
		return true, fmt.Errorf("ReplaceFileW succeeded and target %q was replaced; original backup retained at %q: close backup after delete: %w", path, backupPath, err)
	}
	return false, nil
}

func verifyPreparedFile(path string, snapshot preparedFileSnapshot, expected fileState) error {
	source, err := openSource(path)
	if err != nil {
		return err
	}
	defer source.Close()
	state, err := stateFromFile(source)
	if err != nil {
		return err
	}
	if !expected.same(state) {
		return fmt.Errorf("prepared replacement identity or metadata differs from the creator snapshot")
	}
	data, err := io.ReadAll(source)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, snapshot.output) {
		return fmt.Errorf("prepared replacement contents differ from the creator snapshot")
	}
	after, err := stateFromFile(source)
	if err != nil {
		return err
	}
	if !state.same(after) {
		return fmt.Errorf("prepared replacement changed while it was being verified")
	}
	current, err := captureState(path)
	if err != nil {
		return fmt.Errorf("reinspect prepared replacement path: %w", err)
	}
	if !state.same(current) {
		return fmt.Errorf("prepared replacement path changed while it was being verified")
	}
	return nil
}

func expectedReplacementState(before fileState, snapshot preparedFileSnapshot) fileState {
	expected := snapshot.state
	expected.attrs = expected.attrs&^fileAttributesPreservedByReplace | before.attrs&fileAttributesPreservedByReplace
	expected.attrs &^= syscall.FILE_ATTRIBUTE_NORMAL
	if expected.attrs == 0 {
		expected.attrs = syscall.FILE_ATTRIBUTE_NORMAL
	}
	return expected
}

func verifyOriginalBackup(backupPath string, before fileState, original []byte) error {
	source, err := openVerifiedOriginalBackup(backupPath, before, original)
	if err != nil {
		return err
	}
	return source.Close()
}

func openVerifiedOriginalBackup(backupPath string, before fileState, original []byte) (*os.File, error) {
	source, err := openDeleteHandle(backupPath)
	if err != nil {
		return nil, fmt.Errorf("open original backup: %w", err)
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = source.Close()
		}
	}()
	state, err := stateFromFile(source)
	if err != nil {
		return nil, fmt.Errorf("inspect original backup: %w", err)
	}
	if !before.same(state) {
		return nil, fmt.Errorf("original backup identity or metadata differs from the input")
	}
	data, err := io.ReadAll(source)
	if err != nil {
		return nil, fmt.Errorf("read original backup: %w", err)
	}
	if !bytes.Equal(data, original) {
		return nil, fmt.Errorf("original backup content differs from the input")
	}
	after, err := stateFromFile(source)
	if err != nil {
		return nil, fmt.Errorf("reinspect original backup: %w", err)
	}
	if !state.same(after) {
		return nil, fmt.Errorf("original backup changed while it was being verified")
	}
	current, err := captureDeleteHandleState(backupPath)
	if err != nil {
		return nil, fmt.Errorf("reinspect original backup path: %w", err)
	}
	if !state.same(current) {
		return nil, fmt.Errorf("original backup path changed while it was being verified")
	}
	closeOnError = false
	return source, nil
}

func captureDeleteHandleState(path string) (fileState, error) {
	file, err := openDeleteHandle(path)
	if err != nil {
		return fileState{}, err
	}
	defer file.Close()
	return stateFromFile(file)
}

func newBackupPath(path string) (string, error) {
	backup, err := createDeleteFile(sourceDir(path), ".erafmt-backup-", syscall.GENERIC_READ|fileDeleteAccess, syscall.FILE_ATTRIBUTE_NORMAL|fileFlagDeleteOnClose)
	if err != nil {
		return "", err
	}
	backupPath := backup.Name()
	if err := backup.Close(); err != nil {
		return "", fmt.Errorf("prepare backup %q: close delete-on-close reservation: %w", backupPath, err)
	}
	if _, err := os.Lstat(backupPath); err == nil {
		return "", fmt.Errorf("prepare backup %q: reservation path remains after close", backupPath)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("prepare backup %q: inspect reservation after close: %w", backupPath, err)
	}
	return backupPath, nil
}

func openDeleteHandle(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(
		name,
		syscall.GENERIC_READ|fileDeleteAccess,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, err
	}
	if handle == syscall.InvalidHandle {
		return nil, windowsErrorInvalidHandle
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		_ = syscall.CloseHandle(handle)
		return nil, windowsErrorInvalidHandle
	}
	return file, nil
}

func deleteFileByHandle(file *os.File) error {
	info := fileDispositionInfo{DeleteFile: fileDispositionDelete}
	ret, _, err := setFileInformationByHandleProc.Call(
		file.Fd(),
		fileDispositionInformation,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
	)
	if ret == 0 {
		return err
	}
	return nil
}

func createDeleteFile(dir, prefix string, access, flags uint32) (*os.File, error) {
	for attempt := 0; attempt < 32; attempt++ {
		var suffix [16]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return nil, err
		}
		path := filepath.Join(dir, prefix+hex.EncodeToString(suffix[:]))
		name, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			return nil, err
		}
		handle, err := syscall.CreateFile(
			name,
			access,
			syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
			nil,
			syscall.CREATE_NEW,
			flags,
			0,
		)
		if err == nil {
			file := os.NewFile(uintptr(handle), path)
			if file == nil {
				_ = syscall.CloseHandle(handle)
				return nil, windowsErrorInvalidHandle
			}
			return file, nil
		}
		if !errors.Is(err, syscall.ERROR_FILE_EXISTS) && !errors.Is(err, syscall.ERROR_ALREADY_EXISTS) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("unable to reserve a unique backup path")
}

func replaceFailure(tempPath, path, backupPath string, before fileState, original []byte, callErr error) (bool, error) {
	replaceErr := fmt.Errorf("ReplaceFileW failed: %w", callErr)
	_, backupErr := os.Lstat(backupPath)
	backupExists := backupErr == nil
	if backupErr != nil && !os.IsNotExist(backupErr) {
		return true, fmt.Errorf("%w; inspect target %q and backup %q: %w; replacement temporary path retained at %q", replaceErr, path, backupPath, backupErr, tempPath)
	}
	if backupExists {
		if err := verifyOriginalBackup(backupPath, before, original); err != nil {
			return true, fmt.Errorf("%w; target path %q and backup path %q have uncertain replacement state; original backup verification failed: %w; replacement temporary path retained at %q", replaceErr, path, backupPath, err, tempPath)
		}
	}
	_, targetErr := os.Lstat(path)
	if targetErr == nil {
		if backupExists {
			return true, fmt.Errorf("%w; target path %q remains present with replacement state unknown; verified original backup retained at %q; replacement temporary path retained at %q", replaceErr, path, backupPath, tempPath)
		}
		if replaceFailureDidNotConsumeTemp(callErr) {
			return false, fmt.Errorf("%w; target path %q remains present and replacement temporary path %q was not consumed", replaceErr, path, tempPath)
		}
		return true, fmt.Errorf("%w; target path %q remains present; backup %q is absent; replacement temporary path retained at %q", replaceErr, path, backupPath, tempPath)
	}
	if !os.IsNotExist(targetErr) {
		return true, fmt.Errorf("%w; inspect target %q: %w; backup retained at %q; replacement temporary path retained at %q", replaceErr, path, targetErr, backupPath, tempPath)
	}
	if !backupExists {
		return true, fmt.Errorf("%w; target %q is missing and original backup %q is absent; replacement temporary path retained at %q", replaceErr, path, backupPath, tempPath)
	}
	return true, fmt.Errorf("%w; target %q is missing; verified original backup retained at %q for recovery; replacement temporary path retained at %q", replaceErr, path, backupPath, tempPath)
}

func replaceFailureDidNotConsumeTemp(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	switch errno {
	case syscall.ERROR_ACCESS_DENIED, windowsErrorWriteProtect, windowsErrorSharingViolation, windowsErrorLockViolation, windowsErrorUnableToRemoveReplaced, windowsErrorUnableToMoveReplacement, windowsErrorUnableToMoveReplacement2:
		return true
	default:
		return false
	}
}

const (
	windowsErrorWriteProtect             syscall.Errno = 19
	windowsErrorSharingViolation         syscall.Errno = 32
	windowsErrorLockViolation            syscall.Errno = 33
	windowsErrorUnableToRemoveReplaced   syscall.Errno = 1175
	windowsErrorUnableToMoveReplacement  syscall.Errno = 1176
	windowsErrorUnableToMoveReplacement2 syscall.Errno = 1177
)

const fileAttributesToPreserve = syscall.FILE_ATTRIBUTE_HIDDEN |
	syscall.FILE_ATTRIBUTE_SYSTEM |
	syscall.FILE_ATTRIBUTE_ARCHIVE | fileAttributeNotContentIndexed

const fileAttributeNotContentIndexed = 0x00002000

const fileAttributesPreservedByReplace = 0x00000800 | 0x00004000

func preserveFileAttributes(sourceAttrs uint32, temp *os.File) error {
	tempPath, err := syscall.UTF16PtrFromString(temp.Name())
	if err != nil {
		return err
	}
	tempAttrs, err := syscall.GetFileAttributes(tempPath)
	if err != nil {
		return fmt.Errorf("read temporary file attributes: %w", err)
	}
	tempAttrs = tempAttrs&^fileAttributesToPreserve | sourceAttrs&fileAttributesToPreserve
	if tempAttrs == 0 {
		tempAttrs = syscall.FILE_ATTRIBUTE_NORMAL
	}
	if err := syscall.SetFileAttributes(tempPath, tempAttrs); err != nil {
		return fmt.Errorf("preserve file attributes: %w", err)
	}
	return nil
}
