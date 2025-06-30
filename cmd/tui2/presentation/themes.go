package presentation

import (
	"github.com/kcaldas/genie/cmd/tui2/types"
)

var Themes = map[string]*types.Theme{
	"default": {
		// Legacy colors (for backwards compatibility)
		Primary:   "\033[36m",    // Cyan
		Secondary: "\033[32m",    // Green
		Tertiary:  "\033[33m",    // Yellow
		Error:     "\033[31m",    // Red
		Warning:   "\033[93m",    // Bright Yellow
		Success:   "\033[92m",    // Bright Green
		Muted:     "\033[90m",    // Gray
		BorderDefault: "\033[37m",    // Light Gray
		BorderFocused: "\033[36m",    // Cyan (matches Primary)
		BorderMuted:   "\033[90m",    // Dark Gray
		FocusBackground: "\033[46m",  // Cyan background
		FocusForeground: "\033[30m",  // Black text
		ActiveBackground: "\033[100m", // Dark gray background
		ActiveForeground: "\033[97m",  // Bright white text
		
		// Mode-specific colors
		Normal: &types.ModeColors{
			Primary:          "\033[36m",   // Cyan
			Secondary:        "\033[32m",   // Green  
			Tertiary:         "\033[33m",   // Yellow
			Error:            "\033[31m",   // Red
			Warning:          "\033[33m",   // Yellow (no bright yellow in 8-color)
			Success:          "\033[32m",   // Green
			Muted:            "\033[37m",   // White (no gray in 8-color)
			BorderDefault:    "\033[37m",   // White
			BorderFocused:    "\033[36m",   // Cyan
			BorderMuted:      "\033[30m",   // Black
			FocusBackground:  "\033[46m",   // Cyan background
			FocusForeground:  "\033[30m",   // Black text
			ActiveBackground: "\033[40m",   // Black background
			ActiveForeground: "\033[37m",   // White text
		},
		Color256: &types.ModeColors{
			Primary:          "\033[38;5;51m",  // Bright cyan (256-color)
			Secondary:        "\033[38;5;46m",  // Bright green
			Tertiary:         "\033[38;5;226m", // Bright yellow
			Error:            "\033[38;5;196m", // Bright red
			Warning:          "\033[38;5;208m", // Orange
			Success:          "\033[38;5;82m",  // Lime green
			Muted:            "\033[38;5;244m", // Gray
			BorderDefault:    "\033[38;5;250m", // Light gray
			BorderFocused:    "\033[38;5;51m",  // Bright cyan
			BorderMuted:      "\033[38;5;240m", // Dark gray
			FocusBackground:  "\033[48;5;51m",  // Cyan background
			FocusForeground:  "\033[38;5;232m", // Almost black
			ActiveBackground: "\033[48;5;240m", // Dark gray background
			ActiveForeground: "\033[38;5;255m", // White text
		},
		TrueColor: &types.ModeColors{
			Primary:          "#00FFFF",   // Cyan
			Secondary:        "#00FF00",   // Green
			Tertiary:         "#FFFF00",   // Yellow
			Error:            "#FF0000",   // Red
			Warning:          "#FFA500",   // Orange
			Success:          "#00FF7F",   // Spring green
			Muted:            "#808080",   // Gray
			BorderDefault:    "#D3D3D3",   // Light gray
			BorderFocused:    "#00FFFF",   // Cyan
			BorderMuted:      "#404040",   // Dark gray
			FocusBackground:  "#00FFFF",   // Cyan
			FocusForeground:  "#000000",   // Black
			ActiveBackground: "#404040",   // Dark gray
			ActiveForeground: "#FFFFFF",   // White
		},
	},
	"dracula": {
		// Content colors
		Primary:   "\033[35m",    // Magenta
		Secondary: "\033[32m",    // Green
		Tertiary:  "\033[33m",    // Yellow
		Error:     "\033[31m",    // Red
		Warning:   "\033[93m",    // Bright Yellow
		Success:   "\033[92m",    // Bright Green
		Muted:     "\033[90m",    // Gray
		
		// Border colors
		BorderDefault: "\033[95m",    // Bright Magenta (muted)
		BorderFocused: "\033[35m",    // Magenta (matches Primary)
		BorderMuted:   "\033[90m",    // Dark Gray
		
		// Focus colors
		FocusBackground: "\033[45m",  // Magenta background
		FocusForeground: "\033[97m",  // Bright white text
		
		// Active state colors
		ActiveBackground: "\033[100m", // Dark gray background
		ActiveForeground: "\033[95m",  // Bright magenta text
	},
	"monokai": {
		// Content colors
		Primary:   "\033[95m",    // Bright Magenta
		Secondary: "\033[92m",    // Bright Green
		Tertiary:  "\033[93m",    // Bright Yellow
		Error:     "\033[91m",    // Bright Red
		Warning:   "\033[33m",    // Yellow
		Success:   "\033[32m",    // Green
		Muted:     "\033[90m",    // Gray
		
		// Border colors
		BorderDefault: "\033[37m",    // Light Gray
		BorderFocused: "\033[95m",    // Bright Magenta (matches Primary)
		BorderMuted:   "\033[90m",    // Dark Gray
		
		// Focus colors
		FocusBackground: "\033[105m", // Bright magenta background
		FocusForeground: "\033[30m",  // Black text
		
		// Active state colors
		ActiveBackground: "\033[100m", // Dark gray background
		ActiveForeground: "\033[92m",  // Bright green text
	},
	"solarized": {
		// Content colors
		Primary:   "\033[34m",    // Blue
		Secondary: "\033[32m",    // Green
		Tertiary:  "\033[33m",    // Yellow
		Error:     "\033[31m",    // Red
		Warning:   "\033[93m",    // Bright Yellow
		Success:   "\033[92m",    // Bright Green
		Muted:     "\033[90m",    // Gray
		
		// Border colors
		BorderDefault: "\033[94m",    // Bright Blue (muted)
		BorderFocused: "\033[34m",    // Blue (matches Primary)
		BorderMuted:   "\033[90m",    // Dark Gray
		
		// Focus colors
		FocusBackground: "\033[44m",  // Blue background
		FocusForeground: "\033[97m",  // Bright white text
		
		// Active state colors
		ActiveBackground: "\033[100m", // Dark gray background
		ActiveForeground: "\033[96m",  // Bright cyan text
	},
	"nord": {
		// Content colors
		Primary:   "\033[94m",    // Bright Blue
		Secondary: "\033[96m",    // Bright Cyan
		Tertiary:  "\033[93m",    // Bright Yellow
		Error:     "\033[91m",    // Bright Red
		Warning:   "\033[33m",    // Yellow
		Success:   "\033[92m",    // Bright Green
		Muted:     "\033[37m",    // Light Gray
		
		// Border colors
		BorderDefault: "\033[37m",    // Light Gray
		BorderFocused: "\033[94m",    // Bright Blue (matches Primary)
		BorderMuted:   "\033[90m",    // Dark Gray
		
		// Focus colors
		FocusBackground: "\033[104m", // Bright blue background
		FocusForeground: "\033[30m",  // Black text
		
		// Active state colors
		ActiveBackground: "\033[100m", // Dark gray background
		ActiveForeground: "\033[96m",  // Bright cyan text
	},
}

