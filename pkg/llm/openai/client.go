package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/fileops"
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/kcaldas/genie/pkg/template"
)

const (
	maxToolIterations = 8
	defaultSchemaName = "response"
)

var (
	errMissingAPIKey        = errors.New("openai backend not configured")
	_                ai.Gen = (*Client)(nil)
)

type chatCompletionClient interface {
	New(ctx context.Context, body openai.ChatCompletionNewParams, opts ...option.RequestOption) (*openai.ChatCompletion, error)
}

// Option configures the OpenAI client.
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

// WithChatClient injects a custom Chat Completions client (primarily for tests).
func WithChatClient(chat chatCompletionClient) Option {
	return func(c *Client) {
		if chat != nil {
			c.chatCompletions = chat
		}
	}
}

// Client provides an ai.Gen implementation backed by OpenAI Chat Completions.
type Client struct {
	mu sync.Mutex

	config      config.Manager
	fileManager fileops.Manager
	template    template.Engine
	eventBus    events.EventBus
	logger      logging.Logger

	apiClient       *openai.Client
	chatCompletions chatCompletionClient

	initialized bool
	initErr     error
}

// NewClient builds a new OpenAI-backed ai.Gen implementation.
func NewClient(eventBus events.EventBus, opts ...Option) (ai.Gen, error) {
	client := &Client{
		config:      config.NewConfigManager(),
		fileManager: fileops.NewFileOpsManager(),
		template:    template.NewEngine(),
		eventBus:    eventBus,
		logger:      logging.NewAPILogger("openai"),
	}

	if client.eventBus == nil {
		client.eventBus = &events.NoOpEventBus{}
	}

	for _, opt := range opts {
		opt(client)
	}

	if client.logger == nil {
		client.logger = logging.NewAPILogger("openai")
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

// CountTokens renders the prompt, counts the estimated token usage with string attributes, and returns the result.
func (c *Client) CountTokens(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (*ai.TokenCount, error) {
	attrs := ai.StringsToAttr(args)
	return c.CountTokensAttr(ctx, prompt, debug, attrs)
}

// CountTokensAttr renders the prompt, counts the estimated token usage with structured attributes, and returns the result.
func (c *Client) CountTokensAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (*ai.TokenCount, error) {
	rendered, err := c.renderPrompt(prompt, debug, attrs)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	modelName := c.resolveModelName(rendered.ModelName)
	_, tokenMessages, err := c.buildMessages(*rendered)
	if err != nil {
		return nil, err
	}

	total, err := countTokensForMessages(tokenMessages, modelName)
	if err != nil {
		return nil, fmt.Errorf("counting tokens: %w", err)
	}

	tokenCount := &ai.TokenCount{
		TotalTokens: int32(total),
		InputTokens: int32(total),
	}
	c.publishTokenCount(tokenCount)

	return tokenCount, nil
}

// GetStatus reports whether mandatory configuration is available and which model is configured.
func (c *Client) GetStatus() *ai.Status {
	model := c.config.GetModelConfig()
	modelStr := fmt.Sprintf("%s, Temperature: %.2f, Max Tokens: %d", model.ModelName, model.Temperature, model.MaxTokens)

	apiKey := strings.TrimSpace(c.config.GetStringWithDefault("OPENAI_API_KEY", ""))
	if apiKey == "" {
		return &ai.Status{
			Model:     modelStr,
			Backend:   "openai",
			Connected: false,
			Message:   "OPENAI_API_KEY not configured",
		}
	}

	message := "OpenAI configured"
	if baseURL := strings.TrimSpace(c.config.GetStringWithDefault("OPENAI_BASE_URL", "")); baseURL != "" {
		message = fmt.Sprintf("OpenAI configured (custom endpoint: %s)", baseURL)
	}

	return &ai.Status{
		Model:     modelStr,
		Backend:   "openai",
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

	if c.chatCompletions != nil {
		c.initialized = true
		return nil
	}

	apiKey := strings.TrimSpace(c.config.GetStringWithDefault("OPENAI_API_KEY", ""))
	if apiKey == "" {
		c.initErr = fmt.Errorf("%w: please export OPENAI_API_KEY (and optionally OPENAI_BASE_URL or OPENAI_ORG_ID)", errMissingAPIKey)
		return c.initErr
	}

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if baseURL := strings.TrimSpace(c.config.GetStringWithDefault("OPENAI_BASE_URL", "")); baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	if orgID := strings.TrimSpace(c.config.GetStringWithDefault("OPENAI_ORG_ID", "")); orgID != "" {
		opts = append(opts, option.WithOrganization(orgID))
	}
	if project := strings.TrimSpace(c.config.GetStringWithDefault("OPENAI_PROJECT_ID", "")); project != "" {
		opts = append(opts, option.WithProject(project))
	}
	opts = append(opts, option.WithHeaderAdd(ai.ClientHeaderName, ai.ClientHeaderValue))

	client := openai.NewClient(opts...)
	service := client.Chat.Completions

	c.apiClient = &client
	c.chatCompletions = &service
	c.initialized = true
	c.initErr = nil
	return nil
}

func (c *Client) generateWithPrompt(ctx context.Context, prompt ai.Prompt) (string, error) {
	modelName := c.resolveModelName(prompt.ModelName)
	messages, _, err := c.buildMessages(prompt)
	if err != nil {
		return "", err
	}

	params := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(modelName),
		Messages: messages,
	}
	c.applyGenerationConfig(&params, prompt)

	return c.executeChat(ctx, params, prompt.Handlers)
}

func (c *Client) resolveModelName(promptModel string) string {
	if strings.TrimSpace(promptModel) != "" {
		return promptModel
	}

	model := c.config.GetModelConfig()
	if strings.TrimSpace(model.ModelName) != "" {
		return model.ModelName
	}

	return string(shared.ChatModelGPT4oMini)
}

func (c *Client) buildMessages(prompt ai.Prompt) ([]openai.ChatCompletionMessageParamUnion, []tokenMessage, error) {
	var messages []openai.ChatCompletionMessageParamUnion
	var tokenMessages []tokenMessage

	if instruction := strings.TrimSpace(prompt.Instruction); instruction != "" {
		messages = append(messages, openai.SystemMessage(instruction))
		tokenMessages = append(tokenMessages, tokenMessage{
			Role:    "system",
			Content: instruction,
		})
	}

	if prompt.ResponseSchema != nil {
		schemaJSON, err := schemaToJSON(prompt.ResponseSchema)
		if err != nil {
			return nil, nil, fmt.Errorf("formatting response schema: %w", err)
		}
		if prompt.Instruction == "" {
			instruction := fmt.Sprintf("You must respond with JSON matching this schema:\n%s", schemaJSON)
			messages = append(messages, openai.SystemMessage(instruction))
			tokenMessages = append(tokenMessages, tokenMessage{
				Role:    "system",
				Content: instruction,
			})
		}
	}

	userMessage, tokenMsg := c.buildUserMessage(prompt)
	messages = append(messages, userMessage)
	tokenMessages = append(tokenMessages, tokenMsg)

	return messages, tokenMessages, nil
}

func (c *Client) buildUserMessage(prompt ai.Prompt) (openai.ChatCompletionMessageParamUnion, tokenMessage) {
	text := strings.TrimSpace(prompt.Text)
	var parts []openai.ChatCompletionContentPartUnionParam
	var textualParts []string

	if text != "" {
		parts = append(parts, openai.TextContentPart(text))
		textualParts = append(textualParts, text)
	}

	for _, img := range prompt.Images {
		if img == nil || len(img.Data) == 0 {
			continue
		}
		mimeType := img.Type
		if strings.TrimSpace(mimeType) == "" {
			mimeType = "image/png"
		}
		dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(img.Data))
		parts = append(parts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
			URL: dataURL,
		}))
	}

	if len(parts) == 0 {
		return openai.UserMessage(""), tokenMessage{Role: "user"}
	}

	if len(parts) == 1 && textualParts != nil && len(textualParts) == 1 {
		return openai.UserMessage(textualParts[0]), tokenMessage{
			Role:    "user",
			Content: textualParts[0],
		}
	}

	return openai.UserMessage(parts), tokenMessage{
		Role:    "user",
		Content: strings.Join(textualParts, "\n"),
	}
}

