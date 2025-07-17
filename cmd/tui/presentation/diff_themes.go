package presentation


// DiffTheme defines colors for diff formatting
type DiffTheme struct {
	// Added lines
	AddedFg string
	AddedBg string
	
	// Removed lines
	RemovedFg string
	RemovedBg string
	
	// File headers (--- and +++ lines)
	HeaderFg string
	HeaderBg string
	
	// Hunk headers (@@ lines)
	HunkFg string
	HunkBg string
	
	// Context lines
	ContextFg string
	ContextBg string
}

// DiffThemes defines available diff color schemes
var DiffThemes = map[string]*DiffTheme{
	"default": {
		AddedFg:   "#A3BE8C",  // Nord green
		AddedBg:   "",         // No background
		RemovedFg: "#BF616A",  // Nord red
		RemovedBg: "",         // No background
		HeaderFg:  "#5E81AC",  // Nord blue
		HeaderBg:  "",         // No background
		HunkFg:    "#D08770",  // Nord orange
		HunkBg:    "",         // No background
		ContextFg: "#D8DEE9",  // Nord foreground
		ContextBg: "",         // No background
	},
	"subtle": {
		AddedFg:   "#50A050",  // Muted green
		AddedBg:   "",         // No background
		RemovedFg: "#A05050",  // Muted red
		RemovedBg: "",         // No background
		HeaderFg:  "#6B8CAF",  // Muted blue
		HeaderBg:  "",         // No background
		HunkFg:    "#B8860B",  // Dark gold
		HunkBg:    "",         // No background
		ContextFg: "#CCCCCC",  // Light gray
		ContextBg: "",         // No background
	},
	"vibrant": {
		AddedFg:   "#00FF00",  // Bright green
		AddedBg:   "",         // No background
		RemovedFg: "#FF0000",  // Bright red
		RemovedBg: "",         // No background
		HeaderFg:  "#00FFFF",  // Cyan
		HeaderBg:  "",         // No background
		HunkFg:    "#FFFF00",  // Yellow
		HunkBg:    "",         // No background
		ContextFg: "#FFFFFF",  // White
		ContextBg: "",         // No background
	},
	"github": {
		AddedFg:   "#28A745",  // GitHub green
		AddedBg:   "",         // No background
		RemovedFg: "#D73A49",  // GitHub red
		RemovedBg: "",         // No background
		HeaderFg:  "#586069",  // GitHub gray
		HeaderBg:  "",         // No background
		HunkFg:    "#005CC5",  // GitHub blue
		HunkBg:    "",         // No background
		ContextFg: "#24292E",  // GitHub text
		ContextBg: "",         // No background
	},
	"classic": {
		AddedFg:   "#90EE90",  // Light green
		AddedBg:   "",         // No background
		RemovedFg: "#FFB6C1",  // Light pink
		RemovedBg: "",         // No background
		HeaderFg:  "#87CEEB",  // Sky blue
		HeaderBg:  "",         // No background
		HunkFg:    "#F0E68C",  // Khaki
		HunkBg:    "",         // No background
		ContextFg: "#F5F5F5",  // White smoke
		ContextBg: "",         // No background
	},
	"highlighted": {
		AddedFg:   "#E8E8E8",  // Light gray text
		AddedBg:   "#2F5A1A",  // Slightly more vibrant dark green background
		RemovedFg: "#E8E8E8",  // Light gray text
		RemovedBg: "#6B1F1F",  // Slightly more vibrant dark red background
		HeaderFg:  "#E8E8E8",  // Light gray text
		HeaderBg:  "#234068",  // Slightly more vibrant dark blue background
		HunkFg:    "#E8E8E8",  // Light gray text
		HunkBg:    "#6B3A1C",  // Slightly more vibrant dark brown/orange background
		ContextFg: "#ECEFF4",  // Light gray text
		ContextBg: "",         // No background for context
	},
}

// GetDiffTheme returns the diff theme by name
func GetDiffTheme(name string) *DiffTheme {
	if theme, ok := DiffThemes[name]; ok {
		return theme
	}
	return DiffThemes["default"]
}

// GetDiffThemeForMainTheme maps main theme names to appropriate diff themes
func GetDiffThemeForMainTheme(mainThemeName string) string {
	switch mainThemeName {
	case "default":
		return "default"
	case "minimal":
		return "subtle"
	case "dracula":
		return "highlighted"
	case "monokai":
		return "highlighted"
	case "solarized":
		return "classic"
	case "nord":
		return "highlighted"
	case "catppuccin":
		return "subtle"
	case "tokyo-night":
		return "highlighted"
	case "gruvbox":
		return "classic"
	case "github-dark":
		return "github"
	case "rose-pine":
		return "subtle"
	case "one-dark":
		return "highlighted"
	default:
		return "default"
	}
}

// GetDiffThemeNames returns all available diff theme names
func GetDiffThemeNames() []string {
	names := make([]string, 0, len(DiffThemes))
	for name := range DiffThemes {
		names = append(names, name)
	}
	return names
}