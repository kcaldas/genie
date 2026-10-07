// Package cloudchat calls hosted model providers that speak the
// OpenAI-compatible chat-completions protocol with a Bearer API key. Each
// provider is a Spec; the request layer, tool loop and usage accounting
// are shared.
package cloudchat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/llm/openaicompat"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/pkoukk/tiktoken-go"
)

const defaultMaxToolIterations = 200

var (
	errMissingAPIKey = errors.New("provider API key not configured")

	_ ai.Gen = (*Client)(nil)
)

// Spec describes one provider.
type Spec struct {
	// Provider names the provider in usage events and status.
	Provider string
	// DefaultBaseURL is the chat-completions root; the client appends
	// /chat/completions.
	DefaultBaseURL string
	// DefaultModel is used when neither the prompt nor the configuration
	// names a model.
	DefaultModel string
	// APIKeyEnv and BaseURLEnv are read in order; the first non-empty
	// value wins.
	APIKeyEnv  []string
	BaseURLEnv []string
}

// Maritaca serves the Sabiá models; IDs ending in -br-sp run in São Paulo.
var Maritaca = Spec{
	Provider:       "maritaca",
	DefaultBaseURL: "https://chat.maritaca.ai/api",
	DefaultModel:   "sabia-4",
	APIKeyEnv:      []string{"MARITACA_API_KEY", "GENIE_MARITACA_API_KEY"},
	BaseURLEnv:     []string{"GENIE_MARITACA_BASE_URL", "MARITACA_BASE_URL"},
}

// Option configures the client.
type Option = llmshared.LocalOption

// Shared functional options operating on the embedded client core.
var (
	// WithConfigManager injects a custom configuration manager.
	WithConfigManager = llmshared.WithConfigManager
	// WithLogger injects a custom logger implementation.
	WithLogger = llmshared.WithLogger
	// WithHTTPClient injects a custom HTTP client.
	WithHTTPClient = llmshared.WithHTTPClient
)

// Client provides an ai.Gen implementation for one Spec, a thin
// configuration layer over the shared OpenAI-compat core.
type Client struct {
	openaicompat.Core
	spec Spec
}

// NewClient creates a client for spec. The API key is resolved on first
// use, so construction and status work before the environment is complete.
func NewClient(spec Spec, eventBus events.EventBus, opts ...Option) (ai.Gen, error) {
	client := &Client{Core: openaicompat.NewCore(spec.Provider, eventBus), spec: spec}
	client.StreamIncludeUsage = true
	for _, opt := range opts {
		opt(&client.LocalClientCore)
	}
	if strings.TrimSpace(client.BaseURL) == "" {
		client.BaseURL = strings.TrimRight(client.firstEnv(spec.BaseURLEnv, spec.DefaultBaseURL), "/")
	}
	return client, nil
}

