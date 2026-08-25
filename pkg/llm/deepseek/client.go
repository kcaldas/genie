package deepseek

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/llm/openaicompat"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

const (
	defaultMaxToolIterations = 200
	defaultBaseURL           = "https://api.deepseek.com"
	defaultModelName         = "deepseek-chat"
)

var (
	errMissingAPIKey = errors.New("deepseek backend not configured")

	_ ai.Gen = (*Client)(nil)
)

// Option configures the DeepSeek client.
type Option = llmshared.LocalOption

// Shared functional options operating on the embedded client core.
var (
	// WithConfigManager injects a custom configuration manager.
	WithConfigManager = llmshared.WithConfigManager
	// WithFileManager injects a custom file manager (useful for tests).
	WithFileManager = llmshared.WithFileManager
	// WithTemplateEngine injects a custom template engine.
	WithTemplateEngine = llmshared.WithTemplateEngine
	// WithLogger injects a custom logger implementation.
	WithLogger = llmshared.WithLogger
	// WithHTTPClient injects a custom HTTP client.
	WithHTTPClient = llmshared.WithHTTPClient
)

// WithBaseURL overrides the DeepSeek base URL.
func WithBaseURL(baseURL string) Option {
	return func(c *llmshared.LocalClientCore) {
		if strings.TrimSpace(baseURL) != "" {
			c.BaseURL = normalizeBaseURL(baseURL)
		}
	}
}

// WithAPIKey overrides the DeepSeek API key (useful for tests).
func WithAPIKey(apiKey string) Option {
	return func(c *llmshared.LocalClientCore) {
		if strings.TrimSpace(apiKey) != "" {
			c.AuthToken = strings.TrimSpace(apiKey)
		}
	}
}

// Client provides an ai.Gen implementation backed by the DeepSeek API,
// a thin configuration layer over the shared OpenAI-compat core.
type Client struct {
	openaicompat.Core
}

// NewClient creates a new DeepSeek-backed ai.Gen implementation. The
// API key is resolved lazily so construction (and status reporting)
// works before DEEPSEEK_API_KEY is exported.
func NewClient(eventBus events.EventBus, opts ...Option) (ai.Gen, error) {
	client := &Client{Core: openaicompat.NewCore("deepseek", eventBus)}
	// DeepSeek reports usage (with its cache split) in the final chunk
	// when asked via stream_options.
	client.StreamIncludeUsage = true

	for _, opt := range opts {
		opt(&client.LocalClientCore)
	}

	if strings.TrimSpace(client.BaseURL) == "" {
		client.BaseURL = client.resolveBaseURL()
	}

	return client, nil
}

