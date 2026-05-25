package gen

import "github.com/spf13/cobra"

var GenKeyCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generates new encryption key and assigns it to  a repo in user wide configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		panic("unimplemented")
	},
}
