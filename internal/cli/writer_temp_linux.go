//go:build linux

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const atRemovedir = 0x200

func createReplacementTemp(path string) (*os.File, func(bool) error, error) {
	parent, err := os.Open(sourceDir(path))
	if err != nil {
		return nil, nil, err
	}
	stagePath, err := os.MkdirTemp(procFDPath(parent.Fd()), ".erafmt-stage-*")
	if err != nil {
		_ = parent.Close()
		return nil, nil, err
	}
	stageName := filepath.Base(stagePath)
	stage, err := os.Open(stagePath)
	if err != nil {
		_ = removeDirectoryAt(parent, stageName)
		_ = parent.Close()
		return nil, nil, err
	}
	temp, err := os.CreateTemp(stagePath, "replacement-*")
	if err != nil {
		_ = stage.Close()
		_ = removeDirectoryAt(parent, stageName)
		_ = parent.Close()
		return nil, nil, err
	}
	tempName := filepath.Base(temp.Name())
	creator, err := stateFromFile(temp)
	if err != nil {
		_ = temp.Close()
		_ = syscall.Unlinkat(int(stage.Fd()), tempName)
		_ = stage.Close()
		_ = removeDirectoryAt(parent, stageName)
		_ = parent.Close()
		return nil, nil, err
	}
	cleanup := func(keep bool) error {
		if keep {
			return errors.Join(stage.Close(), parent.Close())
		}
		candidate, err := openSource(filepath.Join(procFDPath(stage.Fd()), tempName))
		if err == nil {
			state, stateErr := stateFromFile(candidate)
			if stateErr != nil {
				err = stateErr
			} else if !creator.sameIdentity(state) {
				err = fmt.Errorf("temporary replacement identity changed before cleanup")
			}
			if closeErr := candidate.Close(); closeErr != nil {
				err = errors.Join(err, closeErr)
			}
		}
		if err == nil {
			err = syscall.Unlinkat(int(stage.Fd()), tempName)
		}
		if err == nil {
			err = removeDirectoryAt(parent, stageName)
		}
		return errors.Join(err, stage.Close(), parent.Close())
	}
	return temp, cleanup, nil
}

func removeDirectoryAt(parent *os.File, name string) error {
	path, err := syscall.BytePtrFromString(name)
	if err != nil {
		return err
	}
	_, _, errno := syscall.Syscall(syscall.SYS_UNLINKAT, parent.Fd(), uintptr(unsafe.Pointer(path)), atRemovedir)
	if errno != 0 {
		return errno
	}
	return nil
}