// GenerateContent renders the prompt using string attributes and executes it.
func (c *Client) GenerateContent(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (string, error) {
	return c.GenerateContentAttr(ctx, prompt, debug, ai.StringsToAttr(args))
}

// GenerateContentAttr renders the prompt and runs the shared agent loop
// until the model answers without tool calls.
func (c *Client) GenerateContentAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (string, error) {
	if err := c.ensureAPIKey(); err != nil {
		return "", err
	}
	rendered, err := c.RenderPrompt(prompt, debug, attrs)
	if err != nil {
		return "", fmt.Errorf("rendering prompt: %w", err)
	}
	request, err := c.buildChatRequest(*rendered)
	if err != nil {
		return "", err
	}
	turn := c.NewTurn(request, rendered.Handlers, openaicompat.TurnOptions{})
	return llmshared.RunToolLoop(ctx, turn, turn.Handlers(), c.loopConfig(*rendered), nil)
}

// GenerateContentStream renders the prompt using string attributes and executes it with streaming.
func (c *Client) GenerateContentStream(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (ai.Stream, error) {
	return c.GenerateContentAttrStream(ctx, prompt, debug, ai.StringsToAttr(args))
}

// GenerateContentAttrStream streams a plain answer; with tools it runs
// the blocking tool loop and wraps the final answer as a single chunk.
func (c *Client) GenerateContentAttrStream(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (ai.Stream, error) {
	if err := c.ensureAPIKey(); err != nil {
		return nil, err
	}
	rendered, err := c.RenderPrompt(prompt, debug, attrs)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}
	request, err := c.buildChatRequest(*rendered)
	if err != nil {
		return nil, err
	}
	if len(rendered.Functions) > 0 && len(rendered.Handlers) > 0 {
		turn := c.NewTurn(request, rendered.Handlers, openaicompat.TurnOptions{})
		return c.BlockingLoopStream(ctx, turn, c.loopConfig(*rendered)), nil
	}
	return c.StreamChat(ctx, request), nil
}

// CountTokens renders the prompt and estimates token usage using string attributes.
func (c *Client) CountTokens(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (*ai.TokenCount, error) {
	return c.CountTokensAttr(ctx, prompt, debug, ai.StringsToAttr(args))
}

// CountTokensAttr estimates token usage locally, so counting makes no
// paid request.
func (c *Client) CountTokensAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (*ai.TokenCount, error) {
	rendered, err := c.RenderPrompt(prompt, debug, attrs)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}
	messages, err := c.buildMessages(*rendered, c.resolveModel(*rendered))
	if err != nil {
		return nil, err
	}
	total, err := countTokensForMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("counting tokens: %w", err)
	}
	tokenCount := &ai.TokenCount{TotalTokens: int32(total), InputTokens: int32(total)}
	c.PublishTokenCount(ctx, tokenCount)
	return tokenCount, nil
}

// GetStatus reports whether the API key is configured and which model is in use.
func (c *Client) GetStatus() *ai.Status {
	model := c.Config.GetModelConfig()
	modelStr := fmt.Sprintf("%s, Temperature: %.2f, Max Tokens: %d", model.ModelName, model.Temperature, model.MaxTokens)
	if c.resolveAPIKey() == "" {
		return &ai.Status{Model: modelStr, Backend: c.spec.Provider, Connected: false, Message: c.spec.APIKeyEnv[0] + " not configured"}
	}
	return &ai.Status{Model: modelStr, Backend: c.spec.Provider, Connected: true, Message: fmt.Sprintf("%s configured (endpoint: %s)", c.spec.Provider, c.BaseURL)}
}

func (c *Client) ensureAPIKey() error {
	if key := c.resolveAPIKey(); key != "" {
		c.AuthToken = key
		return nil
	}
	return ai.NonRetryable(fmt.Errorf("%w: please export %s", errMissingAPIKey, c.spec.APIKeyEnv[0]))
}

func (c *Client) resolveAPIKey() string {
	if token := strings.TrimSpace(c.AuthToken); token != "" {
		return token
	}
	return c.firstEnv(c.spec.APIKeyEnv, "")
}

func (c *Client) firstEnv(keys []string, fallback string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(c.Config.GetStringWithDefault(key, "")); value != "" {
			return value
		}
	}
	return fallback
}

func (c *Client) resolveModel(prompt ai.Prompt) string {
	if model := c.ResolveModelName(prompt.ModelName); strings.TrimSpace(model) != "" {
		return model
	}
	return c.spec.DefaultModel
}

func (c *Client) loopConfig(prompt ai.Prompt) llmshared.LoopConfig {
	return llmshared.NewLoopConfig(c.Config, c.EventBus, prompt, defaultMaxToolIterations)
}

