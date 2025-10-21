package anthropic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	anthropic_sdk "github.com/anthropics/anthropic-sdk-go"
	anthropic_option "github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/fileops"
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/kcaldas/genie/pkg/template"
)

const (
	maxToolIterations  = 8
	defaultClaudeModel = "claude-3-5-sonnet-20241022"
	defaultSchemaName  = "response"
)

var (
	errMissingAPIKey        = errors.New("anthropic backend not configured")
	_                ai.Gen = (*Client)(nil)
)

type messageClient interface {
	New(ctx context.Context, body anthropic_sdk.MessageNewParams, opts ...anthropic_option.RequestOption) (*anthropic_sdk.Message, error)
	CountTokens(ctx context.Context, body anthropic_sdk.MessageCountTokensParams, opts ...anthropic_option.RequestOption) (*anthropic_sdk.MessageTokensCount, error)
}

// Option configures the Anthropic client.
type Option func(*Client)

// WithConfigManager injects a custom configuration manager (useful for tests).
func WithConfigManager(manager config.Manager) Option {
	return func(c *Client) {
		if manager != nil {
			c.config = manager
		}
	}
}

// WithFileManager injects a custom file manager.
func WithFileManager(manager fileops.Manager) Option {
	return func(c *Client) {
		if manager != nil {
			c.fileManager = manager
		}
	}
}

// WithTemplateEngine injects a custom template engine.
func WithTemplateEngine(engine template.Engine) Option {
	return func(c *Client) {
		if engine != nil {
			c.template = engine
		}
	}
}

// WithLogger injects a custom logger implementation.
func WithLogger(logger logging.Logger) Option {
	return func(c *Client) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// WithMessageClient injects a pre-built message client (primarily for tests).
func WithMessageClient(client messageClient) Option {
	return func(c *Client) {
		if client != nil {
			c.messages = client
		}
	}
}

// Client provides an ai.Gen implementation backed by Anthropic Messages API.
type Client struct {
	mu sync.Mutex

	config      config.Manager
	fileManager fileops.Manager
	template    template.Engine
	eventBus    events.EventBus
	logger      logging.Logger

	apiClient *anthropic_sdk.Client
	messages  messageClient

	initialized bool
	initErr     error
}

// NewClient builds a new Anthropic-backed ai.Gen implementation.
func NewClient(eventBus events.EventBus, opts ...Option) (ai.Gen, error) {
	client := &Client{
		config:      config.NewConfigManager(),
		fileManager: fileops.NewFileOpsManager(),
		template:    template.NewEngine(),
		eventBus:    eventBus,
		logger:      logging.NewAPILogger("anthropic"),
	}

	if client.eventBus == nil {
		client.eventBus = &events.NoOpEventBus{}
	}

	for _, opt := range opts {
		opt(client)
	}

	if client.logger == nil {
		client.logger = logging.NewAPILogger("anthropic")
	}

	return client, nil
}

// GenerateContent renders the prompt using string attributes and executes it.
func (c *Client) GenerateContent(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (string, error) {
	attrs := ai.StringsToAttr(args)
	return c.GenerateContentAttr(ctx, prompt, debug, attrs)
}

// GenerateContentAttr renders the prompt using structured attributes and executes it.
func (c *Client) GenerateContentAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (string, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return "", err
	}

	rendered, err := c.renderPrompt(prompt, debug, attrs)
	if err != nil {
		return "", fmt.Errorf("rendering prompt: %w", err)
	}

	return c.generateWithPrompt(ctx, *rendered)
}

// CountTokens renders the prompt and calls the token counting API using string attributes.
func (c *Client) CountTokens(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (*ai.TokenCount, error) {
	attrs := ai.StringsToAttr(args)
	return c.CountTokensAttr(ctx, prompt, debug, attrs)
}

// CountTokensAttr renders the prompt and calls the token counting API using structured attributes.
func (c *Client) CountTokensAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (*ai.TokenCount, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}

	rendered, err := c.renderPrompt(prompt, debug, attrs)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	params, err := c.buildCountTokensParams(*rendered)
	if err != nil {
		return nil, err
	}

	result, err := c.messages.CountTokens(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic count tokens: %w", err)
	}

	tokenCount := &ai.TokenCount{
		TotalTokens: int32(result.InputTokens),
		InputTokens: int32(result.InputTokens),
	}
	c.publishTokenCount(tokenCount)
	return tokenCount, nil
}

// GetStatus reports whether mandatory configuration is available and which model is configured.
func (c *Client) GetStatus() *ai.Status {
	model := c.config.GetModelConfig()
	modelStr := fmt.Sprintf("%s, Temperature: %.2f, Max Tokens: %d", model.ModelName, model.Temperature, model.MaxTokens)

	apiKey := strings.TrimSpace(c.config.GetStringWithDefault("ANTHROPIC_API_KEY", ""))
	if apiKey == "" {
		return &ai.Status{
			Model:     modelStr,
			Backend:   "anthropic",
			Connected: false,
			Message:   "ANTHROPIC_API_KEY not configured",
		}
	}

	message := "Anthropic configured"
	if baseURL := strings.TrimSpace(c.config.GetStringWithDefault("ANTHROPIC_BASE_URL", "")); baseURL != "" {
		message = fmt.Sprintf("Anthropic configured (custom endpoint: %s)", baseURL)
	}

	return &ai.Status{
		Model:     modelStr,
		Backend:   "anthropic",
		Connected: true,
		Message:   message,
	}
}

func (c *Client) ensureInitialized(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return c.initErr
	}

	if c.messages != nil {
		c.initialized = true
		c.initErr = nil
		return nil
	}

	apiKey := strings.TrimSpace(c.config.GetStringWithDefault("ANTHROPIC_API_KEY", ""))
	if apiKey == "" {
		c.initErr = fmt.Errorf("%w: please export ANTHROPIC_API_KEY (and optionally ANTHROPIC_BASE_URL or ANTHROPIC_AUTH_TOKEN)", errMissingAPIKey)
		return c.initErr
	}

	opts := []anthropic_option.RequestOption{
		anthropic_option.WithAPIKey(apiKey),
	}
	if baseURL := strings.TrimSpace(c.config.GetStringWithDefault("ANTHROPIC_BASE_URL", "")); baseURL != "" {
		opts = append(opts, anthropic_option.WithBaseURL(baseURL))
	}
	if authToken := strings.TrimSpace(c.config.GetStringWithDefault("ANTHROPIC_AUTH_TOKEN", "")); authToken != "" {
		opts = append(opts, anthropic_option.WithAuthToken(authToken))
	}

	client := anthropic_sdk.NewClient(opts...)
	service := client.Messages

	c.apiClient = &client
	c.messages = &service
	c.initialized = true
	c.initErr = nil
	return nil
}

