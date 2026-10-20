//go:build !linux && !windows

package cli

import (
	"os"
	"syscall"
)

type fileMetadataState struct{ uid, gid uint32 }

func metadataState(info os.FileInfo) fileMetadataState {
	stat := info.Sys().(*syscall.Stat_t)
	return fileMetadataState{uid: stat.Uid, gid: stat.Gid}
}

func capturePreparedAttributes(*os.File) (map[string][]byte, error) { return nil, nil }

func preserveMetadata(_ string, temp *os.File, before fileState) error {
	info, err := temp.Stat()
	if err != nil {
		return err
	}
	if metadataState(info) != before.meta {
		if err := temp.Chown(int(before.meta.uid), int(before.meta.gid)); err != nil {
			return err
		}
	}
	return temp.Chmod(modeForWrite(before.mode))
}
