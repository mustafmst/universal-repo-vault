package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mustafmst/universal-repo-vault/internal/archive"
	"github.com/mustafmst/universal-repo-vault/internal/config"
	urvcrypto "github.com/mustafmst/universal-repo-vault/internal/crypto"
	"github.com/mustafmst/universal-repo-vault/internal/files"
	"github.com/mustafmst/universal-repo-vault/internal/keystore"
	"github.com/mustafmst/universal-repo-vault/internal/vault"
)

type EncryptResult struct {
	Encrypted bool
}

func EncryptRepo(repoPath string) (*EncryptResult, error) {
	cfg, err := config.Load(repoPath)
	if err != nil {
		return nil, err
	}

	key, err := keystore.NewDefaultFileStore().KeyForRepo(repoPath)
	if err != nil {
		return nil, err
	}

	foundFiles, err := files.ListAllConfiguredFiles(repoPath, cfg.SecretFiles, cfg.Patterns)
	if err != nil {
		return nil, fmt.Errorf("listing files for encryption failed: %w", err)
	}

	hashes, err := files.NewFileHashCollection(repoPath, foundFiles)
	if err != nil {
		return nil, fmt.Errorf("hash state creation failed: %w", err)
	}

	vaultPath := filepath.Join(repoPath, vault.VaultFileName)
	oldVault, err := vault.NewVaultFromFilePath(vaultPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading old vault: %w", err)
	}

	oldHashes := map[string]string{}
	if oldVault != nil {
		oldHashes = oldVault.Hashes
	}

	lockPath := filepath.Join(repoPath, files.LockFileName)
	oldLockfile, err := files.OpenLockFile(repoPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading old lockfile: %w", err)
	}
	if len(oldHashes) == 0 && err == nil {
		oldHashes, err = files.ParseLockFileBody(oldLockfile)
		if err != nil {
			return nil, fmt.Errorf("parsing old lockfile: %w", err)
		}
	}

	if oldVault != nil && files.HashesEqual(hashes.Hashes, oldHashes) {
		oldVault.Version = vault.VaultVersion
		oldVault.Algo = vault.VaultAlgo
		oldVault.Hashes = hashes.Hashes
		if err := oldVault.SaveToFile(vaultPath); err != nil {
			return nil, err
		}
		if err := removeOldLockFile(lockPath); err != nil {
			return nil, err
		}
		return &EncryptResult{Encrypted: false}, nil
	}

	data, err := archive.NewZipArchiver().Pack(repoPath, foundFiles)
	if err != nil {
		return nil, fmt.Errorf("creating secret archive: %w", err)
	}

	cipher, err := urvcrypto.NewCipher(vault.VaultAlgo, key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}
	encryptedData, err := cipher.Encrypt(data)
	if err != nil {
		return nil, fmt.Errorf("encryption error: %w", err)
	}

	v := vault.NewVaultFromData(encryptedData, hashes.Hashes)
	if err = v.SaveToFile(vaultPath); err != nil {
		return nil, err
	}

	if err := removeOldLockFile(lockPath); err != nil {
		return nil, err
	}

	return &EncryptResult{Encrypted: true}, nil
}

func removeOldLockFile(lockPath string) error {
	if err := os.Remove(lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing old lockfile: %w", err)
	}
	return nil
}
