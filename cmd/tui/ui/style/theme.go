// Package style turns the hex colour themes in presentation into Lip Gloss
// styles and draws the bordered frames every panel sits in.
package style

import (
	"charm.land/lipgloss/v2"
	"github.com/kcaldas/genie/cmd/tui/presentation"
)

// Theme is a resolved set of Lip Gloss styles. Gen changes whenever the
// theme is swapped so caches keyed on it invalidate lazily.
type Theme struct {
	Name string
	Gen  uint64

	Border        lipgloss.Style
	BorderFocused lipgloss.Style
	Title         lipgloss.Style
	TitleFocused  lipgloss.Style

	Primary   lipgloss.Style
	Secondary lipgloss.Style
	Tertiary  lipgloss.Style
	Error     lipgloss.Style
	Warning   lipgloss.Style
	Success   lipgloss.Style
	Muted     lipgloss.Style

	TextPrimary   lipgloss.Style
	TextSecondary lipgloss.Style
	TextTertiary  lipgloss.Style

	// Ghost styles inline completion text after the cursor.
	Ghost lipgloss.Style
	// Chip styles a collapsed paste token in the prompt.
	Chip lipgloss.Style
}

// New resolves the named theme (falling back to the default theme for
// unknown names, as presentation.GetTheme does) at the given generation.
func New(name string, gen uint64) *Theme {
	src := presentation.GetTheme(name)
	fg := func(hex string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(hex))
	}
	return &Theme{
		Name:          name,
		Gen:           gen,
		Border:        fg(src.BorderDefault),
		BorderFocused: fg(src.BorderFocused),
		Title:         fg(src.TitleDefault),
		TitleFocused:  fg(src.TitleFocused),
		Primary:       fg(src.Primary),
		Secondary:     fg(src.Secondary),
		Tertiary:      fg(src.Tertiary),
		Error:         fg(src.Error),
		Warning:       fg(src.Warning),
		Success:       fg(src.Success),
		Muted:         fg(src.Muted),
		TextPrimary:   fg(src.TextPrimary),
		TextSecondary: fg(src.TextSecondary),
		TextTertiary:  fg(src.TextTertiary),
		Ghost:         lipgloss.NewStyle().Faint(true),
		Chip:          lipgloss.NewStyle().Foreground(lipgloss.Color(src.Secondary)).Reverse(true),
	}
}
