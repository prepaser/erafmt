//go:build linux

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

func replacePreparedFile(tempPath, path string, before fileState, original []byte, snapshot preparedFileSnapshot) (bool, error) {
	return replacePreparedFileWithOps(tempPath, path, before, original, snapshot, syncDirectory, exchangeFiles)
}

func replacePreparedFileWithOps(tempPath, path string, before fileState, original []byte, snapshot preparedFileSnapshot, sync func(*os.File) error, exchange func(*os.File, string, *os.File, string) error) (bool, error) {
	targetDir, err := os.Open(sourceDir(path))
	if err != nil {
		return false, err
	}
	defer targetDir.Close()
	stageDir, err := os.Open(filepath.Dir(tempPath))
	if err != nil {
		return false, err
	}
	defer stageDir.Close()
	stageParent, err := os.Open(filepath.Dir(filepath.Dir(tempPath)))
	if err != nil {
		return false, err
	}
	defer stageParent.Close()
	stageName := filepath.Base(filepath.Dir(tempPath))
	if !strings.HasPrefix(stageName, ".erafmt-stage-") {
		return false, fmt.Errorf("temporary replacement is outside a private staging directory")
	}
	stageInfo, err := stageDir.Stat()
	if err != nil {
		return false, fmt.Errorf("inspect private staging directory: %w", err)
	}
	if !stageInfo.IsDir() || stageInfo.Mode().Perm() != 0o700 {
		return false, fmt.Errorf("temporary replacement staging directory is not a private directory")
	}
	stageEntry, err := os.Stat(filepath.Join(procFDPath(stageParent.Fd()), stageName))
	if err != nil || !os.SameFile(stageInfo, stageEntry) {
		return false, fmt.Errorf("private staging directory identity changed")
	}
	targetName, tempName := filepath.Base(path), filepath.Base(tempPath)
	targetDirPath := procFDPath(targetDir.Fd())
	stageDirPath := procFDPath(stageDir.Fd())
	displacedPath := filepath.Join(stageDirPath, tempName)
	prepared, err := openSource(displacedPath)
	if err != nil {
		return false, err
	}
	defer prepared.Close()
	preparedState, err := stateFromFile(prepared)
	if err != nil {
		return false, err
	}
	if !snapshot.state.same(preparedState) {
		return false, fmt.Errorf("prepared replacement changed before exchange")
	}
	if err := checkPreparedSnapshot(prepared, snapshot); err != nil {
		return false, err
	}
	attrs, err := readXattrsStable(procFDPath(prepared.Fd()))
	if err != nil {
		return false, fmt.Errorf("read prepared metadata: %w", err)
	}
	if !sameXattrs(snapshot.attrs, attrs) {
		return false, fmt.Errorf("prepared replacement metadata changed while formatting")
	}
	if err := sync(stageDir); err != nil {
		return false, fmt.Errorf("sync private staging directory before exchange: %w", err)
	}
	if err := sync(stageParent); err != nil {
		return false, fmt.Errorf("sync staging parent before exchange: %w", err)
	}
	if err := exchange(stageDir, tempName, targetDir, targetName); err != nil {
		if errors.Is(err, syscall.ENOSYS) || errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.EINVAL) {
			if _, err := renamePreparedFile(tempPath, path, before, original, snapshot); err != nil {
				return false, err
			}
			if err := removeDirectoryAt(stageParent, stageName); err != nil {
				return true, fmt.Errorf("target was replaced using rename; remove empty staging directory %s: %w", recoveryLocation(stageParent, stageName), err)
			}
			if err := syncReplacementDirectories(sync, targetDir, stageParent); err != nil {
				return true, fmt.Errorf("target was replaced using rename; directory sync failed, so durability is unconfirmed: %w", err)
			}
			return true, nil
		}
		return false, fmt.Errorf("atomic file exchange unavailable or failed: %w", err)
	}
	displacedErr := checkDisplacedFile(displacedPath, before, original, attrs)
	if err := checkPreparedFile(prepared, filepath.Join(targetDirPath, targetName), snapshot.state, snapshot.output, attrs); err != nil {
		if syncErr := syncReplacementDirectories(sync, targetDir, stageDir); syncErr != nil {
			return true, fmt.Errorf("%w; directory sync failed; inspect target %s; original backup retained at %s: %w", err, recoveryLocation(targetDir, targetName), recoveryLocation(stageDir, tempName), syncErr)
		}
		return true, fmt.Errorf("%w; inspect target %s; original backup retained at %s", err, recoveryLocation(targetDir, targetName), recoveryLocation(stageDir, tempName))
	}
	if err := displacedErr; err != nil {
		if restoreErr := exchange(stageDir, tempName, targetDir, targetName); restoreErr != nil {
			return true, fmt.Errorf("%w; restore exchange failed: %w; original recovery is uncertain; inspect target %s and displaced path %s", err, restoreErr, recoveryLocation(targetDir, targetName), recoveryLocation(stageDir, tempName))
		}
		if syncErr := syncReplacementDirectories(sync, targetDir, stageDir); syncErr != nil {
			return true, fmt.Errorf("%w; restoration exchange completed but directory sync failed; original recovery is uncertain; inspect target %s and retained output %s: %w", err, recoveryLocation(targetDir, targetName), recoveryLocation(stageDir, tempName), syncErr)
		}
		return true, fmt.Errorf("%w; exchange restored; retained output at %s", err, recoveryLocation(stageDir, tempName))
	}
	if err := syncReplacementDirectories(sync, targetDir, stageDir); err != nil {
		return true, fmt.Errorf("atomic exchange completed but directory sync failed; original backup retained at %s: %w", recoveryLocation(stageDir, tempName), err)
	}
	if err := syscall.Unlinkat(int(stageDir.Fd()), tempName); err != nil {
		return true, fmt.Errorf("target was replaced; remove original backup %s: %w", recoveryLocation(stageDir, tempName), err)
	}
	if err := sync(stageDir); err != nil {
		return true, fmt.Errorf("target was replaced and original backup was removed; directory sync failed, so durability is unconfirmed: %w", err)
	}
	if err := removeDirectoryAt(stageParent, stageName); err != nil {
		return true, fmt.Errorf("target was replaced; remove empty staging directory %q: %w", stageName, err)
	}
	if err := sync(stageParent); err != nil {
		return true, fmt.Errorf("target was replaced and staging directory was removed; directory sync failed, so durability is unconfirmed: %w", err)
	}
	return true, nil
}

