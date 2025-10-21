package genie

import "github.com/kcaldas/genie/pkg/ctx"

type StartOption func(*startOptions)

type startOptions struct {
	chatHistory []ChatHistoryTurn
}

// ChatHistoryTurn represents a prior exchange between user and assistant.
type ChatHistoryTurn struct {
	User      string
	Assistant string
}

// WithChatHistory seeds Genie with prior chat history so prompts include it from the start.
// Empty turns are ignored.
func WithChatHistory(turns ...ChatHistoryTurn) StartOption {
	return func(opts *startOptions) {
		for _, turn := range turns {
			if turn.User == "" && turn.Assistant == "" {
				continue
			}
			opts.chatHistory = append(opts.chatHistory, ChatHistoryTurn{
				User:      turn.User,
				Assistant: turn.Assistant,
			})
		}
	}
}

func applyStartOptions(optionFns ...StartOption) startOptions {
	opts := startOptions{}
	for _, opt := range optionFns {
		if opt == nil {
			continue
		}
		opt(&opts)
	}
	return opts
}

func (s startOptions) toMessages() []ctx.Message {
	if len(s.chatHistory) == 0 {
		return nil
	}
	messages := make([]ctx.Message, 0, len(s.chatHistory))
	for _, turn := range s.chatHistory {
		if turn.User == "" && turn.Assistant == "" {
			continue
		}
		messages = append(messages, ctx.Message{
			User:      turn.User,
			Assistant: turn.Assistant,
		})
	}
	if len(messages) == 0 {
		return nil
	}
	return messages
}
