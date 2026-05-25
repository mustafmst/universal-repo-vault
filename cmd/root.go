package cmd

import (
	"log"

	"github.com/mustafmst/universal-repo-vault/cmd/decrypt"
	"github.com/mustafmst/universal-repo-vault/cmd/encrypt"
	"github.com/mustafmst/universal-repo-vault/cmd/initcmd"
	"github.com/mustafmst/universal-repo-vault/cmd/keys"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initcmd.InitCmd)
	rootCmd.AddCommand(decrypt.DecryptCmd)
	rootCmd.AddCommand(encrypt.EncryptCmd)
	rootCmd.AddCommand(keys.KeysCmd)
}

var rootCmd = &cobra.Command{
	Use:   "universal-repo-vault",
	Short: "URV is a tool for safely manage, store, encrypt and decrypt all secret and env files in repository",
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Magic will be happening here")
	},
}

func Execute() error {
	return rootCmd.Execute()
}
