package commands

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/tui/controllers"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type YankCommand struct {
	BaseCommand
	chatState       *state.ChatState
	clipboardHelper *helpers.Clipboard
	notification    *controllers.ChatController
}

func NewYankCommand(chatState *state.ChatState, clipboardHelper *helpers.Clipboard, notification *controllers.ChatController) *YankCommand {
	return &YankCommand{
		BaseCommand: BaseCommand{
			Name:        "yank",
			Description: "Copy messages to clipboard (vim-style)",
			Usage:       ":y[count][direction] | :y-[count]",
			Examples: []string{
				":y",
				":y3",
				":y2k",
				":y5j",
				":y-1",
				":y-3",
			},
			Aliases:  []string{"y"},
			Category: "Clipboard",
		},
		chatState:       chatState,
		clipboardHelper: clipboardHelper,
		notification:    notification,
	}
}

func (c *YankCommand) Execute(args []string) error {
	// Parse vim-style yank command: :y[count][direction]
	// Examples: :y, :y3, :y2k, :y5j

	count := 1
	direction := "k" // default to up (k = previous messages)

	if len(args) > 0 {
		arg := args[0]
		// Parse count and direction from argument like "2k", "3j", "5"
		parsedCount, parsedDirection := c.parseYankArgument(arg)
		if parsedCount > 0 {
			count = parsedCount
		}
		if parsedDirection != "" {
			direction = parsedDirection
		}
	}

	var messages []types.Message
	var description string

	switch direction {
	case "k", "": // up/previous messages (default)
		messages = c.chatState.GetLastMessages(count)
		if count == 1 {
			description = "last message"
		} else {
			description = fmt.Sprintf("last %d messages", count)
		}
	case "j": // down/next messages (not very useful in chat context, but for completeness)
		// For now, just treat as same as k since we don't have cursor position
		messages = c.chatState.GetLastMessages(count)
		description = fmt.Sprintf("last %d messages", count)
	case "-": // relative positioning: copy the Nth message from the end
		totalMessages := c.chatState.GetMessageCount()
		if count > totalMessages {
			messages = []types.Message{}
		} else {
			// Get a single message at relative position
			// count=1 means last message, count=2 means 2nd to last, etc.
			start := totalMessages - count
			messages = c.chatState.GetMessageRange(start, 1)
		}
		if count == 1 {
			description = "last message"
		} else {
			description = fmt.Sprintf("message %d from end", count)
		}
	default:
		return fmt.Errorf("unknown direction: %s (use k for up, j for down, - for relative)", direction)
	}

	if len(messages) == 0 {
		c.notification.AddSystemMessage("No messages to copy.")
		return nil
	}

	// Format messages for clipboard
	var content strings.Builder
	for i, msg := range messages {
		if i > 0 {
			content.WriteString("\n---\n\n")
		}
		content.WriteString(fmt.Sprintf("[%s] %s", strings.ToUpper(msg.Role), msg.Content))
	}

	// Copy to clipboard
	if err := c.clipboardHelper.Copy(content.String()); err != nil {
		c.notification.AddErrorMessage(fmt.Sprintf("Failed to copy to clipboard: %v", err))
		return nil
	}

	// Success message
	c.notification.AddSystemMessage(fmt.Sprintf("Copied %s to clipboard.", description))
	return nil
}

func (c *YankCommand) parseYankArgument(arg string) (count int, direction string) {
	count = 0
	direction = ""

	// Parse patterns like "2k", "3j", "5", "k", "j", "-2", "-1"
	i := 0
	isRelative := false

	// Check for relative positioning (starts with -)
	if i < len(arg) && arg[i] == '-' {
		isRelative = true
		i++
	}

	// Extract number
	for i < len(arg) && arg[i] >= '0' && arg[i] <= '9' {
		count = count*10 + int(arg[i]-'0')
		i++
	}

	// Extract direction (for non-relative positioning)
	if i < len(arg) && !isRelative {
		direction = string(arg[i])
	}

	// For relative positioning, set direction to indicate relative mode
	if isRelative {
		direction = "-"
	}

	// Default count to 1 if not specified
	if count == 0 {
		count = 1
	}

	return count, direction
}
