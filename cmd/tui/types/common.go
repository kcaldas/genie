package types

import (
	"github.com/awesome-gocui/gocui"
)


type Message struct {
	Role        string
	Content     string
	ContentType string // "text" or "markdown"
}


type BorderStyle string

const (
	BorderStyleNone   BorderStyle = "none"   // No borders
	BorderStyleSingle BorderStyle = "single" // Default ASCII borders
)

type FocusStyle string

const (
	FocusStyleBorder     FocusStyle = "border"     // Colored border only
	FocusStyleNone       FocusStyle = "none"       // No visual focus
)

type KeyBinding struct {
	View    string
	Key     interface{}
	Mod     gocui.Modifier
	Handler func(*gocui.Gui, *gocui.View) error
}


type Theme struct {
	// Accent colors (for UI elements, indicators, borders) - legacy compatibility
	Primary   string // AI assistant accents/indicators
	Secondary string // System accents/indicators  
	Tertiary  string // User accents/indicators
	Error     string
	Warning   string
	Success   string
	Muted     string
	
	// Text colors (for message content)
	TextPrimary   string // AI assistant message text
	TextSecondary string // System message text
	TextTertiary  string // User message text
	
	// Border colors (legacy)
	BorderDefault string // Default border color
	BorderFocused string // Focused border color
	BorderMuted   string // Inactive/dimmed borders
	
	// Focus colors (legacy)
	FocusBackground string // Background when focused
	FocusForeground string // Text color when focused
	
	// Active state colors (legacy)
	ActiveBackground string // Active component background
	ActiveForeground string // Active component text
	
	// Diff-specific colors
	DiffAddedFg      string // Foreground color for added lines
	DiffAddedBg      string // Background color for added lines
	DiffRemovedFg    string // Foreground color for removed lines
	DiffRemovedBg    string // Background color for removed lines
	DiffHeaderFg     string // Foreground color for file headers (+++/---)
	DiffHeaderBg     string // Background color for file headers
	DiffHunkFg       string // Foreground color for hunk headers (@@)
	DiffHunkBg       string // Background color for hunk headers
	DiffContextFg    string // Foreground color for context lines
	DiffContextBg    string // Background color for context lines
	
}

// ToolConfig holds per-tool behavior settings
type ToolConfig struct {
	Hide       bool // Hide tool execution messages in chat
	AutoAccept bool // Auto-accept confirmations for this tool
}

type Config struct {
	ShowCursor          string // "enabled" or "disabled" (default: "enabled")
	MarkdownRendering   string // "enabled" or "disabled" (default: "enabled")
	Theme               string
	WrapMessages        string // "enabled" or "disabled" (default: "enabled")
	ShowTimestamps      bool
	
	// Terminal output configuration
	// OutputMode controls gocui color and Unicode support:
	// - "true": 24-bit color with enhanced Unicode support (default, recommended)
	// - "normal": 8-color mode with basic Unicode
	// - "256": 256-color mode
	OutputMode          string
	
	// Markdown rendering configuration
	// GlamourTheme controls the glamour theme for markdown rendering:
	// Available themes: "dark", "light", "dracula", "tokyo-night", "pink", "ascii", "notty", "auto"
	// Set to "auto" to use theme-based mapping, or specify a specific glamour theme
	GlamourTheme        string
	
	// Diff rendering configuration
	// DiffTheme controls the diff theme for diff rendering:
	// Available themes: "default", "subtle", "vibrant", "github", "classic", "auto"
	// Set to "auto" to use theme-based mapping, or specify a specific diff theme
	DiffTheme           string
	
	// Component border settings
	ShowMessagesBorder  string // "enabled" or "disabled" (default: "enabled")
	
	// Chat behavior settings
	MaxChatMessages     int  // Maximum number of chat messages to keep in memory (default: 500)
	
	// Editor configuration
	VimMode             bool // Enable vim-style editing mode (default: false)
	
	// Mouse configuration
	EnableMouse         string // Enable gocui mouse support for UI interactions: "enabled" or "disabled" (default: "enabled")
	                           // When "disabled", allows terminal native text selection
	
	// Message role labels/symbols
	UserLabel      string // Symbol for user messages (default: "○")
	AssistantLabel string // Symbol for assistant messages (default: "●")
	SystemLabel    string // Symbol for system messages (default: "●")
	ErrorLabel     string // Symbol for error messages (default: "●")
	
	// Tool behavior configurations
	ToolConfigs map[string]ToolConfig // Per-tool configurations (hide/auto-accept)
	
	Layout LayoutConfig
}

type LayoutConfig struct {
	ChatPanelWidth    float64
	ShowSidebar       string // "enabled" or "disabled" (default: "enabled")
	CompactMode       bool
	ResponsePanelMode string
	MinPanelWidth     int
	MinPanelHeight    int
	BorderStyle       BorderStyle // Default border style for all components
	PortraitMode      string
	SidePanelWidth    float64
	ExpandedSidePanel bool
	ShowBorders       bool        // Global borders on/off
	FocusStyle        FocusStyle  // Default focus style for all components
}


// IsStringBoolEnabled returns true if a string boolean field is enabled
// For fields that default to DISABLED (false):
// Treats "enabled", "true" as enabled
// Treats "disabled", "false", and empty string as disabled
func IsStringBoolEnabled(value string) bool {
	return value == "enabled" || value == "true"
}

// IsStringBoolEnabledWithDefault returns true if a string boolean field is enabled
// For fields that default to ENABLED (true):
// Treats "enabled", "true", and empty string as enabled  
// Treats "disabled", "false" as disabled
func IsStringBoolEnabledWithDefault(value string) bool {
	return value == "enabled" || value == "true" || value == ""
}

// IsMouseEnabled returns true if mouse is enabled in config
func (c *Config) IsMouseEnabled() bool {
	return IsStringBoolEnabledWithDefault(c.EnableMouse)
}

// IsShowCursorEnabled returns true if cursor is enabled in config
func (c *Config) IsShowCursorEnabled() bool {
	return IsStringBoolEnabledWithDefault(c.ShowCursor)
}

// IsMarkdownRenderingEnabled returns true if markdown rendering is enabled in config
func (c *Config) IsMarkdownRenderingEnabled() bool {
	return IsStringBoolEnabledWithDefault(c.MarkdownRendering)
}

// IsWrapMessagesEnabled returns true if message wrapping is enabled in config
func (c *Config) IsWrapMessagesEnabled() bool {
	return IsStringBoolEnabledWithDefault(c.WrapMessages)
}

// IsShowMessagesBorderEnabled returns true if messages border is enabled in config
func (c *Config) IsShowMessagesBorderEnabled() bool {
	return IsStringBoolEnabledWithDefault(c.ShowMessagesBorder)
}

// IsShowSidebarEnabled returns true if sidebar is enabled in config
func (lc *LayoutConfig) IsShowSidebarEnabled() bool {
	return IsStringBoolEnabledWithDefault(lc.ShowSidebar)
}