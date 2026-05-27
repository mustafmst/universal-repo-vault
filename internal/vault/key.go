package vault

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mustafmst/universal-repo-vault/internal/files"
	"go.yaml.in/yaml/v3"
)

const keyVariable string = "URV_KEY_NAME"

const keyLength int = 32

type KeyMapping struct {
	keys map[string]string
}

func (k *KeyMapping) Get(repo string) (string, error) {
	key, ok := k.keys[repo]
	if !ok {
		return "", fmt.Errorf("key for repo not found: %s", repo)
	}
	return key, nil
}

func (k *KeyMapping) Add(repo, keyName string, replace bool) error {
	_, ok := k.keys[repo]
	if ok && !replace {
		return fmt.Errorf("key exists for this repo, use --force to change")
	}
	k.keys[repo] = keyName
	return nil
}

func (k *KeyMapping) Save() error {
	data, err := yaml.Marshal(&k.keys)
	if err != nil {
		return fmt.Errorf("serializing key mapping file: %w", err)
	}
	_, err = files.SaveDataToFile(filepath.Join(os.Getenv("HOME"), ".config", "urv", "mapping.yaml"), data, true)
	if err != nil {
		return fmt.Errorf("saving key mapping file: %w", err)
	}
	return nil
}

func NewKeyMapping() (*KeyMapping, error) {
	file := filepath.Join(os.Getenv("HOME"), ".config", "urv", "mapping.yaml")
	rawData, err := os.ReadFile(file)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return &KeyMapping{make(map[string]string)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading key mapping file: %w", err)
	}
	keys := make(map[string]string)
	err = yaml.Unmarshal(rawData, &keys)
	if err != nil {
		return nil, fmt.Errorf("deserializing key mapping file: %w", err)
	}
	return &KeyMapping{keys}, nil
}

func GenNewKey() (string, error) {
	keyBytes := make([]byte, keyLength)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(keyBytes), nil
}

func SaveKeyWithRepoName(key, repoPath string) error {
	return SaveKey(key, repoPath, filepath.Base(repoPath))
}

func SaveKey(key string, repoPath string, keyName string) error {
	if err := ensureKeysDir(); err != nil {
		return fmt.Errorf("creating keys directory: %w", err)
	}

	keyFile := filepath.Join(os.Getenv("HOME"), ".config", "urv", "keys", keyName)
	if _, err := os.Stat(keyFile); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("file %s already exists: %w", keyFile, err)
	}

	f, err := os.Create(keyFile)
	if err != nil {
		return err
	}
	defer f.Close()

	n, err := f.WriteString(key)
	if err != nil {
		return err
	}
	if len(key) != n {
		return fmt.Errorf("key save corrupted, key len: %d, written: %d", len(key), n)
	}
	log.Printf("DEBUG: key lenght written: %d, set key lenght: %d", n, keyLength)

	mapping, err := NewKeyMapping()
	if err != nil {
		return err
	}
	err = mapping.Add(repoPath, keyName, true)
	if err != nil {
		return err
	}
	return mapping.Save()
}

func GetKeyForRepo(repoPath string) (string, error) {
	result := ""

	mapping, err := NewKeyMapping()
	if err != nil {
		return "", err
	}
	keyName, err := mapping.Get(repoPath)
	if err != nil {
		return "", err
	}

	keyFile := filepath.Join(os.Getenv("HOME"), ".config", "urv", "keys", keyName)
	if _, err := os.Stat(keyFile); errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("key file %s does not exists: %w", keyFile, err)
	}

	f, err := os.Open(keyFile)
	if err != nil {
		return result, err
	}
	defer f.Close()

	buff := make([]byte, 2*keyLength)
	if n, err := f.Read(buff); err != nil || n != 2*keyLength {
		return result, fmt.Errorf("reading key from: %s, key len: %d, read: %d, err: %w", keyFile, keyLength, n, err)
	}

	result = string(buff)

	return result, nil
}
