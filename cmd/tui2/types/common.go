package types

import (
	"github.com/awesome-gocui/gocui"
)

type UserInput struct {
	Message string
	IsCommand bool
}

type Message struct {
	Role        string
	Content     string
	ContentType string // "text" or "markdown"
}

type FocusablePanel int

const (
	PanelMessages FocusablePanel = iota
	PanelInput
	PanelDebug
	PanelStatus
)

type BorderStyle string

const (
	BorderStyleNone   BorderStyle = "none"   // No borders
	BorderStyleSingle BorderStyle = "single" // Default ASCII borders
	BorderStyleDouble BorderStyle = "double" // Double-line borders
	BorderStyleRounded BorderStyle = "rounded" // Rounded corners
	BorderStyleThick  BorderStyle = "thick"  // Thick borders
)

type FocusStyle string

const (
	FocusStyleBorder     FocusStyle = "border"     // Colored border only
	FocusStyleBackground FocusStyle = "background" // Background highlight only
	FocusStyleBoth       FocusStyle = "both"       // Border + background
	FocusStyleNone       FocusStyle = "none"       // No visual focus
)

type KeyBinding struct {
	View    string
	Key     interface{}
	Mod     gocui.Modifier
	Handler func(*gocui.Gui, *gocui.View) error
}

// ModeColors defines colors for a specific gocui output mode
type ModeColors struct {
	// Accent colors (for UI elements, indicators, borders)
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
	
	// Border colors
	BorderDefault string // Default border color
	BorderFocused string // Focused border color
	BorderMuted   string // Inactive/dimmed borders
	
	// Focus colors
	FocusBackground string // Background when focused
	FocusForeground string // Text color when focused
	
	// Active state colors
	ActiveBackground string // Active component background
	ActiveForeground string // Active component text
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
	
	// Mode-specific colors (new)
	Normal    *ModeColors // Colors optimized for normal mode (8 colors)
	Color256  *ModeColors // Colors optimized for 256-color mode
	TrueColor *ModeColors // Colors optimized for true-color mode
}

type Config struct {
	ShowCursor          bool
	MarkdownRendering   bool
	Theme               string
	WrapMessages        bool
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
	
	// Component border settings
	ShowMessagesBorder  bool // Show border around messages panel
	
	// Message role labels/symbols
	UserLabel      string // Symbol for user messages (default: "○")
	AssistantLabel string // Symbol for assistant messages (default: "●")
	SystemLabel    string // Symbol for system messages (default: "●")
	ErrorLabel     string // Symbol for error messages (default: "●")
	
	Layout LayoutConfig
}

type LayoutConfig struct {
	ChatPanelWidth    float64
	ShowSidebar       bool
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