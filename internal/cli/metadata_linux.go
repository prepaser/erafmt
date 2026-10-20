//go:build linux

package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func preserveMetadata(path string, temp *os.File, before fileState) error {
	source, err := openSource(path)
	if err != nil {
		return fmt.Errorf("open source metadata: %w", err)
	}
	defer source.Close()

	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("stat source metadata: %w", err)
	}
	current, err := stateFromInfo(info)
	if err != nil {
		return err
	}
	if !before.same(current) || current.hardlink() {
		return fmt.Errorf("file changed while reading metadata")
	}

	tempInfo, err := temp.Stat()
	if err != nil {
		return fmt.Errorf("stat temporary metadata: %w", err)
	}
	tempStat := tempInfo.Sys().(*syscall.Stat_t)
	if before.meta.uid != tempStat.Uid || before.meta.gid != tempStat.Gid {
		if err := temp.Chown(int(before.meta.uid), int(before.meta.gid)); err != nil {
			return fmt.Errorf("preserve owner: %w", err)
		}
	}

	sourcePath := procFDPath(source.Fd())
	tempPath := procFDPath(temp.Fd())
	sourceAttrs, err := readXattrsStable(sourcePath)
	if err != nil {
		return fmt.Errorf("read source xattrs: %w", err)
	}
	tempAttrs, err := readXattrsStable(tempPath)
	if err != nil {
		return fmt.Errorf("read temporary xattrs: %w", err)
	}
	for name := range tempAttrs {
		if _, ok := sourceAttrs[name]; ok {
			continue
		}
		if err := syscall.Removexattr(tempPath, name); err != nil {
			return fmt.Errorf("remove temporary xattr %q: %w", name, err)
		}
	}
	for name, value := range sourceAttrs {
		if existing, ok := tempAttrs[name]; ok && bytes.Equal(existing, value) {
			continue
		}
		if err := syscall.Setxattr(tempPath, name, value, 0); err != nil {
			return fmt.Errorf("set temporary xattr %q: %w", name, err)
		}
	}

	if err := temp.Chmod(modeForWrite(before.mode)); err != nil {
		return fmt.Errorf("preserve mode: %w", err)
	}

	finalAttrs, err := readXattrsStable(tempPath)
	if err != nil {
		return fmt.Errorf("verify temporary xattrs: %w", err)
	}
	if !sameXattrs(sourceAttrs, finalAttrs) {
		return fmt.Errorf("temporary xattrs changed while preserving metadata")
	}
	return nil
}

func capturePreparedAttributes(temp *os.File) (map[string][]byte, error) {
	return readXattrsStable(procFDPath(temp.Fd()))
}

func procFDPath(fd uintptr) string {
	return filepath.Join("/proc/self/fd", fmt.Sprint(fd))
}

func readXattrsStable(path string) (map[string][]byte, error) {
	first, err := readXattrs(path)
	if err != nil {
		return nil, err
	}
	second, err := readXattrs(path)
	if err != nil {
		return nil, err
	}
	if !sameXattrs(first, second) {
		return nil, fmt.Errorf("xattrs changed while reading")
	}
	return first, nil
}

func readXattrs(path string) (map[string][]byte, error) {
	size, err := syscall.Listxattr(path, nil)
	if err != nil {
		return nil, err
	}
	if size == 0 {
		return map[string][]byte{}, nil
	}
	if size < 0 {
		return nil, fmt.Errorf("invalid xattr list size %d", size)
	}
	list := make([]byte, size)
	actual, err := syscall.Listxattr(path, list)
	if err != nil {
		return nil, err
	}
	if actual != size {
		return nil, fmt.Errorf("xattr list changed while reading: size %d, got %d", size, actual)
	}

	attrs := make(map[string][]byte)
	start := 0
	for i, value := range list {
		if value != 0 {
			continue
		}
		if i == start {
			return nil, fmt.Errorf("invalid empty xattr name")
		}
		name := string(list[start:i])
		data, err := readXattr(path, name)
		if err != nil {
			return nil, fmt.Errorf("read xattr %q: %w", name, err)
		}
		attrs[name] = data
		start = i + 1
	}
	if start != len(list) {
		return nil, fmt.Errorf("invalid xattr list")
	}
	return attrs, nil
}

func readXattr(path, name string) ([]byte, error) {
	size, err := syscall.Getxattr(path, name, nil)
	if err != nil {
		return nil, err
	}
	if size < 0 {
		return nil, fmt.Errorf("invalid xattr size %d", size)
	}
	data := make([]byte, size)
	actual, err := syscall.Getxattr(path, name, data)
	if err != nil {
		return nil, err
	}
	if actual != size {
		return nil, fmt.Errorf("xattr %q changed while reading: size %d, got %d", name, size, actual)
	}
	return data, nil
}

func sameXattrs(left, right map[string][]byte) bool {
	if len(left) != len(right) {
		return false
	}
	for name, value := range left {
		other, ok := right[name]
		if !ok || !bytes.Equal(value, other) {
			return false
		}
	}
	return true
}
