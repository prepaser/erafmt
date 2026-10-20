//go:build !linux && !windows

package cli

func replacePreparedFile(tempPath, path string, before fileState, original []byte, snapshot preparedFileSnapshot) (bool, error) {
	return renamePreparedFile(tempPath, path, before, original, snapshot)
}
