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
		foundFiles, _ := files.ListAllConfiguredFiles(repoPath, cfg.SecretFiles, cfg.Patterns)

		// Create lockfile
		hashes, err := files.NewFileHashCollection(repoPath, foundFiles)
		newLockfile := hashes.GetLockfileBody()
		oldLockfile, err := files.OpenLockFile(repoPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("reading old lockfile: %v", err)
		}
		newlockhash, _ := files.GetHexHash(newLockfile)
		oldlockhash, _ := files.GetHexHash(oldLockfile)
		if newlockhash == oldlockhash {
			log.Println("Lockfile same as old one, nothing to encrypt")
			return nil
		}
		log.Printf("[DEBUG]\noldLockfile:\n%s\nhash: %s\newLockfile:\n%s\nhash: %s\n", oldLockfile, oldlockhash, newLockfile, newlockhash)
		err = files.SaveLockFile(filepath.Join(repoPath, files.LockFileName), newLockfile)
		if err != nil {
			return fmt.Errorf("sving lockfile: %w", err)
		}

		// Compress and encrypt data of secret files
		data, err := vault.CreateZipVaultData(repoPath, foundFiles)
		if err != nil {
			log.Fatalf("creating secret archive: %v", err)
		}

		encryptedData, err := vault.Encrypt(key, data)
		if err != nil {
			log.Fatalf("encryption error: %v", err)
		}

		// Save vault data
		v := vault.NewVaultFromData(encryptedData)
		err = v.SaveToFile(filepath.Join(repoPath, vault.VaultFileName))
		if err != nil {
			return err
		}

		log.Println("Vault saves successfully")
		return nil
	},
}
