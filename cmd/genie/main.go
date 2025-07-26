package main

import (
	"os"

	"github.com/kcaldas/genie/cmd/cli"
	"github.com/kcaldas/genie/pkg/version"
)

func main() {
	// Set custom version template that shows more detailed version info
	cli.RootCmd.SetVersionTemplate(version.GetInfo().String() + "\n")
	if err := cli.RootCmd.Execute(); err != nil {
		// Cobra already prints the error, just exit with error code
		os.Exit(1)
	}
}
