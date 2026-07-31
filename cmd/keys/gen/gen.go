package gen

import (
	"fmt"
	"path/filepath"

	"github.com/mustafmst/universal-repo-vault/internal/keystore"
	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate and map a new encryption key",
		Args:  cobra.NoArgs,
		RunE:  runGen,
	}
	cmd.Flags().StringP("name", "n", "", "use a custom key name")
	return cmd
}

func runGen(cmd *cobra.Command, args []string) error {
	repoPath, err := repo.GetCurrentRepoPath()
	if err != nil {
		return err
	}
	key, err := keystore.GenerateKey()
	if err != nil {
		return err
	}
	store := keystore.NewDefaultFileStore()
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return err
	}
	if name != "" {
		if err := store.SaveKey(key, repoPath, name); err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "Key saved to ~/.config/urv/keys")
		return err
	}
	err = store.SaveKey(key, repoPath, filepath.Base(repoPath))
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), "Key saved to ~/.config/urv/keys")
	return err
}
