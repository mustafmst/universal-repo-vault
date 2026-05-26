package files

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type FileHasheCollection struct {
	Hashes     map[string]string
	sortedKeys []string
}

type FileHash struct {
	Path string
	Hash []byte
}

func GetFileHash(absPath string) (*FileHash, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("opening file for hashing: %w", err)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("coping from file to hash: %w", err)
	}
	return &FileHash{absPath, h.Sum(nil)}, nil
}

func (h *FileHash) GetHexString() string {
	return hex.EncodeToString(h.Hash)
}

func NewFileHashCollection(basePath string, files []string) (*FileHasheCollection, error) {
	res := &FileHasheCollection{
		Hashes:     map[string]string{},
		sortedKeys: []string{},
	}
	errs := []error{}
	for _, f := range files {
		fh, err := GetFileHash(filepath.Join(basePath, f))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		res.Hashes[f] = fh.GetHexString()
		res.sortedKeys = append(res.sortedKeys, f)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("collecting hashes for files: %w", errors.Join(errs...))
	}
	sort.Strings(res.sortedKeys)
	return res, nil
}

func (c *FileHasheCollection) GetLockfileBody() []byte {
	body := []byte{}
	for _, k := range c.sortedKeys {
		body = append(body, []byte(fmt.Sprintf("%s: %s\n", k, c.Hashes[k]))...)
	}
	return body
}

func SaveLockFile(filePath string, body []byte) error {
	n, err := SaveDataToFile(filePath, body, true)
	if err != nil {
		return err
	}
	if n != len(body) {
		return fmt.Errorf("lockafile save incomplete")
	}
}
