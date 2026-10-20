//go:build !windows

package cli

import (
	"fmt"
	"os"
	"syscall"
)

type fileState struct {
	dev   uint64
	ino   uint64
	size  int64
	mtime int64
	nlink uint64
	mode  os.FileMode
	meta  fileMetadataState
}

func captureState(path string) (fileState, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return fileState{}, err
	}
	return stateFromInfo(info)
}

func stateFromInfo(info os.FileInfo) (fileState, error) {
	if info.Mode()&os.ModeSymlink != 0 {
		return fileState{}, fmt.Errorf("symbolic links are not writable")
	}
	if !info.Mode().IsRegular() {
		return fileState{}, fmt.Errorf("not a regular file")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileState{}, fmt.Errorf("file identity is unavailable")
	}
	return fileState{dev: uint64(stat.Dev), ino: uint64(stat.Ino), size: info.Size(), mtime: info.ModTime().UnixNano(), nlink: uint64(stat.Nlink), mode: info.Mode(), meta: metadataState(info)}, nil
}

func (s fileState) same(other fileState) bool {
	return s.dev == other.dev && s.ino == other.ino && s.size == other.size && s.mtime == other.mtime && s.mode == other.mode && s.nlink == other.nlink && s.meta == other.meta
}

func (s fileState) sameIdentity(other fileState) bool {
	return s.dev == other.dev && s.ino == other.ino
}

func (s fileState) hardlink() bool { return s.nlink > 1 }