func (c *Client) generateWithPrompt(ctx context.Context, prompt ai.Prompt) (string, error) {
	systemBlocks := c.buildSystemBlocks(prompt)
	messageParams, err := c.buildMessages(prompt)
	if err != nil {
		return "", err
	}

	modelName := c.resolveModelName(prompt.ModelName)
	maxTokens := prompt.MaxTokens
	if maxTokens <= 0 {
		maxTokens = c.config.GetModelConfig().MaxTokens
	}
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	params := anthropic_sdk.MessageNewParams{
		Model:     anthropic_sdk.Model(modelName),
		MaxTokens: int64(maxTokens),
		Messages:  messageParams,
	}
	if len(systemBlocks) > 0 {
		params.System = systemBlocks
	}

	c.applyGenerationConfig(&params, prompt)
	c.applyToolingConfig(&params, prompt)

	return c.executeChat(ctx, params, prompt.Handlers, systemBlocks)
}

func (c *Client) executeChat(ctx context.Context, baseParams anthropic_sdk.MessageNewParams, handlers map[string]ai.HandlerFunc, systemBlocks []anthropic_sdk.TextBlockParam) (string, error) {
	messages := append([]anthropic_sdk.MessageParam(nil), baseParams.Messages...)
	params := baseParams
	params.System = systemBlocks

	for iteration := 0; iteration < maxToolIterations; iteration++ {
		params.Messages = messages

		resp, err := c.messages.New(ctx, params)
		if err != nil {
			return "", fmt.Errorf("anthropic messages: %w", err)
		}

		c.publishUsage(resp.Usage)

		showThinking := c.config.GetBoolWithDefault("ANTHROPIC_SHOW_THINKING", false)
		responseText, toolCalls := c.parseResponse(resp, showThinking)

		if strings.TrimSpace(responseText) != "" {
			c.publishText(responseText)
		}

		messages = append(messages, resp.ToParam())

		if len(toolCalls) == 0 {
			return strings.TrimSpace(responseText), nil
		}

		if len(handlers) == 0 {
			return "", fmt.Errorf("model requested %d tool calls but no handlers were provided", len(toolCalls))
		}

		toolResultBlocks := make([]anthropic_sdk.ContentBlockParamUnion, 0, len(toolCalls))

		for _, tool := range toolCalls {
			handler := handlers[tool.Name]
			if handler == nil {
				return "", fmt.Errorf("no handler registered for tool %q", tool.Name)
			}

			var args map[string]any
			if len(tool.Input) > 0 && string(tool.Input) != "null" {
				if err := json.Unmarshal(tool.Input, &args); err != nil {
					return "", fmt.Errorf("invalid arguments for tool %q: %w", tool.Name, err)
				}
			} else {
				args = map[string]any{}
			}

			result, err := handler(ctx, args)
			if err != nil {
				return "", fmt.Errorf("handler for tool %q failed: %w", tool.Name, err)
			}

			payload, err := json.Marshal(result)
			if err != nil {
				return "", fmt.Errorf("unable to marshal response for tool %q: %w", tool.Name, err)
			}

			toolResultBlocks = append(toolResultBlocks, anthropic_sdk.NewToolResultBlock(tool.ID, string(payload), false))
		}

		messages = append(messages, anthropic_sdk.NewUserMessage(toolResultBlocks...))
	}

	return "", fmt.Errorf("exceeded maximum tool call iterations (%d) without completion", maxToolIterations)
}