func (c *Client) applyGenerationConfig(params *openai.ChatCompletionNewParams, prompt ai.Prompt) {
	modelCfg := c.config.GetModelConfig()
	targetModel := string(params.Model)
	allowSampling := allowsSamplingParams(targetModel)

	maxTokens := prompt.MaxTokens
	if maxTokens <= 0 {
		maxTokens = modelCfg.MaxTokens
	}
	if maxTokens > 0 {
		params.MaxCompletionTokens = openai.Int(int64(maxTokens))
	}

	if allowSampling {
		temperature := prompt.Temperature
		if temperature <= 0 {
			temperature = modelCfg.Temperature
		}
		if temperature > 0 {
			params.Temperature = openai.Float(float64(temperature))
			topP := prompt.TopP
			if supportsTopP(targetModel) && topP > 0 && math.Abs(float64(topP)-1.0) > 1e-6 {
				params.TopP = openai.Float(float64(topP))
			} else if topP > 0 && math.Abs(float64(topP)-1.0) > 1e-6 {
				c.logger.Debug("top_p not supported for model; using default", "model", targetModel)
			}
		}
	} else {
		if prompt.Temperature > 0 && prompt.Temperature != 1.0 {
			c.logger.Debug("temperature not supported for model; using default", "model", targetModel)
		}
		if prompt.TopP > 0 && prompt.TopP != 1.0 {
			c.logger.Debug("top_p not supported for model; using default", "model", targetModel)
		}
	}

	if len(prompt.Functions) > 0 {
		params.Tools = mapFunctions(prompt.Functions)
		params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{
			OfAuto: openai.String("auto"),
		}
	}

	if prompt.ResponseSchema != nil {
		schema := schemaToMap(prompt.ResponseSchema)
		params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   chooseSchemaName(prompt),
					Schema: schema,
					Strict: openai.Bool(true),
				},
			},
		}
	}
}