// GenerateContent renders the prompt using string attributes and executes it.
func (c *Client) GenerateContent(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (string, error) {
	attrs := ai.StringsToAttr(args)
	return c.GenerateContentAttr(ctx, prompt, debug, attrs)
}

// GenerateContentAttr renders the prompt using structured attributes and
// runs the shared agent loop until the model answers without tool calls.
func (c *Client) GenerateContentAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (string, error) {
	if err := c.ensureAPIKey(); err != nil {
		return "", err
	}

	rendered, err := c.RenderPrompt(prompt, debug, attrs)
	if err != nil {
		return "", fmt.Errorf("rendering prompt: %w", err)
	}

	turn, err := c.newTurn(*rendered)
	if err != nil {
		return "", err
	}

	return llmshared.RunToolLoop(ctx, turn, turn.Handlers(), c.loopConfig(*rendered), nil)
}

// GenerateContentStream renders the prompt using string attributes and executes it with streaming.
func (c *Client) GenerateContentStream(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (ai.Stream, error) {
	attrs := ai.StringsToAttr(args)
	return c.GenerateContentAttrStream(ctx, prompt, debug, attrs)
}

// GenerateContentAttrStream renders the prompt using structured attributes and executes it with streaming.
func (c *Client) GenerateContentAttrStream(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (ai.Stream, error) {
	if err := c.ensureAPIKey(); err != nil {
		return nil, err
	}

	rendered, err := c.RenderPrompt(prompt, debug, attrs)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	// If tools are involved, fall back to the blocking shared tool loop
	// and wrap the final answer as a single-chunk stream: the streaming
	// API is not used for tool execution.
	if len(rendered.Functions) > 0 && len(rendered.Handlers) > 0 {
		turn, err := c.newTurn(*rendered)
		if err != nil {
			return nil, err
		}
		return c.BlockingLoopStream(ctx, turn, c.loopConfig(*rendered)), nil
	}

	request, err := c.buildChatRequest(*rendered)
	if err != nil {
		return nil, err
	}

	return c.StreamChat(ctx, request), nil
}

// CountTokens renders the prompt, estimates token usage using string attributes, and returns the result.
func (c *Client) CountTokens(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (*ai.TokenCount, error) {
	attrs := ai.StringsToAttr(args)
	return c.CountTokensAttr(ctx, prompt, debug, attrs)
}

// CountTokensAttr renders the prompt and estimates token usage locally —
// DeepSeek has no free counting endpoint, so no API request is made.
func (c *Client) CountTokensAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (*ai.TokenCount, error) {
	rendered, err := c.RenderPrompt(prompt, debug, attrs)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	messages, err := c.buildMessages(*rendered)
	if err != nil {
		return nil, err
	}

	total, err := countTokensForMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("counting tokens: %w", err)
	}

	tokenCount := &ai.TokenCount{
		TotalTokens: int32(total),
		InputTokens: int32(total),
	}
	c.PublishTokenCount(ctx, tokenCount)

	return tokenCount, nil
}

// GetStatus reports whether mandatory configuration is available and which model is configured.
func (c *Client) GetStatus() *ai.Status {
	model := c.Config.GetModelConfig()
	modelStr := fmt.Sprintf("%s, Temperature: %.2f, Max Tokens: %d", model.ModelName, model.Temperature, model.MaxTokens)

	if c.resolveAPIKey() == "" {
		return &ai.Status{
			Model:     modelStr,
			Backend:   "deepseek",
			Connected: false,
			Message:   "DEEPSEEK_API_KEY not configured",
		}
	}

	return &ai.Status{
		Model:     modelStr,
		Backend:   "deepseek",
		Connected: true,
		Message:   fmt.Sprintf("DeepSeek configured (endpoint: %s)", c.BaseURL),
	}
}

// loopConfig maps prompt and environment settings onto the shared
// agent-loop configuration.
func (c *Client) loopConfig(prompt ai.Prompt) llmshared.LoopConfig {
	return llmshared.NewLoopConfig(c.Config, c.EventBus, prompt, defaultMaxToolIterations)
}

// newTurn starts a chat turn. DeepSeek is text-only, so no blob hooks.
func (c *Client) newTurn(prompt ai.Prompt) (*openaicompat.Turn, error) {
	request, err := c.buildChatRequest(prompt)
	if err != nil {
		return nil, err
	}
	return c.Core.NewTurn(request, prompt.Handlers, openaicompat.TurnOptions{}), nil
}

func (c *Client) buildChatRequest(prompt ai.Prompt) (chatRequest, error) {
	modelName := c.ResolveModelName(prompt.ModelName)
	if strings.TrimSpace(modelName) == "" {
		modelName = defaultModelName
	}

	messages, err := c.buildMessages(prompt)
	if err != nil {
		return chatRequest{}, err
	}

	req := chatRequest{
		Model:    modelName,
		Messages: messages,
		Stream:   false,
	}

	c.applyGenerationConfig(&req, prompt)

	if len(prompt.Functions) > 0 {
		req.Tools = llmshared.MapFunctions(prompt.Functions, schemaToMap)
		if len(req.Tools) > 0 {
			auto := "auto"
			req.ToolChoice = &auto
		}
	}

	if prompt.ResponseSchema != nil {
		// DeepSeek's JSON mode carries no schema — the schema itself is
		// injected into the system prompt by buildMessages.
		req.ResponseFormat = &responseFormat{Type: "json_object"}
	}

	return req, nil
}

// buildMessages assembles the system and user messages. When a response
// schema is set it is always appended to the system prompt: DeepSeek's
// json_object mode enforces JSON output but knows nothing about the
// schema (and requires the word "json" to appear in the prompt).
func (c *Client) buildMessages(prompt ai.Prompt) ([]chatMessage, error) {
	var systemParts []string

	if instruction := strings.TrimSpace(prompt.Instruction); instruction != "" {
		systemParts = append(systemParts, instruction)
		if files := strings.TrimSpace(prompt.SystemPromptFiles); files != "" {
			systemParts = append(systemParts, files)
		}
		if userCtx := strings.TrimSpace(prompt.SystemPromptUserContext); userCtx != "" {
			systemParts = append(systemParts, userCtx)
		}
	}

	if prompt.ResponseSchema != nil {
		schemaJSON, err := schemaToJSON(prompt.ResponseSchema)
		if err != nil {
			return nil, fmt.Errorf("formatting response schema: %w", err)
		}
		if strings.TrimSpace(schemaJSON) != "" {
			systemParts = append(systemParts, fmt.Sprintf("You must respond with JSON matching this schema:\n%s", schemaJSON))
		}
	}

	var messages []chatMessage
	if len(systemParts) > 0 {
		messages = append(messages, chatMessage{
			Role:    "system",
			Content: newMessageContentFromText(strings.Join(systemParts, "\n\n")),
		})
	}

	messages = append(messages, c.buildUserMessage(prompt))
	return messages, nil
}

// buildUserMessage renders the user turn. DeepSeek accepts no image
// input, so prompt images are reported as textual descriptions.
func (c *Client) buildUserMessage(prompt ai.Prompt) chatMessage {
	text := strings.TrimSpace(prompt.Text)

	var notes []string
	for _, img := range prompt.Images {
		if img == nil || len(img.Data) == 0 {
			continue
		}
		mimeType := strings.TrimSpace(img.Type)
		if mimeType == "" {
			mimeType = "image/png"
		}
		notes = append(notes, fmt.Sprintf("[attached image (%s, %d bytes) could not be included: DeepSeek accepts text input only]", mimeType, len(img.Data)))
	}
	if len(notes) > 0 {
		if text != "" {
			text += "\n\n"
		}
		text += strings.Join(notes, "\n")
	}

	return chatMessage{
		Role:    "user",
		Content: newMessageContentFromText(text),
	}
}

func (c *Client) applyGenerationConfig(req *chatRequest, prompt ai.Prompt) {
	modelCfg := c.Config.GetModelConfig()

	maxTokens := prompt.MaxTokens
	if maxTokens <= 0 {
		maxTokens = modelCfg.MaxTokens
	}
	if maxTokens > 0 {
		value := int32(maxTokens)
		req.MaxTokens = &value
	}

	// deepseek-reasoner ignores sampling parameters (without erroring),
	// so they are sent unconditionally like on the other providers.
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

// ensureAPIKey resolves the API key into the core's auth token, failing
// with a non-retryable error when none is configured.
func (c *Client) ensureAPIKey() error {
	if strings.TrimSpace(c.AuthToken) != "" {
		return nil
	}
	key := c.resolveAPIKey()
	if key == "" {
		return ai.NonRetryable(fmt.Errorf("%w: please export DEEPSEEK_API_KEY (and optionally DEEPSEEK_BASE_URL)", errMissingAPIKey))
	}
	c.AuthToken = key
	return nil
}

func (c *Client) resolveAPIKey() string {
	if token := strings.TrimSpace(c.AuthToken); token != "" {
		return token
	}
	if key := strings.TrimSpace(c.Config.GetStringWithDefault("DEEPSEEK_API_KEY", "")); key != "" {
		return key
	}
	return strings.TrimSpace(c.Config.GetStringWithDefault("GENIE_DEEPSEEK_API_KEY", ""))
}

func (c *Client) resolveBaseURL() string {
	if env := strings.TrimSpace(c.Config.GetStringWithDefault("GENIE_DEEPSEEK_BASE_URL", "")); env != "" {
		return normalizeBaseURL(env)
	}
	if env := strings.TrimSpace(c.Config.GetStringWithDefault("DEEPSEEK_BASE_URL", "")); env != "" {
		return normalizeBaseURL(env)
	}
	return defaultBaseURL
}

// normalizeBaseURL trims trailing slashes. DeepSeek serves the chat
// endpoint both at the root and under /v1, so a user-supplied /v1
// suffix is kept as-is.
func normalizeBaseURL(base string) string {
	return strings.TrimRight(strings.TrimSpace(base), "/")
}
