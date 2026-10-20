package cli

import (
	"os"
	"syscall"
)

type fileMetadataState struct {
	ctime    syscall.Timespec
	uid, gid uint32
}

func metadataState(info os.FileInfo) fileMetadataState {
	stat := info.Sys().(*syscall.Stat_t)
	return fileMetadataState{ctime: stat.Ctim, uid: stat.Uid, gid: stat.Gid}
}