func (c *Client) parseResponse(resp *anthropic_sdk.Message, showThinking bool) (string, []toolCall) {
	var textBuilder strings.Builder
	var toolCalls []toolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			if strings.TrimSpace(block.Text) != "" {
				if textBuilder.Len() > 0 {
					textBuilder.WriteString("\n")
				}
				textBuilder.WriteString(block.Text)
			}
		case "thinking":
			if showThinking && strings.TrimSpace(block.Thinking) != "" {
				notification := events.NotificationEvent{
					Message:     strings.TrimSpace(block.Thinking),
					ContentType: "thought",
				}
				c.eventBus.Publish(notification.Topic(), notification)
			}
		case "tool_use":
			toolCalls = append(toolCalls, toolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}

	return textBuilder.String(), toolCalls
}

func (c *Client) applyGenerationConfig(params *anthropic_sdk.MessageNewParams, prompt ai.Prompt) {
	modelCfg := c.config.GetModelConfig()

	if prompt.Temperature > 0 {
		params.Temperature = anthropic_sdk.Float(float64(prompt.Temperature))
	} else if modelCfg.Temperature > 0 {
		params.Temperature = anthropic_sdk.Float(float64(modelCfg.Temperature))
	}

	if prompt.TopP > 0 {
		params.TopP = anthropic_sdk.Float(float64(prompt.TopP))
	} else if modelCfg.TopP > 0 {
		params.TopP = anthropic_sdk.Float(float64(modelCfg.TopP))
	}
}

func (c *Client) applyToolingConfig(params *anthropic_sdk.MessageNewParams, prompt ai.Prompt) {
	tools := mapFunctions(prompt.Functions)
	if len(tools) > 0 {
		toolUnions := make([]anthropic_sdk.ToolUnionParam, 0, len(tools))
		for _, tool := range tools {
			if tool == nil {
				continue
			}
			copy := *tool
			toolUnions = append(toolUnions, anthropic_sdk.ToolUnionParam{OfTool: &copy})
		}
		if len(toolUnions) > 0 {
			params.Tools = toolUnions
			params.ToolChoice = anthropic_sdk.ToolChoiceUnionParam{
				OfAuto: &anthropic_sdk.ToolChoiceAutoParam{Type: constant.Auto("")},
			}
		}
	}
}

func (c *Client) buildSystemBlocks(prompt ai.Prompt) []anthropic_sdk.TextBlockParam {
	var blocks []anthropic_sdk.TextBlockParam

	if strings.TrimSpace(prompt.Instruction) != "" {
		blocks = append(blocks, anthropic_sdk.TextBlockParam{Text: prompt.Instruction})
	}

	if prompt.ResponseSchema != nil {
		schemaJSON, err := schemaToJSON(prompt.ResponseSchema)
		if err == nil && strings.TrimSpace(schemaJSON) != "" {
			instruction := fmt.Sprintf("You must respond with JSON matching this schema:\n%s", schemaJSON)
			blocks = append(blocks, anthropic_sdk.TextBlockParam{Text: instruction})
		}
	}

	return blocks
}

func (c *Client) buildMessages(prompt ai.Prompt) ([]anthropic_sdk.MessageParam, error) {
	userMessage, err := c.buildUserMessage(prompt)
	if err != nil {
		return nil, err
	}
	return []anthropic_sdk.MessageParam{userMessage}, nil
}

func (c *Client) buildUserMessage(prompt ai.Prompt) (anthropic_sdk.MessageParam, error) {
	var blocks []anthropic_sdk.ContentBlockParamUnion

	if text := strings.TrimSpace(prompt.Text); text != "" {
		blocks = append(blocks, anthropic_sdk.NewTextBlock(text))
	}

	for _, img := range prompt.Images {
		if img == nil || len(img.Data) == 0 {
			continue
		}
		mimeType := img.Type
		if strings.TrimSpace(mimeType) == "" {
			mimeType = "image/png"
		}
		blocks = append(blocks, anthropic_sdk.NewImageBlockBase64(mimeType, base64.StdEncoding.EncodeToString(img.Data)))
	}

	if len(blocks) == 0 {
		blocks = append(blocks, anthropic_sdk.NewTextBlock(""))
	}

	return anthropic_sdk.NewUserMessage(blocks...), nil
}