func GetTheme(name string) *types.Theme {
	if theme, ok := Themes[name]; ok {
		return theme
	}
	return Themes["default"]
}

// GetThemeForMode returns a theme with colors optimized for the specified output mode
func GetThemeForMode(name string, outputMode string) *types.Theme {
	baseTheme := GetTheme(name)
	if baseTheme == nil {
		return nil
	}
	
	// Create a copy of the base theme
	theme := *baseTheme
	
	// Override with mode-specific colors if available
	var modeColors *types.ModeColors
	switch outputMode {
	case "normal":
		modeColors = theme.Normal
	case "256":
		modeColors = theme.Color256
	case "true":
		modeColors = theme.TrueColor
	default:
		// Default to true color mode
		modeColors = theme.TrueColor
	}
	
	// If mode-specific colors are available, use them
	if modeColors != nil {
		theme.Primary = modeColors.Primary
		theme.Secondary = modeColors.Secondary
		theme.Tertiary = modeColors.Tertiary
		theme.Error = modeColors.Error
		theme.Warning = modeColors.Warning
		theme.Success = modeColors.Success
		theme.Muted = modeColors.Muted
		theme.BorderDefault = modeColors.BorderDefault
		theme.BorderFocused = modeColors.BorderFocused
		theme.BorderMuted = modeColors.BorderMuted
		theme.FocusBackground = modeColors.FocusBackground
		theme.FocusForeground = modeColors.FocusForeground
		theme.ActiveBackground = modeColors.ActiveBackground
		theme.ActiveForeground = modeColors.ActiveForeground
	}
	
	return &theme
}

func GetThemeNames() []string {
	names := make([]string, 0, len(Themes))
	for name := range Themes {
		names = append(names, name)
	}
	return names
}