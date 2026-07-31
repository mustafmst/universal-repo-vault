package app

import (
	"path/filepath"

	"github.com/mustafmst/universal-repo-vault/internal/archive"
	urvcrypto "github.com/mustafmst/universal-repo-vault/internal/crypto"
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

	cipher, err := urvcrypto.NewCipher(v.Algo, key)
	if err != nil {
		return err
	}
	decryptedArch, err := cipher.Decrypt(vaultData)
	if err != nil {
		return err
	}

	return archive.NewZipArchiver().Unpack(repoPath, decryptedArch, true)
}
