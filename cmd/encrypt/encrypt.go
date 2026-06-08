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

		// Create lockfile
		hashes, err := files.NewFileHashCollection(repoPath, foundFiles)
		if err != nil {
			return fmt.Errorf("lockfile body creation failed: %w", err)
		}
		newLockfile := hashes.GetLockfileBody()
		oldLockfile, err := files.OpenLockFile(repoPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("reading old lockfile: %v", err)
		}
		newlockhash, err := files.GetHexHash(newLockfile)
		if err != nil {
			return fmt.Errorf("getting new lockfile hash failed: %w", err)
		}
		oldlockhash, err := files.GetHexHash(oldLockfile)
		if err != nil {
			return fmt.Errorf("getting old lockfile hash failed: %w", err)
		}
		if newlockhash == oldlockhash {
			log.Println("Lockfile same as old one, nothing to encrypt")
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
		v := vault.NewVaultFromData(encryptedData)
		err = v.SaveToFile(filepath.Join(repoPath, vault.VaultFileName))
		if err != nil {
			return err
		}

		// NOTE: Save new lockfile only after vault save finished successfully.
		// In worst case scanario just remove failed lockfile
		err = files.SaveLockFile(filepath.Join(repoPath, files.LockFileName), newLockfile)
		if err != nil {
			log.Printf("Lockfile saving failed. Vault regerenarion might be needed")
			return fmt.Errorf("saving lockfile: %w", err)
		}

		log.Println("Vault saves successfully")
		return nil
	},
}