func (c *Client) buildChatRequest(prompt ai.Prompt) (chatRequest, error) {
	modelName := c.resolveModel(prompt)
	messages, err := c.buildMessages(prompt, modelName)
	if err != nil {
		return chatRequest{}, err
	}
	req := chatRequest{Model: modelName, Messages: messages}
	c.applyGenerationConfig(&req, prompt)
	if len(prompt.Functions) > 0 {
		req.Tools = llmshared.MapFunctions(prompt.Functions, schemaToMap)
		if len(req.Tools) > 0 {
			auto := "auto"
			req.ToolChoice = &auto
		}
	}
	if prompt.ResponseSchema != nil {
		// JSON mode alone; the schema rides in the system prompt (see
		// buildSystemText).
		req.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	return req, nil
}

// buildMessages lays the conversation out as native chat messages (see
// llmshared.Conversation): system, one message per side of each past
// turn, then the current turn.
func (c *Client) buildMessages(prompt ai.Prompt, modelName string) ([]chatMessage, error) {
	system, err := buildSystemText(prompt)
	if err != nil {
		return nil, err
	}
	layout := llmshared.LayoutConversation(prompt).Messages()
	messages := make([]chatMessage, 0, len(layout)+1)
	if system != "" {
		messages = append(messages, chatMessage{Role: llmshared.RoleSystem, Content: newMessageContentFromText(system)})
	}
	for i, m := range layout {
		switch {
		case m.Role == llmshared.RoleSystem:
			// Already emitted, schema included.
		case i == len(layout)-1:
			messages = append(messages, chatMessage{Role: "user", Content: newMessageContentFromText(withImageNotes(m.Text, m.Images, modelName))})
		default:
			messages = append(messages, chatMessage{Role: m.Role, Content: newMessageContentFromText(m.Text)})
		}
	}
	return messages, nil
}

// buildSystemText is the stable system text plus the response schema, if any.
func buildSystemText(prompt ai.Prompt) (string, error) {
	system := llmshared.LayoutConversation(prompt).System
	if prompt.ResponseSchema == nil {
		return system, nil
	}
	schema, err := json.MarshalIndent(schemaToMap(prompt.ResponseSchema), "", "  ")
	if err != nil {
		return "", fmt.Errorf("formatting response schema: %w", err)
	}
	instruction := fmt.Sprintf("You must respond with JSON matching this schema:\n%s", schema)
	if system == "" {
		return instruction, nil
	}
	return system + "\n\n" + instruction, nil
}

// withImageNotes replaces images with a note each: these models take text
// input only.
func withImageNotes(text string, images []*ai.Image, modelName string) string {
	var notes []string
	for _, img := range images {
		if img == nil || len(img.Data) == 0 {
			continue
		}
		mimeType := strings.TrimSpace(img.Type)
		if mimeType == "" {
			mimeType = "image/png"
		}
		notes = append(notes, fmt.Sprintf("[attached image (%s, %d bytes) could not be included: model %s accepts text input only]", mimeType, len(img.Data), modelName))
	}
	if len(notes) == 0 {
		return text
	}
	if text != "" {
		text += "\n\n"
	}
	return text + strings.Join(notes, "\n")
}

func (c *Client) applyGenerationConfig(req *chatRequest, prompt ai.Prompt) {
	modelCfg := c.Config.GetModelConfig()
	maxTokens := prompt.MaxTokens
	if maxTokens <= 0 {
		maxTokens = modelCfg.MaxTokens
	}
	// Providers here reject max_tokens above the model's output limit
	// rather than capping it.
	if caps := prompt.ModelCapabilities; caps != nil && caps.OutputTokenLimit > 0 && int(maxTokens) > caps.OutputTokenLimit {
		maxTokens = int32(caps.OutputTokenLimit)
	}
	if maxTokens > 0 {
		value := int32(maxTokens)
		req.MaxTokens = &value
	}
	temperature := prompt.Temperature
	if temperature <= 0 {
		temperature = modelCfg.Temperature
	}
	if temperature > 0 {
		value := float32(temperature)
		req.Temperature = &value
	}
	topP := prompt.TopP
	if topP <= 0 {
		topP = modelCfg.TopP
	}
	if topP > 0 && topP < 1.0 {
		value := float32(topP)
		req.TopP = &value
	}
}

func schemaToMap(schema *ai.Schema) map[string]any {
	return llmshared.SchemaToMap(schema, true)
}

// countTokensForMessages estimates prompt tokens locally with the
// cl100k_base encoding: an approximation of each provider's tokenizer
// that keeps counting off the paid API.
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
		total += tokensPerMessage + len(encoder.Encode(msg.Role, nil, nil))
		for _, part := range msg.Content.Parts {
			total += len(encoder.Encode(part.Text, nil, nil))
		}
	}
	return total + 3, nil
}
