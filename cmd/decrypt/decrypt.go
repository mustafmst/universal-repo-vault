package decrypt

import (
	"fmt"
	"strings"

	"github.com/mustafmst/universal-repo-vault/internal/app"
	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decrypt",
		Short: "Decrypt vault files into the repository",
		Args:  cobra.NoArgs,
		RunE:  runDecrypt,
	}
	cmd.Flags().Bool("dry-run", false, "show files that would be decrypted without writing them")
	cmd.Flags().Bool("no-overwrite", false, "fail if decrypt would replace an existing file")
	return cmd
}

func runDecrypt(cmd *cobra.Command, args []string) error {
	repoPath, err := repo.GetCurrentRepoPath()
	if err != nil {
		return err
	}

	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return err
	}
	noOverwrite, err := cmd.Flags().GetBool("no-overwrite")
	if err != nil {
		return err
	}

	result, err := app.DecryptRepoWithOptions(repoPath, app.DecryptOptions{DryRun: dryRun, Overwrite: !noOverwrite})
	if err != nil {
		return err
	}
	if dryRun {
		_, err = fmt.Fprint(cmd.OutOrStdout(), FormatDryRunResult(result))
		return err
	}

	_, err = fmt.Fprintln(cmd.OutOrStdout(), "Vault unpacked successfully")
	return err
}

func FormatDryRunResult(result *app.DecryptResult) string {
	var b strings.Builder
	b.WriteString("Decrypt dry run:\n")
	for _, file := range result.Files {
		fmt.Fprintf(&b, "  %s %s\n", file.Path, file.Action)
	}
	return b.String()
}
