package helpers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/logging"
)


type ConfigManager struct {
	globalConfigPath string
	localConfigPath  string
	config           *types.Config
	loaded           bool
	mu               sync.RWMutex
}

func NewConfigManager() (*ConfigManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	globalConfigDir := filepath.Join(homeDir, ".genie")
	if err := os.MkdirAll(globalConfigDir, 0755); err != nil {
		return nil, err
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return &ConfigManager{
		globalConfigPath: filepath.Join(globalConfigDir, "settings.tui.json"),
		localConfigPath:  filepath.Join(workingDir, ".genie", "settings.tui.json"),
		loaded:           false,
	}, nil
}

func (h *ConfigManager) Load() (*types.Config, error) {
	// Start with defaults
	config := h.GetDefaultConfig()

	// Layer 1: Merge global config if it exists
	if data, err := os.ReadFile(h.globalConfigPath); err == nil {
		var globalMap map[string]interface{}
		if err := json.Unmarshal(data, &globalMap); err != nil {
			return nil, err
		}
		
		globalConfig := &types.Config{}
		if err := json.Unmarshal(data, globalConfig); err != nil {
			return nil, err
		}
		// Debug: Print global config mouse setting
		if logger := logging.GetGlobalLogger(); logger != nil {
			logger.Info("DEBUG: Global config loaded - EnableMouse = %v", globalConfig.EnableMouse)
		}
		h.mergeConfigs(config, globalConfig)
	}

	// Layer 2: Merge local config if it exists (overrides global)
	if data, err := os.ReadFile(h.localConfigPath); err == nil {
		var localMap map[string]interface{}
		if err := json.Unmarshal(data, &localMap); err != nil {
			return nil, err
		}
		
		localConfig := &types.Config{}
		if err := json.Unmarshal(data, localConfig); err != nil {
			return nil, err
		}
		// Debug: Print local config mouse setting
		if logger := logging.GetGlobalLogger(); logger != nil {
			logger.Info("DEBUG: Local config loaded - EnableMouse = %v", localConfig.EnableMouse)
		}
		h.mergeConfigs(config, localConfig)
	}
	
	// Debug: Print final merged config
	if logger := logging.GetGlobalLogger(); logger != nil {
		logger.Info("DEBUG: Final merged config - EnableMouse = %v", config.EnableMouse)
	}

	return config, nil
}

// mergeConfigs merges source config into target config using generic deep merge
func (h *ConfigManager) mergeConfigs(target, source *types.Config) {
	h.deepMerge(reflect.ValueOf(target).Elem(), reflect.ValueOf(source).Elem())
}

// deepMerge performs a deep merge of source into target using reflection
// Only non-zero values from source are copied to target
func (h *ConfigManager) deepMerge(target, source reflect.Value) {
	if !source.IsValid() || !target.IsValid() {
		return
	}

	switch source.Kind() {
	case reflect.Struct:
		for i := 0; i < source.NumField(); i++ {
			sourceField := source.Field(i)
			targetField := target.Field(i)
			
			if !targetField.CanSet() {
				continue
			}
			
			h.deepMerge(targetField, sourceField)
		}
		
	case reflect.Map:
		if source.IsNil() {
			return
		}
		
		if target.IsNil() {
			target.Set(reflect.MakeMap(target.Type()))
		}
		
		for _, key := range source.MapKeys() {
			sourceValue := source.MapIndex(key)
			// For maps, always copy from source (this handles tool configs correctly)
			target.SetMapIndex(key, sourceValue)
		}
		
	case reflect.Slice:
		if !source.IsNil() && source.Len() > 0 {
			target.Set(source)
		}
		
	default:
		// For primitive types, only set if source is non-zero
		if !h.isZeroValue(source) {
			target.Set(source)
		}
	}
}


// isZeroValue checks if a reflect.Value represents a zero value
func (h *ConfigManager) isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr, reflect.Slice, reflect.Map:
		return v.IsNil()
	default:
		// For other types, compare with zero value
		return reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
	}
}

func (h *ConfigManager) Save(config *types.Config) error {
	return h.SaveWithScope(config, false) // Default to local
}

func (h *ConfigManager) SaveWithScope(config *types.Config, global bool) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	configPath := h.localConfigPath
	if global {
		configPath = h.globalConfigPath
	} else {
		// Ensure local .genie directory exists
		localDir := filepath.Dir(h.localConfigPath)
		if err := os.MkdirAll(localDir, 0755); err != nil {
			return err
		}
	}

	return os.WriteFile(configPath, data, 0644)
}

// DeleteLocalConfig removes the local config file to allow global config to take precedence
func (h *ConfigManager) DeleteLocalConfig() error {
	if _, err := os.Stat(h.localConfigPath); os.IsNotExist(err) {
		// File doesn't exist, nothing to delete
		return nil
	}
	return os.Remove(h.localConfigPath)
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

// GetTheme returns the current theme based on config settings
func (h *ConfigManager) GetTheme() *types.Theme {
	config := h.GetConfig()
	return presentation.GetThemeForMode(config.Theme, config.OutputMode)
}

func (h *ConfigManager) GetDefaultConfig() *types.Config {
	return &types.Config{
		ShowCursor:         "enabled", // Default to showing cursor
		MarkdownRendering:  "enabled", // Default to markdown rendering
		Theme:              "default",
		WrapMessages:       "enabled", // Default to wrapping messages
		ShowTimestamps:     false,
		OutputMode:         "true", // Default to 24-bit color with enhanced Unicode support
		GlamourTheme:       "auto", // Use automatic theme mapping by default
		DiffTheme:          "auto", // Use automatic theme mapping by default
		ShowMessagesBorder: "enabled", // Default to showing borders
		MaxChatMessages:    500,    // Default to 500 messages for better context
		VimMode:            false,  // Default to normal editing mode
		EnableMouse:        "enabled",   // Default to gocui mouse support enabled

		// Default message role labels
		UserLabel:      "○",
		AssistantLabel: "●",
		SystemLabel:    "●",
		ErrorLabel:     "●",
		
		// Tool configurations - hide internal tools by default
		ToolConfigs: map[string]types.ToolConfig{
			"sequentialthinking": {Hide: true, AutoAccept: false},
		},
		
		Layout: types.LayoutConfig{
			ChatPanelWidth:    0.7,
			ShowSidebar:       "enabled", // Default to showing sidebar
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