func (c *Client) buildCountTokensParams(prompt ai.Prompt) (anthropic_sdk.MessageCountTokensParams, error) {
	messageParams, err := c.buildMessages(prompt)
	if err != nil {
		return anthropic_sdk.MessageCountTokensParams{}, err
	}

	modelName := c.resolveModelName(prompt.ModelName)
	params := anthropic_sdk.MessageCountTokensParams{
		Model:    anthropic_sdk.Model(modelName),
		Messages: messageParams,
	}

	systemBlocks := c.buildSystemBlocks(prompt)
	if len(systemBlocks) > 0 {
		params.System = anthropic_sdk.MessageCountTokensParamsSystemUnion{
			OfTextBlockArray: systemBlocks,
		}
	}

	tools := mapFunctions(prompt.Functions)
	if len(tools) > 0 {
		countTools := make([]anthropic_sdk.MessageCountTokensToolUnionParam, 0, len(tools))
		for _, tool := range tools {
			if tool == nil {
				continue
			}
			countTools = append(countTools, anthropic_sdk.MessageCountTokensToolParamOfTool(tool.InputSchema, tool.Name))
		}
		if len(countTools) > 0 {
			params.Tools = countTools
		}
	}

	return params, nil
}

func (c *Client) resolveModelName(promptModel string) string {
	if strings.TrimSpace(promptModel) != "" {
		return promptModel
	}

	model := c.config.GetModelConfig()
	if strings.TrimSpace(model.ModelName) != "" {
		return model.ModelName
	}

	return defaultClaudeModel
}

func (c *Client) renderPrompt(prompt ai.Prompt, debug bool, attrs []ai.Attr) (*ai.Prompt, error) {
	if debug {
		if err := c.saveObjectToTmpFile(prompt, fmt.Sprintf("%s-initial-prompt.yaml", prompt.Name)); err != nil {
			return nil, err
		}
		if err := c.saveObjectToTmpFile(attrs, fmt.Sprintf("%s-attrs.yaml", prompt.Name)); err != nil {
			return nil, err
		}
	}

	rendered, err := ai.RenderPrompt(prompt, c.mapAttr(attrs))
	if err != nil {
		return nil, fmt.Errorf("rendering template: %w", err)
	}

	if debug {
		if err := c.saveObjectToTmpFile(rendered, fmt.Sprintf("%s-final-prompt.yaml", rendered.Name)); err != nil {
			return nil, err
		}
	}

	return &rendered, nil
}

func (c *Client) mapAttr(attrs []ai.Attr) map[string]string {
	result := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		result[attr.Key] = attr.Value
	}
	return result
}

func (c *Client) saveObjectToTmpFile(object interface{}, filename string) error {
	filePath := filepath.Join("tmp", filename)
	return c.fileManager.WriteObjectAsYAML(filePath, object)
}

func (c *Client) publishUsage(usage anthropic_sdk.Usage) {
	event := events.TokenCountEvent{
		InputTokens:  int32(usage.InputTokens),
		OutputTokens: int32(usage.OutputTokens),
		TotalTokens:  int32(usage.InputTokens + usage.OutputTokens),
	}
	c.eventBus.Publish(event.Topic(), event)

	if c.config.GetBoolWithDefault("GENIE_TOKEN_DEBUG", false) {
		raw, _ := json.MarshalIndent(usage, "", "  ")
		notification := events.NotificationEvent{
			Message: string(raw),
		}
		c.eventBus.Publish(notification.Topic(), notification)
	}
}

func (c *Client) publishTokenCount(tokenCount *ai.TokenCount) {
	if tokenCount == nil {
		return
	}
	event := events.TokenCountEvent{
		InputTokens:  tokenCount.InputTokens,
		OutputTokens: tokenCount.OutputTokens,
		TotalTokens:  tokenCount.TotalTokens,
	}
	c.eventBus.Publish(event.Topic(), event)
}

func (c *Client) publishText(text string) {
	notification := events.NotificationEvent{
		Message: strings.TrimSpace(text),
	}
	c.eventBus.Publish(notification.Topic(), notification)
}

func chooseSchemaName(prompt ai.Prompt) string {
	if prompt.ResponseSchema != nil && prompt.ResponseSchema.Title != "" {
		return prompt.ResponseSchema.Title
	}
	if strings.TrimSpace(prompt.Name) != "" {
		return prompt.Name
	}
	return defaultSchemaName
}

type toolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}
