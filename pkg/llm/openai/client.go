package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared"
	openai_constant "github.com/openai/openai-go/shared/constant"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/fileops"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/kcaldas/genie/pkg/llm/shared/toolpayload"
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

	return c.executeChat(ctx, params, prompt.Handlers, prompt.MaxToolIterations)
}

func (c *Client) generateWithPromptStream(ctx context.Context, prompt ai.Prompt) (ai.Stream, error) {
	modelName := c.resolveModelName(prompt.ModelName)
	messages, _, err := c.buildMessages(prompt)
	if err != nil {
		return nil, err
	}

	params := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(modelName),
		Messages: messages,
	}
	c.applyGenerationConfig(&params, prompt)

	streamCtx, cancel := context.WithCancel(ctx)
	ch := make(chan llmshared.StreamResult, 1)

	go c.runStreamingChat(streamCtx, ch, params, prompt.Handlers, prompt.MaxToolIterations)

	return llmshared.NewChunkStream(cancel, ch), nil
}

func (c *Client) runStreamingChat(ctx context.Context, ch chan<- llmshared.StreamResult, baseParams openai.ChatCompletionNewParams, handlers map[string]ai.HandlerFunc, maxIterations int32) {
	defer close(ch)

	messages := append([]openai.ChatCompletionMessageParamUnion(nil), baseParams.Messages...)
	params := baseParams

	limit := int(maxIterations)
	if limit <= 0 {
		limit = defaultMaxToolIterations
	}

	for iteration := 0; iteration < limit; iteration++ {
		params.Messages = messages

		done, nextMessages, err := c.streamChatStep(ctx, ch, params, handlers)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			select {
			case ch <- llmshared.StreamResult{Err: err}:
			case <-ctx.Done():
			}
			return
		}

		if done {
			return
		}

		messages = append(messages, nextMessages...)
	}

	select {
	case ch <- llmshared.StreamResult{Err: fmt.Errorf("exceeded maximum tool call iterations (%d) without completion", limit)}:
	case <-ctx.Done():
	}
}

type toolCallState struct {
	id        string
	name      string
	arguments strings.Builder
}

