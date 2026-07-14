package ollama

import (
	"encoding/json"
	"fmt"
	"strings"

	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

// OpenAI-style tool wire types shared with other local providers.
type (
	toolDefinition   = llmshared.ChatToolDefinition
	toolCall         = llmshared.ChatToolCall
	toolCallFunction = llmshared.ChatToolCallFunction
)

type chatRequest struct {
	Model    string           `json:"model"`
	Messages []chatMessage    `json:"messages"`
	Tools    []toolDefinition `json:"tools,omitempty"`
	Stream   bool             `json:"stream"`
	Options  map[string]any   `json:"options,omitempty"`
	Format   map[string]any   `json:"format,omitempty"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    messageContent `json:"content"`
	Images     []string       `json:"images,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []toolCall     `json:"tool_calls,omitempty"`
}

type messageContent struct {
	Parts []messagePart
}

func newMessageContent(parts []messagePart) messageContent {
	return messageContent{Parts: parts}
}

func newMessageContentFromText(text string) messageContent {
	if strings.TrimSpace(text) == "" {
		return messageContent{Parts: []messagePart{{Type: "text", Text: ""}}}
	}
	return messageContent{Parts: []messagePart{{Type: "text", Text: text}}}
}

func (mc messageContent) MarshalJSON() ([]byte, error) {
	// Ollama API expects content as a plain string, never an array.
	// Images are sent via the separate "images" field on chatMessage.
	var text strings.Builder
	for _, part := range mc.Parts {
		if part.Type == "text" && part.Text != "" {
			if text.Len() > 0 {
				text.WriteString("\n")
			}
			text.WriteString(part.Text)
		}
	}
	return json.Marshal(text.String())
}

func (mc *messageContent) UnmarshalJSON(data []byte) error {
	data = llmshared.TrimJSONSpace(data)
	if len(data) == 0 {
		mc.Parts = nil
		return nil
	}
	if data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return fmt.Errorf("decode message text: %w", err)
		}
		mc.Parts = []messagePart{{Type: "text", Text: text}}
		return nil
	}
	var parts []messagePart
	if err := json.Unmarshal(data, &parts); err != nil {
		return fmt.Errorf("decode message content parts: %w", err)
	}
	mc.Parts = parts
	return nil
}

type messagePart struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	Image string `json:"image,omitempty"`
}

type chatResponse struct {
	Model           string          `json:"model"`
	Message         responseMessage `json:"message"`
	Done            bool            `json:"done"`
	PromptEvalCount int             `json:"prompt_eval_count"`
	EvalCount       int             `json:"eval_count"`
	PromptEvalTime  int64           `json:"prompt_eval_duration"`
	EvalTime        int64           `json:"eval_duration"`
	TotalDuration   int64           `json:"total_duration"`
	LoadDuration    int64           `json:"load_duration"`
	Error           string          `json:"error"`
}

type responseMessage struct {
	Role      string          `json:"role"`
	Content   responseContent `json:"content"`
	ToolCalls []toolCall      `json:"tool_calls"`
}

func (rm responseMessage) toChatMessage() chatMessage {
	return chatMessage{
		Role:      rm.Role,
		Content:   rm.Content.toMessageContent(),
		ToolCalls: rm.ToolCalls,
	}
}

type responseContent struct {
	parts []messagePart
}

func (rc responseContent) Text() string {
	if len(rc.parts) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, part := range rc.parts {
		if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
			if builder.Len() > 0 {
				builder.WriteString("\n")
			}
			builder.WriteString(part.Text)
		}
	}
	return builder.String()
}

func (rc responseContent) toMessageContent() messageContent {
	if len(rc.parts) == 0 {
		return newMessageContentFromText("")
	}
	return newMessageContent(rc.parts)
}

func (rc responseContent) MarshalJSON() ([]byte, error) {
	if len(rc.parts) == 0 {
		return json.Marshal("")
	}
	if len(rc.parts) == 1 && rc.parts[0].Type == "text" {
		return json.Marshal(rc.parts[0].Text)
	}
	return json.Marshal(rc.parts)
}

func (rc *responseContent) UnmarshalJSON(data []byte) error {
	data = llmshared.TrimJSONSpace(data)
	if len(data) == 0 {
		rc.parts = nil
		return nil
	}
	switch data[0] {
	case '{':
		var part messagePart
		if err := json.Unmarshal(data, &part); err != nil {
			return fmt.Errorf("decode message part: %w", err)
		}
		rc.parts = []messagePart{part}
	case '[':
		var parts []messagePart
		if err := json.Unmarshal(data, &parts); err != nil {
			return fmt.Errorf("decode message parts: %w", err)
		}
		rc.parts = parts
	case '"':
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return fmt.Errorf("decode message text: %w", err)
		}
		rc.parts = []messagePart{{Type: "text", Text: text}}
	default:
		rc.parts = []messagePart{{Type: "text", Text: string(data)}}
	}
	return nil
}
