package initcmd

import (
	"log"

	"github.com/mustafmst/universal-repo-vault/internal/config"
	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/spf13/cobra"
)

const errorFormat string = "init command: %v"

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise configuration in repository",
	Run: func(cmd *cobra.Command, args []string) {
		// Implementation for init command
		repoDir, err := repo.GetCurrentRepoPath()
		log.Printf("Got repo dir: %s and err: %v", repoDir, err)
		if err != nil {
			log.Fatalf(errorFormat, err)
			return
		}
		err = repo.CheckGitignore(repoDir)
		if err != nil {
			log.Fatalf("Error while adding gitignore entry: %v", err)
		}

		err = config.Initialize(repoDir)
		if err != nil {
			log.Fatalf(errorFormat, err)
		}

		log.Printf("Configuration successfuly initialized in %s", repoDir)
		log.Printf("List Your secret files in %s.urv for future management", repoDir)
	},
}
