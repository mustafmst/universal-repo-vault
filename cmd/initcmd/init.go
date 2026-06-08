package initcmd

import (
	"fmt"
	"log"

	"github.com/mustafmst/universal-repo-vault/internal/config"
	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/spf13/cobra"
)

const errorFormat string = "init command: %v"

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise configuration in repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Implementation for init command
		repoDir, err := repo.GetCurrentRepoPath()
		log.Printf("Got repo dir: %s and err: %v", repoDir, err)
		if err != nil {
			return fmt.Errorf(errorFormat, err)
		}

		err = config.Initialize(repoDir)
		if err != nil {
			return fmt.Errorf(errorFormat, err)
		}

		log.Printf("Configuration successfuly initialized in %s", repoDir)
		log.Printf("List Your secret files in %s.urv for future management", repoDir)
		return nil
	},
}
