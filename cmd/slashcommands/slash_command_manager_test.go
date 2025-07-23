package slashcommands

import (
	"testing"
)

func TestSlashCommandStruct(t *testing.T) {
	cmd := SlashCommand{
		Name:        "test:command",
		Description: "A test command",
		Expand: func(args []string) (string, error) {
			return "executed", nil
		},
	}

	if cmd.Name != "test:command" {
		t.Errorf("Expected name to be 'test:command', got %s", cmd.Name)
	}
	if cmd.Description != "A test command" {
		t.Errorf("Expected description to be 'A test command', got %s", cmd.Description)
	}
}
