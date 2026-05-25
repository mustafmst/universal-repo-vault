package keys

import (
	"github.com/mustafmst/universal-repo-vault/cmd/keys/add"
	"github.com/mustafmst/universal-repo-vault/cmd/keys/gen"
	"github.com/spf13/cobra"
)

var KeysCmd = &cobra.Command{
	Use:   "keys",
	Short: "manage Your vault keys",
}

func init() {
	KeysCmd.AddCommand(add.AddKeyCmd)
	KeysCmd.AddCommand(gen.GenKeyCmd)
}
