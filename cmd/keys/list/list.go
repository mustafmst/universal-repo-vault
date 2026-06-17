package list

import (
	"fmt"

	"github.com/mustafmst/universal-repo-vault/internal/vault"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List repository key mappings",
		Args:  cobra.NoArgs,
		RunE:  runList,
	}
}

func runList(cmd *cobra.Command, args []string) error {
	mapping, err := vault.NewKeyMapping()
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), mapping.String())
	return err
}
