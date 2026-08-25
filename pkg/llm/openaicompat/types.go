// Package openaicompat implements the OpenAI-compatible chat-completions
// protocol shared by providers that speak it (LM Studio, DeepSeek, and
// any future compatible endpoint). Providers embed Core and keep only
// what genuinely differs: base-URL and credential resolution, request
// assembly (messages, response format, sampling), and status reporting.
package openaicompat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

// ChatRequest is the chat-completions request body.
type ChatRequest struct {
	Model          string                         `json:"model"`
	Messages       []ChatMessage                  `json:"messages"`
	Stream         bool                           `json:"stream"`
	StreamOptions  *StreamOptions                 `json:"stream_options,omitempty"`
	Temperature    *float32                       `json:"temperature,omitempty"`
	MaxTokens      *int32                         `json:"max_tokens,omitempty"`
	TopP           *float32                       `json:"top_p,omitempty"`
	Tools          []llmshared.ChatToolDefinition `json:"tools,omitempty"`
	ToolChoice     *string                        `json:"tool_choice,omitempty"`
	ResponseFormat *ResponseFormat                `json:"response_format,omitempty"`
}

// StreamOptions asks the server to include usage in the final stream chunk.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// ChatMessage is one conversation message sent to the server.
type ChatMessage struct {
	Role       string                   `json:"role"`
	Content    MessageContent           `json:"content"`
	ToolCallID string                   `json:"tool_call_id,omitempty"`
	ToolCalls  []llmshared.ChatToolCall `json:"tool_calls,omitempty"`
}

// MessageContent marshals as a bare string for single text parts and as
// a part array otherwise, matching what compatible servers accept.
type MessageContent struct {
	Parts []ContentPart
}

// NewMessageContent wraps content parts.
func NewMessageContent(parts []ContentPart) MessageContent {
	return MessageContent{Parts: parts}
}

// NewMessageContentFromText wraps plain text as a single text part.
func NewMessageContentFromText(text string) MessageContent {
	return MessageContent{Parts: []ContentPart{{Type: "text", Text: text}}}
}

func (mc MessageContent) MarshalJSON() ([]byte, error) {
	if len(mc.Parts) == 0 {
		return json.Marshal("")
	}
	if len(mc.Parts) == 1 && mc.Parts[0].Type == "text" {
		return json.Marshal(mc.Parts[0].Text)
	}
	return json.Marshal(mc.Parts)
}

func (mc *MessageContent) UnmarshalJSON(data []byte) error {
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
		mc.Parts = []ContentPart{{Type: "text", Text: text}}
		return nil
	}

	var parts []ContentPart
	if err := json.Unmarshal(data, &parts); err != nil {
		return fmt.Errorf("decode message content parts: %w", err)
	}
	mc.Parts = parts
	return nil
}

// ContentPart is one typed chunk of message content.
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL carries an image reference (usually a data URL).
type ImageURL struct {
	URL string `json:"url"`
}

// ChatResponse is the blocking chat-completions response body.
type ChatResponse struct {
	Model   string          `json:"model"`
	Choices []ChatChoice    `json:"choices"`
	Usage   *Usage          `json:"usage,omitempty"`
	Error   *APIError       `json:"error,omitempty"`
	Object  string          `json:"object,omitempty"`
	Created int64           `json:"created,omitempty"`
	ID      string          `json:"id,omitempty"`
	System  json.RawMessage `json:"system,omitempty"`
}

