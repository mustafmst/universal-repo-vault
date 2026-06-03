package add

import (
	"fmt"
	"log"
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
			log.Println("Current path not in git repository")
			return nil
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
}
