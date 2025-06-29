package presentation

import (
	"github.com/kcaldas/genie/cmd/tui2/types"
)

var Themes = map[string]*types.Theme{
	"default": {
		// Content colors
		Primary:   "\033[36m",    // Cyan
		Secondary: "\033[32m",    // Green
		Tertiary:  "\033[33m",    // Yellow
		Error:     "\033[31m",    // Red
		Warning:   "\033[93m",    // Bright Yellow
		Success:   "\033[92m",    // Bright Green
		Muted:     "\033[90m",    // Gray
		
		// Border colors
		BorderDefault: "\033[37m",    // Light Gray
		BorderFocused: "\033[36m",    // Cyan (matches Primary)
		BorderMuted:   "\033[90m",    // Dark Gray
		
		// Focus colors
		FocusBackground: "\033[46m",  // Cyan background
		FocusForeground: "\033[30m",  // Black text
		
		// Active state colors
		ActiveBackground: "\033[100m", // Dark gray background
		ActiveForeground: "\033[97m",  // Bright white text
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

func GetThemeNames() []string {
	names := make([]string, 0, len(Themes))
	for name := range Themes {
		names = append(names, name)
	}
	return names
}