package tui2

import (
	"github.com/awesome-gocui/gocui"
)

// Theme represents a UI theme with colors and styles
type Theme struct {
	Name        string
	Description string
	
	// Primary colors
	PrimaryColor     gocui.Attribute
	SecondaryColor   gocui.Attribute
	AccentColor      gocui.Attribute
	
	// Background and text
	BackgroundColor  gocui.Attribute
	ForegroundColor  gocui.Attribute
	
	// State colors
	SuccessColor     gocui.Attribute
	ErrorColor       gocui.Attribute
	WarningColor     gocui.Attribute
	InfoColor        gocui.Attribute
	
	// Border and focus
	BorderColor      gocui.Attribute
	FocusBorderColor gocui.Attribute
	
	// Special elements
	StatusColor      gocui.Attribute
	LoadingColor     gocui.Attribute
	ToolColor        gocui.Attribute
	
	// Theme behavior flags
	IsMinimalTheme bool // Minimal theme: no borders/backgrounds when unfocused
	
	// Glamour theme name to match
	GlamourTheme     string
}

// ThemeManager manages application themes
type ThemeManager struct {
	themes       map[string]*Theme
	currentTheme *Theme
}

// NewThemeManager creates a new theme manager with built-in themes
func NewThemeManager() *ThemeManager {
	tm := &ThemeManager{
		themes: make(map[string]*Theme),
	}
	
	// Register built-in themes
	tm.registerBuiltinThemes()
	
	// Set default theme
	tm.currentTheme = tm.themes["dark"]
	
	return tm
}

