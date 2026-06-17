package encrypt

import (
	"fmt"

	"github.com/mustafmst/universal-repo-vault/internal/app"
	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "encrypt",
		Short: "Encrypt configured secret files",
		Args:  cobra.NoArgs,
		RunE:  runEncrypt,
	}
}

func runEncrypt(cmd *cobra.Command, args []string) error {
	repoPath, err := repo.GetCurrentRepoPath()
	if err != nil {
		return err
	}

	result, err := app.EncryptRepo(repoPath)
	if err != nil {
		return err
	}

	if !result.Encrypted {
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "Vault hashes unchanged; nothing to encrypt")
		return err
	}

	_, err = fmt.Fprintln(cmd.OutOrStdout(), "Vault saved successfully")
	return err
}
