package main

import (
	"fmt"
	"os"

	"github.com/mustafmst/universal-repo-vault/cmd"
)

func main() {
	// setup here

	defer func() {
		// shutdown here
	}()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error during execution: %s\n", err)
		os.Exit(1)
	}
}
