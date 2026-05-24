package encrypt

import "github.com/spf13/cobra"

var EncryptCmd = &cobra.Command{
	Use:   "encrypt",
	Short: "Encrypt secrets in repository",
}
