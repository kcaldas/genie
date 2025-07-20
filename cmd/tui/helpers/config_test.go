package helpers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kcaldas/genie/cmd/tui/types"
)

func TestDeepMerge(t *testing.T) {
	configManager := &ConfigManager{}

	tests := []struct {
		name     string
		target   *types.Config
		source   *types.Config
		expected *types.Config
	}{
		{
			name: "basic field merge",
			target: &types.Config{
				Theme:      "dark",
				OutputMode: "true",
			},
			source: &types.Config{
				Theme: "light", // Should override
				// OutputMode empty, should not override
			},
			expected: &types.Config{
				Theme:      "light", // Overridden
				OutputMode: "true",  // Preserved
			},
		},
		{
			name: "tool configs merge",
			target: &types.Config{
				ToolConfigs: map[string]types.ToolConfig{
					"tool1": {Hide: true, AutoAccept: false},
					"tool2": {Hide: false, AutoAccept: true},
				},
			},
			source: &types.Config{
				ToolConfigs: map[string]types.ToolConfig{
					"tool1": {Hide: false, AutoAccept: true}, // Should override tool1
					"tool3": {Hide: true, AutoAccept: false}, // Should add tool3
				},
			},
			expected: &types.Config{
				ToolConfigs: map[string]types.ToolConfig{
					"tool1": {Hide: false, AutoAccept: true}, // Overridden
					"tool2": {Hide: false, AutoAccept: true}, // Preserved
					"tool3": {Hide: true, AutoAccept: false}, // Added
				},
			},
		},
		{
			name: "nested struct merge",
			target: &types.Config{
				Layout: types.LayoutConfig{
					ChatPanelWidth: 0.7,
					ShowSidebar:    true,
					CompactMode:    false,
				},
			},
			source: &types.Config{
				Layout: types.LayoutConfig{
					ChatPanelWidth: 0.8, // Should override
					// ShowSidebar zero value, should not override
					CompactMode: true, // Should override
				},
			},
			expected: &types.Config{
				Layout: types.LayoutConfig{
					ChatPanelWidth: 0.8,  // Overridden
					ShowSidebar:    true,  // Preserved
					CompactMode:    true,  // Overridden
				},
			},
		},
		{
			name: "empty source should not change target",
			target: &types.Config{
				Theme:           "dark",
				OutputMode:      "true",
				ShowCursor:      true,
				MaxChatMessages: 500,
			},
			source: &types.Config{}, // All zero values
			expected: &types.Config{
				Theme:           "dark", // Preserved
				OutputMode:      "true", // Preserved
				ShowCursor:      true,   // Preserved
				MaxChatMessages: 500,    // Preserved
			},
		},
		{
			name: "string labels merge",
			target: &types.Config{
				UserLabel:      "○",
				AssistantLabel: "●",
			},
			source: &types.Config{
				UserLabel: ">", // Should override
				// AssistantLabel empty, should not override
				SystemLabel: "■", // Should set new value
			},
			expected: &types.Config{
				UserLabel:      ">", // Overridden
				AssistantLabel: "●", // Preserved
				SystemLabel:    "■", // Added
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configManager.mergeConfigs(tt.target, tt.source)

			// Compare relevant fields
			if tt.target.Theme != tt.expected.Theme {
				t.Errorf("Theme: got %q, want %q", tt.target.Theme, tt.expected.Theme)
			}
			if tt.target.OutputMode != tt.expected.OutputMode {
				t.Errorf("OutputMode: got %q, want %q", tt.target.OutputMode, tt.expected.OutputMode)
			}
			if tt.target.ShowCursor != tt.expected.ShowCursor {
				t.Errorf("ShowCursor: got %v, want %v", tt.target.ShowCursor, tt.expected.ShowCursor)
			}
			if tt.target.MaxChatMessages != tt.expected.MaxChatMessages {
				t.Errorf("MaxChatMessages: got %d, want %d", tt.target.MaxChatMessages, tt.expected.MaxChatMessages)
			}
			if tt.target.UserLabel != tt.expected.UserLabel {
				t.Errorf("UserLabel: got %q, want %q", tt.target.UserLabel, tt.expected.UserLabel)
			}
			if tt.target.AssistantLabel != tt.expected.AssistantLabel {
				t.Errorf("AssistantLabel: got %q, want %q", tt.target.AssistantLabel, tt.expected.AssistantLabel)
			}
			if tt.target.SystemLabel != tt.expected.SystemLabel {
				t.Errorf("SystemLabel: got %q, want %q", tt.target.SystemLabel, tt.expected.SystemLabel)
			}

			// Compare layout
			if tt.target.Layout.ChatPanelWidth != tt.expected.Layout.ChatPanelWidth {
				t.Errorf("Layout.ChatPanelWidth: got %f, want %f", tt.target.Layout.ChatPanelWidth, tt.expected.Layout.ChatPanelWidth)
			}
			if tt.target.Layout.ShowSidebar != tt.expected.Layout.ShowSidebar {
				t.Errorf("Layout.ShowSidebar: got %v, want %v", tt.target.Layout.ShowSidebar, tt.expected.Layout.ShowSidebar)
			}
			if tt.target.Layout.CompactMode != tt.expected.Layout.CompactMode {
				t.Errorf("Layout.CompactMode: got %v, want %v", tt.target.Layout.CompactMode, tt.expected.Layout.CompactMode)
			}

			// Compare tool configs
			if tt.expected.ToolConfigs != nil {
				if tt.target.ToolConfigs == nil {
					t.Error("ToolConfigs should not be nil")
				} else {
					for toolName, expectedConfig := range tt.expected.ToolConfigs {
						actualConfig, exists := tt.target.ToolConfigs[toolName]
						if !exists {
							t.Errorf("ToolConfig for %q should exist", toolName)
						} else {
							if actualConfig.Hide != expectedConfig.Hide {
								t.Errorf("ToolConfig[%q].Hide: got %v, want %v", toolName, actualConfig.Hide, expectedConfig.Hide)
							}
							if actualConfig.AutoAccept != expectedConfig.AutoAccept {
								t.Errorf("ToolConfig[%q].AutoAccept: got %v, want %v", toolName, actualConfig.AutoAccept, expectedConfig.AutoAccept)
							}
						}
					}
				}
			}
		})
	}
}

