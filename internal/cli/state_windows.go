//go:build windows

package cli

import (
	"fmt"
	"os"
	"syscall"
)

type fileState struct {
	volume uint32
	index  uint64
	size   int64
	mtime  int64
	nlink  uint32
	mode   os.FileMode
	attrs  uint32
}

func captureState(path string) (fileState, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return fileState{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fileState{}, fmt.Errorf("symbolic links are not writable")
	}
	if !info.Mode().IsRegular() {
		return fileState{}, fmt.Errorf("not a regular file")
	}
	f, err := openSource(path)
	if err != nil {
		return fileState{}, err
	}
	defer f.Close()
	return stateFromFile(f)
}

func stateFromFile(f *os.File) (fileState, error) {
	info, err := f.Stat()
	if err != nil {
		return fileState{}, err
	}
	var data syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &data); err != nil {
		return fileState{}, err
	}
	if data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.Mode()&os.ModeSymlink != 0 {
		return fileState{}, fmt.Errorf("symbolic links are not writable")
	}
	if !info.Mode().IsRegular() {
		return fileState{}, fmt.Errorf("not a regular file")
	}
	return fileState{
		volume: data.VolumeSerialNumber,
		index:  uint64(data.FileIndexHigh)<<32 | uint64(data.FileIndexLow),
		size:   info.Size(),
		mtime:  info.ModTime().UnixNano(),
		nlink:  data.NumberOfLinks,
		mode:   info.Mode(),
		attrs:  data.FileAttributes,
	}, nil
}

func (s fileState) same(other fileState) bool {
	return s.volume == other.volume && s.index == other.index && s.size == other.size && s.mtime == other.mtime && s.nlink == other.nlink && s.mode == other.mode && s.attrs == other.attrs
}

func (s fileState) sameIdentity(other fileState) bool {
	return s.volume == other.volume && s.index == other.index
}

func (s fileState) hardlink() bool { return s.nlink > 1 }
