package initcmd

import (
	"fmt"

	"github.com/mustafmst/universal-repo-vault/internal/config"
	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize URV configuration",
		Args:  cobra.NoArgs,
		RunE:  runInit,
	}
}

func runInit(cmd *cobra.Command, args []string) error {
	repoDir, err := repo.GetCurrentRepoPath()
	if err != nil {
		return fmt.Errorf("init command: %w", err)
	}

	err = config.Initialize(repoDir)
	if err != nil {
		return fmt.Errorf("init command: %w", err)
	}

	err = repo.CheckGitignore(repoDir)
	if err != nil {
		return fmt.Errorf("init command: %w", err)
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Configuration successfully initialized in %s\n", repoDir)
	return err
}
