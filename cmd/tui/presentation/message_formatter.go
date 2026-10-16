package presentation

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"charm.land/glamour/v2"
	"github.com/kcaldas/genie/cmd/tui/types"
)

// leadingANSI matches a single SGR sequence at the very start of a string.
// Glamour emits invisible resets before the first visible glyph; stripping
// them stops the rendered block from appearing indented.
var leadingANSI = regexp.MustCompile(`^\x1b\[[0-9;]*m`)

// defaultRenderWidth is the wrap width used to validate the glamour style up
// front, so a bad style name fails at construction rather than on first render.
const defaultRenderWidth = 80

// MessageFormatter turns chat messages into ANSI-styled text. Glamour
// renderers are expensive to build, so one is kept per wrap width; a formatter
// is tied to a single theme and config, and callers build a new one when those
// change.
type MessageFormatter struct {
	config *types.Config
	theme  *types.Theme

	mu        sync.Mutex
	renderers map[int]*glamour.TermRenderer
}

func NewMessageFormatter(config *types.Config, theme *types.Theme) (*MessageFormatter, error) {
	f := &MessageFormatter{
		config:    config,
		theme:     theme,
		renderers: make(map[int]*glamour.TermRenderer),
	}
	if _, err := f.rendererFor(defaultRenderWidth); err != nil {
		return nil, err
	}
	return f, nil
}

// rendererFor returns the cached glamour renderer for the given wrap width,
// creating it on first use. Callers must hold f.mu.
func (f *MessageFormatter) rendererFor(width int) (*glamour.TermRenderer, error) {
	if r, ok := f.renderers[width]; ok {
		return r, nil
	}
	r, err := createMarkdownRendererWithWidth(f.theme, f.config.Theme, f.config.GlamourTheme, width)
	if err != nil {
		return nil, err
	}
	f.renderers[width] = r
	return r, nil
}

// renderMarkdown renders content through the width-specific glamour renderer.
// Glamour renderers are not safe for concurrent use, so the whole render is
// serialised under f.mu.
func (f *MessageFormatter) renderMarkdown(content string, width int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, err := f.rendererFor(width)
	if err != nil {
		return "", err
	}
	return r.Render(content)
}

// FormatMessageWithWidth renders a whole message: header, body and the
// blank separator line.
func (f *MessageFormatter) FormatMessageWithWidth(msg types.Message, width int) string {
	return f.FormatHeader(msg) + f.FormatBody(msg, width) + "\n\n"
}

// FormatHeader renders the role label (and timestamp when enabled) that
// precedes a message body.
func (f *MessageFormatter) FormatHeader(msg types.Message) string {
	header := fmt.Sprintf("%s%s\033[0m ", f.getRoleColor(msg.Role), f.getRolePrefix(msg.Role))
	if f.config.ShowTimestamps {
		header = fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), header)
	}
	return header
}

// FormatBody renders just the content of a message at the given width:
// markdown through glamour when enabled, otherwise coloured and wrapped
// plain text.
func (f *MessageFormatter) FormatBody(msg types.Message, width int) string {
	content := msg.Content

	// Apply text colors BEFORE markdown processing (so they don't get stripped)
	// Only for user and system messages - assistant messages use markdown styling
	if msg.Role == "error" {
		// Apply red color to error content for better visibility
		errorColor := ConvertColorToAnsi(f.theme.Error)
		content = fmt.Sprintf("%s%s%s", errorColor, content, "\033[0m")
	} else if msg.ContentType == "thought" {
		// Apply muted styling for thought messages (internal AI thinking)
		mutedColor := ConvertColorToAnsi(f.theme.Muted)
		content = fmt.Sprintf("%s%s%s", mutedColor, content, "\033[0m")
	} else if (msg.Role == "user" || msg.Role == "system") && msg.ContentType != "markdown" {
		// Apply role-specific text color for user and system messages (but not markdown)
		textColor := f.getRoleTextColor(msg.Role)
		content = fmt.Sprintf("%s%s%s", textColor, content, "\033[0m")
	}

	// Process markdown AFTER applying text colors (based on content type)
	if f.IsMarkdown(msg) {
		if rendered, err := f.renderMarkdown(content, width-2); err == nil {
			content = strings.TrimSpace(rendered)
			for leadingANSI.MatchString(content) {
				content = leadingANSI.ReplaceAllString(content, "")
			}
			content = strings.TrimSpace(content)
		}
	}

	// Only apply additional wrapping if markdown rendering is disabled
	// (markdown renderer already handles wrapping)
	if f.config.IsWrapMessagesEnabled() && !f.config.IsMarkdownRenderingEnabled() && width > 10 {
		content = f.wrapText(content, width-2) // Leave some margin
	}

	return content
}

// IsMarkdown reports whether the message body goes through glamour.
func (f *MessageFormatter) IsMarkdown(msg types.Message) bool {
	return f.config.IsMarkdownRenderingEnabled() && msg.ContentType == "markdown"
}

