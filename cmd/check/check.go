package check

import (
	"fmt"
	"strings"

	"github.com/mustafmst/universal-repo-vault/internal/app"
	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Fail when repository vault safety status is unsafe",
		Args:  cobra.NoArgs,
		RunE:  runCheck,
	}
}

func runCheck(cmd *cobra.Command, args []string) error {
	repoPath, err := repo.GetCurrentRepoPath()
	if err != nil {
		return err
	}

	result, err := app.CheckRepo(repoPath)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprint(cmd.OutOrStdout(), FormatCheckResult(result)); err != nil {
		return err
	}
	if !result.Safe {
		return fmt.Errorf("repository is not safe to commit")
	}
	return nil
}

func FormatCheckResult(result *app.CheckResult) string {
	var b strings.Builder
	if result.Safe {
		b.WriteString("URV check passed\n")
		return b.String()
	}
	b.WriteString("URV check failed\n")
	for _, msg := range result.Messages {
		fmt.Fprintf(&b, "  %s\n", msg)
	}
	return b.String()
}
