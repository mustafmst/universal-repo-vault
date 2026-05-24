package decrypt

import "github.com/spf13/cobra"

var DecryptCmd = &cobra.Command{
	Use:   "decrypt",
	Short: "Decrypt secrets in repository",
}
