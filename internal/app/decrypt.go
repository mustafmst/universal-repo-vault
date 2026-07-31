package app

import (
	"path/filepath"

	urvcrypto "github.com/mustafmst/universal-repo-vault/internal/crypto"
	"github.com/mustafmst/universal-repo-vault/internal/vault"
)

func DecryptRepo(repoPath string) error {
	return DecryptRepoWithServices(repoPath, DefaultServices())
}

func DecryptRepoWithServices(repoPath string, services Services) error {
	services = services.withDefaults()
	key, err := services.KeyStore.KeyForRepo(repoPath)
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

	return services.Archiver.Unpack(repoPath, decryptedArch, true)
}