// registerBuiltinThemes registers the built-in themes
func (tm *ThemeManager) registerBuiltinThemes() {
	// Dark theme (default)
	tm.themes["dark"] = &Theme{
		Name:             "Dark",
		Description:      "Dark theme with blue accents",
		PrimaryColor:     gocui.ColorBlue,
		SecondaryColor:   gocui.ColorCyan,
		AccentColor:      gocui.ColorMagenta,
		BackgroundColor:  gocui.ColorBlack,
		ForegroundColor:  gocui.ColorWhite,
		SuccessColor:     gocui.ColorGreen,
		ErrorColor:       gocui.ColorRed,
		WarningColor:     gocui.ColorYellow,
		InfoColor:        gocui.ColorCyan,
		BorderColor:      gocui.ColorDefault,
		FocusBorderColor: gocui.ColorYellow,
		StatusColor:      gocui.ColorBlue,
		LoadingColor:     gocui.ColorCyan,
		ToolColor:        gocui.ColorMagenta,
		GlamourTheme:     "dark",
	}
	
	// Light theme
	tm.themes["light"] = &Theme{
		Name:             "Light",
		Description:      "Light theme with dark text",
		PrimaryColor:     gocui.ColorBlue,
		SecondaryColor:   gocui.ColorMagenta,
		AccentColor:      gocui.ColorCyan,
		BackgroundColor:  gocui.ColorWhite,
		ForegroundColor:  gocui.ColorBlack,
		SuccessColor:     gocui.ColorGreen,
		ErrorColor:       gocui.ColorRed,
		WarningColor:     gocui.ColorYellow,
		InfoColor:        gocui.ColorBlue,
		BorderColor:      gocui.ColorBlack,
		FocusBorderColor: gocui.ColorBlue,
		StatusColor:      gocui.ColorBlue,
		LoadingColor:     gocui.ColorMagenta,
		ToolColor:        gocui.ColorCyan,
		GlamourTheme:     "light",
	}
	
	// Dracula theme
	tm.themes["dracula"] = &Theme{
		Name:             "Dracula",
		Description:      "Dracula color scheme",
		PrimaryColor:     gocui.ColorMagenta,
		SecondaryColor:   gocui.ColorCyan,
		AccentColor:      gocui.ColorGreen,
		BackgroundColor:  gocui.ColorBlack,
		ForegroundColor:  gocui.ColorWhite,
		SuccessColor:     gocui.ColorGreen,
		ErrorColor:       gocui.ColorRed,
		WarningColor:     gocui.ColorYellow,
		InfoColor:        gocui.ColorCyan,
		BorderColor:      gocui.ColorMagenta,
		FocusBorderColor: gocui.ColorGreen,
		StatusColor:      gocui.ColorMagenta,
		LoadingColor:     gocui.ColorCyan,
		ToolColor:        gocui.ColorGreen,
		GlamourTheme:     "dracula",
	}
	
	// Tokyo Night theme
	tm.themes["tokyo-night"] = &Theme{
		Name:             "Tokyo Night",
		Description:      "Tokyo Night color scheme",
		PrimaryColor:     gocui.ColorBlue,
		SecondaryColor:   gocui.ColorMagenta,
		AccentColor:      gocui.ColorCyan,
		BackgroundColor:  gocui.ColorBlack,
		ForegroundColor:  gocui.ColorWhite,
		SuccessColor:     gocui.ColorGreen,
		ErrorColor:       gocui.ColorRed,
		WarningColor:     gocui.ColorYellow,
		InfoColor:        gocui.ColorCyan,
		BorderColor:      gocui.ColorBlue,
		FocusBorderColor: gocui.ColorMagenta,
		StatusColor:      gocui.ColorBlue,
		LoadingColor:     gocui.ColorMagenta,
		ToolColor:        gocui.ColorCyan,
		GlamourTheme:     "tokyo-night",
	}
	
	// High Contrast theme
	tm.themes["high-contrast"] = &Theme{
		Name:             "High Contrast",
		Description:      "High contrast theme for accessibility",
		PrimaryColor:     gocui.ColorWhite,
		SecondaryColor:   gocui.ColorYellow,
		AccentColor:      gocui.ColorCyan,
		BackgroundColor:  gocui.ColorBlack,
		ForegroundColor:  gocui.ColorWhite,
		SuccessColor:     gocui.ColorGreen,
		ErrorColor:       gocui.ColorRed,
		WarningColor:     gocui.ColorYellow,
		InfoColor:        gocui.ColorWhite,
		BorderColor:      gocui.ColorWhite,
		FocusBorderColor: gocui.ColorYellow,
		StatusColor:      gocui.ColorWhite,
		LoadingColor:     gocui.ColorYellow,
		ToolColor:        gocui.ColorCyan,
		GlamourTheme:     "dark",
	}
	
	// Minimal theme
	tm.themes["minimal"] = &Theme{
		Name:                     "Minimal",
		Description:              "Minimal theme - no backgrounds, only focused borders",
		PrimaryColor:             gocui.ColorDefault,
		SecondaryColor:           gocui.ColorDefault,
		AccentColor:              gocui.ColorBlue,
		BackgroundColor:          gocui.ColorDefault, // No background anywhere
		ForegroundColor:          gocui.ColorDefault,
		SuccessColor:             gocui.ColorGreen,
		ErrorColor:               gocui.ColorRed,
		WarningColor:             gocui.ColorYellow,
		InfoColor:                gocui.ColorBlue,
		BorderColor:              gocui.ColorBlack | gocui.AttrDim,
		FocusBorderColor:         gocui.ColorBlack | gocui.AttrDim,
		StatusColor:              gocui.ColorDefault,
		LoadingColor:             gocui.ColorBlue,
		ToolColor:                gocui.ColorBlue,
		IsMinimalTheme:           true, // Key feature: minimal styling when not focused
		GlamourTheme:             "auto",
	}
}

// SetTheme sets the current theme by name
func (tm *ThemeManager) SetTheme(name string) bool {
	if theme, exists := tm.themes[name]; exists {
		tm.currentTheme = theme
		return true
	}
	return false
}

// GetCurrentTheme returns the current theme
func (tm *ThemeManager) GetCurrentTheme() *Theme {
	return tm.currentTheme
}

// GetTheme returns a theme by name
func (tm *ThemeManager) GetTheme(name string) *Theme {
	return tm.themes[name]
}

// GetAvailableThemes returns a sorted list of available theme names
func (tm *ThemeManager) GetAvailableThemes() []string {
	// Return themes in a logical order
	orderedNames := []string{"dark", "light", "dracula", "tokyo-night", "high-contrast", "minimal"}
	
	// Filter to only include themes that actually exist
	var availableNames []string
	for _, name := range orderedNames {
		if _, exists := tm.themes[name]; exists {
			availableNames = append(availableNames, name)
		}
	}
	
	return availableNames
}

