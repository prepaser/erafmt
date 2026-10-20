//go:build !windows

package cli

import (
	"os"
	"syscall"
)

func openSource(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
