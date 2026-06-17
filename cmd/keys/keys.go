package keys

import (
	"github.com/mustafmst/universal-repo-vault/cmd/keys/add"
	"github.com/mustafmst/universal-repo-vault/cmd/keys/gen"
	"github.com/mustafmst/universal-repo-vault/cmd/keys/list"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage vault keys",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(add.NewCommand())
	cmd.AddCommand(gen.NewCommand())
	cmd.AddCommand(list.NewCommand())

	return cmd
}
