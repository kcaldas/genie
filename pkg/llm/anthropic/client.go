package anthropic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	anthropic_sdk "github.com/anthropics/anthropic-sdk-go"
	anthropic_option "github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"

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
	defaultClaudeModel       = "claude-3-5-sonnet-20241022"
	defaultSchemaName        = "response"

	// promptCacheConfigKey gates Anthropic prompt caching. Default ON; set to
	// "false" to disable if a cache marker ever causes problems.
	promptCacheConfigKey = "ANTHROPIC_PROMPT_CACHE"

	// promptCacheTTLKey overrides the cache TTL. Valid values: "5m", "1h".
	// Default "1h" — Mutiro is async messaging, gaps between turns are
	// regularly tens of minutes, so 5m would be cold most of the time. The
	// 2x write cost (vs 1.25x for 5m) breaks even after ~2 reads.
	promptCacheTTLKey = "ANTHROPIC_PROMPT_CACHE_TTL"
)

// promptCachingEnabled reports whether to attach cache_control markers to
// Anthropic system prompts and tool definitions. Defaults to true.
func (c *Client) promptCachingEnabled() bool {
	return c.config.GetBoolWithDefault(promptCacheConfigKey, true)
}

// newCacheMarker builds a cache_control marker honouring the configured TTL.
// Defaults to 1h, which fits async messaging traffic where conversations have
// minute-to-hour gaps between turns.
func (c *Client) newCacheMarker() anthropic_sdk.CacheControlEphemeralParam {
	ttl := c.config.GetStringWithDefault(promptCacheTTLKey, "1h")
	if ttl == "5m" {
		return anthropic_sdk.CacheControlEphemeralParam{TTL: anthropic_sdk.CacheControlEphemeralTTLTTL5m}
	}
	return anthropic_sdk.CacheControlEphemeralParam{TTL: anthropic_sdk.CacheControlEphemeralTTLTTL1h}
}

var (
	errMissingAPIKey        = errors.New("anthropic backend not configured")
	errEmptyResponse        = errors.New("anthropic returned an empty response")
	_                ai.Gen = (*Client)(nil)
)

type messageClient interface {
	New(ctx context.Context, body anthropic_sdk.MessageNewParams, opts ...anthropic_option.RequestOption) (*anthropic_sdk.Message, error)
	CountTokens(ctx context.Context, body anthropic_sdk.MessageCountTokensParams, opts ...anthropic_option.RequestOption) (*anthropic_sdk.MessageTokensCount, error)
	NewStreaming(ctx context.Context, body anthropic_sdk.MessageNewParams, opts ...anthropic_option.RequestOption) *ssestream.Stream[anthropic_sdk.MessageStreamEventUnion]
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
	c.publishTokenCount(c.resolveModelName(prompt.ModelName), tokenCount)
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
		c.initErr = ai.NonRetryable(fmt.Errorf("%w: please export ANTHROPIC_API_KEY (and optionally ANTHROPIC_BASE_URL or ANTHROPIC_AUTH_TOKEN)", errMissingAPIKey))
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
	opts = append(opts, anthropic_option.WithHeaderAdd(ai.ClientHeaderName, ai.ClientHeaderValue))

	client := anthropic_sdk.NewClient(opts...)
	service := client.Messages

	c.apiClient = &client
	c.messages = &service
	c.initialized = true
	c.initErr = nil
	return nil
}

