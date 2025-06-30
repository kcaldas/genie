package helpers

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

type ConfigHelper struct {
	configPath string
}

func NewConfigHelper() (*ConfigHelper, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	
	configDir := filepath.Join(homeDir, ".genie")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}
	
	return &ConfigHelper{
		configPath: filepath.Join(configDir, "settings.tui.json"),
	}, nil
}

func (h *ConfigHelper) Load() (*types.Config, error) {
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

func (h *ConfigHelper) Save(config *types.Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(h.configPath, data, 0644)
}

func (h *ConfigHelper) GetDefaultConfig() *types.Config {
	return &types.Config{
		ShowCursor:         true,
		MarkdownRendering:  true,
		Theme:              "default",
		WrapMessages:       true,
		ShowTimestamps:     false,
		OutputMode:         "true", // Default to 24-bit color with enhanced Unicode support
		GlamourTheme:       "auto", // Use automatic theme mapping by default
		ShowMessagesBorder: true,   // Default to showing borders
		
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
func (h *ConfigHelper) GetGocuiOutputMode(outputMode string) gocui.OutputMode {
	switch outputMode {
	case "normal":
		return gocui.OutputNormal // 8-color mode
	case "256":
		return gocui.Output256    // 256-color mode
	case "true", "":
		return gocui.OutputTrue  // 24-bit color (default)
	default:
		return gocui.OutputTrue  // Default to best mode
	}
}