func (c *Client) executeChat(ctx context.Context, baseParams openai.ChatCompletionNewParams, handlers map[string]ai.HandlerFunc) (string, error) {
	messages := append([]openai.ChatCompletionMessageParamUnion(nil), baseParams.Messages...)
	params := baseParams

	for iteration := 0; iteration < maxToolIterations; iteration++ {
		params.Messages = messages

		resp, err := c.chatCompletions.New(ctx, params)
		if err != nil {
			return "", fmt.Errorf("openai chat completion: %w", err)
		}

		c.publishUsage(resp.Usage)

		if len(resp.Choices) == 0 {
			return "", errors.New("openai chat completion returned no choices")
		}

		choice := resp.Choices[0]
		assistantMessage := choice.Message

		if content := strings.TrimSpace(assistantMessage.Content); content != "" {
			notification := events.NotificationEvent{
				Message: content,
			}
			c.eventBus.Publish(notification.Topic(), notification)
		}

		messages = append(messages, assistantMessage.ToParam())

		if len(assistantMessage.ToolCalls) == 0 {
			response := strings.TrimSpace(assistantMessage.Content)
			if response == "" {
				return "", errors.New("openai returned an empty response")
			}
			return response, nil
		}

		if len(handlers) == 0 {
			return "", fmt.Errorf("model requested %d tool calls but no handlers were provided", len(assistantMessage.ToolCalls))
		}

		toolMessages, err := c.executeToolCalls(ctx, assistantMessage.ToolCalls, handlers)
		if err != nil {
			return "", err
		}
		messages = append(messages, toolMessages...)
	}

	return "", fmt.Errorf("exceeded maximum tool call iterations (%d) without completion", maxToolIterations)
}

func (c *Client) executeToolCalls(ctx context.Context, calls []openai.ChatCompletionMessageToolCall, handlers map[string]ai.HandlerFunc) ([]openai.ChatCompletionMessageParamUnion, error) {
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(calls))

	for _, call := range calls {
		handler := handlers[call.Function.Name]
		if handler == nil {
			return nil, fmt.Errorf("no handler registered for function %q", call.Function.Name)
		}

		var args map[string]any
		if strings.TrimSpace(call.Function.Arguments) != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("invalid arguments for function %q: %w", call.Function.Name, err)
			}
		} else {
			args = map[string]any{}
		}

		result, err := handler(ctx, args)
		if err != nil {
			return nil, fmt.Errorf("handler for function %q failed: %w", call.Function.Name, err)
		}

		payload, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal response for function %q: %w", call.Function.Name, err)
		}

		messages = append(messages, openai.ToolMessage(string(payload), call.ID))
	}

	return messages, nil
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

func (c *Client) publishUsage(usage openai.CompletionUsage) {
	if usage.TotalTokens == 0 && usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return
	}

	event := events.TokenCountEvent{
		InputTokens:  int32(usage.PromptTokens),
		OutputTokens: int32(usage.CompletionTokens),
		TotalTokens:  int32(usage.TotalTokens),
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

func chooseSchemaName(prompt ai.Prompt) string {
	if prompt.ResponseSchema != nil && prompt.ResponseSchema.Title != "" {
		return prompt.ResponseSchema.Title
	}
	if strings.TrimSpace(prompt.Name) != "" {
		return prompt.Name
	}
	return defaultSchemaName
}

type tokenMessage struct {
	Role    string
	Content string
	Name    string
}

func allowsSamplingParams(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(model, "o1"),
		strings.HasPrefix(model, "o3"),
		strings.HasPrefix(model, "o4"):
		return false
	default:
		return true
	}
}

func supportsTopP(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(model, "gpt-4o"),
		strings.HasPrefix(model, "gpt-4-turbo"),
		strings.HasPrefix(model, "gpt-4-"),
		strings.HasPrefix(model, "gpt-3.5"):
		return true
	default:
		return false
	}
}
