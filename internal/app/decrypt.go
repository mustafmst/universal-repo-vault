package app

import (
	"path/filepath"

	"github.com/mustafmst/universal-repo-vault/internal/vault"
)

func DecryptRepo(repoPath string) error {
	key, err := vault.GetKeyForRepo(repoPath)
	if err != nil {
		return err
	}

	v, err := vault.NewVaultFromFilePath(filepath.Join(repoPath, vault.VaultFileName))
	if err != nil {
		return err
	}
	if err := v.ValidateForDecrypt(); err != nil {
		return err
	}

	vaultData, err := v.GetByteData()
	if err != nil {
		return err
	}

	decryptedArch, err := vault.AesGcmDecrypt(key, vaultData)
	if err != nil {
		return err
	}

	return vault.UnpackZipVaultData(repoPath, decryptedArch)
}
