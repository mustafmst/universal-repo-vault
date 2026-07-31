package keystore

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mustafmst/universal-repo-vault/internal/files"
	"go.yaml.in/yaml/v3"
)

const KeyVariable = "URV_KEY_NAME"
const KeyLength = 32

type KeyMapping struct {
	keys  map[string]string
	store *FileStore
}

type FileStore struct {
	home string
}

func NewDefaultFileStore() *FileStore {
	return NewFileStore(os.Getenv("HOME"))
}

func NewFileStore(home string) *FileStore {
	return &FileStore{home: home}
}

func (fs *FileStore) keysDir() string {
	return filepath.Join(fs.home, ".config", "urv", "keys")
}

func (fs *FileStore) mappingPath() string {
	return filepath.Join(fs.home, ".config", "urv", "mapping.yaml")
}

func (fs *FileStore) keyPath(keyName string) string {
	return filepath.Join(fs.keysDir(), keyName)
}

func (k *KeyMapping) List() {
	for k, v := range k.keys {
		fmt.Printf("%s -> %s\n", k, v)
	}
}

func (k *KeyMapping) String() string {
	repos := make([]string, 0, len(k.keys))
	for repo := range k.keys {
		repos = append(repos, repo)
	}
	sort.Strings(repos)

	var b strings.Builder
	for _, repo := range repos {
		fmt.Fprintf(&b, "%s -> %s\n", repo, k.keys[repo])
	}
	return b.String()
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

func (k *KeyMapping) UseKeyForRepo(keyName string, repoPath string) error {
	if k.store == nil {
		k.store = NewDefaultFileStore()
	}
	if _, err := os.Stat(k.store.keyPath(keyName)); err != nil {
		return fmt.Errorf("key file not found: %w", err)
	}

	k.keys[repoPath] = keyName
	return nil
}

func (k *KeyMapping) Save() error {
	if k.store == nil {
		k.store = NewDefaultFileStore()
	}
	data, err := yaml.Marshal(&k.keys)
	if err != nil {
		return fmt.Errorf("serializing key mapping file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(k.store.mappingPath()), 0o755); err != nil {
		return fmt.Errorf("creating key mapping directory: %w", err)
	}
	_, err = files.SaveDataToFile(k.store.mappingPath(), data, true)
	if err != nil {
		return fmt.Errorf("saving key mapping file: %w", err)
	}
	return nil
}

func GenerateKey() (string, error) {
	keyBytes := make([]byte, KeyLength)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(keyBytes), nil
}

func (fs *FileStore) Mapping() (*KeyMapping, error) {
	rawData, err := os.ReadFile(fs.mappingPath())
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return &KeyMapping{keys: map[string]string{}, store: fs}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading key mapping file: %w", err)
	}
	keys := map[string]string{}
	if err := yaml.Unmarshal(rawData, &keys); err != nil {
		return nil, fmt.Errorf("deserializing key mapping file: %w", err)
	}
	return &KeyMapping{keys: keys, store: fs}, nil
}

func (fs *FileStore) SaveKeyWithRepoName(key, repoPath string) error {
	return fs.SaveKey(key, repoPath, filepath.Base(repoPath))
}

func (fs *FileStore) UseKeyForRepo(keyName string, repoPath string) error {
	mapping, err := fs.Mapping()
	if err != nil {
		return err
	}
	if err := mapping.UseKeyForRepo(keyName, repoPath); err != nil {
		return err
	}
	return mapping.Save()
}

func (fs *FileStore) SaveKey(key string, repoPath string, keyName string) error {
	if err := os.MkdirAll(fs.keysDir(), 0o700); err != nil {
		return fmt.Errorf("creating keys directory: %w", err)
	}

	keyFile := fs.keyPath(keyName)
	f, err := os.OpenFile(keyFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
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
	mapping, err := fs.Mapping()
	if err != nil {
		return err
	}
	if err := mapping.Add(repoPath, keyName, true); err != nil {
		return err
	}
	return mapping.Save()
}

func (fs *FileStore) KeyForRepo(repoPath string) (string, error) {
	mapping, err := fs.Mapping()
	if err != nil {
		return "", err
	}
	keyName, err := mapping.Get(repoPath)
	if err != nil {
		return "", err
	}

	keyFile := fs.keyPath(keyName)
	if _, err := os.Stat(keyFile); errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("key file %s does not exists: %w", keyFile, err)
	}

	raw, err := os.ReadFile(keyFile)
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(raw))
	if len(key) != 2*KeyLength {
		return "", fmt.Errorf("reading key from: %s, expected key len: %d, read: %d", keyFile, 2*KeyLength, len(key))
	}
	return key, nil
}
