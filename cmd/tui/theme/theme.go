// Package theme provides a configurable theme system for Genie TUI components.
// Themes define colors, styles, and styling patterns used across all UI components.
package theme

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme defines all colors and styling properties for the TUI
type Theme struct {
	// Theme metadata
	Name        string `json:"name"`
	Description string `json:"description"`
	Author      string `json:"author"`

	// Core color palette
	Colors ColorPalette `json:"colors"`

	// Component-specific styling
	Borders BorderTheme `json:"borders"`
	Spacing SpacingTheme `json:"spacing"`
}

// ColorPalette defines the core colors used throughout the TUI
type ColorPalette struct {
	// Primary brand colors
	Primary   string `json:"primary"`   // Main brand color (purple)
	Secondary string `json:"secondary"` // Secondary accent (amber)

	// Semantic colors
	Success string `json:"success"` // Success states, AI responses
	Warning string `json:"warning"` // Warnings, confirmations
	Error   string `json:"error"`   // Errors, failures
	Info    string `json:"info"`    // Information, file paths

	// Text hierarchy
	TextPrimary   string `json:"text_primary"`   // Main text (white)
	TextSecondary string `json:"text_secondary"` // Secondary text (gray)
	TextMuted     string `json:"text_muted"`     // Muted text (light gray)
	TextDisabled  string `json:"text_disabled"`  // Disabled text (dark gray)

	// UI Chrome
	Background   string `json:"background"`    // Background color
	Surface      string `json:"surface"`       // Surface/container color
	Border       string `json:"border"`        // Default border color
	BorderFocus  string `json:"border_focus"`  // Focused border color
	BorderMuted  string `json:"border_muted"`  // Muted border color

	// Diff/code colors
	DiffAdded   string `json:"diff_added"`   // Added lines in diffs
	DiffRemoved string `json:"diff_removed"` // Removed lines in diffs
	DiffContext string `json:"diff_context"` // Context lines in diffs
	DiffHeader  string `json:"diff_header"`  // Diff headers

	// Tool indicators
	ToolSuccess string `json:"tool_success"` // Tool success indicators
}

// BorderTheme defines border styling
type BorderTheme struct {
	Radius string `json:"radius"` // "rounded" or "normal"
	Width  int    `json:"width"`  // Border width (usually 1)
}

// SpacingTheme defines spacing values
type SpacingTheme struct {
	Small  int `json:"small"`  // Small padding/margin
	Medium int `json:"medium"` // Medium padding/margin
	Large  int `json:"large"`  // Large padding/margin
}

// Styles holds all the computed lipgloss styles for a theme
type Styles struct {
	// Input components
	Input       lipgloss.Style
	InputFocus  lipgloss.Style

	// Messages
	UserMessage     lipgloss.Style
	AIMessage       lipgloss.Style
	SystemMessage   lipgloss.Style
	ErrorMessage    lipgloss.Style
	ToolCallMessage lipgloss.Style

	// Dialogs
	Dialog      lipgloss.Style
	DialogTitle lipgloss.Style

	// Confirmations
	ConfirmationDialog    lipgloss.Style
	ConfirmationTitle     lipgloss.Style
	ConfirmationMessage   lipgloss.Style
	ConfirmationOption    lipgloss.Style
	ConfirmationSelected  lipgloss.Style
	ConfirmationHelp      lipgloss.Style

	// Context viewer
	ContextDialog       lipgloss.Style
	ContextPanel        lipgloss.Style
	ContextPanelContent lipgloss.Style
	ContextInstructions lipgloss.Style

	// Scrollable confirmation (diffs/plans)
	ScrollDialog     lipgloss.Style
	ScrollTitle      lipgloss.Style
	ScrollFilePath   lipgloss.Style
	ScrollContainer  lipgloss.Style
	ScrollOption     lipgloss.Style
	ScrollSelected   lipgloss.Style
	ScrollHelp       lipgloss.Style

	// Diff syntax highlighting
	DiffAdded     lipgloss.Style
	DiffRemoved   lipgloss.Style
	DiffContext   lipgloss.Style
	DiffHeader    lipgloss.Style
	DiffFilePath  lipgloss.Style
	DiffContainer lipgloss.Style

	// Tool results
	ToolIndicatorSuccess lipgloss.Style
	ToolIndicatorError   lipgloss.Style
	ToolIndicatorInfo    lipgloss.Style
	ToolName             lipgloss.Style
	ToolSummary          lipgloss.Style
	ToolKey              lipgloss.Style
	ToolValue            lipgloss.Style
	ToolError            lipgloss.Style
	ToolTruncated        lipgloss.Style
}

