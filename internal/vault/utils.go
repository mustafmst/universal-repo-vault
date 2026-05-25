package vault

import (
	"os"
	"path/filepath"
)

func ensureKeysDir() error {
	keysDir := filepath.Join(os.Getenv("HOME"), ".config", "urv", "keys")
	return os.MkdirAll(keysDir, 0o700)
}
