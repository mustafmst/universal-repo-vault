package encrypt

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mustafmst/universal-repo-vault/internal/config"
	"github.com/mustafmst/universal-repo-vault/internal/files"
	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/mustafmst/universal-repo-vault/internal/vault"
	"github.com/spf13/cobra"
)

var EncryptCmd = &cobra.Command{
	Use:   "encrypt",
	Short: "Encrypt secrets in repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get current repository path
		repoPath, err := repo.GetCurrentRepoPath()
		if err != nil {
			return err
		}

		// Get local configuration
		cfg, err := config.Load(repoPath)
		if err != nil {
			return err
		}

		// Get vault key
		key, err := vault.GetKeyForRepo(repoPath)
		if err != nil {
			return err
		}

		// Get files for encryption
		foundFiles, err := files.ListAllConfiguredFiles(repoPath, cfg.SecretFiles, cfg.Patterns)
		if err != nil {
			return fmt.Errorf("listing files for enryption failed: %w", err)
		}

		// Create current hash state for change detection
		hashes, err := files.NewFileHashCollection(repoPath, foundFiles)
		if err != nil {
			return fmt.Errorf("hash state creation failed: %w", err)
		}

		vaultPath := filepath.Join(repoPath, vault.VaultFileName)
		oldVault, err := vault.NewVaultFromFilePath(vaultPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("reading old vault: %w", err)
		}

		oldHashes := map[string]string{}
		if oldVault != nil {
			oldHashes = oldVault.Hashes
		}

		lockPath := filepath.Join(repoPath, files.LockFileName)
		oldLockfile, err := files.OpenLockFile(repoPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("reading old lockfile: %w", err)
		}
		if len(oldHashes) == 0 && err == nil {
			oldHashes, err = files.ParseLockFileBody(oldLockfile)
			if err != nil {
				return fmt.Errorf("parsing old lockfile: %w", err)
			}
		}

		if oldVault != nil && files.HashesEqual(hashes.Hashes, oldHashes) {
			oldVault.Version = vault.VaultVersion
			oldVault.Algo = vault.VaultAlgo
			oldVault.Hashes = hashes.Hashes
			if err := oldVault.SaveToFile(vaultPath); err != nil {
				return err
			}
			if err := os.Remove(lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("removing old lockfile: %w", err)
			}
			log.Println("Vault hashes unchanged, nothing to encrypt")
			return nil
		}

		// Compress and encrypt data of secret files
		data, err := vault.CreateZipVaultData(repoPath, foundFiles)
		if err != nil {
			return fmt.Errorf("creating secret archive: %w", err)
		}

		encryptedData, err := vault.AesGcmEncrypt(key, data)
		if err != nil {
			return fmt.Errorf("encryption error: %w", err)
		}

		// Save vault data
		v := vault.NewVaultFromData(encryptedData, hashes.Hashes)
		err = v.SaveToFile(vaultPath)
		if err != nil {
			return err
		}

		if err := os.Remove(lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing old lockfile: %w", err)
		}

		log.Println("Vault saves successfully")
		return nil
	},
}