// buildParams assembles the request template for one turn: model, max
// tokens, system blocks (with cache markers), the initial user message,
// and generation/tooling config. The turnState grows Messages from there.
func (c *Client) buildParams(prompt ai.Prompt) (anthropic_sdk.MessageNewParams, error) {
	systemBlocks := c.buildSystemBlocks(prompt)
	messageParams, err := c.buildMessages(prompt)
	if err != nil {
		return anthropic_sdk.MessageNewParams{}, err
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

	return params, nil
}

func (c *Client) generateWithPrompt(ctx context.Context, prompt ai.Prompt) (string, error) {
	turn, err := c.newTurn(prompt)
	if err != nil {
		return "", err
	}
	return llmshared.RunToolLoop(ctx, turn, prompt.Handlers, c.loopConfig(prompt), nil)
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
// agent-loop configuration. Step-level retry wraps a single model
// request, so transient API failures never re-execute tool side effects.
func (c *Client) loopConfig(prompt ai.Prompt) llmshared.LoopConfig {
	retry := ai.GetRetryConfigFromEnv(c.config)
	cfg := llmshared.LoopConfig{
		MaxIterations: normalizeToolIterations(prompt.MaxToolIterations),
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
	temp := prompt.Temperature
	topP := prompt.TopP

	if temp > 0 && topP > 0 {
		c.logger.Debug("temperature and top_p are both set; using temperature only", "model", params.Model)
		topP = 0
	}

	if temp > 0 {
		params.Temperature = anthropic_sdk.Float(float64(temp))
	} else if topP > 0 {
		params.TopP = anthropic_sdk.Float(float64(topP))
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
			// No cache marker on tools intentionally. Tool schemas almost
			// always change together with the system instructions that
			// document them, so a tools-only cache rarely hits in isolation.
			// Marking the system block is enough — that cache covers the
			// tools prefix anyway since markers cache the WHOLE prefix up
			// to the marker. Saves a marker slot for a more useful one
			// (e.g. SystemPromptSuffix below).
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

	// Three system-block layout, each with its own cache marker.
	//
	//   block A (main Instruction):     persona + agent_instructions + tools
	//                                   → AGENT-WIDE: byte-identical across
	//                                     all conversations of the same agent
	//                                   → cache shareable across conversations
	//
	//   block C (SystemPromptFiles):    tool-read files accumulator
	//                                   → per-conversation in general but
	//                                     SHAREABLE across users with the
	//                                     same files (shared allow_dirs)
	//
	//   block B (SystemPromptUserContext): MEMORY.md + working memory + project
	//                                   → per-user / per-conversation
	//
	// Order is [A][C][B] so that:
	//   - A cache is shareable across all conversations of the agent
	//   - C cache is shareable across users when files match (B is downstream
	//     so memory differences don't pollute the C cache)
	//   - memory_write only invalidates B (not A or C)
	//   - readFile invalidates C and B (B is downstream of C)
	//
	// For Mutiro's chat-heavy positioning this trade-off favors memory_write
	// over readFile (memory writes are common; readFile is rare in chat agents).
	markersEnabled := c.promptCachingEnabled() && !prompt.DisableCache
	addMarker := func(blockIdx int) {
		if markersEnabled && blockIdx >= 0 && blockIdx < len(blocks) {
			blocks[blockIdx].CacheControl = c.newCacheMarker()
		}
	}

	// Marker A: end of main Instruction (block index = len-1 right now).
	if len(blocks) > 0 {
		addMarker(len(blocks) - 1)
	}

	// Marker C: end of files block (if any).
	if files := strings.TrimSpace(prompt.SystemPromptFiles); files != "" {
		blocks = append(blocks, anthropic_sdk.TextBlockParam{Text: prompt.SystemPromptFiles})
		addMarker(len(blocks) - 1)
	}

	// Marker B: end of user-context block (if any).
	if userCtx := strings.TrimSpace(prompt.SystemPromptUserContext); userCtx != "" {
		blocks = append(blocks, anthropic_sdk.TextBlockParam{Text: prompt.SystemPromptUserContext})
		addMarker(len(blocks) - 1)
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
			// Tools no longer carry a cache marker (the system block marker
			// already covers the tools prefix). Mirror that here so the
			// estimator agrees with the real request shape.
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
	return llmshared.RenderPromptWithDebug(c.fileManager, prompt, debug, attrs)
}

func (c *Client) publishUsage(modelName string, usage anthropic_sdk.Usage) {
	// Anthropic's InputTokens already excludes cached reads/writes; total billable
	// input is InputTokens + CacheCreationInputTokens + CacheReadInputTokens.
	cachedRead := int32(usage.CacheReadInputTokens)
	cachedWrite := int32(usage.CacheCreationInputTokens)
	totalInput := int32(usage.InputTokens) + cachedRead + cachedWrite
	if strings.TrimSpace(modelName) == "" {
		modelName = c.resolveModelName("")
	}
	event := events.TokenCountEvent{
		Provider:                 "anthropic",
		Model:                    modelName,
		InputTokens:              int32(usage.InputTokens),
		OutputTokens:             int32(usage.OutputTokens),
		CachedTokens:             cachedRead,
		CacheCreationInputTokens: cachedWrite,
		CacheReadInputTokens:     cachedRead,
		TotalTokens:              totalInput + int32(usage.OutputTokens),
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

func (c *Client) publishTokenCount(modelName string, tokenCount *ai.TokenCount) {
	if tokenCount == nil {
		return
	}
	if strings.TrimSpace(modelName) == "" {
		modelName = c.resolveModelName("")
	}
	event := events.TokenCountEvent{
		Provider:     "anthropic",
		Model:        modelName,
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
