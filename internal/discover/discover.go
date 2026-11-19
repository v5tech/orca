// Package discover finds configuration files under the given paths.
package discover

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var skipDirs = map[string]struct{}{
	"__pycache__": {}, "build": {}, "dist": {}, "logs": {},
	"node_modules": {}, "outputs": {}, "reports": {}, "venv": {},
}

// Files returns unique files under paths. warn reports missing paths.
func Files(paths []string, warn func(string)) ([]string, error) {
	seen := make(map[string]struct{})
	var files []string

	for _, raw := range paths {
		info, err := os.Stat(raw)
		if err != nil {
			if warn != nil {
				warn("跳过不存在的路径: " + raw)
			}
			continue
		}

		if !info.IsDir() {
			appendFile(raw, seen, &files)
			continue
		}

		root := filepath.Clean(raw)
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if path != root && skipDirectory(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			appendFile(path, seen, &files)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	sort.Strings(files)
	return files, nil
}

func skipDirectory(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	_, ok := skipDirs[name]
	return ok
}

func appendFile(path string, seen map[string]struct{}, files *[]string) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}
	if _, ok := seen[absolute]; ok {
		return
	}
	seen[absolute] = struct{}{}
	*files = append(*files, path)
}