func TestConfigLayering(t *testing.T) {
	// Test the full config layering: defaults -> global -> local
	configManager := &ConfigManager{}

	// Simulate default config
	defaults := configManager.GetDefaultConfig()
	
	// Create a "global" config that overrides some defaults
	global := &types.Config{
		Theme:      "dark",
		OutputMode: "256",
		ToolConfigs: map[string]types.ToolConfig{
			"global-tool": {Hide: true, AutoAccept: false},
		},
	}
	
	// Create a "local" config that overrides some global settings
	local := &types.Config{
		Theme: "light", // Override global theme
		ToolConfigs: map[string]types.ToolConfig{
			"local-tool": {Hide: false, AutoAccept: true}, // Add local tool
		},
		Layout: types.LayoutConfig{
			ChatPanelWidth: 0.8, // Override default layout
		},
	}

	// Apply layering: defaults -> global -> local
	result := configManager.GetDefaultConfig()
	configManager.mergeConfigs(result, global)
	configManager.mergeConfigs(result, local)

	// Verify final result
	if result.Theme != "light" { // Should be from local
		t.Errorf("Theme: got %q, want %q", result.Theme, "light")
	}
	if result.OutputMode != "256" { // Should be from global
		t.Errorf("OutputMode: got %q, want %q", result.OutputMode, "256")
	}
	if result.ShowCursor != defaults.ShowCursor { // Should be from defaults
		t.Errorf("ShowCursor: got %v, want %v", result.ShowCursor, defaults.ShowCursor)
	}
	if result.Layout.ChatPanelWidth != 0.8 { // Should be from local
		t.Errorf("Layout.ChatPanelWidth: got %f, want %f", result.Layout.ChatPanelWidth, 0.8)
	}

	// Check tool configs
	if _, exists := result.ToolConfigs["global-tool"]; !exists {
		t.Error("global-tool should exist")
	}
	if _, exists := result.ToolConfigs["local-tool"]; !exists {
		t.Error("local-tool should exist")
	}
}

