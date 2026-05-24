package initcmd

import (
	"log"

	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/spf13/cobra"
)

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise configuration in repository",
	Run: func(cmd *cobra.Command, args []string) {
		// Implementation for init command
		repoDir, err := repo.GetCurrentRepoPath()
		log.Printf("Got repo dir: %s and err: %v", repoDir, err)
	},
}
