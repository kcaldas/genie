package genie

import (
	"path/filepath"

	"github.com/kcaldas/genie/pkg/ctx"
)

type StartOption func(*startOptions)

type startOptions struct {
	chatHistory []ChatHistoryTurn
	personaYAML []byte
	allowedDirs []string
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

// WithAllowedDirs adds extra directories that file tools may access.
// Only absolute paths are accepted; relative paths are silently skipped.
func WithAllowedDirs(dirs ...string) StartOption {
	return func(opts *startOptions) {
		for _, d := range dirs {
			if filepath.IsAbs(d) {
				opts.allowedDirs = append(opts.allowedDirs, filepath.Clean(d))
			}
		}
	}
}

// WithPersonaYAML provides persona configuration directly as YAML bytes,
// bypassing file-based persona discovery. The YAML must be valid prompt.yaml content.
// When provided, the persona parameter to Start() can be nil.
func WithPersonaYAML(yamlContent []byte) StartOption {
	return func(opts *startOptions) {
		opts.personaYAML = yamlContent
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