// ChatChoice is one completion alternative.
type ChatChoice struct {
	Index        int             `json:"index"`
	Message      ResponseMessage `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

// ResponseMessage is the assistant message returned by the server.
// Reasoning payloads arrive under different keys depending on the
// server — `reasoning` (LM Studio) or `reasoning_content` (DeepSeek) —
// so both are decoded. They are surfaced via ReasoningText and must
// never be echoed back into the conversation (some APIs reject them in
// input messages): ToChatMessage drops them.
type ResponseMessage struct {
	Role             string                   `json:"role"`
	Content          ResponseContent          `json:"content"`
	ToolCalls        []llmshared.ChatToolCall `json:"tool_calls"`
	Reasoning        json.RawMessage          `json:"reasoning,omitempty"`
	ReasoningContent json.RawMessage          `json:"reasoning_content,omitempty"`
}

// ReasoningText decodes the message's reasoning payload from whichever
// key the server used.
func (rm ResponseMessage) ReasoningText() string {
	if text := DecodeContentText(rm.ReasoningContent); text != "" {
		return text
	}
	return DecodeContentText(rm.Reasoning)
}

// ToChatMessage converts the response into a history message, dropping
// reasoning payloads.
func (rm ResponseMessage) ToChatMessage() ChatMessage {
	return ChatMessage{
		Role:      rm.Role,
		Content:   rm.Content.ToMessageContent(),
		ToolCalls: rm.ToolCalls,
	}
}

// ResponseContent tolerates the content shapes compatible servers emit:
// a bare string, a single part object, or a part array.
type ResponseContent struct {
	Parts []ContentPart
}

// Text renders the textual parts joined by newlines.
func (rc ResponseContent) Text() string {
	if len(rc.Parts) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, part := range rc.Parts {
		if strings.TrimSpace(part.Text) != "" {
			if builder.Len() > 0 {
				builder.WriteString("\n")
			}
			builder.WriteString(part.Text)
		}
	}
	return builder.String()
}

// ToMessageContent converts response content into sendable content.
func (rc ResponseContent) ToMessageContent() MessageContent {
	if len(rc.Parts) == 0 {
		return NewMessageContentFromText("")
	}
	return NewMessageContent(rc.Parts)
}

func (rc ResponseContent) MarshalJSON() ([]byte, error) {
	if len(rc.Parts) == 0 {
		return json.Marshal("")
	}
	if len(rc.Parts) == 1 && rc.Parts[0].Type == "text" {
		return json.Marshal(rc.Parts[0].Text)
	}
	return json.Marshal(rc.Parts)
}

func (rc *ResponseContent) UnmarshalJSON(data []byte) error {
	data = llmshared.TrimJSONSpace(data)
	if len(data) == 0 || string(data) == "null" {
		rc.Parts = nil
		return nil
	}
	switch data[0] {
	case '{':
		var part ContentPart
		if err := json.Unmarshal(data, &part); err != nil {
			return fmt.Errorf("decode message part: %w", err)
		}
		rc.Parts = []ContentPart{part}
	case '[':
		var parts []ContentPart
		if err := json.Unmarshal(data, &parts); err != nil {
			return fmt.Errorf("decode message parts: %w", err)
		}
		rc.Parts = parts
	case '"':
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return fmt.Errorf("decode message text: %w", err)
		}
		rc.Parts = []ContentPart{{Type: "text", Text: text}}
	default:
		rc.Parts = []ContentPart{{Type: "text", Text: string(data)}}
	}
	return nil
}

// Usage is the server's token accounting. PromptTokens includes any
// cached portion; the cache-hit/miss fields split it on servers with
// cache-aware pricing (DeepSeek) and stay zero elsewhere.
type Usage struct {
	PromptTokens          int32 `json:"prompt_tokens"`
	CompletionTokens      int32 `json:"completion_tokens"`
	TotalTokens           int32 `json:"total_tokens"`
	PromptCacheHitTokens  int32 `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens int32 `json:"prompt_cache_miss_tokens"`
}

// TokenCount maps usage onto the provider-neutral token count. A
// missing total falls back to input + output.
func (u *Usage) TokenCount() *ai.TokenCount {
	if u == nil {
		return nil
	}
	total := u.TotalTokens
	if total == 0 {
		total = u.PromptTokens + u.CompletionTokens
	}
	return &ai.TokenCount{
		TotalTokens:  total,
		InputTokens:  u.PromptTokens,
		OutputTokens: u.CompletionTokens,
	}
}

// APIError is the in-body error envelope.
type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	Code    any    `json:"code,omitempty"`
}

// ChatStreamResponse is one SSE chunk of a streaming completion.
type ChatStreamResponse struct {
	Choices []StreamChoice `json:"choices"`
	Usage   *Usage         `json:"usage,omitempty"`
	Error   *APIError      `json:"error,omitempty"`
}

// StreamChoice is one streamed completion alternative.
type StreamChoice struct {
	Delta        StreamDelta `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

// StreamDelta is the incremental payload of one stream chunk. Like
// ResponseMessage, reasoning is accepted under both wire keys.
type StreamDelta struct {
	Content          json.RawMessage `json:"content"`
	Role             string          `json:"role"`
	ToolCalls        []DeltaToolCall `json:"tool_calls"`
	System           json.RawMessage `json:"system,omitempty"`
	Refusal          json.RawMessage `json:"refusal,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	Audio            json.RawMessage `json:"audio,omitempty"`
	Reasoning        json.RawMessage `json:"reasoning,omitempty"`
	ReasoningContent json.RawMessage `json:"reasoning_content,omitempty"`
}

// Text decodes the delta's content payload.
func (d StreamDelta) Text() string {
	return DecodeContentText(d.Content)
}

// ReasoningText decodes the delta's reasoning payload from whichever
// key the server used.
func (d StreamDelta) ReasoningText() string {
	if text := DecodeContentText(d.ReasoningContent); text != "" {
		return text
	}
	return DecodeContentText(d.Reasoning)
}

// DeltaToolCall is an incremental tool-call fragment.
type DeltaToolCall struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Function DeltaToolFunction `json:"function"`
}

// DeltaToolFunction carries incremental tool name/argument fragments.
type DeltaToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// DecodeContentText renders an OpenAI-compat content payload — a JSON
// string or an array of content parts — as plain text. Unknown shapes
// yield "".
func DecodeContentText(raw json.RawMessage) string {
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

	var parts []ContentPart
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

// ResponseFormat selects the server's structured-output mode: plain
// {"type": "json_object"} on servers without schema support (DeepSeek),
// or "json_schema" with the schema attached (LM Studio).
type ResponseFormat struct {
	Type       string                `json:"type"`
	JSONSchema *ResponseFormatSchema `json:"json_schema,omitempty"`
}

// ResponseFormatSchema is the strict-schema variant's payload.
type ResponseFormatSchema struct {
	Name   string         `json:"name"`
	Schema map[string]any `json:"schema"`
	Strict bool           `json:"strict"`
}
