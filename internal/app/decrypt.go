package app

import (
	"path/filepath"

	"github.com/mustafmst/universal-repo-vault/internal/archive"
	urvcrypto "github.com/mustafmst/universal-repo-vault/internal/crypto"
	"github.com/mustafmst/universal-repo-vault/internal/vault"
)

type DecryptOptions struct {
	DryRun    bool
	Overwrite bool
}

type DecryptResult struct {
	DryRun bool
	Files  []archive.EntryPlan
}

func DecryptRepo(repoPath string) error {
	_, err := DecryptRepoWithOptions(repoPath, DecryptOptions{Overwrite: true})
	return err
}

func DecryptRepoWithOptions(repoPath string, options DecryptOptions) (*DecryptResult, error) {
	return DecryptRepoWithServicesAndOptions(repoPath, DefaultServices(), options)
}

func PlanDecryptRepo(repoPath string) (*DecryptResult, error) {
	return DecryptRepoWithOptions(repoPath, DecryptOptions{DryRun: true, Overwrite: true})
}

func DecryptRepoWithServices(repoPath string, services Services) error {
	_, err := DecryptRepoWithServicesAndOptions(repoPath, services, DecryptOptions{Overwrite: true})
	return err
}

func DecryptRepoWithServicesAndOptions(repoPath string, services Services, options DecryptOptions) (*DecryptResult, error) {
	services = services.withDefaults()
	key, err := services.KeyStore.KeyForRepo(repoPath)
	if err != nil {
		return nil, err
	}

	v, err := vault.NewVaultFromFilePath(filepath.Join(repoPath, vault.VaultFileName))
	if err != nil {
		return nil, err
	}
	if err := v.ValidateForDecrypt(); err != nil {
		return nil, err
	}

	vaultData, err := v.GetByteData()
	if err != nil {
		return nil, err
	}

	cipher, err := urvcrypto.NewCipher(v.Algo, key)
	if err != nil {
		return nil, err
	}
	decryptedArch, err := cipher.Decrypt(vaultData)
	if err != nil {
		return nil, err
	}

	files, err := services.Archiver.PlanUnpack(repoPath, decryptedArch)
	if err != nil {
		return nil, err
	}
	result := &DecryptResult{DryRun: options.DryRun, Files: files}
	if options.DryRun {
		return result, nil
	}
	return result, services.Archiver.Unpack(repoPath, decryptedArch, options.Overwrite)
}
