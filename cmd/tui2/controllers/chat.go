package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/tui2/types"
	"github.com/kcaldas/genie/pkg/genie"
)

type ChatController struct {
	*BaseController
	genie         genie.Genie
	stateAccessor types.IStateAccessor
	commandHandler CommandHandler
}

type CommandHandler interface {
	HandleCommand(command string, args []string) error
	GetAvailableCommands() []string
}

func NewChatController(
	ctx types.Component,
	gui types.IGuiCommon,
	genieService genie.Genie,
	state types.IStateAccessor,
	cmdHandler CommandHandler,
) *ChatController {
	return &ChatController{
		BaseController: NewBaseController(ctx, gui),
		genie:          genieService,
		stateAccessor:  state,
		commandHandler: cmdHandler,
	}
}

func (c *ChatController) HandleInput(input string) error {
	userInput := types.UserInput{
		Message:        input,
		IsSlashCommand: strings.HasPrefix(input, "/"),
	}
	
	if userInput.IsSlashCommand {
		return c.handleSlashCommand(userInput.Message)
	}
	
	return c.handleChatMessage(userInput.Message)
}

func (c *ChatController) handleSlashCommand(command string) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil
	}
	
	cmd := parts[0]
	args := parts[1:]
	
	if c.commandHandler != nil {
		return c.commandHandler.HandleCommand(cmd, args)
	}
	
	return fmt.Errorf("unknown command: %s", cmd)
}

func (c *ChatController) handleChatMessage(message string) error {
	// Add user message to display
	c.stateAccessor.AddMessage(types.Message{
		Role:    "user",
		Content: message,
	})
	
	// Set loading state
	c.stateAccessor.SetLoading(true)
	
	// Send message to Genie service
	ctx := context.Background()
	if err := c.genie.Chat(ctx, message); err != nil {
		c.stateAccessor.SetLoading(false)
		c.stateAccessor.AddMessage(types.Message{
			Role:    "error",
			Content: fmt.Sprintf("Failed to send message: %v", err),
		})
		return err
	}
	
	return nil
}

func (c *ChatController) ClearConversation() error {
	c.stateAccessor.ClearMessages()
	// TODO: Implement session reset when integrated with proper Genie service
	return nil
}

func (c *ChatController) GetConversationHistory() []types.Message {
	return c.stateAccessor.GetMessages()
}