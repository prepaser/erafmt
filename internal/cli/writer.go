package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
)

type preparedFileSnapshot struct {
	state  fileState
	output []byte
	attrs  map[string][]byte
}

func replaceFile(path string, before fileState, original, output []byte) (resultErr error) {
	if bytes.Equal(original, output) {
		return nil
	}
	current, err := captureState(path)
	if err != nil {
		return err
	}
	if !before.same(current) {
		return fmt.Errorf("file changed while formatting")
	}
	if current.hardlink() {
		return fmt.Errorf("hard-linked files are not writable")
	}

	temp, cleanup, err := createReplacementTemp(path)
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	keepTemp := false
	defer func() {
		if err := cleanup(keepTemp); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("clean up temporary replacement: %w", err))
		}
	}()
	if _, err := io.Copy(temp, bytes.NewReader(output)); err != nil {
		_ = temp.Close()
		return err
	}
	if err := preserveMetadata(path, temp, before); err != nil {
		_ = temp.Close()
		return fmt.Errorf("preserve file metadata: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	preparedState, err := stateFromFile(temp)
	if err != nil {
		_ = temp.Close()
		return err
	}
	preparedAttrs, err := capturePreparedAttributes(temp)
	if err != nil {
		_ = temp.Close()
		return err
	}
	snapshot := preparedFileSnapshot{state: preparedState, output: append([]byte(nil), output...), attrs: preparedAttrs}
	if err := temp.Close(); err != nil {
		return err
	}
	current, err = captureState(path)
	if err != nil {
		return err
	}
	if !before.same(current) {
		return fmt.Errorf("file changed while formatting")
	}
	if current.hardlink() {
		return fmt.Errorf("hard-linked files are not writable")
	}
	retainTemp, err := replacePreparedFile(tempPath, path, before, original, snapshot)
	keepTemp = retainTemp || err == nil
	if err != nil {
		return fmt.Errorf("replace file: %w", err)
	}
	return nil
}

func renamePreparedFile(tempPath, path string, before fileState, original []byte, snapshot preparedFileSnapshot) (bool, error) {
	prepared, _, err := readSource(tempPath, &snapshot.state)
	if err != nil {
		return false, fmt.Errorf("verify prepared replacement: %w", err)
	}
	if !bytes.Equal(prepared, snapshot.output) {
		return false, fmt.Errorf("prepared replacement contents changed")
	}
	current, _, err := readSource(path, &before)
	if err != nil {
		return false, fmt.Errorf("verify original: %w", err)
	}
	if !bytes.Equal(current, original) {
		return false, fmt.Errorf("file contents changed while formatting")
	}
	return false, os.Rename(tempPath, path)
}

func modeForWrite(mode os.FileMode) os.FileMode {
	return mode.Perm() | mode&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky)
}
