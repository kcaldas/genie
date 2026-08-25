package deepseek

import (
	"fmt"
	"strings"

	"github.com/pkoukk/tiktoken-go"
)

// countTokensForMessages estimates prompt tokens locally with the
// cl100k_base encoding. DeepSeek uses its own tokenizer, so this is an
// approximation — but it keeps CountTokens off the paid API entirely.
func countTokensForMessages(messages []chatMessage) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}

	encoder, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return 0, fmt.Errorf("get encoding: %w", err)
	}

	const tokensPerMessage = 3
	total := 0

	for _, msg := range messages {
		total += tokensPerMessage
		if role := strings.TrimSpace(msg.Role); role != "" {
			total += len(encoder.Encode(role, nil, nil))
		}
		var content strings.Builder
		for _, part := range msg.Content.Parts {
			content.WriteString(part.Text)
		}
		if text := strings.TrimSpace(content.String()); text != "" {
			total += len(encoder.Encode(text, nil, nil))
		}
	}

	total += 3 // Every reply is primed with <|start|>assistant
	return total, nil
}
