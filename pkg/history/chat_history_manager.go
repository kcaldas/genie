package history

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ChatHistoryManager manages command history with persistent storage
type ChatHistoryManager interface {
	AddCommand(command string)
	GetHistory() []string
	Load() error
	Save() error
}

// FileChatHistoryManager implements ChatHistoryManager with file persistence
type FileChatHistoryManager struct {
	filePath string
	commands []string
	maxSize  int
}

// NewChatHistoryManager creates a new chat history manager
func NewChatHistoryManager(filePath string) ChatHistoryManager {
	return &FileChatHistoryManager{
		filePath: filePath,
		commands: make([]string, 0),
		maxSize:  50,
	}
}

// AddCommand adds a command to history, avoiding duplicates and auto-saving
func (m *FileChatHistoryManager) AddCommand(command string) {
	command = strings.TrimSpace(command)
	if command == "" {
		return
	}

	// Remove command if it already exists (avoid duplicates)
	for i, existing := range m.commands {
		if existing == command {
			// Remove the existing occurrence
			m.commands = append(m.commands[:i], m.commands[i+1:]...)
			break
		}
	}

	// Add command to the end
	m.commands = append(m.commands, command)

	// Trim to max size (keep last 50)
	if len(m.commands) > m.maxSize {
		m.commands = m.commands[len(m.commands)-m.maxSize:]
	}

	// Auto-save after adding
	m.Save()
}

// GetHistory returns a copy of the command history
func (m *FileChatHistoryManager) GetHistory() []string {
	result := make([]string, len(m.commands))
	copy(result, m.commands)
	return result
}

// Load reads command history from file
func (m *FileChatHistoryManager) Load() error {
	// If file doesn't exist, that's not an error - just start with empty history
	if _, err := os.Stat(m.filePath); os.IsNotExist(err) {
		return nil
	}

	file, err := os.Open(m.filePath)
	if err != nil {
		return fmt.Errorf("failed to open history file: %w", err)
	}
	defer file.Close()

	m.commands = make([]string, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		command := strings.TrimSpace(scanner.Text())
		if command != "" {
			m.commands = append(m.commands, command)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read history file: %w", err)
	}

	// Trim to max size after loading
	if len(m.commands) > m.maxSize {
		m.commands = m.commands[len(m.commands)-m.maxSize:]
	}

	return nil
}

// Save writes command history to file
func (m *FileChatHistoryManager) Save() error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	file, err := os.Create(m.filePath)
	if err != nil {
		return fmt.Errorf("failed to create history file: %w", err)
	}
	defer file.Close()

	for _, command := range m.commands {
		if _, err := fmt.Fprintln(file, command); err != nil {
			return fmt.Errorf("failed to write command to history file: %w", err)
		}
	}

	return nil
}