// GetThemeInfo returns theme info for display
func (tm *ThemeManager) GetThemeInfo() map[string]string {
	info := make(map[string]string)
	for name, theme := range tm.themes {
		info[name] = theme.Description
	}
	return info
}

// ApplyTheme applies the current theme to a view
func (tm *ThemeManager) ApplyTheme(v *gocui.View, element ThemeElement) {
	if v == nil || tm.currentTheme == nil {
		return
	}
	
	switch element {
	case ElementDefault:
		v.BgColor = tm.currentTheme.BackgroundColor
		v.FgColor = tm.currentTheme.ForegroundColor
		v.FrameColor = tm.currentTheme.BorderColor
	case ElementFocused:
		v.BgColor = tm.currentTheme.BackgroundColor
		v.FgColor = tm.currentTheme.ForegroundColor
		v.FrameColor = tm.currentTheme.FocusBorderColor
	case ElementPrimary:
		v.BgColor = tm.currentTheme.BackgroundColor
		v.FgColor = tm.currentTheme.PrimaryColor
		v.FrameColor = tm.currentTheme.PrimaryColor
	case ElementSecondary:
		v.BgColor = tm.currentTheme.BackgroundColor
		v.FgColor = tm.currentTheme.SecondaryColor
		v.FrameColor = tm.currentTheme.SecondaryColor
	case ElementSuccess:
		v.BgColor = tm.currentTheme.BackgroundColor
		v.FgColor = tm.currentTheme.SuccessColor
		v.FrameColor = tm.currentTheme.SuccessColor
	case ElementError:
		v.BgColor = tm.currentTheme.BackgroundColor
		v.FgColor = tm.currentTheme.ErrorColor
		v.FrameColor = tm.currentTheme.ErrorColor
	case ElementWarning:
		v.BgColor = tm.currentTheme.BackgroundColor
		v.FgColor = tm.currentTheme.WarningColor
		v.FrameColor = tm.currentTheme.WarningColor
	case ElementInfo:
		v.BgColor = tm.currentTheme.BackgroundColor
		v.FgColor = tm.currentTheme.InfoColor
		v.FrameColor = tm.currentTheme.InfoColor
	}
}

// ThemeElement represents different UI elements that can be themed
type ThemeElement int

const (
	ElementDefault ThemeElement = iota
	ElementFocused
	ElementPrimary
	ElementSecondary
	ElementSuccess
	ElementError
	ElementWarning
	ElementInfo
)

// GetANSIColor returns ANSI color code for use in text output
func (tm *ThemeManager) GetANSIColor(element ThemeElement) string {
	if tm.currentTheme == nil {
		return ""
	}
	
	var color gocui.Attribute
	switch element {
	case ElementPrimary:
		color = tm.currentTheme.PrimaryColor
	case ElementSecondary:
		color = tm.currentTheme.SecondaryColor
	case ElementSuccess:
		color = tm.currentTheme.SuccessColor
	case ElementError:
		color = tm.currentTheme.ErrorColor
	case ElementWarning:
		color = tm.currentTheme.WarningColor
	case ElementInfo:
		color = tm.currentTheme.InfoColor
	default:
		color = tm.currentTheme.ForegroundColor
	}
	
	return gocuiColorToANSI(color)
}

// gocuiColorToANSI converts gocui color to ANSI escape code
func gocuiColorToANSI(color gocui.Attribute) string {
	switch color {
	case gocui.ColorBlack:
		return "\033[30m"
	case gocui.ColorRed:
		return "\033[31m"
	case gocui.ColorGreen:
		return "\033[32m"
	case gocui.ColorYellow:
		return "\033[33m"
	case gocui.ColorBlue:
		return "\033[34m"
	case gocui.ColorMagenta:
		return "\033[35m"
	case gocui.ColorCyan:
		return "\033[36m"
	case gocui.ColorWhite:
		return "\033[37m"
	default:
		return "\033[0m" // Reset/default
	}
}