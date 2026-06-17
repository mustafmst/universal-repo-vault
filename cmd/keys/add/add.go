package add

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/mustafmst/universal-repo-vault/internal/vault"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Map an existing encryption key",
		Long:  "Read a key file or map an already stored key to the current repository.",
		Args:  cobra.NoArgs,
		RunE:  runAdd,
	}
	cmd.Flags().StringP("file", "f", "", "file with key for encryption")
	cmd.Flags().StringP("key", "k", "", "already stored key name")
	return cmd
}

func runAdd(cmd *cobra.Command, args []string) error {
	repoPath, err := repo.GetCurrentRepoPath()
	if err != nil {
		return err
	}
	keyName, err := cmd.Flags().GetString("key")
	if err != nil {
		return err
	}
	file, err := cmd.Flags().GetString("file")
	if err != nil {
		return err
	}
	if keyName == "" && file == "" {
		return fmt.Errorf("provide --key or --file")
	}
	if keyName != "" && file != "" {
		return fmt.Errorf("provide only one of --key or --file")
	}

	if keyName != "" {
		mapping, err := vault.NewKeyMapping()
		if err != nil {
			return err
		}
		if err := mapping.UseKeyForRepo(keyName, repoPath); err != nil {
			return err
		}
		return mapping.Save()
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
}
