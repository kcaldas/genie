package main

import (
	"fmt"
	"os"

	"github.com/kcaldas/genie/cmd/cli"
)

func main() {
	cli.RootCmd.SetVersionTemplate("genie version {{.Version}}\n")
	if err := cli.RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
