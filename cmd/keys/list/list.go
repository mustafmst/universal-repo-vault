package list

import (
	"github.com/mustafmst/universal-repo-vault/internal/vault"
	"github.com/spf13/cobra"
)

var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists all keys in key directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		mapping, err := vault.NewKeyMapping()
		if err != nil {
			return err
		}
		mapping.List()
		return nil
	},
}
