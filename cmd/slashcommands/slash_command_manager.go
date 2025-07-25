package slashcommands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type SlashCommand struct {
	Name        string
	Description string
	Expand      func(args []string) (string, error)
	Source      string // "project" or "user"
}

type Manager struct {
	commands     map[string]SlashCommand
	commandNames []string // cached list of command names
}

func NewManager() *Manager {
	return &Manager{
		commands:     make(map[string]SlashCommand),
		commandNames: make([]string, 0),
	}
}

// GetCommand returns a SlashCommand by its name.
func (m *Manager) GetCommand(commandName string) (SlashCommand, bool) {
	cmd, ok := m.commands[commandName]
	return cmd, ok
}

// GetCommandNames returns a slice of all available slash command names.
func (m *Manager) GetCommandNames() []string {
	return m.commandNames
}

// rebuildCommandNames rebuilds the cached command names slice
func (m *Manager) rebuildCommandNames() {
	m.commandNames = make([]string, 0, len(m.commands))
	for name := range m.commands {
		m.commandNames = append(m.commandNames, name)
	}
}

// DiscoverCommands finds slash command definition files in specified directories.
func (m *Manager) DiscoverCommands(projectRoot string, getUserHomeDir func() (string, error)) error {
	discoveryPaths := []struct {
		path   string
		source string
	}{
		{filepath.Join(projectRoot, ".genie", "commands"), "project"},
		{filepath.Join(projectRoot, ".claude", "commands"), "project"},
	}

	homeDir, err := getUserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	discoveryPaths = append(discoveryPaths,
		struct {
			path   string
			source string
		}{filepath.Join(homeDir, ".genie", "commands"), "user"},
		struct {
			path   string
			source string
		}{filepath.Join(homeDir, ".claude", "commands"), "user"},
	)

	for _, dp := range discoveryPaths {
		err := filepath.Walk(dp.path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
				relPath, _ := filepath.Rel(dp.path, path)
				cmdName := strings.TrimSuffix(relPath, ".md")
				cmdName = strings.ReplaceAll(cmdName, string(filepath.Separator), ":")

				// Read file content for description and command template
				fileContent, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("failed to read command file %s: %w", path, err)
				}
				description := strings.TrimSpace(string(fileContent))
				if len(description) > 100 { // Truncate long descriptions for display
					description = description[:100] + "..."
				}

				// Capture the command template from the file content
				commandTemplate := strings.TrimSpace(string(fileContent))

				m.commands[cmdName] = SlashCommand{
					Name:        cmdName,
					Description: description,
					Source:      dp.source,
					Expand: func(args []string) (string, error) {
						// Expand arguments in the command template
						expandedCommand := ExpandArguments(commandTemplate, args)
						// Return the expanded command string
						return expandedCommand, nil
					},
				}
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("error walking path %s: %w", dp.path, err)
		}
	}
	
	// Rebuild the cached command names after discovery
	m.rebuildCommandNames()
	return nil
}