// DefaultTheme returns the default Genie theme
func DefaultTheme() Theme {
	return Theme{
		Name:        "Default",
		Description: "Default Genie theme with purple and amber accents",
		Author:      "Genie Team",
		Colors: ColorPalette{
			// Primary colors
			Primary:   "#7C3AED", // Purple
			Secondary: "#F59E0B", // Amber

			// Semantic colors
			Success: "#10B981", // Green
			Warning: "#F59E0B", // Amber (same as secondary)
			Error:   "#EF4444", // Red
			Info:    "#3B82F6", // Blue

			// Text hierarchy
			TextPrimary:   "#FFFFFF", // White
			TextSecondary: "#6B7280", // Gray
			TextMuted:     "#9CA3AF", // Light Gray
			TextDisabled:  "#374151", // Dark Gray

			// UI Chrome
			Background:  "#000000", // Black (terminal default)
			Surface:     "#1F2937", // Dark Gray
			Border:      "#374151", // Dark Gray
			BorderFocus: "#7C3AED", // Purple
			BorderMuted: "#6B7280", // Gray

			// Diff colors
			DiffAdded:   "#22C55E", // Light Green
			DiffRemoved: "#EF4444", // Red
			DiffContext: "#6B7280", // Gray
			DiffHeader:  "#3B82F6", // Blue

			// Tool indicators
			ToolSuccess: "#10B981", // Green
		},
		Borders: BorderTheme{
			Radius: "rounded",
			Width:  1,
		},
		Spacing: SpacingTheme{
			Small:  1,
			Medium: 2,
			Large:  4,
		},
	}
}

// DarkTheme returns a darker variant theme
func DarkTheme() Theme {
	return Theme{
		Name:        "Dark",
		Description: "Dark theme with blue accents and softer colors",
		Author:      "Genie Team",
		Colors: ColorPalette{
			// Primary colors - blue instead of purple
			Primary:   "#3B82F6", // Blue
			Secondary: "#06B6D4", // Cyan

			// Semantic colors - softer
			Success: "#059669", // Darker green
			Warning: "#D97706", // Darker amber
			Error:   "#DC2626", // Darker red
			Info:    "#0EA5E9", // Sky blue

			// Text hierarchy - softer contrast
			TextPrimary:   "#E5E7EB", // Off-white
			TextSecondary: "#9CA3AF", // Light gray
			TextMuted:     "#6B7280", // Medium gray
			TextDisabled:  "#4B5563", // Dark gray

			// UI Chrome - true dark
			Background:  "#000000", // Black
			Surface:     "#111827", // Very dark gray
			Border:      "#1F2937", // Dark gray
			BorderFocus: "#3B82F6", // Blue
			BorderMuted: "#374151", // Muted gray

			// Diff colors - softer
			DiffAdded:   "#059669", // Darker green
			DiffRemoved: "#DC2626", // Darker red
			DiffContext: "#4B5563", // Medium gray
			DiffHeader:  "#3B82F6", // Blue

			// Tool indicators
			ToolSuccess: "#059669", // Darker green
		},
		Borders: BorderTheme{
			Radius: "rounded",
			Width:  1,
		},
		Spacing: SpacingTheme{
			Small:  1,
			Medium: 2,
			Large:  4,
		},
	}
}

