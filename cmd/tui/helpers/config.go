package helpers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type ConfigManager struct {
	configPath string
	config     *types.Config
	loaded     bool
	mu         sync.RWMutex
}

func NewConfigManager() (*ConfigManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(homeDir, ".genie")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}

	return &ConfigManager{
		configPath: filepath.Join(configDir, "settings.tui.json"),
		loaded:     false,
	}, nil
}

func (h *ConfigManager) Load() (*types.Config, error) {
	config := h.GetDefaultConfig()

	data, err := os.ReadFile(h.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

func (h *ConfigManager) Save(config *types.Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(h.configPath, data, 0644)
}

// GetConfig returns the current config (thread-safe with lazy loading)
func (h *ConfigManager) GetConfig() *types.Config {
	h.mu.RLock()
	if h.loaded {
		defer h.mu.RUnlock()
		return h.config
	}
	h.mu.RUnlock()

	// Need to load config - upgrade to write lock
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// Double-check in case another goroutine loaded it while we were waiting
	if h.loaded {
		return h.config
	}
	
	// Load the config
	config, err := h.Load()
	if err != nil {
		// If loading fails, return default config and log the error
		// This prevents the app from crashing but allows it to continue with defaults
		config = h.GetDefaultConfig()
	}
	
	h.config = config
	h.loaded = true
	return h.config
}

// UpdateConfig updates the config and optionally saves to disk (thread-safe)
func (h *ConfigManager) UpdateConfig(fn func(*types.Config), save bool) error {
	// Ensure config is loaded first
	_ = h.GetConfig()
	
	h.mu.Lock()
	defer h.mu.Unlock()
	
	fn(h.config)
	
	if save {
		return h.Save(h.config)
	}
	return nil
}

// Reload reloads the config from disk (thread-safe)
func (h *ConfigManager) Reload() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	config, err := h.Load()
	if err != nil {
		return err
	}
	h.config = config
	h.loaded = true
	return nil
}

func (h *ConfigManager) GetDefaultConfig() *types.Config {
	return &types.Config{
		ShowCursor:         true,
		MarkdownRendering:  true,
		Theme:              "default",
		WrapMessages:       true,
		ShowTimestamps:     false,
		DebugEnabled:       false,  // Default to debug disabled
		OutputMode:         "true", // Default to 24-bit color with enhanced Unicode support
		GlamourTheme:       "auto", // Use automatic theme mapping by default
		ShowMessagesBorder: true,   // Default to showing borders
		MaxChatMessages:    500,    // Default to 500 messages for better context

		// Default message role labels
		UserLabel:      "○",
		AssistantLabel: "●",
		SystemLabel:    "●",
		ErrorLabel:     "●",
		Layout: types.LayoutConfig{
			ChatPanelWidth:    0.7,
			ShowSidebar:       true,
			CompactMode:       false,
			ResponsePanelMode: "split",
			MinPanelWidth:     20,
			MinPanelHeight:    3,
			BorderStyle:       types.BorderStyleSingle,
			PortraitMode:      "auto",
			SidePanelWidth:    0.25,
			ExpandedSidePanel: false,
			ShowBorders:       true,
			FocusStyle:        types.FocusStyleBorder,
		},
	}
}

// GetGocuiOutputMode converts the string config to the appropriate gocui.OutputMode
// This controls terminal color depth and Unicode character support:
//
//   - "true": 24-bit color (16M colors) with enhanced Unicode support (recommended)
//   - "256": 256-color mode with standard Unicode
//   - "normal": 8-color mode with basic character support
//
// Defaults to OutputTrue for the best experience on modern terminals.
func (h *ConfigManager) GetGocuiOutputMode(outputMode string) gocui.OutputMode {
	switch outputMode {
	case "normal":
		return gocui.OutputNormal // 8-color mode
	case "256":
		return gocui.Output256 // 256-color mode
	case "simulator":
		return gocui.OutputSimulator // Simulator mode for testing
	case "true", "":
		return gocui.OutputTrue // 24-bit color (default)
	default:
		return gocui.OutputTrue // Default to best mode
	}
}

