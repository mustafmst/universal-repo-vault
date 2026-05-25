package files

import (
	"fmt"
	"io/fs"
	"path/filepath"
)

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
