package deepseek

import (
	"encoding/json"
	"fmt"
	"strings"

	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

// OpenAI-style tool wire types shared with the other chat-completions providers.
type (
	toolDefinition   = llmshared.ChatToolDefinition
	toolCall         = llmshared.ChatToolCall
	toolCallFunction = llmshared.ChatToolCallFunction
)

type chatRequest struct {
	Model          string           `json:"model"`
	Messages       []chatMessage    `json:"messages"`
	Stream         bool             `json:"stream"`
	StreamOptions  *streamOptions   `json:"stream_options,omitempty"`
	Temperature    *float32         `json:"temperature,omitempty"`
	MaxTokens      *int32           `json:"max_tokens,omitempty"`
	TopP           *float32         `json:"top_p,omitempty"`
	Tools          []toolDefinition `json:"tools,omitempty"`
	ToolChoice     *string          `json:"tool_choice,omitempty"`
	ResponseFormat *responseFormat  `json:"response_format,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    messageContent `json:"content"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []toolCall     `json:"tool_calls,omitempty"`
}

type messageContent struct {
	Parts []contentPart
}

func newMessageContent(parts []contentPart) messageContent {
	return messageContent{Parts: parts}
}

func newMessageContentFromText(text string) messageContent {
	return messageContent{Parts: []contentPart{{Type: "text", Text: text}}}
}

func (mc messageContent) MarshalJSON() ([]byte, error) {
	if len(mc.Parts) == 0 {
		return json.Marshal("")
	}
	if len(mc.Parts) == 1 && mc.Parts[0].Type == "text" {
		return json.Marshal(mc.Parts[0].Text)
	}
	return json.Marshal(mc.Parts)
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
		mc.Parts = []contentPart{{Type: "text", Text: text}}
		return nil
	}

	var parts []contentPart
	if err := json.Unmarshal(data, &parts); err != nil {
		return fmt.Errorf("decode message content parts: %w", err)
	}
	mc.Parts = parts
	return nil
}

type contentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type chatResponse struct {
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
	Usage   *usage       `json:"usage,omitempty"`
	Error   *apiError    `json:"error,omitempty"`
	Object  string       `json:"object,omitempty"`
	Created int64        `json:"created,omitempty"`
	ID      string       `json:"id,omitempty"`
}

type chatChoice struct {
	Index        int             `json:"index"`
	Message      responseMessage `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

type responseMessage struct {
	Role      string          `json:"role"`
	Content   responseContent `json:"content"`
	ToolCalls []toolCall      `json:"tool_calls"`
	// Reasoning carries deepseek-reasoner's chain of thought. It is
	// surfaced as a thinking event and must never be echoed back into
	// the conversation (the API rejects it in input messages), so
	// toChatMessage drops it.
	Reasoning json.RawMessage `json:"reasoning_content,omitempty"`
}

func (rm responseMessage) toChatMessage() chatMessage {
	return chatMessage{
		Role:      rm.Role,
		Content:   rm.Content.toMessageContent(),
		ToolCalls: rm.ToolCalls,
	}
}

type responseContent struct {
	parts []contentPart
}

func (rc responseContent) Text() string {
	if len(rc.parts) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, part := range rc.parts {
		if strings.TrimSpace(part.Text) != "" {
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
		var part contentPart
		if err := json.Unmarshal(data, &part); err != nil {
			return fmt.Errorf("decode message part: %w", err)
		}
		rc.parts = []contentPart{part}
	case '[':
		var parts []contentPart
		if err := json.Unmarshal(data, &parts); err != nil {
			return fmt.Errorf("decode message parts: %w", err)
		}
		rc.parts = parts
	case '"':
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return fmt.Errorf("decode message text: %w", err)
		}
		rc.parts = []contentPart{{Type: "text", Text: text}}
	default:
		rc.parts = []contentPart{{Type: "text", Text: string(data)}}
	}
	return nil
}

// usage mirrors DeepSeek's usage block. PromptTokens includes the cached
// portion; PromptCacheHitTokens/PromptCacheMissTokens split it for
// DeepSeek's cache-hit/miss pricing.
type usage struct {
	PromptTokens          int32 `json:"prompt_tokens"`
	CompletionTokens      int32 `json:"completion_tokens"`
	TotalTokens           int32 `json:"total_tokens"`
	PromptCacheHitTokens  int32 `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens int32 `json:"prompt_cache_miss_tokens"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	Code    any    `json:"code,omitempty"`
}

type chatStreamResponse struct {
	Choices []streamChoice `json:"choices"`
	Usage   *usage         `json:"usage,omitempty"`
	Error   *apiError      `json:"error,omitempty"`
}

type streamChoice struct {
	Delta        streamDelta `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

type streamDelta struct {
	Content   json.RawMessage `json:"content"`
	Role      string          `json:"role"`
	ToolCalls []deltaToolCall `json:"tool_calls"`
	Reasoning json.RawMessage `json:"reasoning_content,omitempty"`
}

type deltaToolCall struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Function deltaToolFunction `json:"function"`
}

type deltaToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func (d streamDelta) Text() string {
	return decodeContentText(d.Content)
}

// ReasoningText decodes the delta's reasoning_content payload streamed by
// deepseek-reasoner.
func (d streamDelta) ReasoningText() string {
	return decodeContentText(d.Reasoning)
}

// decodeContentText renders an OpenAI-compat content payload — a JSON
// string or an array of content parts — as plain text. Unknown shapes
// yield "".
func decodeContentText(raw json.RawMessage) string {
	data := llmshared.TrimJSONSpace(raw)
	if len(data) == 0 {
		return ""
	}

	if data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err == nil {
			return text
		}
	}

	var parts []contentPart
	if err := json.Unmarshal(data, &parts); err == nil {
		var builder strings.Builder
		for _, part := range parts {
			if strings.TrimSpace(part.Text) != "" {
				if builder.Len() > 0 {
					builder.WriteString("\n")
				}
				builder.WriteString(part.Text)
			}
		}
		return builder.String()
	}

	return ""
}

// responseFormat is DeepSeek's JSON-mode selector. Unlike OpenAI, only
// {"type": "json_object"} is supported — there is no strict json_schema
// variant, so the schema itself travels in the system prompt.
type responseFormat struct {
	Type string `json:"type"`
}
