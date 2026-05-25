package add

import "github.com/spf13/cobra"

var AddKeyCmd = &cobra.Command{
	Use:   "add",
	Short: "add existing key",
	RunE: func(cmd *cobra.Command, args []string) error {
		panic("unimplemented")
	},
}
