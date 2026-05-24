package files

import (
	"crypto/sha256"
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
		return nil, err
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return &FileHash{absPath, h.Sum(nil)}, nil
}

func (h *FileHash) GetHexString() string {
	return fmt.Sprintf("%x", h.Hash)
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
		return nil, errors.Join(errs...)
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
