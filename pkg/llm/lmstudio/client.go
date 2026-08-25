package lmstudio

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
	defaultBaseURL           = "http://127.0.0.1:1234"
)

var _ ai.Gen = (*Client)(nil)

// Option configures the LM Studio client.
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

// WithBaseURL overrides the LM Studio base URL.
func WithBaseURL(baseURL string) Option {
	return func(c *llmshared.LocalClientCore) {
		if strings.TrimSpace(baseURL) != "" {
			c.BaseURL = ensureV1Suffix(baseURL)
		}
	}
}

// Client provides an ai.Gen implementation backed by the LM Studio REST
// API, a thin configuration layer over the shared OpenAI-compat core.
type Client struct {
	openaicompat.Core
}

// NewClient creates a new LM Studio-backed ai.Gen implementation.
func NewClient(eventBus events.EventBus, opts ...Option) (ai.Gen, error) {
	client := &Client{Core: openaicompat.NewCore("lmstudio", eventBus)}

	for _, opt := range opts {
		opt(&client.LocalClientCore)
	}

	if strings.TrimSpace(client.BaseURL) == "" {
		client.BaseURL = client.resolveBaseURL()
	}

	if strings.TrimSpace(client.BaseURL) == "" {
		return nil, errors.New("lm studio base URL not configured")
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
	rendered, err := c.RenderPrompt(prompt, debug, attrs)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	// If tools are involved, fall back to the blocking shared tool loop
	// and wrap the final answer as a single-chunk stream: LM Studio's
	// streaming API is not used for tool execution.
	if len(rendered.Functions) > 0 && len(rendered.Handlers) > 0 {
		turn, err := c.newTurn(*rendered)
		if err != nil {
			return nil, err
		}
		return c.BlockingLoopStream(ctx, turn, c.loopConfig(*rendered)), nil
	}

	request, err := c.buildChatRequest(*rendered, normalMode)
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

// CountTokensAttr renders the prompt, estimates token usage via a
// zero-completion request against the local server, and returns the result.
func (c *Client) CountTokensAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (*ai.TokenCount, error) {
	rendered, err := c.RenderPrompt(prompt, debug, attrs)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	request, err := c.buildChatRequest(*rendered, countTokensMode)
	if err != nil {
		return nil, err
	}

	response, err := c.SendChat(ctx, request)
	if err != nil {
		return nil, err
	}

	tokenCount := response.Usage.TokenCount()
	c.PublishTokenCount(ctx, tokenCount)

	return tokenCount, nil
}

// GetStatus reports the configured model and target endpoint.
func (c *Client) GetStatus() *ai.Status {
	model := c.Config.GetModelConfig()
	modelStr := fmt.Sprintf("%s, Temperature: %.2f, Max Tokens: %d", model.ModelName, model.Temperature, model.MaxTokens)

	message := fmt.Sprintf("LM Studio configured (endpoint: %s)", c.BaseURL)
	return &ai.Status{
		Model:     modelStr,
		Backend:   "lmstudio",
		Connected: true,
		Message:   message,
	}
}

// loopConfig maps prompt and environment settings onto the shared
// agent-loop configuration.
func (c *Client) loopConfig(prompt ai.Prompt) llmshared.LoopConfig {
	return llmshared.NewLoopConfig(c.Config, c.EventBus, prompt, defaultMaxToolIterations)
}

// newTurn starts a chat turn. Tool-result images are appended as
// data-URL user messages when the model supports image input.
func (c *Client) newTurn(prompt ai.Prompt) (*openaicompat.Turn, error) {
	request, err := c.buildChatRequest(prompt, normalMode)
	if err != nil {
		return nil, err
	}

	return c.Core.NewTurn(request, prompt.Handlers, openaicompat.TurnOptions{
		SupportsBlob: llmshared.SupportsBlobForModel(prompt.ModelCapabilities, llmshared.SupportsImagesOnly),
		BlobMessage:  buildImageUserMessage,
	}), nil
}

func buildImageUserMessage(img ai.BlobContent) chatMessage {
	parts := []contentPart{
		{Type: "text", Text: llmshared.DescribeBlob(img)},
		{Type: "image_url", ImageURL: &imageURL{URL: llmshared.BlobDataURL(img)}},
	}
	return chatMessage{
		Role:    "user",
		Content: newMessageContent(parts),
	}
}

func (c *Client) buildChatRequest(prompt ai.Prompt, mode requestMode) (chatRequest, error) {
	modelName := c.ResolveModelName(prompt.ModelName)
	if strings.TrimSpace(modelName) == "" {
		return chatRequest{}, errors.New("no LM Studio model configured")
	}

	messages := c.buildMessages(prompt)

	req := chatRequest{
		Model:    modelName,
		Messages: messages,
		Stream:   false,
	}

	c.applyGenerationConfig(&req, prompt, mode)

	if len(prompt.Functions) > 0 {
		req.Tools = llmshared.MapFunctions(prompt.Functions, schemaToMap)
		if len(req.Tools) > 0 {
			auto := "auto"
			req.ToolChoice = &auto
		}
	}

	if prompt.ResponseSchema != nil {
		format, err := schemaToResponseFormat(prompt)
		if err != nil {
			return chatRequest{}, err
		}
		req.ResponseFormat = format
	}

	return req, nil
}

func (c *Client) buildMessages(prompt ai.Prompt) []chatMessage {
	var messages []chatMessage

	if instruction := strings.TrimSpace(prompt.Instruction); instruction != "" {
		if files := strings.TrimSpace(prompt.SystemPromptFiles); files != "" {
			instruction = instruction + "\n\n" + files
		}
		if userCtx := strings.TrimSpace(prompt.SystemPromptUserContext); userCtx != "" {
			instruction = instruction + "\n\n" + userCtx
		}
		messages = append(messages, chatMessage{
			Role:    "system",
			Content: newMessageContentFromText(instruction),
		})
	}

	if prompt.ResponseSchema != nil && strings.TrimSpace(prompt.Instruction) == "" {
		schemaJSON, err := schemaToJSON(prompt.ResponseSchema)
		if err == nil && strings.TrimSpace(schemaJSON) != "" {
			instruction := fmt.Sprintf("You must respond with JSON matching this schema:\n%s", schemaJSON)
			messages = append(messages, chatMessage{
				Role:    "system",
				Content: newMessageContentFromText(instruction),
			})
		}
	}

	userMessage := c.buildUserMessage(prompt)
	messages = append(messages, userMessage)

	return messages
}

func (c *Client) buildUserMessage(prompt ai.Prompt) chatMessage {
	text := strings.TrimSpace(prompt.Text)
	var parts []contentPart

	if text != "" {
		parts = append(parts, contentPart{Type: "text", Text: text})
	}

	for _, img := range prompt.Images {
		if img == nil || len(img.Data) == 0 {
			continue
		}
		dataURL := llmshared.EncodeImageDataURL(img)
		if dataURL == "" {
			continue
		}
		parts = append(parts, contentPart{
			Type:     "image_url",
			ImageURL: &imageURL{URL: dataURL},
		})
	}

	if len(parts) == 0 {
		return chatMessage{
			Role:    "user",
			Content: newMessageContentFromText(""),
		}
	}

	if len(parts) == 1 && parts[0].Type == "text" {
		return chatMessage{
			Role:    "user",
			Content: newMessageContentFromText(parts[0].Text),
		}
	}

	return chatMessage{
		Role:    "user",
		Content: newMessageContent(parts),
	}
}

func (c *Client) applyGenerationConfig(req *chatRequest, prompt ai.Prompt, mode requestMode) {
	modelCfg := c.Config.GetModelConfig()

	maxTokens := prompt.MaxTokens
	if maxTokens <= 0 {
		maxTokens = modelCfg.MaxTokens
	}
	if mode == countTokensMode {
		maxTokens = 0
	}
	if maxTokens > 0 || mode == countTokensMode {
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

func (c *Client) resolveBaseURL() string {
	if env := strings.TrimSpace(c.Config.GetStringWithDefault("GENIE_LMSTUDIO_BASE_URL", "")); env != "" {
		return ensureV1Suffix(env)
	}
	if env := strings.TrimSpace(c.Config.GetStringWithDefault("LMSTUDIO_BASE_URL", "")); env != "" {
		return ensureV1Suffix(env)
	}
	if env := strings.TrimSpace(c.Config.GetStringWithDefault("LM_STUDIO_BASE_URL", "")); env != "" {
		return ensureV1Suffix(env)
	}
	return ensureV1Suffix(defaultBaseURL)
}

func ensureV1Suffix(base string) string {
	base = strings.TrimRight(base, "/")
	if base == "" {
		return ""
	}
	if strings.HasSuffix(base, "/v1") {
		return base
	}
	return base + "/v1"
}
