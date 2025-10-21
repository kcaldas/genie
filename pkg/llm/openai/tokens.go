package openai

import (
	"fmt"
	"strings"

	"github.com/pkoukk/tiktoken-go"
)

func countTokensForMessages(messages []tokenMessage, model string) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}

	encoder, err := tiktoken.EncodingForModel(model)
	if err != nil {
		encoderFallback, fallbackErr := tiktoken.GetEncoding("cl100k_base")
		if fallbackErr != nil {
			return 0, fmt.Errorf("get encoding: %w", fallbackErr)
		}
		encoder = encoderFallback
	}

	tokensPerMessage, tokensPerName := tokensPerMessageForModel(model)
	total := 0

	for _, msg := range messages {
		role := strings.TrimSpace(msg.Role)
		content := strings.TrimSpace(msg.Content)

		total += tokensPerMessage
		if role != "" {
			total += len(encoder.Encode(role, nil, nil))
		}

		if name := strings.TrimSpace(msg.Name); name != "" {
			total += len(encoder.Encode(name, nil, nil))
			total += tokensPerName
		}

		if content != "" {
			total += len(encoder.Encode(content, nil, nil))
		}
	}

	total += 3 // Every reply is primed with <|start|>assistant
	return total, nil
}

func tokensPerMessageForModel(model string) (int, int) {
	switch model {
	case "gpt-3.5-turbo-0301":
		return 4, -1
	case "gpt-3.5-turbo-0613",
		"gpt-3.5-turbo-16k-0613",
		"gpt-4-0314",
		"gpt-4-32k-0314",
		"gpt-4-0613",
		"gpt-4-32k-0613":
		return 3, 1
	}

	switch {
	case strings.Contains(model, "gpt-4o"),
		strings.Contains(model, "gpt-4.1"),
		strings.Contains(model, "gpt-4.5"),
		strings.Contains(model, "gpt-4"),
		strings.Contains(model, "o1-"),
		strings.Contains(model, "o3-"),
		strings.Contains(model, "gpt-3.5-turbo"):
		return 3, 1
	default:
		return 3, 1
	}
}
