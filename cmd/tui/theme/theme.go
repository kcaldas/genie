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
	UserMessage   lipgloss.Style
	AIMessage     lipgloss.Style
	SystemMessage lipgloss.Style
	ErrorMessage  lipgloss.Style

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
	theme := DefaultTheme()
	theme.Name = "Dark"
	theme.Description = "Dark theme with reduced contrast"
	
	// Adjust colors for dark theme
	theme.Colors.Primary = "#8B5CF6"   // Lighter purple
	theme.Colors.Success = "#059669"   // Darker green
	theme.Colors.Error = "#DC2626"     // Darker red
	theme.Colors.TextPrimary = "#F3F4F6" // Slightly dimmer white
	theme.Colors.TextSecondary = "#9CA3AF" // Keep gray
	
	return theme
}

// LightTheme returns a light variant theme
func LightTheme() Theme {
	theme := DefaultTheme()
	theme.Name = "Light"
	theme.Description = "Light theme for bright environments"
	
	// Adjust colors for light theme
	theme.Colors.Background = "#FFFFFF"  // White
	theme.Colors.Surface = "#F9FAFB"     // Light gray
	theme.Colors.TextPrimary = "#111827" // Dark text
	theme.Colors.TextSecondary = "#374151" // Darker gray
	theme.Colors.Border = "#D1D5DB"      // Light border
	theme.Colors.BorderMuted = "#E5E7EB" // Very light border
	
	return theme
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
			Bold(true),
		AIMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Success)),
		SystemMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.TextSecondary)).
			Italic(true),
		ErrorMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Error)),

		// Dialogs
		Dialog: lipgloss.NewStyle().
			Border(getBorder()).
			BorderForeground(lipgloss.Color(t.Colors.Primary)).
			Padding(t.Spacing.Small),
		DialogTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Primary)).
			Bold(true),

		// Confirmations
		ConfirmationDialog: lipgloss.NewStyle().
			Border(getBorder()).
			BorderForeground(lipgloss.Color(t.Colors.Warning)).
			Padding(t.Spacing.Small, t.Spacing.Medium),
		ConfirmationTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Warning)).
			Bold(true),
		ConfirmationMessage: lipgloss.NewStyle().
			PaddingLeft(t.Spacing.Large),
		ConfirmationOption: lipgloss.NewStyle(),
		ConfirmationSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Warning)).
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
			Border(getBorder()).
			BorderForeground(lipgloss.Color(t.Colors.BorderMuted)).
			Padding(t.Spacing.Small),

		// Tool results
		ToolIndicatorSuccess: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Colors.Success)),
		ToolIndicatorError: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Colors.Error)),
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