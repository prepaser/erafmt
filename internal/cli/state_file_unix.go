//go:build !windows

package cli

import "os"

func stateFromFile(file *os.File) (fileState, error) {
	info, err := file.Stat()
	if err != nil {
		return fileState{}, err
	}
	return stateFromInfo(info)
}
