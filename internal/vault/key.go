package vault

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const keyVariable string = "URV_KEY_NAME"

const keyLength int = 32

func GenNewKey() (string, error) {
	keyBytes := make([]byte, keyLength)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(keyBytes), nil
}

func SaveKey(key string, repoPath string) error {
	// TODO: Partial implementation, in future use user config and repo name to get right key
	if err := ensureKeysDir(); err != nil {
		return fmt.Errorf("creating keys directory: %w", err)
	}

	keyName := os.Getenv(keyVariable)
	if keyName == "" {
		return fmt.Errorf("key name not set")
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

	return nil
}

func GetKeyForRepo(repoPath string) (string, error) {
	result := ""

	keyName := os.Getenv(keyVariable)
	if keyName == "" {
		return result, fmt.Errorf("key name not set")
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
