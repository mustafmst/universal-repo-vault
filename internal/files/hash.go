package files

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const LockFileName string = ".urv.lock"

type FileHasheCollection struct {
	Hashes map[string]string
}

type FileHash struct {
	Path string
	Hash []byte
}

// GetHexHash return hash of given data in hex format string and error if something goes wrong
func GetHexHash(data []byte) (string, error) {
	h := sha256.New()
	if _, err := h.Write(data); err != nil {
		return "", fmt.Errorf("writing to hash failed: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func GetFileHash(absPath string) (*FileHash, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("opening file for hashing: %w", err)
	}
	defer f.Close()
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
		Hashes: map[string]string{},
	}
	errs := []error{}
	for _, f := range files {
		fh, err := GetFileHash(filepath.Join(basePath, f))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		res.Hashes[f] = fh.GetHexString()
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("collecting hashes for files: %w", errors.Join(errs...))
	}
	return res, nil
}

func OpenLockFile(repoPath string) ([]byte, error) {
	return os.ReadFile(filepath.Join(repoPath, LockFileName))
}

func ParseLockFileBody(body []byte) (map[string]string, error) {
	hashes := map[string]string{}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		path, hash, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("invalid lockfile line: %s", line)
		}

		path = strings.TrimSpace(path)
		hash = strings.TrimSpace(hash)
		if path == "" || hash == "" {
			return nil, fmt.Errorf("invalid lockfile line: %s", line)
		}

		hashes[path] = hash
	}
	return hashes, nil
}

func HashesEqual(a map[string]string, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		if b[k] != av {
			return false
		}
	}
	return true
}
