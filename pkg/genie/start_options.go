package genie

import (
	"path/filepath"

	"github.com/kcaldas/genie/pkg/ctx"
)

type StartOption func(*startOptions)

type startOptions struct {
	chatHistory       []ChatHistoryTurn
	personaYAML       []byte
	allowedDirs       []string
	deniedPaths       []string
	readOnlyPaths     []string
	commitAuthorName  string
	commitAuthorEmail string
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

// WithDeniedPaths sets glob patterns the agent must not touch at all.
// Patterns are workspace-relative; matching is the same as fileops
// `denied_paths` (supports `dir/**`, `**/dir`, `*.ext`, exact paths).
// Read and mutate operations against a denied path are both refused.
func WithDeniedPaths(patterns ...string) StartOption {
	return func(opts *startOptions) {
		for _, p := range patterns {
			if p != "" {
				opts.deniedPaths = append(opts.deniedPaths, p)
			}
		}
	}
}

// WithReadOnlyPaths sets glob patterns the agent may read but not
// mutate. Same matching rules as WithDeniedPaths.
func WithReadOnlyPaths(patterns ...string) StartOption {
	return func(opts *startOptions) {
		for _, p := range patterns {
			if p != "" {
				opts.readOnlyPaths = append(opts.readOnlyPaths, p)
			}
		}
	}
}

// WithCommitAuthor sets the opaque author identity gitCommit attributes
// commits to. Genie writes both fields verbatim — they're whatever the
// host wants. Pass empty values to fall back to the platform default
// (mutiro-agent / noreply@mutiro.local).
func WithCommitAuthor(name, email string) StartOption {
	return func(opts *startOptions) {
		opts.commitAuthorName = name
		opts.commitAuthorEmail = email
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
