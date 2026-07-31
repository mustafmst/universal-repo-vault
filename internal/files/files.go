package files

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

var ErrNotReplacingFile error = errors.New("file was not replaced")

// ListAllConfiguredFiles gathers a list of absolute file paths for files matching configuration
func ListAllConfiguredFiles(basePath string, fileList []string, patternlist []string) ([]string, error) {
	explicit := map[string]struct{}{}
	for _, f := range fileList {
		explicit[filepath.ToSlash(filepath.Clean(f))] = struct{}{}
	}

	for _, p := range patternlist {
		if _, err := filepath.Match(p, ""); err != nil {
			return nil, fmt.Errorf("invalid file pattern %q: %w", p, err)
		}
	}

	seen := map[string]struct{}{}
	err := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(filepath.Clean(relPath))

		matched := false
		if _, ok := explicit[relPath]; ok {
			matched = true
		}

		for _, p := range patternlist {
			ok, err := filepath.Match(p, d.Name())
			if err != nil {
				return fmt.Errorf("invalid file pattern %q: %w", p, err)
			}
			if ok {
				matched = true
			}
		}

		if matched {
			seen[relPath] = struct{}{}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing files: %w", err)
	}

	result := make([]string, 0, len(seen))
	for path := range seen {
		result = append(result, path)
	}
	sort.Strings(result)
	return result, nil
}

// SaveDataToFile removes existing file if replace is true and creates new to save given data
func SaveDataToFile(fullPath string, data []byte, replace bool) (int, error) {
	stat, err := os.Stat(fullPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return 0, fmt.Errorf("could not sts the file %s: %w", fullPath, err)
	}

	exist := !errors.Is(err, os.ErrNotExist) && !stat.IsDir()
	if exist && !replace {
		return 0, fmt.Errorf("%s: %w", fullPath, ErrNotReplacingFile)
	}

	if exist {
		err := os.Remove(fullPath)
		if err != nil {
			return 0, fmt.Errorf("could not remove file %s: %w", fullPath, errors.Join(ErrNotReplacingFile, err))
		}
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return 0, fmt.Errorf("creating file %s: %w", fullPath, err)
	}
	defer f.Close()

	n, err := f.Write(data)
	if err != nil {
		return n, err
	}
	return n, nil
}
