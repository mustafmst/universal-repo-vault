package add

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/mustafmst/universal-repo-vault/internal/vault"
	"github.com/spf13/cobra"
)

var AddKeyCmd = &cobra.Command{
	Use:   "add",
	Short: "add existing key",
	Long:  "Command reads given file and saves its content to keys director and add row in keys mapping",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath, err := repo.GetCurrentRepoPath()
		if err != nil {
			return err
		}
		keyName, err := cmd.Flags().GetString("key")
		if err != nil {
			return err
		}
		if keyName != "" {
			// NOTE: Key flag takes precedence over file

			mapping, err := vault.NewKeyMapping()
			if err != nil {
				return err
			}
			return mapping.UseKeyForRepo(keyName, repoPath)
		}

		file, err := cmd.Flags().GetString("file")
		if err != nil {
			return err
		}
		if file == "" {
			return fmt.Errorf("provide file with key to add")
		}

		file, err = filepath.Abs(file)
		if err != nil {
			return err
		}

		key, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		err = vault.SaveKeyWithRepoName(string(key), repoPath)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	AddKeyCmd.Flags().StringP("file", "f", "", "file with key for encryption")
	AddKeyCmd.Flags().StringP("key", "k", "", "already existing key")
}