// MinimalTheme returns a very discrete minimalist theme
func MinimalTheme() Theme {
	return Theme{
		Name:        "Minimal",
		Description: "Ultra-minimalist theme with subtle grays and no color distractions",
		Author:      "Genie Team",
		Colors: ColorPalette{
			// Primary colors - very subtle
			Primary:   "#506050", // Very subtle green tint
			Secondary: "#605040", // Very subtle amber tint

			// Semantic colors - very subtle indicators
			Success: "#C0C0C0", // Pure light gray for AI responses
			Warning: "#A09070", // Very subtle amber tint for warnings
			Error:   "#A08080", // Very subtle red tint for errors
			Info:    "#7090A0", // Very subtle blue tint for info

			// Text hierarchy - minimal contrast
			TextPrimary:   "#707070", // Darker gray for user messages
			TextSecondary: "#808080", // Medium gray
			TextMuted:     "#606060", // Dark gray
			TextDisabled:  "#3A3530", // Very dark brownish gray, subtle but distinct

			// UI Chrome - barely visible, pure grays
			Background:  "#000000", // Black (terminal default)
			Surface:     "#0A0A0A", // Almost black
			Border:      "#252525", // Very dark border
			BorderFocus: "#353535", // Slightly lighter when focused
			BorderMuted: "#1A1A1A", // Extremely subtle

			// Diff colors - clearly visible for code review
			DiffAdded:   "#70A070", // Clear green for additions
			DiffRemoved: "#A07070", // Clear red for removals  
			DiffContext: "#808080", // Medium gray for context
			DiffHeader:  "#7090B0", // Clear blue for headers

			// Tool indicators - more noticeable green for minimal theme
			ToolSuccess: "#60A060", // Noticeable but still subtle green
		},
		Borders: BorderTheme{
			Radius: "normal", // No rounded corners for clean look
			Width:  1,
		},
		Spacing: SpacingTheme{
			Small:  1,
			Medium: 2,
			Large:  3, // Reduced spacing for minimalism
		},
	}
}

// NeonTheme returns a vibrant neon theme
func NeonTheme() Theme {
	return Theme{
		Name:        "Neon",
		Description: "Vibrant neon colors on dark background",
		Author:      "Genie Team",
		Colors: ColorPalette{
			// Primary colors - hot pink and electric blue
			Primary:   "#FF006E", // Hot pink
			Secondary: "#00F5FF", // Electric cyan

			// Semantic colors - neon
			Success: "#00FF41", // Neon green
			Warning: "#FFAA00", // Neon orange
			Error:   "#FF0000", // Bright red
			Info:    "#00BFFF", // Neon blue

			// Text hierarchy - bright
			TextPrimary:   "#FFFFFF", // White
			TextSecondary: "#00F5FF", // Cyan
			TextMuted:     "#FF006E", // Pink
			TextDisabled:  "#666666", // Gray

			// UI Chrome - dark with neon accents
			Background:  "#000000", // Black
			Surface:     "#1A1A1A", // Very dark gray
			Border:      "#FF006E", // Pink border
			BorderFocus: "#00F5FF", // Cyan focus
			BorderMuted: "#333333", // Dark gray

			// Diff colors - neon
			DiffAdded:   "#00FF41", // Neon green
			DiffRemoved: "#FF0000", // Bright red
			DiffContext: "#666666", // Gray
			DiffHeader:  "#00BFFF", // Neon blue

			// Tool indicators
			ToolSuccess: "#00FF41", // Neon green
		},
		Borders: BorderTheme{
			Radius: "normal", // Sharp edges for cyberpunk feel
			Width:  1,
		},
		Spacing: SpacingTheme{
			Small:  1,
			Medium: 2,
			Large:  4,
		},
	}
}

// LightTheme returns a light variant theme
func LightTheme() Theme {
	return Theme{
		Name:        "Light",
		Description: "Light theme with green accents for bright environments",
		Author:      "Genie Team",
		Colors: ColorPalette{
			// Primary colors - green focused
			Primary:   "#059669", // Emerald
			Secondary: "#0891B2", // Cyan

			// Semantic colors - vibrant
			Success: "#10B981", // Green
			Warning: "#F59E0B", // Amber
			Error:   "#EF4444", // Red
			Info:    "#3B82F6", // Blue

			// Text hierarchy - dark on light
			TextPrimary:   "#111827", // Near black
			TextSecondary: "#4B5563", // Dark gray
			TextMuted:     "#6B7280", // Medium gray
			TextDisabled:  "#9CA3AF", // Light gray

			// UI Chrome - light backgrounds
			Background:  "#FFFFFF", // White
			Surface:     "#F9FAFB", // Off-white
			Border:      "#D1D5DB", // Light gray
			BorderFocus: "#059669", // Emerald
			BorderMuted: "#E5E7EB", // Very light gray

			// Diff colors - on light background
			DiffAdded:   "#059669", // Emerald
			DiffRemoved: "#DC2626", // Red
			DiffContext: "#6B7280", // Gray
			DiffHeader:  "#0891B2", // Cyan

			// Tool indicators
			ToolSuccess: "#059669", // Emerald
		},
		Borders: BorderTheme{
			Radius: "normal", // Square borders for light theme
			Width:  1,
		},
		Spacing: SpacingTheme{
			Small:  1,
			Medium: 2,
			Large:  4,
		},
	}
}

