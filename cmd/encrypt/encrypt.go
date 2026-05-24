package encrypt

import (
	"github.com/mustafmst/universal-repo-vault/internal/config"
	"github.com/mustafmst/universal-repo-vault/internal/files"
	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/spf13/cobra"
)

var EncryptCmd = &cobra.Command{
	Use:   "encrypt",
	Short: "Encrypt secrets in repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath, err := repo.GetCurrentRepoPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(repoPath)
		if err != nil {
			return err
		}

		files.ListAllConfiguredFiles(repoPath, cfg.SecretFiles, cfg.Patterns)
		return nil
	},
}
