package presentation

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

type MessageFormatter struct {
	config         *types.Config
	theme          *types.Theme
	markdownRenderer *glamour.TermRenderer
}

func NewMessageFormatter(config *types.Config, theme *types.Theme) (*MessageFormatter, error) {
	renderer, err := createMarkdownRenderer(theme)
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
	var output strings.Builder
	
	roleColor := f.getRoleColor(msg.Role)
	rolePrefix := f.getRolePrefix(msg.Role)
	
	header := fmt.Sprintf("%s%s%s", roleColor, rolePrefix, "\033[0m")
	
	if f.config.ShowTimestamps {
		timestamp := time.Now().Format("15:04:05")
		header = fmt.Sprintf("[%s] %s", timestamp, header)
	}
	
	output.WriteString(header)
	output.WriteString("\n")
	
	content := msg.Content
	if f.config.MarkdownRendering && f.markdownRenderer != nil {
		rendered, err := f.markdownRenderer.Render(content)
		if err == nil {
			content = strings.TrimSpace(rendered)
		}
	}
	
	if f.config.WrapMessages {
		content = f.wrapText(content, 80)
	}
	
	output.WriteString(content)
	output.WriteString("\n\n")
	
	return output.String()
}

func (f *MessageFormatter) FormatLoadingIndicator() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frame := frames[time.Now().UnixNano()/100000000%int64(len(frames))]
	return fmt.Sprintf("\n%s%s Thinking...%s\n", f.theme.Primary, frame, "\033[0m")
}

func (f *MessageFormatter) getRoleColor(role string) string {
	switch role {
	case "user":
		return f.theme.Primary
	case "assistant":
		return f.theme.Secondary
	case "system":
		return f.theme.Tertiary
	case "error":
		return f.theme.Error
	default:
		return f.theme.Muted
	}
}

func (f *MessageFormatter) getRolePrefix(role string) string {
	switch role {
	case "user":
		return "You:"
	case "assistant":
		return "Genie:"
	case "system":
		return "System:"
	case "error":
		return "Error:"
	default:
		return role + ":"
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

func createMarkdownRenderer(theme *types.Theme) (*glamour.TermRenderer, error) {
	return glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
}