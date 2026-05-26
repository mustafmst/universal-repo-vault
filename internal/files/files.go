package files

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var ErrNotReplacingFile error = errors.New("file was not replaced")

// ListAllConfiguredFiles gathers a list of absolute file paths for files matching configuration
func ListAllConfiguredFiles(basePath string, fileList []string, patternlist []string) ([]string, error) {
	result := []string{}
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

		for _, f := range fileList {
			if relPath == f {
				result = append(result, relPath)
			}
		}

		for _, p := range patternlist {
			if matched, _ := filepath.Match(p, d.Name()); matched {
				result = append(result, relPath)
			}
		}

		return nil
	})
	if err != nil {
		return result, fmt.Errorf("listing files: %w", err)
	}
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

	return f.Write(data)
}
