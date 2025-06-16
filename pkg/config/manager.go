package config

import (
	"fmt"
	"os"
)

// Manager provides configuration management functionality
type Manager interface {
	GetString(key string) (string, error)
	GetStringWithDefault(key, defaultValue string) string
	RequireString(key string) string
}

// DefaultManager implements the Manager interface
type DefaultManager struct {
}

// NewManager creates a new default config manager
func NewManager() Manager {
	return &DefaultManager{}
}

// GetString gets a configuration value by key, returns error if not found
func (m *DefaultManager) GetString(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("configuration key %s not found", key)
	}
	return value, nil
}

// GetStringWithDefault gets a configuration value by key, returns default if not found
func (m *DefaultManager) GetStringWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// RequireString gets a configuration value by key, panics if not found
func (m *DefaultManager) RequireString(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("required configuration key %s not found", key))
	}
	return value
}