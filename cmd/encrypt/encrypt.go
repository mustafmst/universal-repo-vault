package encrypt

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"

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
		repoPath, err := repo.GetCurrentRepoPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(repoPath)
		if err != nil {
			return err
		}
		key, err := vault.GetKeyForRepo(repoPath)
		if err != nil {
			return err
		}
		// FIXME: remove this log
		log.Printf("Key: %s", key)

		foundFiles, _ := files.ListAllConfiguredFiles(repoPath, cfg.SecretFiles, cfg.Patterns)
		hashes, err := files.NewFileHashCollection(repoPath, foundFiles)

		lockfile := hashes.GetLockfileBody()
		log.Printf("Lockfile:\n%s", lockfile)
		f, err := os.Create(filepath.Join(repoPath, ".urv.lock"))
		if err != nil {
			return err
		}
		defer f.Close()
		n, err := f.Write(lockfile)
		if err != nil {
			return err
		}
		if n != len(lockfile) {
			return fmt.Errorf("lockafile incomplete write, size: %d, written: %d", len(lockfile), n)
		}

		data, err := vault.CreateZipVaultData(repoPath, foundFiles)
		if err != nil {
			log.Fatalf("creating secret archive: %v", err)
		}

		log.Printf("Secret zip archive data:\n%x\n\n", data)

		encryptedData, err := vault.Encrypt(key, data)
		if err != nil {
			log.Fatalf("encryption error: %v", err)
		}

		log.Printf("Encrypted data:\n%x\n\n", encryptedData)

		// for testing
		decryptedData, err := vault.Decrypt(key, encryptedData)
		if err != nil {
			log.Fatalf("decrypting error: %v", err)
		}
		log.Printf("Decrypted data:\n%x\n\n", decryptedData)
		log.Printf("Is same as input data: %d", slices.Compare(decryptedData, data))

		return nil
	},
}
