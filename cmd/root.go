package cmd

import (
	"github.com/mustafmst/universal-repo-vault/cmd/decrypt"
	"github.com/mustafmst/universal-repo-vault/cmd/encrypt"
	"github.com/mustafmst/universal-repo-vault/cmd/initcmd"
	"github.com/mustafmst/universal-repo-vault/cmd/keys"
	"github.com/mustafmst/universal-repo-vault/cmd/status"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "urv",
		Short:        "Encrypt and decrypt repository secret files",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(initcmd.NewCommand())
	cmd.AddCommand(decrypt.NewCommand())
	cmd.AddCommand(encrypt.NewCommand())
	cmd.AddCommand(keys.NewCommand())
	cmd.AddCommand(status.NewCommand())

	return cmd
}

func Execute() error {
	return NewRootCommand().Execute()
}
