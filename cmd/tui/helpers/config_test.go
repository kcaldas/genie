package helpers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kcaldas/genie/cmd/tui/types"
)

func TestConfigManager_LazyLoading(t *testing.T) {
	// Create a temporary directory for this test
	tempDir := t.TempDir()
	
	// Create a config manager with a temporary config path
	cm := &ConfigManager{
		configPath: filepath.Join(tempDir, "test_settings.json"),
		loaded:     false,
	}
	
	// GetConfig should work even without explicit loading
	config := cm.GetConfig()
	if config == nil {
		t.Fatal("GetConfig returned nil")
	}
	
	// Should have default values
	if !config.ShowCursor {
		t.Error("Expected default ShowCursor to be true")
	}
	
	if config.MaxChatMessages != 500 {
		t.Errorf("Expected default MaxChatMessages to be 500, got %d", config.MaxChatMessages)
	}
	
	// Config should now be marked as loaded
	if !cm.loaded {
		t.Error("Expected config to be marked as loaded after GetConfig()")
	}
	
	// Subsequent calls should return the same config
	config2 := cm.GetConfig()
	if config != config2 {
		t.Error("Expected subsequent GetConfig() calls to return the same instance")
	}
}

func TestConfigManager_LazyLoadingWithExistingFile(t *testing.T) {
	// Create a temporary directory for this test
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_settings.json")
	
	// Create a config file with custom values
	configData := `{
		"showCursor": false,
		"maxChatMessages": 100,
		"theme": "custom"
	}`
	
	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	
	// Create a config manager
	cm := &ConfigManager{
		configPath: configPath,
		loaded:     false,
	}
	
	// GetConfig should load from file
	config := cm.GetConfig()
	if config == nil {
		t.Fatal("GetConfig returned nil")
	}
	
	// Should have values from file
	if config.ShowCursor {
		t.Error("Expected ShowCursor to be false (from file)")
	}
	
	if config.MaxChatMessages != 100 {
		t.Errorf("Expected MaxChatMessages to be 100 (from file), got %d", config.MaxChatMessages)
	}
	
	if config.Theme != "custom" {
		t.Errorf("Expected Theme to be 'custom' (from file), got %s", config.Theme)
	}
}

func TestConfigManager_UpdateConfigLazyLoads(t *testing.T) {
	// Create a temporary directory for this test
	tempDir := t.TempDir()
	
	// Create a config manager
	cm := &ConfigManager{
		configPath: filepath.Join(tempDir, "test_settings.json"),
		loaded:     false,
	}
	
	// UpdateConfig should work even without explicit loading
	err := cm.UpdateConfig(func(config *types.Config) {
		config.ShowCursor = false
	}, false)
	
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}
	
	// Config should now be loaded and updated
	if !cm.loaded {
		t.Error("Expected config to be marked as loaded after UpdateConfig()")
	}
	
	config := cm.GetConfig()
	if config.ShowCursor {
		t.Error("Expected ShowCursor to be false after update")
	}
}