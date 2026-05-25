package vault

import (
	"crypto/rand"
	"encoding/hex"
)

const keyLength int = 32

func GenNewKey() (string, error) {
	keyBytes := make([]byte, keyLength)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(keyBytes), nil
}

func SaveKey(key string, repoPath string) error {
	// TODO: this function will save key to a file in $HOME/.config/urv/keys
	// and add a entry to global configuration
	panic("unimplemented")
}

func GetKeyForRepo(repoPath string) (string, error) {
	// TODO: this one will get the saved key  for curent repo
	panic("unimplemented")
}
