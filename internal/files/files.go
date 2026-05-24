package files

import (
	"io/fs"
	"log"
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

		log.Printf("Rel path: %s", relPath)

		// for _, f := range fileList

		return nil
	})
	if err != nil {
		return result, err
	}
	return result, nil
}
