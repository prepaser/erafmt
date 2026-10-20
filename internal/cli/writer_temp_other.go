//go:build !linux && !windows

package cli

import (
	"fmt"
	"os"
)

func createReplacementTemp(path string) (*os.File, func(bool) error, error) {
	temp, err := os.CreateTemp(sourceDir(path), ".erafmt-*")
	if err != nil {
		return nil, nil, err
	}
	creator, err := stateFromFile(temp)
	if err != nil {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
		return nil, nil, err
	}
	cleanup := func(keep bool) error {
		if keep {
			return nil
		}
		current, err := captureState(temp.Name())
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !creator.sameIdentity(current) {
			return fmt.Errorf("temporary replacement identity changed before cleanup: %q", temp.Name())
		}
		return os.Remove(temp.Name())
	}
	return temp, cleanup, nil
}
