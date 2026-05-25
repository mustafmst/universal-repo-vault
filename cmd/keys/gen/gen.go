package gen

import (
	"log"

	"github.com/mustafmst/universal-repo-vault/internal/vault"
	"github.com/spf13/cobra"
)

var GenKeyCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generates new encryption key and assigns it to  a repo in user wide configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := vault.GenNewKey()
		if err != nil {
			return err
		}
		err = vault.SaveKey(key, "tmp")
		if err != nil {
			return err
		}
		log.Println("Key saved to ~/.config/urv/keys")
		return nil
	},
}