func syncReplacementDirectories(sync func(*os.File) error, target, stage *os.File) error {
	if err := sync(target); err != nil {
		return err
	}
	return sync(stage)
}

func recoveryLocation(dir *os.File, basename string) string {
	dirInfo, dirStatErr := dir.Stat()
	resolved, readlinkErr := os.Readlink(procFDPath(dir.Fd()))
	reason := "anchored path unavailable"
	if readlinkErr == nil && resolved != "" && dirStatErr == nil {
		if resolvedInfo, statErr := os.Stat(resolved); statErr == nil {
			if os.SameFile(dirInfo, resolvedInfo) {
				return fmt.Sprintf("path %q", filepath.Join(resolved, basename))
			}
			reason = "resolved path identity changed"
		}
	}
	if readlinkErr == nil && resolved == "" {
		reason = "anchored path is empty"
	}
	identity := "unavailable"
	if dirStatErr == nil {
		if stat, ok := dirInfo.Sys().(*syscall.Stat_t); ok {
			identity = fmt.Sprintf("dev=%d ino=%d", uint64(stat.Dev), uint64(stat.Ino))
		}
	} else {
		identity = fmt.Sprintf("unavailable: %v", dirStatErr)
	}
	return fmt.Sprintf("directory identity %s; entry basename %q (%s)", identity, basename, reason)
}

