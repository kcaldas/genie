package commands

// Retrier resends the last user message.
type Retrier interface {
	RetryLast() error
}

// RetryCommand resends the most recent user message, for when a turn
// failed or was cancelled.
type RetryCommand struct {
	BaseCommand
	chat Retrier
}

// NewRetryCommand builds the :retry command.
func NewRetryCommand(chat Retrier) *RetryCommand {
	return &RetryCommand{
		BaseCommand: BaseCommand{
			Name:        "retry",
			Description: "Send the last message again",
			Usage:       ":retry",
			Examples:    []string{":retry"},
			Aliases:     []string{"again"},
			Category:    "General",
		},
		chat: chat,
	}
}

// Execute resends the last user message, or queues it if a request runs.
func (c *RetryCommand) Execute(args []string) error {
	return c.chat.RetryLast()
}
