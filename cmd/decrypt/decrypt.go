package decrypt

import (
	"fmt"

	"github.com/mustafmst/universal-repo-vault/internal/app"
	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "decrypt",
		Short: "Decrypt vault files into the repository",
		Args:  cobra.NoArgs,
		RunE:  runDecrypt,
	}
}

func runDecrypt(cmd *cobra.Command, args []string) error {
	repoPath, err := repo.GetCurrentRepoPath()
	if err != nil {
		return err
	}

	if err := app.DecryptRepo(repoPath); err != nil {
		return err
	}

	_, err = fmt.Fprintln(cmd.OutOrStdout(), "Vault unpacked successfully")
	return err
}