// ComputeStyles generates all lipgloss styles from the theme
func (t Theme) ComputeStyles() Styles {
	// Helper function to get border style
	getBorder := func() lipgloss.Border {
		if t.Borders.Radius == "rounded" {
			return lipgloss.RoundedBorder()
		}
		return lipgloss.NormalBorder()
	}

	return Styles{
		// Input components
		Input: lipgloss.NewStyle().
			Border(getBorder()).
			BorderForeground(lipgloss.Color(t.Colors.Border)).
			Padding(0, t.Spacing.Small),
		InputFocus: lipgloss.NewStyle().
			Border(getBorder()).
			BorderForeground(lipgloss.Color(t.Colors.BorderFocus)).
			Padding(0, t.Spacing.Small),

		// Messages
		UserMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.TextPrimary)).
			Bold(true).
			PaddingLeft(1),
		AIMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Success)).
			PaddingLeft(1),
		SystemMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.TextSecondary)).
			Italic(true).
			PaddingLeft(1),
		ErrorMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Error)).
			PaddingLeft(1),
		ToolCallMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.TextDisabled)),

		// Dialogs
		Dialog: lipgloss.NewStyle().
			Border(getBorder()).
			BorderForeground(lipgloss.Color(t.Colors.Primary)).
			Background(lipgloss.Color(t.Colors.Surface)).
			Padding(t.Spacing.Small),
		DialogTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Primary)).
			Bold(true),

		// Confirmations
		ConfirmationDialog: lipgloss.NewStyle().
			Padding(t.Spacing.Small, t.Spacing.Medium),
		ConfirmationTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Warning)).
			Bold(true),
		ConfirmationMessage: lipgloss.NewStyle().
			PaddingLeft(t.Spacing.Large),
		ConfirmationOption: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.TextSecondary)),
		ConfirmationSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Success)).
			Bold(true),
		ConfirmationHelp: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.TextMuted)),

		// Context viewer
		ContextDialog: lipgloss.NewStyle().
			Border(getBorder()).
			BorderForeground(lipgloss.Color(t.Colors.Primary)).
			Padding(t.Spacing.Small),
		ContextPanel: lipgloss.NewStyle().
			Padding(t.Spacing.Small),
		ContextPanelContent: lipgloss.NewStyle().
			Border(getBorder()).
			BorderForeground(lipgloss.Color(t.Colors.BorderMuted)).
			Foreground(lipgloss.Color(t.Colors.TextSecondary)).
			Padding(t.Spacing.Small),
		ContextInstructions: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.TextSecondary)).
			Italic(true),

		// Scrollable confirmation
		ScrollDialog: lipgloss.NewStyle().
			Border(getBorder()).
			BorderForeground(lipgloss.Color(t.Colors.Warning)).
			Background(lipgloss.Color(t.Colors.Surface)).
			Padding(t.Spacing.Small, t.Spacing.Medium),
		ScrollTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Warning)).
			Bold(true),
		ScrollFilePath: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Info)).
			Italic(true),
		ScrollContainer: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(t.Colors.Border)).
			Padding(0, t.Spacing.Small).
			MarginTop(t.Spacing.Small).
			MarginBottom(t.Spacing.Small),
		ScrollOption: lipgloss.NewStyle(),
		ScrollSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Warning)).
			Bold(true),
		ScrollHelp: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.TextMuted)),

		// Diff syntax highlighting
		DiffAdded: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.DiffAdded)),
		DiffRemoved: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.DiffRemoved)),
		DiffContext: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.DiffContext)),
		DiffHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.DiffHeader)),
		DiffFilePath: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Info)),
		DiffContainer: lipgloss.NewStyle().
			Padding(t.Spacing.Small),

		// Tool results
		ToolIndicatorSuccess: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Colors.ToolSuccess)),
		ToolIndicatorError: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Colors.Error)),
		ToolIndicatorInfo: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Colors.TextDisabled)),
		ToolName: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Colors.Info)),
		ToolSummary: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.TextSecondary)),
		ToolKey: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Colors.TextMuted)),
		ToolValue: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.TextSecondary)),
		ToolError: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Error)),
		ToolTruncated: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.TextMuted)),
	}
}

