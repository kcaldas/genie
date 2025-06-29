package types

import (
	"github.com/awesome-gocui/gocui"
)

type UserInput struct {
	Message string
	IsSlashCommand bool
}

type Message struct {
	Role    string
	Content string
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

type Theme struct {
	// Content colors
	Primary   string
	Secondary string
	Tertiary  string
	Error     string
	Warning   string
	Success   string
	Muted     string
	
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