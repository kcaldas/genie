package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/fileops"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/kcaldas/genie/pkg/template"
)

const (
	defaultMaxToolIterations = 200
	defaultSchemaName        = "response"
)

var (
	errMissingAPIKey        = errors.New("openai backend not configured")
	_                ai.Gen = (*Client)(nil)
)

type chatCompletionClient interface {
	New(ctx context.Context, body openai.ChatCompletionNewParams, opts ...option.RequestOption) (*openai.ChatCompletion, error)
	NewStreaming(ctx context.Context, body openai.ChatCompletionNewParams, opts ...option.RequestOption) *ssestream.Stream[openai.ChatCompletionChunk]
}

type responsesClient interface {
	New(ctx context.Context, body responses.ResponseNewParams, opts ...option.RequestOption) (*responses.Response, error)
	NewStreaming(ctx context.Context, body responses.ResponseNewParams, opts ...option.RequestOption) *ssestream.Stream[responses.ResponseStreamEventUnion]
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

// WithResponsesClient injects a custom Responses client (primarily for tests).
func WithResponsesClient(responses responsesClient) Option {
	return func(c *Client) {
		if responses != nil {
			c.responses = responses
		}
	}
}

// Client provides an ai.Gen implementation backed by OpenAI.
type Client struct {
	mu sync.Mutex

	config      config.Manager
	fileManager fileops.Manager
	template    template.Engine
	eventBus    events.EventBus
	logger      logging.Logger

	apiClient       *openai.Client
	chatCompletions chatCompletionClient
	responses       responsesClient

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

func (c *Client) GenerateContentStream(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (ai.Stream, error) {
	attrs := ai.StringsToAttr(args)
	return c.GenerateContentAttrStream(ctx, prompt, debug, attrs)
}

func (c *Client) GenerateContentAttrStream(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (ai.Stream, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}

	rendered, err := c.renderPrompt(prompt, debug, attrs)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	return c.generateWithPromptStream(ctx, *rendered)
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
	c.publishTokenCount(modelName, tokenCount)

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

	if c.chatCompletions != nil || c.responses != nil {
		c.initialized = true
		return nil
	}

	apiKey := strings.TrimSpace(c.config.GetStringWithDefault("OPENAI_API_KEY", ""))
	if apiKey == "" {
		c.initErr = ai.NonRetryable(fmt.Errorf("%w: please export OPENAI_API_KEY (and optionally OPENAI_BASE_URL or OPENAI_ORG_ID)", errMissingAPIKey))
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
	chatService := client.Chat.Completions
	responsesService := client.Responses

	c.apiClient = &client
	c.chatCompletions = &chatService
	c.responses = &responsesService
	c.initialized = true
	c.initErr = nil
	return nil
}

func (c *Client) generateWithPrompt(ctx context.Context, prompt ai.Prompt) (string, error) {
	turn, err := c.newTurn(prompt)
	if err != nil {
		return "", err
	}
	return llmshared.RunToolLoop(ctx, turn, prompt.Handlers, c.loopConfig(prompt), nil)
}

func (c *Client) newTurn(prompt ai.Prompt) (llmshared.TurnState, error) {
	modelName := c.resolveModelName(prompt.ModelName)
	if useResponsesAPI(modelName) {
		return c.newResponsesTurn(prompt, modelName)
	}
	return c.newChatTurn(prompt, modelName)
}

func (c *Client) generateWithPromptStream(ctx context.Context, prompt ai.Prompt) (ai.Stream, error) {
	turn, err := c.newTurn(prompt)
	if err != nil {
		return nil, err
	}

	streamCtx, cancel := context.WithCancel(ctx)
	ch := make(chan llmshared.StreamResult, 1)

	go func() {
		defer close(ch)
		defer llmshared.RecoverToStream(ch)
		emit := func(chunk *ai.StreamChunk) {
			select {
			case ch <- llmshared.StreamResult{Chunk: chunk}:
			case <-streamCtx.Done():
			}
		}
		if _, err := llmshared.RunToolLoop(streamCtx, turn, prompt.Handlers, c.loopConfig(prompt), emit); err != nil {
			if streamCtx.Err() != nil {
				return
			}
			select {
			case ch <- llmshared.StreamResult{Err: err}:
			case <-streamCtx.Done():
			}
		}
	}()

	return llmshared.NewChunkStream(streamCtx, cancel, ch), nil
}

// loopConfig maps prompt and environment settings onto the shared
// agent-loop configuration. Step-level retry replaces the old
// whole-turn retry middleware, so transient API failures never
// re-execute tool side effects.
func (c *Client) loopConfig(prompt ai.Prompt) llmshared.LoopConfig {
	retry := ai.GetRetryConfigFromEnv(c.config)
	cfg := llmshared.LoopConfig{
		MaxIterations:   normalizeToolIterations(prompt.MaxToolIterations),
		InputTokenLimit: llmshared.ModelInputAdmissionLimit(prompt),
		Limits:          llmshared.ToolResultLimitsFromEnv(c.config),
		Bus:             c.eventBus,
	}
	if retry.Enabled {
		cfg.StepRetries = retry.MaxRetries
		cfg.StepBackoff = retry.InitialBackoff
	}
	return cfg
}

func normalizeToolIterations(value int32) int {
	if value <= 0 {
		return defaultMaxToolIterations
	}
	return int(value)
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

func (c *Client) buildInstructions(prompt ai.Prompt) (string, error) {
	var parts []string
	if instruction := strings.TrimSpace(prompt.Instruction); instruction != "" {
		parts = append(parts, instruction)
	}
	if files := strings.TrimSpace(prompt.SystemPromptFiles); files != "" {
		parts = append(parts, files)
	}
	if userCtx := strings.TrimSpace(prompt.SystemPromptUserContext); userCtx != "" {
		parts = append(parts, userCtx)
	}

	if prompt.ResponseSchema != nil && strings.TrimSpace(prompt.Instruction) == "" {
		schemaJSON, err := schemaToJSON(prompt.ResponseSchema)
		if err != nil {
			return "", fmt.Errorf("formatting response schema: %w", err)
		}
		parts = append(parts, fmt.Sprintf("You must respond with JSON matching this schema:\n%s", schemaJSON))
	}

	return strings.Join(parts, "\n\n"), nil
}

func (c *Client) buildMessages(prompt ai.Prompt) ([]openai.ChatCompletionMessageParamUnion, []tokenMessage, error) {
	var messages []openai.ChatCompletionMessageParamUnion
	var tokenMessages []tokenMessage

	if instruction, err := c.buildInstructions(prompt); err != nil {
		return nil, nil, err
	} else if instruction != "" {
		messages = append(messages, openai.SystemMessage(instruction))
		tokenMessages = append(tokenMessages, tokenMessage{
			Role:    "system",
			Content: instruction,
		})
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

func buildImageUserMessage(img ai.BlobContent) openai.ChatCompletionMessageParamUnion {
	parts := []openai.ChatCompletionContentPartUnionParam{
		openai.TextContentPart(llmshared.DescribeBlob(img)),
		openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{URL: llmshared.BlobDataURL(img)}),
	}
	return openai.UserMessage(parts)
}

func (c *Client) renderPrompt(prompt ai.Prompt, debug bool, attrs []ai.Attr) (*ai.Prompt, error) {
	return llmshared.RenderPromptWithDebug(c.fileManager, prompt, debug, attrs)
}

func (c *Client) publishUsage(modelName string, usage openai.CompletionUsage) *ai.TokenCount {
	if usage.TotalTokens == 0 && usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return nil
	}

	// OpenAI's PromptTokens INCLUDES cached_tokens (cached is a subset, not a
	// separate bucket). Subtract so InputTokens means "uncached input" — keeps
	// the cross-provider semantics consistent with Anthropic.
	cached := int32(usage.PromptTokensDetails.CachedTokens)
	if strings.TrimSpace(modelName) == "" {
		modelName = c.resolveModelName("")
	}
	event := events.TokenCountEvent{
		Provider:             "openai",
		Model:                modelName,
		InputTokens:          int32(usage.PromptTokens) - cached,
		OutputTokens:         int32(usage.CompletionTokens),
		CachedTokens:         cached,
		CacheReadInputTokens: cached,
		TotalTokens:          int32(usage.TotalTokens),
	}
	c.eventBus.Publish(event.Topic(), event)

	if c.config.GetBoolWithDefault("GENIE_TOKEN_DEBUG", false) {
		raw, _ := json.MarshalIndent(usage, "", "  ")
		notification := events.NotificationEvent{
			Message: string(raw),
		}
		c.eventBus.Publish(notification.Topic(), notification)
	}
	return &ai.TokenCount{
		TotalTokens:  int32(usage.TotalTokens),
		InputTokens:  int32(usage.PromptTokens),
		OutputTokens: int32(usage.CompletionTokens),
	}
}

func (c *Client) publishTokenCount(modelName string, tokenCount *ai.TokenCount) {
	if tokenCount == nil {
		return
	}
	if strings.TrimSpace(modelName) == "" {
		modelName = c.resolveModelName("")
	}
	event := events.TokenCountEvent{
		Provider:     "openai",
		Model:        modelName,
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

// allowsSamplingParams reports whether the model still accepts temperature and
// top_p. OpenAI retired them from gpt-5 onward: a request carrying temperature
// returns 400 "Unsupported parameter: 'temperature' is not supported with this
// model", on /v1/responses and chat/completions alike, so the turn fails
// outright rather than degrading.
//
// The test is the generation, not a name list, so a family released tomorrow
// is covered — and it shares gptGeneration with useResponsesAPI, so the two
// cannot drift apart. Models that are not OpenAI's own (OpenAI-compatible
// endpoints reached through OPENAI_BASE_URL) keep their previous behaviour:
// this is about OpenAI's generations, not about every server speaking its
// protocol.
func allowsSamplingParams(model string) bool {
	if version, ok := gptGeneration(model); ok {
		return version < 5
	}
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

func useResponsesAPI(model string) bool {
	version, ok := gptGeneration(model)
	return ok && version >= 5
}

// gptGeneration parses the leading version out of a gpt-* model id:
// "gpt-5.6-luna" is 5.6, "gpt-4o" is 4, "o4-mini" is not a gpt model at all.
func gptGeneration(model string) (float64, bool) {
	model = strings.ToLower(strings.TrimSpace(model))
	rest, found := strings.CutPrefix(model, "gpt-")
	if !found {
		return 0, false
	}
	end := 0
	for end < len(rest) && (rest[end] == '.' || (rest[end] >= '0' && rest[end] <= '9')) {
		end++
	}
	version, err := strconv.ParseFloat(strings.TrimSuffix(rest[:end], "."), 64)
	if err != nil {
		return 0, false
	}
	return version, true
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
