package status

import (
	"fmt"
	"strings"

	"github.com/mustafmst/universal-repo-vault/internal/app"
	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show repository vault safety status",
		Args:  cobra.NoArgs,
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	repoPath, err := repo.GetCurrentRepoPath()
	if err != nil {
		return err
	}

	report, err := app.StatusRepo(repoPath)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(cmd.OutOrStdout(), FormatReport(report))
	return err
}

func FormatReport(report *app.StatusReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Overall: %s\n", report.Overall)

	if report.ConfigOK {
		b.WriteString("Config: ok\n")
	} else {
		b.WriteString("Config: not ok\n")
	}

	if report.VaultOK {
		b.WriteString("Vault: ok\n")
	} else if report.VaultExists {
		b.WriteString("Vault: not ok\n")
	} else {
		b.WriteString("Vault: missing\n")
	}

	if report.KeyMapped {
		fmt.Fprintf(&b, "Key: %s", report.KeyName)
		if !report.KeyFileExists {
			b.WriteString(" (missing file)")
		} else if !report.KeyLengthValid {
			b.WriteString(" (invalid length)")
		}
		b.WriteString("\n")
	} else {
		b.WriteString("Key: not mapped\n")
	}

	if len(report.Files) > 0 {
		b.WriteString("Files:\n")
		for _, file := range report.Files {
			fmt.Fprintf(&b, "  %s %s\n", file.Path, file.Status)
		}
	}

	if len(report.Warnings) > 0 {
		b.WriteString("Warnings:\n")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&b, "  %s\n", warning)
		}
	}

	if len(report.Errors) > 0 {
		b.WriteString("Errors:\n")
		for _, err := range report.Errors {
			fmt.Fprintf(&b, "  %s\n", err)
		}
	}

	return b.String()
}
