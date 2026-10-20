package cli

import (
	"fmt"
	"io"
)

func readSource(path string, expected *fileState) ([]byte, fileState, error) {
	var before fileState
	if expected != nil {
		before = *expected
	} else {
		var err error
		before, err = captureState(path)
		if err != nil {
			return nil, fileState{}, err
		}
	}

	source, err := openSource(path)
	if err != nil {
		return nil, fileState{}, err
	}
	defer source.Close()

	opened, err := stateFromFile(source)
	if err != nil {
		return nil, fileState{}, err
	}
	if !before.same(opened) {
		if expected != nil {
			return nil, fileState{}, fmt.Errorf("file changed after discovery")
		}
		return nil, fileState{}, fmt.Errorf("file changed while opening")
	}

	original, err := io.ReadAll(source)
	if err != nil {
		return nil, fileState{}, err
	}
	after, err := stateFromFile(source)
	if err != nil {
		return nil, fileState{}, err
	}
	if !before.same(after) {
		return nil, fileState{}, fmt.Errorf("file changed while reading")
	}

	current, err := captureState(path)
	if err != nil {
		return nil, fileState{}, err
	}
	if !before.same(current) {
		return nil, fileState{}, fmt.Errorf("file changed while reading")
	}
	return original, before, nil
}
