package gen

import (
	"log"

	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/mustafmst/universal-repo-vault/internal/vault"
	"github.com/spf13/cobra"
)

var GenKeyCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generates new encryption key and assigns it to  a repo in user wide configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath, err := repo.GetCurrentRepoPath()
		if err != nil {
			log.Println("Current path not in git repository")
			return nil
		}
		key, err := vault.GenNewKey()
		if err != nil {
			return err
		}
		nameOverride, err := cmd.Flags().GetString("name-override")
		if err != nil {
			return err
		}
		if nameOverride != "" {
			// NOTE: Override name
			return vault.SaveKey(key, repoPath, nameOverride)
		}
		err = vault.SaveKeyWithRepoName(key, repoPath)
		if err != nil {
			return err
		}
		log.Println("Key saved to ~/.config/urv/keys")
		return nil
	},
}

func init() {
	GenKeyCmd.Flags().StringP("name-override", "no", "", "use other name for the key that repo name")
}
