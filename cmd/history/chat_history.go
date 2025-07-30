package history

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ChatHistory manages command history for TUI with persistent storage
type ChatHistory interface {
	AddCommand(command string)
	GetHistory() []string
	Load() error
	Save() error
	NavigateNext() string
	NavigatePrev() string
	ResetNavigation() string
}

// FileChatHistory implements ChatHistory with optional file persistence
type FileChatHistory struct {
	filePath      string
	commands      []string
	maxSize       int
	currentIndex  int // -1 means no selection (at end)
	saveEnabled   bool // whether to save to disk
}

// NewChatHistory creates a new TUI chat history manager
func NewChatHistory(filePath string, saveEnabled bool) ChatHistory {
	return &FileChatHistory{
		filePath:     filePath,
		commands:     make([]string, 0),
		maxSize:      50,
		currentIndex: -1,
		saveEnabled:  saveEnabled,
	}
}

// AddCommand adds a command to history, avoiding duplicates and auto-saving
func (h *FileChatHistory) AddCommand(command string) {
	command = strings.TrimSpace(command)
	if command == "" {
		return
	}

	// Remove command if it already exists (avoid duplicates)
	for i, existing := range h.commands {
		if existing == command {
			// Remove the existing occurrence
			h.commands = append(h.commands[:i], h.commands[i+1:]...)
			break
		}
	}

	// Add command to the end
	h.commands = append(h.commands, command)

	// Trim to max size (keep last 50)
	if len(h.commands) > h.maxSize {
		h.commands = h.commands[len(h.commands)-h.maxSize:]
	}

	// Reset navigation after adding new command
	h.currentIndex = -1
	
	// Auto-save after adding (if enabled)
	if h.saveEnabled {
		h.Save()
	}
}

// escapeForHistory escapes a command for storage in history file
// Converts newlines to \n and backslashes to \\
func escapeForHistory(command string) string {
	// First escape backslashes, then newlines
	escaped := strings.ReplaceAll(command, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\n", "\\n")
	return escaped
}

// unescapeFromHistory unescapes a command from history file
// Converts \n to newlines and \\ to backslashes
func unescapeFromHistory(escaped string) string {
	// We need to be careful about the order and handle escaped backslashes properly
	var result strings.Builder
	i := 0
	for i < len(escaped) {
		if i+1 < len(escaped) && escaped[i] == '\\' {
			switch escaped[i+1] {
			case 'n':
				result.WriteByte('\n')
				i += 2
			case '\\':
				result.WriteByte('\\')
				i += 2
			default:
				// Unknown escape sequence, keep as is
				result.WriteByte(escaped[i])
				i++
			}
		} else {
			result.WriteByte(escaped[i])
			i++
		}
	}
	return result.String()
}

// GetHistory returns a copy of the command history
func (h *FileChatHistory) GetHistory() []string {
	result := make([]string, len(h.commands))
	copy(result, h.commands)
	return result
}

// Load reads command history from file
func (h *FileChatHistory) Load() error {
	// If saving is disabled, skip loading
	if !h.saveEnabled {
		return nil
	}
	
	// If file doesn't exist, that's not an error - just start with empty history
	if _, err := os.Stat(h.filePath); os.IsNotExist(err) {
		return nil
	}

	file, err := os.Open(h.filePath)
	if err != nil {
		return fmt.Errorf("failed to open history file: %w", err)
	}
	defer file.Close()

	h.commands = make([]string, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		escaped := strings.TrimSpace(scanner.Text())
		if escaped != "" {
			// Unescape the command when loading from file
			command := unescapeFromHistory(escaped)
			h.commands = append(h.commands, command)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read history file: %w", err)
	}

	// Trim to max size after loading
	if len(h.commands) > h.maxSize {
		h.commands = h.commands[len(h.commands)-h.maxSize:]
	}

	return nil
}

// Save writes command history to file
func (h *FileChatHistory) Save() error {
	// If saving is disabled, skip writing
	if !h.saveEnabled {
		return nil
	}
	
	// Create directory if it doesn't exist
	dir := filepath.Dir(h.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	file, err := os.Create(h.filePath)
	if err != nil {
		return fmt.Errorf("failed to create history file: %w", err)
	}
	defer file.Close()

	for _, command := range h.commands {
		// Escape the command before writing to file
		escaped := escapeForHistory(command)
		if _, err := fmt.Fprintln(file, escaped); err != nil {
			return fmt.Errorf("failed to write command to history file: %w", err)
		}
	}

	return nil
}

// NavigateNext moves forward in history (towards newer commands)
func (h *FileChatHistory) NavigateNext() string {
	if len(h.commands) == 0 {
		return ""
	}

	// Move towards newer commands (decrease index)
	newIndex := h.currentIndex - 1
	
	// Handle bounds
	if newIndex < -1 {
		newIndex = -1
	}
	
	h.currentIndex = newIndex
	
	// Return command at new position
	if h.currentIndex == -1 {
		// At the end of history - return empty string
		return ""
	} else {
		// Return historical command (reverse order, most recent first)
		return h.commands[len(h.commands)-1-h.currentIndex]
	}
}

// NavigatePrev moves backward in history (towards older commands)
func (h *FileChatHistory) NavigatePrev() string {
	if len(h.commands) == 0 {
		return ""
	}

	// Move towards older commands (increase index)
	newIndex := h.currentIndex + 1
	
	// Handle bounds
	if newIndex >= len(h.commands) {
		newIndex = len(h.commands) - 1
	}
	
	h.currentIndex = newIndex
	
	// Return historical command (reverse order, most recent first)
	return h.commands[len(h.commands)-1-h.currentIndex]
}

// ResetNavigation resets to no selection and returns empty string
func (h *FileChatHistory) ResetNavigation() string {
	h.currentIndex = -1
	return ""
}