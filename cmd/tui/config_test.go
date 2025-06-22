package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	// Default should be solid cursor (no blink)
	if config.CursorBlink != false {
		t.Errorf("Expected default cursor_blink to be false, got %t", config.CursorBlink)
	}
}

func TestLoadConfigWithDefaults(t *testing.T) {
	// Should return defaults when no config file exists
	config, err := LoadConfig()
	if err != nil {
		t.Errorf("LoadConfig should not return error with defaults, got: %v", err)
	}
	
	if config.CursorBlink != false {
		t.Errorf("Expected default cursor_blink to be false, got %t", config.CursorBlink)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "genie-tui-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create .genie directory
	genieDir := filepath.Join(tempDir, ".genie")
	err = os.MkdirAll(genieDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .genie dir: %v", err)
	}
	
	// Override home directory for this test
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)
	
	// Create config with cursor blinking enabled
	config := DefaultConfig()
	config.CursorBlink = true
	
	// Save config
	err = config.Save()
	if err != nil {
		t.Errorf("Failed to save config: %v", err)
	}
	
	// Load config back
	loadedConfig, err := LoadConfig()
	if err != nil {
		t.Errorf("Failed to load config: %v", err)
	}
	
	// Should have cursor blinking enabled
	if loadedConfig.CursorBlink != true {
		t.Errorf("Expected loaded cursor_blink to be true, got %t", loadedConfig.CursorBlink)
	}
}