package slashcommands

import (
	"fmt"
	"io/fs"
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
		root, err := os.OpenRoot(dp.path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("error opening path %s: %w", dp.path, err)
		}

		err = filepath.WalkDir(dp.path, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
				relPath, _ := filepath.Rel(dp.path, path)
				cmdName := strings.TrimSuffix(relPath, ".md")
				cmdName = strings.ReplaceAll(cmdName, string(filepath.Separator), ":")

				// Read file content using root-scoped API to prevent symlink traversal
				fileContent, err := fs.ReadFile(root.FS(), relPath)
				if err != nil {
					return fmt.Errorf("failed to read command file %s: %w", relPath, err)
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
		root.Close()
		if err != nil {
			return fmt.Errorf("error walking path %s: %w", dp.path, err)
		}
	}
	
	// Rebuild the cached command names after discovery
	m.rebuildCommandNames()
	return nil
}