func (c *Client) streamChatStep(ctx context.Context, ch chan<- llmshared.StreamResult, params openai.ChatCompletionNewParams, handlers map[string]ai.HandlerFunc) (bool, []openai.ChatCompletionMessageParamUnion, error) {
	stream := c.chatCompletions.NewStreaming(ctx, params)
	defer stream.Close()

	var assistantBuilder strings.Builder
	toolStates := make(map[int64]*toolCallState)
	toolOrder := make([]int64, 0, 2)
	seenToolIndex := make(map[int64]bool)
	var finishReason string
	var lastUsage openai.CompletionUsage

	for stream.Next() {
		chunk := stream.Current()
		if chunk.Usage.TotalTokens != 0 || chunk.Usage.PromptTokens != 0 || chunk.Usage.CompletionTokens != 0 {
			lastUsage = chunk.Usage
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		if choice.Delta.Content != "" {
			if err := c.emitStreamResult(ctx, ch, &ai.StreamChunk{Text: choice.Delta.Content}); err != nil {
				return true, nil, err
			}
			assistantBuilder.WriteString(choice.Delta.Content)
		}
		if choice.Delta.FunctionCall.Name != "" || choice.Delta.FunctionCall.Arguments != "" {
			index := int64(0)
			state := toolStates[index]
			if state == nil {
				state = &toolCallState{}
				toolStates[index] = state
			}
			if !seenToolIndex[index] {
				toolOrder = append(toolOrder, index)
				seenToolIndex[index] = true
			}
			if choice.Delta.FunctionCall.Name != "" {
				state.name = choice.Delta.FunctionCall.Name
			}
			if choice.Delta.FunctionCall.Arguments != "" {
				state.arguments.WriteString(choice.Delta.FunctionCall.Arguments)
			}
		}
		if len(choice.Delta.ToolCalls) > 0 {
			for _, tc := range choice.Delta.ToolCalls {
				state := toolStates[tc.Index]
				if state == nil {
					state = &toolCallState{}
					toolStates[tc.Index] = state
				}
				if !seenToolIndex[tc.Index] {
					toolOrder = append(toolOrder, tc.Index)
					seenToolIndex[tc.Index] = true
				}
				if tc.ID != "" {
					state.id = tc.ID
				}
				if tc.Function.Name != "" {
					state.name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					state.arguments.WriteString(tc.Function.Arguments)
				}
			}
		}
		if choice.FinishReason != "" {
			finishReason = choice.FinishReason
		}
	}

	if err := stream.Err(); err != nil {
		return true, nil, fmt.Errorf("openai chat completion stream: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return true, nil, err
	}

	if lastUsage.TotalTokens != 0 || lastUsage.PromptTokens != 0 || lastUsage.CompletionTokens != 0 {
		c.publishUsage(lastUsage)
		if err := c.emitOpenAITokenChunk(ctx, ch, lastUsage); err != nil {
			return true, nil, err
		}
	}

	assistantParam := openai.ChatCompletionAssistantMessageParam{
		Content: openai.ChatCompletionAssistantMessageParamContentUnion{
			OfString: openai.String(assistantBuilder.String()),
		},
	}

	var toolCalls []openai.ChatCompletionMessageToolCall
	if len(toolStates) > 0 {
		parsed, err := c.buildToolCalls(toolOrder, toolStates)
		if err != nil {
			return true, nil, err
		}
		toolCalls = parsed
		assistantParam.ToolCalls = make([]openai.ChatCompletionMessageToolCallParam, len(parsed))
		for i, call := range parsed {
			// call.ToParam() relies on raw JSON captured from the API response.
			// Since we construct these tool calls ourselves while streaming, the raw
			// JSON is empty and would fail to marshal. Build the param struct
			// directly to ensure valid JSON encoding.
			assistantParam.ToolCalls[i] = openai.ChatCompletionMessageToolCallParam{
				ID: call.ID,
				Function: openai.ChatCompletionMessageToolCallFunctionParam{
					Name:      call.Function.Name,
					Arguments: call.Function.Arguments,
				},
				Type: call.Type,
			}
		}
		if err := c.emitOpenAIToolChunk(ctx, ch, parsed); err != nil {
			return true, nil, err
		}
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		{OfAssistant: &assistantParam},
	}

	if len(toolCalls) == 0 {
		if strings.TrimSpace(assistantBuilder.String()) == "" {
			if finishReason == "" {
				return true, nil, errors.New("openai returned an empty response")
			}
		}
		return true, messages, nil
	}

	if len(handlers) == 0 {
		return true, nil, fmt.Errorf("model requested %d tool calls but no handlers were provided", len(toolCalls))
	}

	toolMessages, err := c.executeToolCalls(ctx, toolCalls, handlers)
	if err != nil {
		return true, nil, err
	}

	messages = append(messages, toolMessages...)
	return false, messages, nil
}

func (c *Client) buildToolCalls(order []int64, states map[int64]*toolCallState) ([]openai.ChatCompletionMessageToolCall, error) {
	if len(states) == 0 {
		return nil, nil
	}

	calls := make([]openai.ChatCompletionMessageToolCall, 0, len(states))
	for _, idx := range order {
		state := states[idx]
		if state == nil {
			continue
		}
		call := openai.ChatCompletionMessageToolCall{
			ID: state.id,
			Function: openai.ChatCompletionMessageToolCallFunction{
				Name:      state.name,
				Arguments: state.arguments.String(),
			},
			Type: openai_constant.Function("function"),
		}
		calls = append(calls, call)
	}
	return calls, nil
}

func (c *Client) emitStreamResult(ctx context.Context, ch chan<- llmshared.StreamResult, chunk *ai.StreamChunk) error {
	if chunk == nil {
		return nil
	}
	select {
	case ch <- llmshared.StreamResult{Chunk: chunk}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) emitOpenAITokenChunk(ctx context.Context, ch chan<- llmshared.StreamResult, usage openai.CompletionUsage) error {
	if usage.TotalTokens == 0 && usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return nil
	}
	chunk := &ai.StreamChunk{
		TokenCount: &ai.TokenCount{
			TotalTokens:  int32(usage.TotalTokens),
			InputTokens:  int32(usage.PromptTokens),
			OutputTokens: int32(usage.CompletionTokens),
		},
	}
	return c.emitStreamResult(ctx, ch, chunk)
}

func (c *Client) emitOpenAIToolChunk(ctx context.Context, ch chan<- llmshared.StreamResult, calls []openai.ChatCompletionMessageToolCall) error {
	if len(calls) == 0 {
		return nil
	}

	toolChunks := make([]*ai.ToolCallChunk, 0, len(calls))
	for _, call := range calls {
		var params map[string]any
		if strings.TrimSpace(call.Function.Arguments) != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &params); err != nil {
				params = map[string]any{
					"raw": call.Function.Arguments,
				}
			}
		}
		toolChunks = append(toolChunks, &ai.ToolCallChunk{
			ID:         call.ID,
			Name:       call.Function.Name,
			Parameters: params,
		})
	}

	return c.emitStreamResult(ctx, ch, &ai.StreamChunk{ToolCalls: toolChunks})
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

func (c *Client) executeChat(ctx context.Context, baseParams openai.ChatCompletionNewParams, handlers map[string]ai.HandlerFunc, maxIterations int32) (string, error) {
	messages := append([]openai.ChatCompletionMessageParamUnion(nil), baseParams.Messages...)
	params := baseParams
	toolUsed := false

	limit := int(maxIterations)
	if limit <= 0 {
		limit = defaultMaxToolIterations
	}

	for iteration := 0; iteration < limit; iteration++ {
		params.Messages = messages

		resp, err := c.chatCompletions.New(ctx, params)
		if err != nil {
			return "", fmt.Errorf("openai chat completion: %w", err)
		}

		c.publishUsage(resp.Usage)

		if len(resp.Choices) == 0 {
			if toolUsed {
				return "", nil
			}
			return "", errors.New("openai chat completion returned no choices")
		}

		choice := resp.Choices[0]
		assistantMessage := choice.Message

		content := strings.TrimSpace(assistantMessage.Content)
		hasToolCalls := len(assistantMessage.ToolCalls) > 0
		if hasToolCalls {
			toolUsed = true
		}

		if hasToolCalls && content != "" {
			notification := events.NotificationEvent{Message: content}
			c.eventBus.Publish(notification.Topic(), notification)
		}

		messages = append(messages, assistantMessage.ToParam())

		if !hasToolCalls {
			if content == "" {
				if toolUsed {
					return "", nil
				}
				return "", errors.New("openai returned an empty response")
			}
			return content, nil
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

	return "", fmt.Errorf("exceeded maximum tool call iterations (%d) without completion", limit)
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

		if call.Function.Name == "viewImage" {
			img, sanitized, err := toolpayload.Extract(result)
			if err != nil {
				return nil, fmt.Errorf("invalid viewImage response: %w", err)
			}
			result = sanitized

			payload, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("unable to marshal response for function %q: %w", call.Function.Name, err)
			}
			messages = append(messages, openai.ToolMessage(string(payload), call.ID))

			if img != nil {
				messages = append(messages, buildImageUserMessage(img))
			}
			continue
		}

		if call.Function.Name == "viewDocument" {
			doc, sanitized, err := toolpayload.Extract(result)
			if err != nil {
				return nil, fmt.Errorf("invalid viewDocument response: %w", err)
			}
			result = sanitized

			payload, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("unable to marshal response for function %q: %w", call.Function.Name, err)
			}
			messages = append(messages, openai.ToolMessage(string(payload), call.ID))

			if doc != nil {
				messages = append(messages, buildDocumentUserMessage(doc))
			}
			continue
		}

		payload, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal response for function %q: %w", call.Function.Name, err)
		}

		messages = append(messages, openai.ToolMessage(string(payload), call.ID))
	}

	return messages, nil
}

func buildImageUserMessage(img *toolpayload.Payload) openai.ChatCompletionMessageParamUnion {
	text := toolpayload.SanitizePath(img.Path)
	parts := []openai.ChatCompletionContentPartUnionParam{
		openai.TextContentPart(fmt.Sprintf("Image retrieved from %s", text)),
		openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{URL: img.DataURL()}),
	}
	return openai.UserMessage(parts)
}

func buildDocumentUserMessage(doc *toolpayload.Payload) openai.ChatCompletionMessageParamUnion {
	text := toolpayload.SanitizePath(doc.Path)
	content := fmt.Sprintf("Document retrieved from %s (MIME: %s, %d bytes).", text, doc.MIMEType, doc.SizeBytes)
	notice := "This provider does not support inline PDFs; refer to the tool response for access."
	return openai.UserMessage([]openai.ChatCompletionContentPartUnionParam{
		openai.TextContentPart(content),
		openai.TextContentPart(notice),
	})
}

func (c *Client) renderPrompt(prompt ai.Prompt, debug bool, attrs []ai.Attr) (*ai.Prompt, error) {
	return llmshared.RenderPromptWithDebug(c.fileManager, prompt, debug, attrs)
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
