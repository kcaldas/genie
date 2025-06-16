package fileops

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Manager provides file operation functionality
type Manager interface {
	EnsureDir(path string) error
	WriteFile(path string, content []byte) error
	ReadFile(path string) ([]byte, error)
	FileExists(path string) bool
	WriteObjectAsYAML(path string, object interface{}) error
}

// DefaultManager implements the Manager interface
type DefaultManager struct {
}

// NewFileOpsManager creates a new default file manager
func NewFileOpsManager() Manager {
	return &DefaultManager{}
}

// EnsureDir creates a directory if it doesn't exist
func (m *DefaultManager) EnsureDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}

// WriteFile writes content to a file, creating directories as needed
func (m *DefaultManager) WriteFile(path string, content []byte) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := m.EnsureDir(dir); err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}
	
	// Write file
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()
	
	_, err = file.Write(content)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}
	
	return nil
}

// ReadFile reads content from a file
func (m *DefaultManager) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// FileExists checks if a file exists
func (m *DefaultManager) FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// WriteObjectAsYAML marshals an object to YAML and writes it to a file
func (m *DefaultManager) WriteObjectAsYAML(path string, object interface{}) error {
	data, err := yaml.Marshal(object)
	if err != nil {
		return fmt.Errorf("error marshalling to YAML: %w", err)
	}
	
	return m.WriteFile(path, data)
}