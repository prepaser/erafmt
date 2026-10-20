//go:build windows

package cli

import (
	"os"
	"syscall"
)

func openSource(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
}
