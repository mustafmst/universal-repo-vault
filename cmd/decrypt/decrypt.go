package decrypt

import (
	"log"
	"path/filepath"

	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/mustafmst/universal-repo-vault/internal/vault"
	"github.com/spf13/cobra"
)

var DecryptCmd = &cobra.Command{
	Use:   "decrypt",
	Short: "Decrypt secrets in repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath, err := repo.GetCurrentRepoPath()
		if err != nil {
			return err
		}
		key, err := vault.GetKeyForRepo(repoPath)
		if err != nil {
			return err
		}

		v, err := vault.NewVaultFromFilePath(filepath.Join(repoPath, vault.VaultFileName))
		if err != nil {
			return err
		}

		decryptedArch, err := vault.Decrypt(key, v.GetByteData())
		if err != nil {
			return err
		}

		err = vault.UnpackZipVaultData(repoPath, decryptedArch)
		if err != nil {
			return err
		}

		log.Println("Vault unpacked successfully")
		return nil
	},
}
