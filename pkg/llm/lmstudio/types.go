package lmstudio

import (
	"encoding/json"
	"fmt"
	"strings"
)

type requestMode int

const (
	normalMode requestMode = iota
	countTokensMode
)

type chatRequest struct {
	Model          string           `json:"model"`
	Messages       []chatMessage    `json:"messages"`
	Stream         bool             `json:"stream"`
	Temperature    *float32         `json:"temperature,omitempty"`
	MaxTokens      *int32           `json:"max_tokens,omitempty"`
	TopP           *float32         `json:"top_p,omitempty"`
	Tools          []toolDefinition `json:"tools,omitempty"`
	ToolChoice     *string          `json:"tool_choice,omitempty"`
	ResponseFormat *responseFormat  `json:"response_format,omitempty"`
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
	data = bytesTrim(data)
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
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type toolDefinition struct {
	Type     string                `json:"type"`
	Function toolDefinitionDetails `json:"function"`
}

type toolDefinitionDetails struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type toolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function toolCallFunction `json:"function"`
}

type toolCallFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (f toolCallFunction) ArgumentsAsMap() (map[string]any, error) {
	if len(f.Arguments) == 0 {
		return map[string]any{}, nil
	}

	trimmed := bytesTrim(f.Arguments)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return map[string]any{}, nil
	}

	var args map[string]any
	if trimmed[0] == '"' {
		var raw string
		if err := json.Unmarshal(f.Arguments, &raw); err != nil {
			return nil, err
		}
		if raw == "" {
			return map[string]any{}, nil
		}
		if err := json.Unmarshal([]byte(raw), &args); err != nil {
			return nil, err
		}
		return args, nil
	}

	if err := json.Unmarshal(f.Arguments, &args); err != nil {
		return nil, err
	}
	if args == nil {
		return map[string]any{}, nil
	}
	return args, nil
}

type chatResponse struct {
	Model   string          `json:"model"`
	Choices []chatChoice    `json:"choices"`
	Usage   *usage          `json:"usage,omitempty"`
	Error   *apiError       `json:"error,omitempty"`
	Object  string          `json:"object,omitempty"`
	Created int64           `json:"created,omitempty"`
	ID      string          `json:"id,omitempty"`
	System  json.RawMessage `json:"system,omitempty"`
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
	data = bytesTrim(data)
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

type usage struct {
	PromptTokens     int32 `json:"prompt_tokens"`
	CompletionTokens int32 `json:"completion_tokens"`
	TotalTokens      int32 `json:"total_tokens"`
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
	System    json.RawMessage `json:"system,omitempty"`
	Refusal   json.RawMessage `json:"refusal,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	Audio     json.RawMessage `json:"audio,omitempty"`
	Reasoning json.RawMessage `json:"reasoning,omitempty"`
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
	data := bytesTrim(d.Content)
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

type responseFormat struct {
	Type       string                 `json:"type"`
	JSONSchema *responseFormatSchema  `json:"json_schema,omitempty"`
	Extra      map[string]interface{} `json:"-"`
}

type responseFormatSchema struct {
	Name   string         `json:"name"`
	Schema map[string]any `json:"schema"`
	Strict bool           `json:"strict"`
}

func bytesTrim(data []byte) []byte {
	start := 0
	end := len(data)
	for start < end && (data[start] == ' ' || data[start] == '\n' || data[start] == '\r' || data[start] == '\t') {
		start++
	}
	for end > start && (data[end-1] == ' ' || data[end-1] == '\n' || data[end-1] == '\r' || data[end-1] == '\t') {
		end--
	}
	return data[start:end]
}