// Manager handles theme loading, saving, and management
type Manager struct {
	configDir    string
	currentTheme Theme
	styles       Styles
}

// NewManager creates a new theme manager
func NewManager(configDir string) (*Manager, error) {
	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	manager := &Manager{
		configDir: configDir,
	}

	// Try to load theme from file, fall back to default
	if err := manager.LoadTheme(""); err != nil {
		// If loading fails, use default theme
		manager.SetTheme(DefaultTheme())
	}

	return manager, nil
}

// LoadTheme loads a theme from file. If name is empty, loads the current theme.
func (m *Manager) LoadTheme(name string) error {
	// Check for built-in themes first
	switch name {
	case "default":
		m.SetTheme(DefaultTheme())
		return nil
	case "dark":
		m.SetTheme(DarkTheme())
		return nil
	case "light":
		m.SetTheme(LightTheme())
		return nil
	case "minimal":
		m.SetTheme(MinimalTheme())
		return nil
	case "neon":
		m.SetTheme(NeonTheme())
		return nil
	}
	
	var filePath string
	
	if name == "" {
		// Load current theme (theme.json)
		filePath = filepath.Join(m.configDir, "theme.json")
	} else {
		// Load specific theme
		filePath = filepath.Join(m.configDir, "themes", name+".json")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read theme file %s: %w", filePath, err)
	}

	var theme Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		return fmt.Errorf("failed to parse theme file %s: %w", filePath, err)
	}

	m.SetTheme(theme)
	return nil
}

// SaveTheme saves a theme to file
func (m *Manager) SaveTheme(theme Theme, name string) error {
	themesDir := filepath.Join(m.configDir, "themes")
	if err := os.MkdirAll(themesDir, 0755); err != nil {
		return fmt.Errorf("failed to create themes directory: %w", err)
	}

	filePath := filepath.Join(themesDir, name+".json")
	data, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal theme: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write theme file: %w", err)
	}

	return nil
}

// SaveCurrentTheme saves the current theme as the active theme
func (m *Manager) SaveCurrentTheme() error {
	filePath := filepath.Join(m.configDir, "theme.json")
	data, err := json.MarshalIndent(m.currentTheme, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal current theme: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write current theme: %w", err)
	}

	return nil
}

// SetTheme sets the current theme and recomputes styles
func (m *Manager) SetTheme(theme Theme) {
	m.currentTheme = theme
	m.styles = theme.ComputeStyles()
}

// GetTheme returns the current theme
func (m *Manager) GetTheme() Theme {
	return m.currentTheme
}

// GetStyles returns the computed styles for the current theme
func (m *Manager) GetStyles() Styles {
	return m.styles
}

// ListThemes returns a list of available theme names
func (m *Manager) ListThemes() ([]string, error) {
	themesDir := filepath.Join(m.configDir, "themes")
	
	entries, err := os.ReadDir(themesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read themes directory: %w", err)
	}

	var themes []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			name := strings.TrimSuffix(entry.Name(), ".json")
			themes = append(themes, name)
		}
	}

	return themes, nil
}

// CreateBuiltinThemes creates the built-in themes in the themes directory
func (m *Manager) CreateBuiltinThemes() error {
	builtinThemes := map[string]Theme{
		"default": DefaultTheme(),
		"dark":    DarkTheme(),
		"light":   LightTheme(),
	}

	for name, theme := range builtinThemes {
		if err := m.SaveTheme(theme, name); err != nil {
			return fmt.Errorf("failed to save builtin theme %s: %w", name, err)
		}
	}

	return nil
}