package presentation

import (
	"github.com/kcaldas/genie/cmd/tui2/types"
)

// Theme Color Convention:
// PRIMARY colors are used for AI assistant content (responses, indicators, etc.)
// SECONDARY colors are used for system messages (status, info, etc.)
// TERTIARY colors are used for user content (messages, input, etc.)
// 
// This design philosophy prioritizes AI assistant visibility since users spend most
// of their time reading AI responses. User content gets the least visual prominence
// since users already know what they typed.

var Themes = map[string]*types.Theme{
	"default": {
		// Accent colors (for UI elements, indicators, borders)
		Primary:   "\033[32m",    // Green (for AI assistant accents/indicators)
		Secondary: "\033[34m",    // Blue (for system accents/indicators)
		Tertiary:  "\033[90m",    // Gray (for user accents/indicators)
		Error:     "\033[91m",    // Red (keeping for visibility)
		Warning:   "\033[93m",    // Yellow (keeping for visibility)
		Success:   "\033[92m",    // Green (keeping for visibility)
		Muted:     "\033[90m",    // Gray
		
		// Text colors (for message content)
		TextPrimary:   "\033[37m",    // Light gray (AI assistant message text - readable)
		TextSecondary: "\033[37m",    // Light gray (system message text)
		TextTertiary:  "\033[90m",    // Dark gray (user message text - muted)
		
		BorderDefault: "",    // No color - use terminal default
		BorderFocused: "",    // No color - use terminal default
		BorderMuted:   "",    // No color - use terminal default
		FocusBackground: "",  // No background color
		FocusForeground: "",  // No foreground color
		ActiveBackground: "", // No background color
		ActiveForeground: "",  // No foreground color
		
		// Diff colors (subtle but readable for default theme)
		DiffAddedFg:   "\033[32m",    // Green text for additions
		DiffAddedBg:   "",            // No background
		DiffRemovedFg: "\033[31m",    // Red text for removals
		DiffRemovedBg: "",            // No background
		DiffHeaderFg:  "\033[36m",    // Cyan for file headers
		DiffHeaderBg:  "",            // No background
		DiffHunkFg:    "\033[90m",    // Gray for hunk headers
		DiffHunkBg:    "",            // No background
		DiffContextFg: "",            // Default terminal color for context
		DiffContextBg: "",            // No background
		
		// Mode-specific colors
		Normal: &types.ModeColors{
			Primary:          "\033[32m",   // Green (AI assistant accents/indicators)
			Secondary:        "\033[34m",   // Blue (system accents/indicators)
			Tertiary:         "\033[90m",   // Gray (user accents/indicators)
			Error:            "\033[31m",   // Red
			Warning:          "\033[33m",   // Yellow
			Success:          "\033[32m",   // Green
			Muted:            "\033[90m",   // Gray
			TextPrimary:      "\033[37m",   // Light gray (AI assistant message text)
			TextSecondary:    "\033[37m",   // Light gray (system message text)
			TextTertiary:     "\033[90m",   // Dark gray (user message text - muted)
			BorderDefault:    "",   // No color - use terminal default
			BorderFocused:    "",   // No color - use terminal default
			BorderMuted:      "",   // No color - use terminal default
			FocusBackground:  "",   // No background color
			FocusForeground:  "",   // No foreground color
			ActiveBackground: "",   // No background color
			ActiveForeground: "",   // No foreground color
		},
		Color256: &types.ModeColors{
			Primary:          "\033[38;2;182;215;168m", // #b6d7a8 - Light green (AI assistant accents)
			Secondary:        "\033[38;2;79;129;168m",  // #4f81a8 - Blue (system accents)
			Tertiary:         "\033[38;5;244m",         // Medium gray (user accents)
			Error:            "\033[38;2;224;102;102m", // #e06666 - Red
			Warning:          "\033[38;2;255;229;153m", // #ffe599 - Yellow
			Success:          "\033[38;2;106;168;79m",  // #6aa84f - Green
			Muted:            "\033[38;5;244m",         // Gray
			TextPrimary:      "\033[38;5;255m",         // White (AI assistant message text - very readable)
			TextSecondary:    "\033[38;5;250m",         // Light gray (system message text)
			TextTertiary:     "\033[38;5;244m",         // Medium gray (user message text - muted)
			BorderDefault:    "",         // No color - use terminal default
			BorderFocused:    "",         // No color - use terminal default
			BorderMuted:      "",         // No color - use terminal default
			FocusBackground:  "",         // No background color
			FocusForeground:  "",         // No foreground color
			ActiveBackground: "",         // No background color
			ActiveForeground: "",         // No foreground color
		},
		TrueColor: &types.ModeColors{
			Primary:          "#b6d7a8",   // Light green (AI assistant accents)
			Secondary:        "#4f81a8",   // Blue (system accents)
			Tertiary:         "#808080",   // Medium gray (user accents)
			Error:            "#e06666",   // Red
			Warning:          "#ffe599",   // Yellow
			Success:          "#6aa84f",   // Green
			Muted:            "#808080",   // Gray
			TextPrimary:      "#FFFFFF",   // White (AI assistant message text - highest readability)
			TextSecondary:    "#E0E0E0",   // Light gray (system message text)
			TextTertiary:     "#A0A0A0",   // Medium gray (user message text - muted but readable)
			BorderDefault:    "",   // No color - use terminal default
			BorderFocused:    "",   // No color - use terminal default
			BorderMuted:      "",   // No color - use terminal default
			FocusBackground:  "",   // No background color
			FocusForeground:  "",   // No foreground color
			ActiveBackground: "",   // No background color
			ActiveForeground: "",   // No foreground color
		},
	},
	"minimal": {
		// Ultra-minimalist theme with maximum content focus and minimal visual distractions
		// Based on the "almost nothing" philosophy - only essential information is visible
		
		// Accent colors (for UI elements, indicators, borders) - barely visible
		Primary:   "\033[90m",    // Dark gray (AI assistant accents - subtle)
		Secondary: "\033[90m",    // Dark gray (system accents - minimal)
		Tertiary:  "\033[90m",    // Dark gray (user accents - invisible)
		Error:     "\033[31m",    // Red (functional visibility required)
		Warning:   "\033[33m",    // Yellow (functional visibility required)
		Success:   "\033[32m",    // Green (functional visibility required)
		Muted:     "\033[90m",    // Dark gray
		
		// Text colors (for message content) - maximum readability for AI, minimal for others
		TextPrimary:   "\033[37m",    // Light gray (AI assistant text - most readable)
		TextSecondary: "\033[90m",    // Dark gray (system text - functional only)
		TextTertiary:  "\033[90m",    // Dark gray (user text - barely visible)
		
		BorderDefault: "\033[90m",    // Barely visible borders
		BorderFocused: "\033[90m",    // Same as default - no focus distraction
		BorderMuted:   "\033[90m",    // Consistent minimal borders
		FocusBackground: "\033[0m",   // No background change
		FocusForeground: "\033[37m",  // Light gray text
		ActiveBackground: "\033[0m",  // No background change
		ActiveForeground: "\033[37m", // Light gray text
		
		// Diff colors (minimal but functional)
		DiffAddedFg:   "\033[32m",    // Green for additions (functional visibility)
		DiffAddedBg:   "",            // No background
		DiffRemovedFg: "\033[31m",    // Red for removals (functional visibility)
		DiffRemovedBg: "",            // No background
		DiffHeaderFg:  "\033[90m",    // Subtle gray for headers
		DiffHeaderBg:  "",            // No background
		DiffHunkFg:    "\033[90m",    // Subtle gray for hunk headers
		DiffHunkBg:    "",            // No background
		DiffContextFg: "\033[90m",    // Subtle gray for context
		DiffContextBg: "",            // No background
		
		// Mode-specific colors
		Normal: &types.ModeColors{
			Primary:          "\033[37m",   // Light gray (AI assistant - readable)
			Secondary:        "\033[90m",   // Dark gray (system - minimal)
			Tertiary:         "\033[90m",   // Dark gray (user - barely visible)
			Error:            "\033[31m",   // Red (functional only)
			Warning:          "\033[33m",   // Yellow (functional only)
			Success:          "\033[32m",   // Green (functional only)
			Muted:            "\033[90m",   // Dark gray
			BorderDefault:    "\033[90m",   // Barely visible
			BorderFocused:    "\033[90m",   // No focus indication
			BorderMuted:      "\033[90m",   // Consistent
			FocusBackground:  "\033[0m",    // No background
			FocusForeground:  "\033[37m",   // Light gray
			ActiveBackground: "\033[0m",    // No background
			ActiveForeground: "\033[37m",   // Light gray
		},
		Color256: &types.ModeColors{
			Primary:          "\033[38;5;250m",         // AI assistant (readable but not bright)
			Secondary:        "\033[38;5;240m",         // System (very subtle)
			Tertiary:         "\033[38;5;237m",         // User (barely visible)
			Error:            "\033[38;5;124m",         // Dark red (functional)
			Warning:          "\033[38;5;136m",         // Dark yellow (functional)
			Success:          "\033[38;5;64m",          // Dark green (functional)
			Muted:            "\033[38;5;237m",         // Very dark gray
			BorderDefault:    "\033[38;5;235m",         // Almost invisible
			BorderFocused:    "\033[38;5;235m",         // Same as default
			BorderMuted:      "\033[38;5;233m",         // Even darker
			FocusBackground:  "\033[0m",                // No background
			FocusForeground:  "\033[38;5;250m",         // Light gray
			ActiveBackground: "\033[0m",                // No background
			ActiveForeground: "\033[38;5;250m",         // Light gray
		},
		TrueColor: &types.ModeColors{
			Primary:          "#505050",   // AI assistant accents (barely visible)
			Secondary:        "#404040",   // System accents (minimal)
			Tertiary:         "#303030",   // User accents (almost invisible)
			Error:            "#804040",   // Dark red (functional only)
			Warning:          "#807040",   // Dark amber (functional only)
			Success:          "#408040",   // Dark green (functional only)
			Muted:            "#404040",   // Very dark gray
			TextPrimary:      "#C0C0C0",   // AI assistant text (readable but not bright)
			TextSecondary:    "#606060",   // System text (subtle)
			TextTertiary:     "#505050",   // User text (barely visible)
			BorderDefault:    "#202020",   // Almost invisible borders
			BorderFocused:    "#202020",   // No focus indication - minimal distraction
			BorderMuted:      "#181818",   // Even more subtle
			FocusBackground:  "#000000",   // Pure black (terminal default)
			FocusForeground:  "#C0C0C0",   // Light gray
			ActiveBackground: "#000000",   // Pure black
			ActiveForeground: "#C0C0C0",   // Light gray
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
		
		// Diff colors (Dracula theme)
		DiffAddedFg:   "\033[92m",     // Bright green for additions
		DiffAddedBg:   "\033[42m",     // Green background
		DiffRemovedFg: "\033[91m",     // Bright red for removals
		DiffRemovedBg: "\033[41m",     // Red background
		DiffHeaderFg:  "\033[95m",     // Bright magenta for headers
		DiffHeaderBg:  "\033[45m",     // Magenta background
		DiffHunkFg:    "\033[96m",     // Bright cyan for hunk headers
		DiffHunkBg:    "\033[46m",     // Cyan background
		DiffContextFg: "\033[97m",     // Bright white for context
		DiffContextBg: "",             // No background for context
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
		
		// Diff colors (Monokai theme)
		DiffAddedFg:   "\033[92m",     // Bright green for additions
		DiffAddedBg:   "\033[42m",     // Green background
		DiffRemovedFg: "\033[91m",     // Bright red for removals
		DiffRemovedBg: "\033[41m",     // Red background
		DiffHeaderFg:  "\033[95m",     // Bright magenta for headers
		DiffHeaderBg:  "\033[105m",    // Bright magenta background
		DiffHunkFg:    "\033[93m",     // Bright yellow for hunk headers
		DiffHunkBg:    "\033[103m",    // Bright yellow background
		DiffContextFg: "\033[97m",     // Bright white for context
		DiffContextBg: "",             // No background for context
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
		
		// Diff colors (Solarized theme)
		DiffAddedFg:   "\033[92m",     // Bright green for additions
		DiffAddedBg:   "\033[42m",     // Green background
		DiffRemovedFg: "\033[91m",     // Bright red for removals
		DiffRemovedBg: "\033[41m",     // Red background
		DiffHeaderFg:  "\033[94m",     // Bright blue for headers
		DiffHeaderBg:  "\033[44m",     // Blue background
		DiffHunkFg:    "\033[96m",     // Bright cyan for hunk headers
		DiffHunkBg:    "\033[46m",     // Cyan background
		DiffContextFg: "\033[97m",     // Bright white for context
		DiffContextBg: "",             // No background for context
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
		
		// Diff colors (Nord theme)
		DiffAddedFg:   "\033[92m",     // Bright green for additions
		DiffAddedBg:   "\033[42m",     // Green background
		DiffRemovedFg: "\033[91m",     // Bright red for removals
		DiffRemovedBg: "\033[41m",     // Red background
		DiffHeaderFg:  "\033[94m",     // Bright blue for headers
		DiffHeaderBg:  "\033[44m",     // Blue background
		DiffHunkFg:    "\033[96m",     // Bright cyan for hunk headers
		DiffHunkBg:    "\033[46m",     // Cyan background
		DiffContextFg: "\033[97m",     // Bright white for context
		DiffContextBg: "",             // No background for context
	},
}

// GetThemeForMode returns a theme with colors optimized for the specified output mode
func GetThemeForMode(name string, outputMode string) *types.Theme {
	// Get base theme
	var baseTheme *types.Theme
	if theme, ok := Themes[name]; ok {
		baseTheme = theme
	} else {
		baseTheme = Themes["default"]
	}
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
		theme.TextPrimary = modeColors.TextPrimary
		theme.TextSecondary = modeColors.TextSecondary
		theme.TextTertiary = modeColors.TextTertiary
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