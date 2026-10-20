package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type discoveryError struct {
	path string
	err  error
}

func (e discoveryError) Error() string { return e.err.Error() }

func (e discoveryError) Unwrap() error { return e.err }

type discoveredTarget struct {
	path  string
	state fileState
}

type discoveryResult struct {
	targets     []discoveredTarget
	errors      []error
	containsDir bool
}

func discoverTargetsWithMetadata(args []string) discoveryResult {
	seen := make(map[string][]discoveredTarget)
	result := discoveryResult{}
	add := func(path string) {
		state, err := captureState(path)
		if err != nil {
			result.errors = append(result.errors, discoveryError{path: path, err: err})
			return
		}
		target := discoveredTarget{path: path, state: state}
		key := canonicalPath(path)
		for _, previous := range seen[key] {
			if previous.state.sameIdentity(target.state) {
				return
			}
		}
		seen[key] = append(seen[key], target)
		result.targets = append(result.targets, target)
	}

	for _, root := range args {
		info, err := os.Lstat(symlinkInspectionPath(root))
		if err != nil {
			result.errors = append(result.errors, discoveryError{path: root, err: err})
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			result.errors = append(result.errors, discoveryError{path: root, err: fmt.Errorf("symbolic links are skipped")})
			continue
		}
		if info.IsDir() {
			result.containsDir = true
			walkRoot, err := filepath.EvalSymlinks(root)
			if err != nil {
				result.errors = append(result.errors, discoveryError{path: root, err: err})
				continue
			}
			if isVCSDir(filepath.Base(walkRoot)) {
				continue
			}
			err = filepath.WalkDir(walkRoot, func(walkPath string, entry fs.DirEntry, walkErr error) error {
				rel, err := filepath.Rel(walkRoot, walkPath)
				if err != nil {
					return err
				}
				path := root
				if rel != "." {
					path = appendSourcePath(root, rel)
				}
				if walkErr != nil {
					result.errors = append(result.errors, discoveryError{path: path, err: walkErr})
					return nil
				}
				if entry.IsDir() {
					if path != root && isVCSDir(entry.Name()) {
						return fs.SkipDir
					}
					return nil
				}
				if entry.Type()&os.ModeSymlink != 0 {
					return nil
				}
				if isTarget(path) {
					add(path)
				}
				return nil
			})
			if err != nil {
				result.errors = append(result.errors, discoveryError{path: root, err: err})
			}
			continue
		}
		if !info.Mode().IsRegular() {
			result.errors = append(result.errors, discoveryError{path: root, err: fmt.Errorf("not a regular file")})
			continue
		}
		if !isTarget(root) {
			result.errors = append(result.errors, discoveryError{path: root, err: fmt.Errorf("unsupported file extension")})
			continue
		}
		add(root)
	}
	sort.Slice(result.targets, func(i, j int) bool { return result.targets[i].path < result.targets[j].path })
	return result
}

func symlinkInspectionPath(path string) string {
	path = trimTrailingSeparators(path)
	for len(path) >= 2 && path[len(path)-1] == '.' && os.IsPathSeparator(path[len(path)-2]) {
		prefix := path[:len(path)-2]
		if prefix == "" && os.IsPathSeparator(path[0]) {
			prefix = path[:1]
		}
		if volume := filepath.VolumeName(prefix); volume != "" && len(prefix) == len(volume) && len(path) > len(prefix) && os.IsPathSeparator(path[len(prefix)]) {
			prefix = path[:len(prefix)+1]
		}
		path = trimTrailingSeparators(prefix)
	}
	return path
}

func trimTrailingSeparators(path string) string {
	end := len(path)
	for end > 0 && os.IsPathSeparator(path[end-1]) {
		end--
	}
	if end == 0 {
		if path == "" {
			return path
		}
		return path[:1]
	}
	if end == len(path) {
		return path
	}
	trimmed := path[:end]
	if volume := filepath.VolumeName(trimmed); volume != "" && end == len(volume) {
		return path[:end] + string(filepath.Separator)
	}
	return trimmed
}

func canonicalPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

// Keep .. intact until the filesystem resolves ancestor symlinks.
func sourceDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if os.IsPathSeparator(path[i]) {
			return path[:i+1]
		}
	}
	if volume := filepath.VolumeName(path); volume != "" {
		return volume + "."
	}
	return "."
}

func appendSourcePath(dir, name string) string {
	if dir == "" {
		return name
	}
	if os.IsPathSeparator(dir[len(dir)-1]) || dir == filepath.VolumeName(dir) && !filepath.IsAbs(dir) {
		return dir + name
	}
	return dir + string(filepath.Separator) + name
}

func isTarget(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".erb" || ext == ".erh"
}

func isVCSDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".hg", ".svn", ".bzr":
		return true
	default:
		return false
	}
}
