package presentation

import (
	"github.com/kcaldas/genie/cmd/tui2/types"
)

var Themes = map[string]*types.Theme{
	"default": {
		Primary:   "\033[36m",    // Cyan
		Secondary: "\033[32m",    // Green
		Tertiary:  "\033[33m",    // Yellow
		Error:     "\033[31m",    // Red
		Warning:   "\033[93m",    // Bright Yellow
		Success:   "\033[92m",    // Bright Green
		Muted:     "\033[90m",    // Gray
	},
	"dracula": {
		Primary:   "\033[35m",    // Magenta
		Secondary: "\033[32m",    // Green
		Tertiary:  "\033[33m",    // Yellow
		Error:     "\033[31m",    // Red
		Warning:   "\033[93m",    // Bright Yellow
		Success:   "\033[92m",    // Bright Green
		Muted:     "\033[90m",    // Gray
	},
	"monokai": {
		Primary:   "\033[95m",    // Bright Magenta
		Secondary: "\033[92m",    // Bright Green
		Tertiary:  "\033[93m",    // Bright Yellow
		Error:     "\033[91m",    // Bright Red
		Warning:   "\033[33m",    // Yellow
		Success:   "\033[32m",    // Green
		Muted:     "\033[90m",    // Gray
	},
	"solarized": {
		Primary:   "\033[34m",    // Blue
		Secondary: "\033[32m",    // Green
		Tertiary:  "\033[33m",    // Yellow
		Error:     "\033[31m",    // Red
		Warning:   "\033[93m",    // Bright Yellow
		Success:   "\033[92m",    // Bright Green
		Muted:     "\033[90m",    // Gray
	},
	"nord": {
		Primary:   "\033[94m",    // Bright Blue
		Secondary: "\033[96m",    // Bright Cyan
		Tertiary:  "\033[93m",    // Bright Yellow
		Error:     "\033[91m",    // Bright Red
		Warning:   "\033[33m",    // Yellow
		Success:   "\033[92m",    // Bright Green
		Muted:     "\033[37m",    // Light Gray
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