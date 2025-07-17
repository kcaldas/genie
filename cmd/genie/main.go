package main

import (
	"os"

	"github.com/kcaldas/genie/cmd/cli"
)

func main() {
	cli.RootCmd.SetVersionTemplate("genie version {{.Version}}\n")
	if err := cli.RootCmd.Execute(); err != nil {
		// Cobra already prints the error, just exit with error code
		os.Exit(1)
	}
}