func syncDirectory(dir *os.File) error {
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	return nil
}

func checkPreparedFile(f *os.File, path string, expected fileState, output []byte, attrs map[string][]byte) error {
	state, err := stateFromFile(f)
	if err != nil || !expected.sameDisplaced(state) {
		return fmt.Errorf("replacement changed while formatting")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	if !bytes.Equal(output, data) {
		return fmt.Errorf("replacement contents changed while formatting")
	}
	actual, err := readXattrsStable(procFDPath(f.Fd()))
	if err != nil {
		return err
	}
	if !sameXattrs(attrs, actual) {
		return fmt.Errorf("replacement metadata changed while formatting")
	}
	after, err := stateFromFile(f)
	if err != nil || !state.same(after) {
		return fmt.Errorf("replacement changed while verifying")
	}
	current, err := captureState(path)
	if err != nil || !after.same(current) {
		return fmt.Errorf("replacement path changed while verifying")
	}
	return nil
}

func checkPreparedSnapshot(f *os.File, snapshot preparedFileSnapshot) error {
	state, err := stateFromFile(f)
	if err != nil {
		return err
	}
	if !snapshot.state.same(state) {
		return fmt.Errorf("prepared replacement changed while formatting")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	if !bytes.Equal(snapshot.output, data) {
		return fmt.Errorf("prepared replacement contents changed while formatting")
	}
	after, err := stateFromFile(f)
	if err != nil {
		return err
	}
	if !snapshot.state.same(after) {
		return fmt.Errorf("prepared replacement changed while verifying")
	}
	return nil
}

func checkDisplacedFile(path string, before fileState, original []byte, attrs map[string][]byte) error {
	f, err := openSource(path)
	if err != nil {
		return fmt.Errorf("open displaced original: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	state, err := stateFromInfo(info)
	if err != nil || !before.sameDisplaced(state) {
		return fmt.Errorf("file changed while formatting")
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	if !bytes.Equal(original, data) {
		return fmt.Errorf("file contents changed while formatting")
	}
	actual, err := readXattrsStable(procFDPath(f.Fd()))
	if err != nil {
		return fmt.Errorf("read displaced metadata: %w", err)
	}
	if !sameXattrs(attrs, actual) {
		return fmt.Errorf("file metadata changed while formatting")
	}
	info, err = f.Stat()
	if err != nil {
		return err
	}
	after, err := stateFromInfo(info)
	if err != nil || !state.same(after) {
		return fmt.Errorf("file changed while verifying replacement")
	}
	current, err := captureState(path)
	if err != nil || !after.same(current) {
		return fmt.Errorf("displaced original path changed while verifying replacement")
	}
	return nil
}

func (s fileState) sameDisplaced(other fileState) bool {
	// Exchanging directory entries updates ctime on both inodes.
	s.meta.ctime = other.meta.ctime
	return s.same(other)
}

func exchangeFiles(leftDir *os.File, left string, rightDir *os.File, right string) error {
	var number uintptr
	switch runtime.GOARCH {
	case "amd64":
		number = 316
	case "386":
		number = 353
	case "arm":
		number = 382
	case "arm64", "loong64", "riscv64":
		number = 276
	case "ppc64", "ppc64le":
		number = 357
	case "mips", "mipsle":
		number = 4351
	case "mips64", "mips64le":
		number = 5311
	case "s390x":
		number = 347
	default:
		return syscall.ENOSYS
	}
	oldPath, err := syscall.BytePtrFromString(left)
	if err != nil {
		return err
	}
	newPath, err := syscall.BytePtrFromString(right)
	if err != nil {
		return err
	}
	const renameExchange = 2
	_, _, errno := syscall.Syscall6(number, leftDir.Fd(), uintptr(unsafe.Pointer(oldPath)), rightDir.Fd(), uintptr(unsafe.Pointer(newPath)), renameExchange, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
