package presentation

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

type MessageFormatter struct {
	config           *types.Config
	theme            *types.Theme
	markdownRenderer *glamour.TermRenderer
}

func NewMessageFormatter(config *types.Config, theme *types.Theme) (*MessageFormatter, error) {
	renderer, err := createMarkdownRenderer(theme, config.Theme)
	if err != nil {
		return nil, err
	}

	return &MessageFormatter{
		config:           config,
		theme:            theme,
		markdownRenderer: renderer,
	}, nil
}

func (f *MessageFormatter) FormatMessage(msg types.Message) string {
	return f.FormatMessageWithWidth(msg, 80) // Default width for backward compatibility
}

func (f *MessageFormatter) FormatMessageWithWidth(msg types.Message, width int) string {
	var output strings.Builder

	roleColor := f.getRoleColor(msg.Role)
	rolePrefix := f.getRolePrefix(msg.Role)

	header := fmt.Sprintf("%s%s\033[0m ", roleColor, rolePrefix)

	if f.config.ShowTimestamps {
		timestamp := time.Now().Format("15:04:05")
		header = fmt.Sprintf("[%s] %s", timestamp, header)
	}

	output.WriteString(header)

	content := msg.Content
	if f.config.MarkdownRendering {
		// Create renderer with dynamic width instead of using cached one
		renderer, err := createMarkdownRendererWithWidth(f.theme, f.config.Theme, width-2)
		if err == nil {
			rendered, err := renderer.Render(content)
			if err == nil {
				content = strings.TrimSpace(rendered)
			}
		}
	}

	// Only apply additional wrapping if markdown rendering is disabled
	// (markdown renderer already handles wrapping)
	if f.config.WrapMessages && !f.config.MarkdownRendering && width > 10 {
		content = f.wrapText(content, width-2) // Leave some margin
	}

	// Special content formatting for error messages
	if msg.Role == "error" {
		// Apply red color to error content for better visibility
		errorColor := ConvertColorToAnsi(f.theme.Error)
		content = fmt.Sprintf("%s%s%s", errorColor, content, "\033[0m")
	}

	output.WriteString(content)
	output.WriteString("\n\n")

	return output.String()
}

func (f *MessageFormatter) FormatLoadingIndicator() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frame := frames[time.Now().UnixNano()/100000000%int64(len(frames))]
	primaryColor := ConvertColorToAnsi(f.theme.Primary)
	return fmt.Sprintf("\n%s%s Thinking...%s\n", primaryColor, frame, "\033[0m")
}

func (f *MessageFormatter) getRoleColor(role string) string {
	var color string
	switch role {
	case "user":
		color = f.theme.Primary
	case "assistant":
		color = f.theme.Secondary
	case "system":
		color = f.theme.Tertiary
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
	case "dracula":
		return "dracula"     // Perfect match - official Dracula theme
	case "monokai":
		return "tokyo-night" // Best match for monokai's bright colors
	case "solarized":
		return "dark"        // Good match for solarized's blue tones  
	case "nord":
		return "dark"        // Complements nord's blue palette
	default: // "default"
		return "auto"        // Auto-detects terminal background
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

func createMarkdownRenderer(theme *types.Theme, themeName string) (*glamour.TermRenderer, error) {
	glamourStyle := GetGlamourStyleForTheme(themeName)
	return glamour.NewTermRenderer(
		glamour.WithStandardStyle(glamourStyle),
		glamour.WithWordWrap(80), // Default width for backward compatibility
	)
}

func createMarkdownRendererWithWidth(theme *types.Theme, themeName string, width int) (*glamour.TermRenderer, error) {
	// Ensure minimum width
	if width < 20 {
		width = 20
	}
	glamourStyle := GetGlamourStyleForTheme(themeName)
	return glamour.NewTermRenderer(
		glamour.WithStandardStyle(glamourStyle),
		glamour.WithWordWrap(width),
	)
}