func TestConfigScopeIntegration(t *testing.T) {
	// Test the full global/local config integration
	
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	// Create config manager (this will use tempDir as working directory)
	configManager, err := NewConfigManager()
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	// Create and save global config
	globalConfig := &types.Config{
		Theme:      "dark",
		OutputMode: "256",
		ToolConfigs: map[string]types.ToolConfig{
			"GlobalTool": {Hide: true, AutoAccept: false},
		},
	}
	
	if err := configManager.SaveWithScope(globalConfig, true); err != nil {
		t.Fatalf("Failed to save global config: %v", err)
	}

	// Create local .genie directory
	if err := os.MkdirAll(".genie", 0755); err != nil {
		t.Fatalf("Failed to create local .genie dir: %v", err)
	}

	// Create and save local config
	localConfig := &types.Config{
		Theme: "light", // Override global theme
		ToolConfigs: map[string]types.ToolConfig{
			"LocalTool": {Hide: false, AutoAccept: true}, // Add local tool
		},
		Layout: types.LayoutConfig{
			ChatPanelWidth: 0.8, // Override default layout
		},
	}

	if err := configManager.SaveWithScope(localConfig, false); err != nil {
		t.Fatalf("Failed to save local config: %v", err)
	}

	// Load merged config
	mergedConfig, err := configManager.Load()
	if err != nil {
		t.Fatalf("Failed to load merged config: %v", err)
	}

	// Verify layering: defaults -> global -> local
	if mergedConfig.Theme != "light" {
		t.Errorf("Theme: got %q, want %q (should be from local)", mergedConfig.Theme, "light")
	}
	if mergedConfig.OutputMode != "256" {
		t.Errorf("OutputMode: got %q, want %q (should be from global)", mergedConfig.OutputMode, "256")
	}
	if !mergedConfig.ShowCursor {
		t.Errorf("ShowCursor: got %v, want %v (should be from defaults)", mergedConfig.ShowCursor, true)
	}
	if mergedConfig.Layout.ChatPanelWidth != 0.8 {
		t.Errorf("Layout.ChatPanelWidth: got %f, want %f (should be from local)", mergedConfig.Layout.ChatPanelWidth, 0.8)
	}

	// Verify tool configs are merged
	if _, exists := mergedConfig.ToolConfigs["GlobalTool"]; !exists {
		t.Error("GlobalTool should exist from global config")
	}
	if _, exists := mergedConfig.ToolConfigs["LocalTool"]; !exists {
		t.Error("LocalTool should exist from local config")
	}

	// Verify files exist in correct locations
	globalPath := filepath.Join(os.Getenv("HOME"), ".genie", "settings.tui.json")
	if _, err := os.Stat(globalPath); err != nil {
		t.Errorf("Global config file should exist at %s", globalPath)
	}

	localPath := filepath.Join(tempDir, ".genie", "settings.tui.json")
	if _, err := os.Stat(localPath); err != nil {
		t.Errorf("Local config file should exist at %s", localPath)
	}
}

func TestDeleteLocalConfig(t *testing.T) {
	// Test local config deletion behavior
	
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	configManager, err := NewConfigManager()
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	// Create and save a global config
	globalConfig := &types.Config{
		Theme:      "dark",
		OutputMode: "256",
	}
	if err := configManager.SaveWithScope(globalConfig, true); err != nil {
		t.Fatalf("Failed to save global config: %v", err)
	}

	// Create local .genie directory and save local config
	if err := os.MkdirAll(".genie", 0755); err != nil {
		t.Fatalf("Failed to create local .genie dir: %v", err)
	}

	localConfig := &types.Config{
		Theme: "light", // Override global
	}
	if err := configManager.SaveWithScope(localConfig, false); err != nil {
		t.Fatalf("Failed to save local config: %v", err)
	}

	// Verify local file exists
	localPath := filepath.Join(tempDir, ".genie", "settings.tui.json")
	if _, err := os.Stat(localPath); err != nil {
		t.Fatalf("Local config file should exist before deletion")
	}

	// Load config - should have local override (theme=light)
	config1, err := configManager.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if config1.Theme != "light" {
		t.Errorf("Expected theme 'light' from local config, got %q", config1.Theme)
	}

	// Delete local config
	if err := configManager.DeleteLocalConfig(); err != nil {
		t.Fatalf("Failed to delete local config: %v", err)
	}

	// Verify local file is gone
	if _, err := os.Stat(localPath); !os.IsNotExist(err) {
		t.Errorf("Local config file should be deleted")
	}

	// Reload config - should now use global config (theme=dark)
	if err := configManager.Reload(); err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}

	config2 := configManager.GetConfig()
	if config2.Theme != "dark" {
		t.Errorf("Expected theme 'dark' from global config after local deletion, got %q", config2.Theme)
	}
	if config2.OutputMode != "256" {
		t.Errorf("Expected OutputMode '256' from global config, got %q", config2.OutputMode)
	}

	// Delete local config again (should be safe when file doesn't exist)
	if err := configManager.DeleteLocalConfig(); err != nil {
		t.Errorf("Deleting non-existent local config should not error: %v", err)
	}
}