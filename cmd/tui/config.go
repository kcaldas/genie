package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds TUI-specific settings
type Config struct {
	// Cursor settings
	CursorBlink bool `json:"cursor_blink"`
	
	// Timeout settings (in seconds)
	ChatTimeoutSeconds int `json:"chat_timeout_seconds"`
	
	// Theme settings
	Theme string `json:"theme"` // Theme name to load ("default", "dark", "light", or custom)
}

// DefaultConfig returns the default TUI configuration
func DefaultConfig() *Config {
	return &Config{
		CursorBlink:        false,     // Default to solid cursor (no blink)
		ChatTimeoutSeconds: 180,       // Default 3 minutes timeout
		Theme:              "default", // Default theme
	}
}

// LoadConfig loads TUI configuration from settings file
func LoadConfig() (*Config, error) {
	config := DefaultConfig()
	
	// Try to load from .genie/settings.tui.json
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return config, nil // Return defaults on error
	}
	
	configPath := filepath.Join(homeDir, ".genie", "settings.tui.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Also try local project settings
		configPath = filepath.Join(".genie", "settings.tui.json")
		data, err = os.ReadFile(configPath)
		if err != nil {
			return config, nil // Return defaults if no config file
		}
	}
	
	// Parse JSON config
	if err := json.Unmarshal(data, config); err != nil {
		return DefaultConfig(), err
	}
	
	return config, nil
}

// Save saves the TUI configuration to file
func (c *Config) Save() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	
	configDir := filepath.Join(homeDir, ".genie")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	
	configPath := filepath.Join(configDir, "settings.tui.json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(configPath, data, 0644)
}