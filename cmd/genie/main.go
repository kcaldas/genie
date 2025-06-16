package main

import (
	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "genie",
	Short: "Genie CLI tool",
	Version: version,
}

func main() {
	rootCmd.SetVersionTemplate("genie version {{.Version}}\n")
	rootCmd.Execute()
}