//go:build windows

package cli

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func createReplacementTemp(path string) (*os.File, func(bool) error, error) {
	temp, err := createDeleteFile(sourceDir(path), ".erafmt-", syscall.GENERIC_READ|syscall.GENERIC_WRITE|fileDeleteAccess, syscall.FILE_ATTRIBUTE_NORMAL)
	if err != nil {
		return nil, nil, err
	}
	creator, err := stateFromFile(temp)
	if err != nil {
		deleteErr := deleteFileByHandle(temp)
		return nil, nil, errors.Join(err, deleteErr, temp.Close())
	}
	tempPath := temp.Name()
	cleanup := func(keep bool) (cleanupErr error) {
		if keep {
			return nil
		}
		defer func() {
			if cleanupErr != nil {
				cleanupErr = fmt.Errorf("temporary replacement %q: %w", tempPath, cleanupErr)
			}
		}()
		candidate, err := openDeleteHandle(tempPath)
		if err != nil {
			return fmt.Errorf("open temporary replacement for cleanup: %w", err)
		}
		state, err := stateFromFile(candidate)
		if err == nil && !creator.sameIdentity(state) {
			err = fmt.Errorf("temporary replacement identity changed before cleanup")
		}
		if err == nil {
			err = deleteFileByHandle(candidate)
		}
		return errors.Join(err, candidate.Close())
	}
	return temp, cleanup, nil
}
