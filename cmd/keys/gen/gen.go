package gen

import (
	"fmt"

	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/mustafmst/universal-repo-vault/internal/vault"
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
	key, err := vault.GenNewKey()
	if err != nil {
		return err
	}
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return err
	}
	if name != "" {
		if err := vault.SaveKey(key, repoPath, name); err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "Key saved to ~/.config/urv/keys")
		return err
	}
	err = vault.SaveKeyWithRepoName(key, repoPath)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), "Key saved to ~/.config/urv/keys")
	return err
}
