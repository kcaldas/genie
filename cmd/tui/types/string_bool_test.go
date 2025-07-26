package types

import "testing"

func TestIsStringBoolEnabled(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"enabled should be true", "enabled", true},
		{"true should be true", "true", true},
		{"disabled should be false", "disabled", false},
		{"false should be false", "false", false},
		{"empty should be false", "", false},
		{"random string should be false", "random", false},
		{"on should be false", "on", false},
		{"off should be false", "off", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsStringBoolEnabled(tt.value)
			if result != tt.expected {
				t.Errorf("IsStringBoolEnabled(%q) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestIsStringBoolEnabledWithDefault(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"enabled should be true", "enabled", true},
		{"true should be true", "true", true},
		{"disabled should be false", "disabled", false},
		{"false should be false", "false", false},
		{"empty should be true (default)", "", true},
		{"random string should be false", "random", false},
		{"on should be false", "on", false},
		{"off should be false", "off", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsStringBoolEnabledWithDefault(tt.value)
			if result != tt.expected {
				t.Errorf("IsStringBoolEnabledWithDefault(%q) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestConfigStringBoolHelpers(t *testing.T) {
	config := &Config{
		EnableMouse:         "disabled",
		ShowCursor:          "disabled", 
		MarkdownRendering:   "disabled",
		WrapMessages:        "disabled",
		ShowMessagesBorder:  "disabled",
		Layout: LayoutConfig{
			ShowSidebar: "disabled",
		},
	}

	// Test all helper methods with disabled values
	if config.IsMouseEnabled() {
		t.Error("IsMouseEnabled() should be false when set to 'disabled'")
	}
	if config.IsShowCursorEnabled() {
		t.Error("IsShowCursorEnabled() should be false when set to 'disabled'")
	}
	if config.IsMarkdownRenderingEnabled() {
		t.Error("IsMarkdownRenderingEnabled() should be false when set to 'disabled'")
	}
	if config.IsWrapMessagesEnabled() {
		t.Error("IsWrapMessagesEnabled() should be false when set to 'disabled'")
	}
	if config.IsShowMessagesBorderEnabled() {
		t.Error("IsShowMessagesBorderEnabled() should be false when set to 'disabled'")
	}
	if config.Layout.IsShowSidebarEnabled() {
		t.Error("IsShowSidebarEnabled() should be false when set to 'disabled'")
	}

	// Test with enabled values
	config.EnableMouse = "enabled"
	config.ShowCursor = "enabled"
	config.MarkdownRendering = "enabled" 
	config.WrapMessages = "enabled"
	config.ShowMessagesBorder = "enabled"
	config.Layout.ShowSidebar = "enabled"

	if !config.IsMouseEnabled() {
		t.Error("IsMouseEnabled() should be true when set to 'enabled'")
	}
	if !config.IsShowCursorEnabled() {
		t.Error("IsShowCursorEnabled() should be true when set to 'enabled'")
	}
	if !config.IsMarkdownRenderingEnabled() {
		t.Error("IsMarkdownRenderingEnabled() should be true when set to 'enabled'")
	}
	if !config.IsWrapMessagesEnabled() {
		t.Error("IsWrapMessagesEnabled() should be true when set to 'enabled'")
	}
	if !config.IsShowMessagesBorderEnabled() {
		t.Error("IsShowMessagesBorderEnabled() should be true when set to 'enabled'")
	}
	if !config.Layout.IsShowSidebarEnabled() {
		t.Error("IsShowSidebarEnabled() should be true when set to 'enabled'")
	}

	// Test with empty values (should default to enabled for these fields)
	config.EnableMouse = ""
	config.ShowCursor = ""
	config.MarkdownRendering = ""
	config.WrapMessages = ""
	config.ShowMessagesBorder = ""
	config.Layout.ShowSidebar = ""

	if !config.IsMouseEnabled() {
		t.Error("IsMouseEnabled() should be true when empty (default enabled)")
	}
	if !config.IsShowCursorEnabled() {
		t.Error("IsShowCursorEnabled() should be true when empty (default enabled)")
	}
	if !config.IsMarkdownRenderingEnabled() {
		t.Error("IsMarkdownRenderingEnabled() should be true when empty (default enabled)")
	}
	if !config.IsWrapMessagesEnabled() {
		t.Error("IsWrapMessagesEnabled() should be true when empty (default enabled)")
	}
	if !config.IsShowMessagesBorderEnabled() {
		t.Error("IsShowMessagesBorderEnabled() should be true when empty (default enabled)")
	}
	if !config.Layout.IsShowSidebarEnabled() {
		t.Error("IsShowSidebarEnabled() should be true when empty (default enabled)")
	}
}

// TestStringBoolConfigMerging tests that string boolean fields merge correctly
// This addresses the original issue where false boolean values weren't being merged
func TestStringBoolConfigMerging(t *testing.T) {
	// Test the scenario that was broken before:
	// Default config has enabled=true, user config has enabled=false
	// The user's false should override the default true
	
	// Simulate default config (enabled by default)
	defaultConfig := &Config{
		ShowCursor:         "enabled",
		MarkdownRendering:  "enabled", 
		WrapMessages:       "enabled",
		ShowMessagesBorder: "enabled",
		Layout: LayoutConfig{
			ShowSidebar: "enabled",
		},
	}
	
	// Simulate user config wanting to disable these features
	userConfig := &Config{
		ShowCursor:         "disabled", // This should override default "enabled"
		MarkdownRendering:  "disabled", // This should override default "enabled"
		WrapMessages:       "disabled", // This should override default "enabled"
		ShowMessagesBorder: "disabled", // This should override default "enabled"
		Layout: LayoutConfig{
			ShowSidebar: "disabled", // This should override default "enabled"
		},
	}
	
	// Simulate merging (this would happen in the config manager)
	// Since we're using strings, empty values won't override non-empty values
	mergedConfig := *defaultConfig
	
	// Non-empty string values should always override
	if userConfig.ShowCursor != "" {
		mergedConfig.ShowCursor = userConfig.ShowCursor
	}
	if userConfig.MarkdownRendering != "" {
		mergedConfig.MarkdownRendering = userConfig.MarkdownRendering
	}
	if userConfig.WrapMessages != "" {
		mergedConfig.WrapMessages = userConfig.WrapMessages
	}
	if userConfig.ShowMessagesBorder != "" {
		mergedConfig.ShowMessagesBorder = userConfig.ShowMessagesBorder
	}
	if userConfig.Layout.ShowSidebar != "" {
		mergedConfig.Layout.ShowSidebar = userConfig.Layout.ShowSidebar
	}
	
	// Verify that user's "disabled" values actually override defaults
	if mergedConfig.IsShowCursorEnabled() {
		t.Error("ShowCursor should be disabled after merge")
	}
	if mergedConfig.IsMarkdownRenderingEnabled() {
		t.Error("MarkdownRendering should be disabled after merge")
	}
	if mergedConfig.IsWrapMessagesEnabled() {
		t.Error("WrapMessages should be disabled after merge")
	}
	if mergedConfig.IsShowMessagesBorderEnabled() {
		t.Error("ShowMessagesBorder should be disabled after merge")
	}
	if mergedConfig.Layout.IsShowSidebarEnabled() {
		t.Error("ShowSidebar should be disabled after merge")
	}
	
	// Verify the actual string values
	if mergedConfig.ShowCursor != "disabled" {
		t.Errorf("ShowCursor string value should be 'disabled', got %q", mergedConfig.ShowCursor)
	}
	if mergedConfig.MarkdownRendering != "disabled" {
		t.Errorf("MarkdownRendering string value should be 'disabled', got %q", mergedConfig.MarkdownRendering)
	}
	if mergedConfig.WrapMessages != "disabled" {
		t.Errorf("WrapMessages string value should be 'disabled', got %q", mergedConfig.WrapMessages)
	}
	if mergedConfig.ShowMessagesBorder != "disabled" {
		t.Errorf("ShowMessagesBorder string value should be 'disabled', got %q", mergedConfig.ShowMessagesBorder)
	}
	if mergedConfig.Layout.ShowSidebar != "disabled" {
		t.Errorf("ShowSidebar string value should be 'disabled', got %q", mergedConfig.Layout.ShowSidebar)
	}
}