// getRoleColor returns accent colors for UI elements (indicators, prefixes)
func (f *MessageFormatter) getRoleColor(role string) string {
	var color string
	switch role {
	case "user":
		color = f.theme.Tertiary // User accents use TERTIARY (least prominent)
	case "assistant":
		color = f.theme.Primary // AI assistant accents use PRIMARY (most prominent)
	case "system":
		color = f.theme.Secondary // System accents use SECONDARY (moderate prominence)
	case "error":
		color = f.theme.Error
	default:
		color = f.theme.Muted
	}

	// Convert color to ANSI escape sequence (handles hex colors in true color mode)
	return ConvertColorToAnsi(color)
}

// getRoleTextColor returns text colors for message content
func (f *MessageFormatter) getRoleTextColor(role string) string {
	var color string
	switch role {
	case "user":
		color = f.theme.TextTertiary // User text uses TextTertiary (least prominent)
	case "assistant":
		color = f.theme.TextPrimary // AI assistant text uses TextPrimary (most prominent)
	case "system":
		color = f.theme.TextSecondary // System text uses TextSecondary (moderate prominence)
	case "error":
		color = f.theme.Error
	default:
		color = f.theme.Muted
	}

	// Convert color to ANSI escape sequence (handles hex colors in true color mode)
	return ConvertColorToAnsi(color)
}

func (f *MessageFormatter) getRolePrefix(role string) string {
	switch role {
	case "user":
		return f.config.UserLabel
	case "assistant":
		return f.config.AssistantLabel
	case "system":
		return f.config.SystemLabel
	case "error":
		return f.config.ErrorLabel
	default:
		return f.config.UserLabel
	}
}

func (f *MessageFormatter) wrapText(text string, width int) string {
	var wrapped strings.Builder
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if len(line) <= width {
			wrapped.WriteString(line)
			wrapped.WriteString("\n")
			continue
		}

		words := strings.Fields(line)
		currentLine := ""

		for _, word := range words {
			if len(currentLine)+len(word)+1 > width {
				wrapped.WriteString(currentLine)
				wrapped.WriteString("\n")
				currentLine = word
			} else {
				if currentLine != "" {
					currentLine += " "
				}
				currentLine += word
			}
		}

		if currentLine != "" {
			wrapped.WriteString(currentLine)
			wrapped.WriteString("\n")
		}
	}

	return strings.TrimRight(wrapped.String(), "\n")
}

// GetGlamourStyleForTheme maps our theme names to appropriate glamour styles
func GetGlamourStyleForTheme(themeName string) string {
	switch themeName {
	// Original themes
	case "dracula":
		return "dracula" // Perfect match - official Dracula theme
	case "monokai":
		return "tokyo-night" // Best match for monokai's bright colors
	case "solarized":
		return "dark" // Good match for solarized's blue tones
	case "nord":
		return "dark" // Complements nord's blue palette
	case "minimal":
		return "notty" // Minimal styling for minimal theme

	// Modern themes
	case "catppuccin":
		return "pink" // Closest match to catppuccin's soft pastels
	case "tokyo-night":
		return "tokyo-night" // Perfect match - official Tokyo Night theme
	case "gruvbox":
		return "dark" // Good match for gruvbox's warm tones
	case "github-dark":
		return "dark" // GitHub's dark theme works well with glamour dark
	case "rose-pine":
		return "pink" // Pink glamour theme matches rose-pine's soft colors
	case "one-dark":
		return "dark" // One Dark works well with glamour dark

	default: // "default"
		return "dark" // Bright text for dark terminals
	}
}

// GetAllAvailableGlamourStyles returns all built-in glamour themes
func GetAllAvailableGlamourStyles() []string {
	return []string{
		"ascii",
		"auto",
		"dark",
		"dracula",
		"light",
		"notty",
		"pink",
		"tokyo-night",
	}
}

// CreateMarkdownRendererWithWidth creates a markdown renderer with specified width - exported for use by components
func CreateMarkdownRendererWithWidth(theme *types.Theme, themeName string, glamourTheme string, width int) (*glamour.TermRenderer, error) {
	return createMarkdownRendererWithWidth(theme, themeName, glamourTheme, width)
}

func createMarkdownRendererWithWidth(theme *types.Theme, themeName string, glamourTheme string, width int) (*glamour.TermRenderer, error) {
	// Ensure minimum width
	if width < 20 {
		width = 20
	}
	glamourStyle := getGlamourStyle(themeName, glamourTheme)
	return glamour.NewTermRenderer(
		glamour.WithStandardStyle(glamourStyle),
		glamour.WithWordWrap(width),
	)
}

// getGlamourStyle determines the glamour style to use based on config
func getGlamourStyle(themeName string, glamourTheme string) string {
	// If a specific glamour theme is set (not "auto"), use it directly
	if glamourTheme != "" && glamourTheme != "auto" {
		return glamourTheme
	}

	// Otherwise, fall back to automatic theme mapping
	return GetGlamourStyleForTheme(themeName)
}
