package main

import (
	"os"

	"github.com/kcaldas/genie/cmd/cli"
	"github.com/kcaldas/genie/cmd/tui"
)

func main() {
	// Mode detection: if no arguments, start REPL
	if len(os.Args) == 1 {
		tui.StartREPL()
		return
	}

	// Otherwise, run CLI commands
	cli.Execute()
}
