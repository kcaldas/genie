package presentation

import (
	"fmt"
	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/types"
)

// Theme defines a theme using W3C hex colors - clean and precise!
type Theme struct {
	// Border colors
	BorderDefault string
	BorderFocused string
	BorderMuted   string
	
	// Title colors (for titles and subtitles)
	TitleDefault string
	TitleFocused string
	TitleMuted   string
	
	// Text colors (for content that needs gocui colors)
	TextPrimary   string
	TextSecondary string
	TextTertiary  string
	
	// Accent colors
	Primary   string
	Secondary string
	Tertiary  string
	Error     string
	Warning   string
	Success   string
	Muted     string
}

var Themes = map[string]*Theme{
	"default": {
		// Border colors - matte/subdued colors
		BorderDefault: "#6B6B6B",  // Matte gray - visible but dimmed
		BorderFocused: "#B0B0B0",  // Light matte gray - clearly visible when focused
		BorderMuted:   "#3C3C3C",  // Dark matte gray - very dimmed
		
		// Title colors - slightly lighter than borders for better readability
		TitleDefault: "#8A8A8A",  // Light gray - more readable than border
		TitleFocused: "#D0D0D0",  // Brighter gray when focused
		TitleMuted:   "#5A5A5A",  // Dimmed gray
		
		// Text colors - softer whites and grays
		TextPrimary:   "#E8E8E8",  // Off-white - AI assistant text (less harsh than pure white)
		TextSecondary: "#D4D4D4",  // Light gray - System text
		TextTertiary:  "#8A8A8A",  // Medium gray - User text (dimmed)
		
		// Accent colors - matte/desaturated versions
		Primary:   "#6B9B6B",      // Matte green - AI assistant accents
		Secondary: "#6B8CAF",      // Matte blue - System accents
		Tertiary:  "#8A8A8A",      // Matte gray - User accents
		Error:     "#C85450",      // Matte red - Errors
		Warning:   "#D4A854",      // Matte yellow/orange - Warnings
		Success:   "#6B9B6B",      // Matte green - Success
		Muted:     "#5A5A5A",      // Matte dark gray - Muted elements
	},
	"minimal": {
		// Border colors - very subtle, minimal
		BorderDefault: "#505050",  // Dark gray - very subtle
		BorderFocused: "#808080",  // Medium gray - minimal focus indication
		BorderMuted:   "#2A2A2A",  // Very dark gray - barely visible
		
		// Title colors - minimal but readable
		TitleDefault: "#707070",  // Slightly lighter for readability
		TitleFocused: "#A0A0A0",  // Brighter when focused
		TitleMuted:   "#404040",  // Very muted
		
		// Text colors - minimal contrast
		TextPrimary:   "#D0D0D0",  // Light gray - subtle
		TextSecondary: "#B0B0B0",  // Medium gray - even more subtle
		TextTertiary:  "#707070",  // Dark gray - very muted
		
		// Accent colors - minimal, monochromatic
		Primary:   "#808080",      // Neutral gray - minimal accent
		Secondary: "#707070",      // Darker gray - system accents
		Tertiary:  "#606060",      // Even darker gray - user accents
		Error:     "#A05050",      // Muted red - subtle error
		Warning:   "#A0A050",      // Muted yellow - subtle warning
		Success:   "#50A050",      // Muted green - subtle success
		Muted:     "#404040",      // Dark gray - muted elements
	},
	"dracula": {
		// Border colors - Dracula theme inspired
		BorderDefault: "#6272A4",  // Dracula comment blue
		BorderFocused: "#8BE9FD",  // Dracula cyan
		BorderMuted:   "#44475A",  // Dracula current line
		
		// Title colors - Dracula inspired, slightly brighter than borders
		TitleDefault: "#8BE9FD",  // Dracula cyan - more prominent
		TitleFocused: "#F1FA8C",  // Dracula yellow - bright when focused
		TitleMuted:   "#6272A4",  // Dracula comment - muted
		
		// Text colors - Dracula foreground colors
		TextPrimary:   "#F8F8F2",  // Dracula foreground
		TextSecondary: "#E6E6E6",  // Slightly dimmed
		TextTertiary:  "#6272A4",  // Dracula comment
		
		// Accent colors - Dracula palette
		Primary:   "#50FA7B",      // Dracula green
		Secondary: "#BD93F9",      // Dracula purple
		Tertiary:  "#6272A4",      // Dracula comment
		Error:     "#FF5555",      // Dracula red
		Warning:   "#F1FA8C",      // Dracula yellow
		Success:   "#50FA7B",      // Dracula green
		Muted:     "#44475A",      // Dracula current line
	},
	"monokai": {
		// Border colors - Monokai inspired
		BorderDefault: "#75715E",  // Monokai comment
		BorderFocused: "#A6E22E",  // Monokai green
		BorderMuted:   "#49483E",  // Monokai line highlight
		
		// Title colors - Monokai inspired, more vibrant
		TitleDefault: "#A6E22E",  // Monokai green - prominent
		TitleFocused: "#E6DB74",  // Monokai yellow - bright when focused
		TitleMuted:   "#75715E",  // Monokai comment - muted
		
		// Text colors - Monokai foreground
		TextPrimary:   "#F8F8F2",  // Monokai foreground
		TextSecondary: "#E6E6E6",  // Slightly dimmed
		TextTertiary:  "#75715E",  // Monokai comment
		
		// Accent colors - Monokai palette
		Primary:   "#A6E22E",      // Monokai green
		Secondary: "#66D9EF",      // Monokai cyan
		Tertiary:  "#75715E",      // Monokai comment
		Error:     "#F92672",      // Monokai red
		Warning:   "#E6DB74",      // Monokai yellow
		Success:   "#A6E22E",      // Monokai green
		Muted:     "#49483E",      // Monokai line highlight
	},
	"solarized": {
		// Border colors - Solarized inspired
		BorderDefault: "#657B83",  // Solarized base00
		BorderFocused: "#839496",  // Solarized base0
		BorderMuted:   "#073642",  // Solarized base02
		
		// Title colors - Solarized inspired, using accent colors
		TitleDefault: "#268BD2",  // Solarized blue - prominent
		TitleFocused: "#B58900",  // Solarized yellow - bright when focused
		TitleMuted:   "#657B83",  // Solarized base00 - muted
		
		// Text colors - Solarized foreground
		TextPrimary:   "#EEE8D5",  // Solarized base3
		TextSecondary: "#93A1A1",  // Solarized base1
		TextTertiary:  "#657B83",  // Solarized base00
		
		// Accent colors - Solarized palette
		Primary:   "#859900",      // Solarized green
		Secondary: "#268BD2",      // Solarized blue
		Tertiary:  "#657B83",      // Solarized base00
		Error:     "#DC322F",      // Solarized red
		Warning:   "#B58900",      // Solarized yellow
		Success:   "#859900",      // Solarized green
		Muted:     "#586E75",      // Solarized base01
	},
	"nord": {
		// Border colors - Nord theme inspired
		BorderDefault: "#616E88",  // Nord frost
		BorderFocused: "#88C0D0",  // Nord frost light
		BorderMuted:   "#3B4252",  // Nord polar night
		
		// Title colors - Nord inspired, using accent colors
		TitleDefault: "#88C0D0",  // Nord frost light - prominent
		TitleFocused: "#EBCB8B",  // Nord yellow - bright when focused
		TitleMuted:   "#616E88",  // Nord frost - muted
		
		// Text colors - Nord snow storm
		TextPrimary:   "#ECEFF4",  // Nord snow storm
		TextSecondary: "#E5E9F0",  // Nord snow storm
		TextTertiary:  "#616E88",  // Nord frost
		
		// Accent colors - Nord palette
		Primary:   "#A3BE8C",      // Nord green
		Secondary: "#5E81AC",      // Nord blue
		Tertiary:  "#616E88",      // Nord frost
		Error:     "#BF616A",      // Nord red
		Warning:   "#EBCB8B",      // Nord yellow
		Success:   "#A3BE8C",      // Nord green
		Muted:     "#4C566A",      // Nord polar night
	},
	"catppuccin": {
		// Border colors - Catppuccin Mocha inspired
		BorderDefault: "#585B70",  // Catppuccin surface2
		BorderFocused: "#89B4FA",  // Catppuccin blue
		BorderMuted:   "#313244",  // Catppuccin surface0
		
		// Title colors - Catppuccin inspired, using accent colors
		TitleDefault: "#89B4FA",  // Catppuccin blue - prominent
		TitleFocused: "#F9E2AF",  // Catppuccin yellow - bright when focused
		TitleMuted:   "#585B70",  // Catppuccin surface2 - muted
		
		// Text colors - Catppuccin text colors
		TextPrimary:   "#CDD6F4",  // Catppuccin text
		TextSecondary: "#BAC2DE",  // Catppuccin subtext1
		TextTertiary:  "#6C7086",  // Catppuccin overlay1
		
		// Accent colors - Catppuccin palette
		Primary:   "#A6E3A1",      // Catppuccin green
		Secondary: "#89B4FA",      // Catppuccin blue
		Tertiary:  "#6C7086",      // Catppuccin overlay1
		Error:     "#F38BA8",      // Catppuccin red
		Warning:   "#F9E2AF",      // Catppuccin yellow
		Success:   "#A6E3A1",      // Catppuccin green
		Muted:     "#45475A",      // Catppuccin surface1
	},
	"tokyo-night": {
		// Border colors - Tokyo Night inspired
		BorderDefault: "#565F89",  // Tokyo Night comment
		BorderFocused: "#7AA2F7",  // Tokyo Night blue
		BorderMuted:   "#32344A",  // Tokyo Night darker
		
		// Title colors - Tokyo Night inspired, using accent colors
		TitleDefault: "#7AA2F7",  // Tokyo Night blue - prominent
		TitleFocused: "#E0AF68",  // Tokyo Night orange - bright when focused
		TitleMuted:   "#565F89",  // Tokyo Night comment - muted
		
		// Text colors - Tokyo Night foreground
		TextPrimary:   "#C0CAF5",  // Tokyo Night foreground
		TextSecondary: "#A9B1D6",  // Tokyo Night foreground dimmed
		TextTertiary:  "#565F89",  // Tokyo Night comment
		
		// Accent colors - Tokyo Night palette
		Primary:   "#9ECE6A",      // Tokyo Night green
		Secondary: "#7AA2F7",      // Tokyo Night blue
		Tertiary:  "#565F89",      // Tokyo Night comment
		Error:     "#F7768E",      // Tokyo Night red
		Warning:   "#E0AF68",      // Tokyo Night orange
		Success:   "#9ECE6A",      // Tokyo Night green
		Muted:     "#414868",      // Tokyo Night bg_highlight
	},
	"gruvbox": {
		// Border colors - Gruvbox dark inspired
		BorderDefault: "#928374",  // Gruvbox gray
		BorderFocused: "#83A598",  // Gruvbox blue
		BorderMuted:   "#504945",  // Gruvbox dark2
		
		// Title colors - Gruvbox inspired, using accent colors
		TitleDefault: "#83A598",  // Gruvbox blue - prominent
		TitleFocused: "#FABD2F",  // Gruvbox yellow - bright when focused
		TitleMuted:   "#928374",  // Gruvbox gray - muted
		
		// Text colors - Gruvbox foreground
		TextPrimary:   "#EBDBB2",  // Gruvbox light0
		TextSecondary: "#D5C4A1",  // Gruvbox light1
		TextTertiary:  "#928374",  // Gruvbox gray
		
		// Accent colors - Gruvbox palette
		Primary:   "#B8BB26",      // Gruvbox green
		Secondary: "#83A598",      // Gruvbox blue
		Tertiary:  "#928374",      // Gruvbox gray
		Error:     "#FB4934",      // Gruvbox red
		Warning:   "#FABD2F",      // Gruvbox yellow
		Success:   "#B8BB26",      // Gruvbox green
		Muted:     "#665C54",      // Gruvbox dark3
	},
	"github-dark": {
		// Border colors - GitHub Dark inspired
		BorderDefault: "#484F58",  // GitHub border
		BorderFocused: "#58A6FF",  // GitHub blue
		BorderMuted:   "#21262D",  // GitHub canvas subtle
		
		// Title colors - GitHub Dark inspired, using accent colors
		TitleDefault: "#58A6FF",  // GitHub blue - prominent
		TitleFocused: "#D29922",  // GitHub orange - bright when focused
		TitleMuted:   "#484F58",  // GitHub border - muted
		
		// Text colors - GitHub Dark foreground
		TextPrimary:   "#E6EDF3",  // GitHub foreground default
		TextSecondary: "#B1BAC4",  // GitHub foreground muted
		TextTertiary:  "#7D8590",  // GitHub foreground subtle
		
		// Accent colors - GitHub palette
		Primary:   "#3FB950",      // GitHub green
		Secondary: "#58A6FF",      // GitHub blue
		Tertiary:  "#7D8590",      // GitHub foreground subtle
		Error:     "#F85149",      // GitHub red
		Warning:   "#D29922",      // GitHub orange
		Success:   "#3FB950",      // GitHub green
		Muted:     "#30363D",      // GitHub canvas default
	},
	"rose-pine": {
		// Border colors - Rosé Pine inspired
		BorderDefault: "#6E6A86",  // Rosé Pine muted
		BorderFocused: "#9CCFD8",  // Rosé Pine foam
		BorderMuted:   "#26233A",  // Rosé Pine surface
		
		// Title colors - Rosé Pine inspired, using accent colors
		TitleDefault: "#9CCFD8",  // Rosé Pine foam - prominent
		TitleFocused: "#F6C177",  // Rosé Pine gold - bright when focused
		TitleMuted:   "#6E6A86",  // Rosé Pine muted - muted
		
		// Text colors - Rosé Pine text
		TextPrimary:   "#E0DEF4",  // Rosé Pine text
		TextSecondary: "#908CAA",  // Rosé Pine subtle
		TextTertiary:  "#6E6A86",  // Rosé Pine muted
		
		// Accent colors - Rosé Pine palette
		Primary:   "#31748F",      // Rosé Pine pine
		Secondary: "#9CCFD8",      // Rosé Pine foam
		Tertiary:  "#6E6A86",      // Rosé Pine muted
		Error:     "#EB6F92",      // Rosé Pine love
		Warning:   "#F6C177",      // Rosé Pine gold
		Success:   "#31748F",      // Rosé Pine pine
		Muted:     "#403D52",      // Rosé Pine overlay
	},
	"one-dark": {
		// Border colors - One Dark inspired
		BorderDefault: "#5C6370",  // One Dark comment
		BorderFocused: "#61AFEF",  // One Dark blue
		BorderMuted:   "#353B45",  // One Dark gutter
		
		// Title colors - One Dark inspired, using accent colors
		TitleDefault: "#61AFEF",  // One Dark blue - prominent
		TitleFocused: "#E5C07B",  // One Dark yellow - bright when focused
		TitleMuted:   "#5C6370",  // One Dark comment - muted
		
		// Text colors - One Dark foreground
		TextPrimary:   "#ABB2BF",  // One Dark foreground
		TextSecondary: "#9CA3AF",  // One Dark foreground dimmed
		TextTertiary:  "#5C6370",  // One Dark comment
		
		// Accent colors - One Dark palette
		Primary:   "#98C379",      // One Dark green
		Secondary: "#61AFEF",      // One Dark blue
		Tertiary:  "#5C6370",      // One Dark comment
		Error:     "#E06C75",      // One Dark red
		Warning:   "#E5C07B",      // One Dark yellow
		Success:   "#98C379",      // One Dark green
		Muted:     "#4B5263",      // One Dark selection
	},
}

// GetThemeColor converts a theme hex color to gocui.Attribute using gocui.GetColor
func GetThemeColor(hexColor string) gocui.Attribute {
	return gocui.GetColor(hexColor)
}

// ConvertColorToAnsi converts hex color to ANSI escape sequence for text coloring
func ConvertColorToAnsi(hexColor string) string {
	// Simple conversion - most modern terminals support hex colors directly
	// in ANSI escape sequences, but for maximum compatibility, we can convert
	// to RGB and use the standard ANSI true color format
	
	if len(hexColor) == 7 && hexColor[0] == '#' {
		// Convert hex to RGB
		r, g, b := hexToRGB(hexColor)
		// Return ANSI true color escape sequence
		return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
	}
	
	// Return empty string for invalid or empty hex
	return ""
}

// ConvertColorToAnsiBg converts hex color to ANSI escape sequence for background coloring
func ConvertColorToAnsiBg(hexColor string) string {
	if len(hexColor) == 7 && hexColor[0] == '#' {
		// Convert hex to RGB
		r, g, b := hexToRGB(hexColor)
		// Return ANSI true color escape sequence for background
		return fmt.Sprintf("\033[48;2;%d;%d;%dm", r, g, b)
	}
	
	// Return empty string for invalid or empty hex
	return ""
}

// hexToRGB converts hex color to RGB values
func hexToRGB(hex string) (int, int, int) {
	// Remove # prefix
	hex = hex[1:]
	
	// Parse hex values
	var r, g, b int
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	
	return r, g, b
}

// ConvertAnsiToGocuiColor converts ANSI color to gocui.Attribute (legacy compatibility)
func ConvertAnsiToGocuiColor(ansiColor string) gocui.Attribute {
	return gocui.GetColor(ansiColor)
}

// GetThemeFocusColors returns focus colors (legacy compatibility)
func GetThemeFocusColors(theme *types.Theme) (gocui.Attribute, gocui.Attribute) {
	// Return default focus colors
	return gocui.ColorDefault, gocui.ColorDefault
}

// GetTheme returns the theme by name
func GetTheme(name string) *Theme {
	if theme, ok := Themes[name]; ok {
		return theme
	}
	return Themes["default"]
}

// GetThemeNames returns all available theme names
func GetThemeNames() []string {
	names := make([]string, 0, len(Themes))
	for name := range Themes {
		names = append(names, name)
	}
	return names
}

// Legacy compatibility functions (these can be removed once all code is updated)

// GetThemeForMode returns a theme (ignores mode now)
func GetThemeForMode(name string, outputMode string) *types.Theme {
	// Just return the hex colors directly - let the caller handle conversion
	newTheme := GetTheme(name)
	return &types.Theme{
		BorderDefault: newTheme.BorderDefault,
		BorderFocused: newTheme.BorderFocused,
		BorderMuted:   newTheme.BorderMuted,
		
		Primary:   newTheme.Primary,
		Secondary: newTheme.Secondary,
		Tertiary:  newTheme.Tertiary,
		Error:     newTheme.Error,
		Warning:   newTheme.Warning,
		Success:   newTheme.Success,
		Muted:     newTheme.Muted,
		
		TextPrimary:   newTheme.TextPrimary,
		TextSecondary: newTheme.TextSecondary,
		TextTertiary:  newTheme.TextTertiary,
	}
}