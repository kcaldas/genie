package lmstudio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

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
	defaultBaseURL           = "http://127.0.0.1:1234"
	chatEndpoint             = "/chat/completions"
)

var (
	errEmptyResponse     = errors.New("lm studio returned an empty response")
	errToolCallNoHandler = errors.New("model requested tool calls but no handlers were provided")

	_ ai.Gen = (*Client)(nil)
)

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Option configures the LM Studio client.
type Option func(*Client)

// WithConfigManager injects a custom configuration manager.
func WithConfigManager(manager config.Manager) Option {
	return func(c *Client) {
		if manager != nil {
			c.config = manager
		}
	}
}

// WithFileManager injects a custom file manager (useful for tests).
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

// WithHTTPClient injects a custom HTTP client.
func WithHTTPClient(client httpDoer) Option {
	return func(c *Client) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// WithBaseURL overrides the LM Studio base URL.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		if strings.TrimSpace(baseURL) != "" {
			c.baseURL = ensureV1Suffix(baseURL)
		}
	}
}

// Client provides an ai.Gen implementation backed by the LM Studio REST API.
type Client struct {
	mu sync.Mutex

	config      config.Manager
	fileManager fileops.Manager
	template    template.Engine
	eventBus    events.EventBus
	logger      logging.Logger

	httpClient httpDoer
	baseURL    string
}

// NewClient creates a new LM Studio-backed ai.Gen implementation.
func NewClient(eventBus events.EventBus, opts ...Option) (ai.Gen, error) {
	client := &Client{
		config:      config.NewConfigManager(),
		fileManager: fileops.NewFileOpsManager(),
		template:    template.NewEngine(),
		eventBus:    eventBus,
		logger:      logging.NewAPILogger("lmstudio"),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}

	if client.eventBus == nil {
		client.eventBus = &events.NoOpEventBus{}
	}

	for _, opt := range opts {
		opt(client)
	}

	if client.logger == nil {
		client.logger = logging.NewAPILogger("lmstudio")
	}

	if strings.TrimSpace(client.baseURL) == "" {
		client.baseURL = client.resolveBaseURL()
	}

	if strings.TrimSpace(client.baseURL) == "" {
		return nil, errors.New("lm studio base URL not configured")
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
	rendered, err := c.renderPrompt(prompt, debug, attrs)
	if err != nil {
		return "", fmt.Errorf("rendering prompt: %w", err)
	}

	return c.generateWithPrompt(ctx, *rendered)
}

// GenerateContentStream renders the prompt using string attributes and executes it with streaming.
func (c *Client) GenerateContentStream(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (ai.Stream, error) {
	attrs := ai.StringsToAttr(args)
	return c.GenerateContentAttrStream(ctx, prompt, debug, attrs)
}

// GenerateContentAttrStream renders the prompt using structured attributes and executes it with streaming.
func (c *Client) GenerateContentAttrStream(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (ai.Stream, error) {
	rendered, err := c.renderPrompt(prompt, debug, attrs)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	// If tools are involved, fall back to the non-streaming tool-execution flow and wrap the result as a stream.
	if len(rendered.Functions) > 0 && len(rendered.Handlers) > 0 {
		streamCtx, cancel := context.WithCancel(ctx)
		ch := make(chan llmshared.StreamResult, 1)

		go func() {
			defer close(ch)
			resp, err := c.GenerateContentAttr(streamCtx, *rendered, debug, nil)
			if err != nil {
				select {
				case ch <- llmshared.StreamResult{Err: err}:
				case <-streamCtx.Done():
				}
				return
			}
			select {
			case ch <- llmshared.StreamResult{Chunk: &ai.StreamChunk{Text: resp}}:
			case <-streamCtx.Done():
			}
		}()

		return llmshared.NewChunkStream(cancel, ch), nil
	}

	request, err := c.buildChatRequest(*rendered, normalMode)
	if err != nil {
		return nil, err
	}

	streamCtx, cancel := context.WithCancel(ctx)
	ch := make(chan llmshared.StreamResult, 1)

	go c.runStreamingChat(streamCtx, ch, request)

	return llmshared.NewChunkStream(cancel, ch), nil
}

// CountTokens renders the prompt, estimates token usage using string attributes, and returns the result.
func (c *Client) CountTokens(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (*ai.TokenCount, error) {
	attrs := ai.StringsToAttr(args)
	return c.CountTokensAttr(ctx, prompt, debug, attrs)
}

// CountTokensAttr renders the prompt, estimates token usage using structured attributes, and returns the result.
func (c *Client) CountTokensAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (*ai.TokenCount, error) {
	rendered, err := c.renderPrompt(prompt, debug, attrs)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	request, err := c.buildChatRequest(*rendered, countTokensMode)
	if err != nil {
		return nil, err
	}

	response, err := c.sendChat(ctx, request)
	if err != nil {
		return nil, err
	}

	tokenCount := c.buildTokenCount(response.Usage)
	c.publishTokenCount(tokenCount)

	return tokenCount, nil
}

// GetStatus reports the configured model and target endpoint.
func (c *Client) GetStatus() *ai.Status {
	model := c.config.GetModelConfig()
	modelStr := fmt.Sprintf("%s, Temperature: %.2f, Max Tokens: %d", model.ModelName, model.Temperature, model.MaxTokens)

	message := fmt.Sprintf("LM Studio configured (endpoint: %s)", c.baseURL)
	return &ai.Status{
		Model:     modelStr,
		Backend:   "lmstudio",
		Connected: true,
		Message:   message,
	}
}

func (c *Client) generateWithPrompt(ctx context.Context, prompt ai.Prompt) (string, error) {
	request, err := c.buildChatRequest(prompt, normalMode)
	if err != nil {
		return "", err
	}

	messages := append([]chatMessage(nil), request.Messages...)
	toolUsed := false

	limit := int(prompt.MaxToolIterations)
	if limit <= 0 {
		limit = defaultMaxToolIterations
	}

	for iteration := 0; iteration < limit; iteration++ {
		request.Messages = messages

		response, err := c.sendChat(ctx, request)
		if err != nil {
			return "", err
		}

		c.publishTokenCount(c.buildTokenCount(response.Usage))

		if len(response.Choices) == 0 {
			return "", errEmptyResponse
		}
		assistant := response.Choices[0].Message
		assistantContent := strings.TrimSpace(assistant.Content.Text())
		hasToolCalls := len(assistant.ToolCalls) > 0
		if hasToolCalls {
			toolUsed = true
		}

		if hasToolCalls && assistantContent != "" {
			notification := events.NotificationEvent{Message: assistantContent}
			c.eventBus.Publish(notification.Topic(), notification)
		}

		messages = append(messages, assistant.toChatMessage())

		if !hasToolCalls {
			if assistantContent == "" {
				if toolUsed {
					return "", nil
				}
				return "", errEmptyResponse
			}
			return assistantContent, nil
		}

		if len(prompt.Handlers) == 0 {
			return "", errToolCallNoHandler
		}

		toolMessages, err := c.executeToolCalls(ctx, assistant.ToolCalls, prompt.Handlers)
		if err != nil {
			return "", err
		}
		messages = append(messages, toolMessages...)
	}

	return "", fmt.Errorf("exceeded maximum tool call iterations (%d) without completion", limit)
}

func (c *Client) executeToolCalls(ctx context.Context, calls []toolCall, handlers map[string]ai.HandlerFunc) ([]chatMessage, error) {
	messages := make([]chatMessage, 0, len(calls))

	for _, call := range calls {
		handler := handlers[call.Function.Name]
		if handler == nil {
			return nil, fmt.Errorf("no handler registered for function %q", call.Function.Name)
		}

		args, err := call.Function.ArgumentsAsMap()
		if err != nil {
			return nil, fmt.Errorf("invalid arguments for function %q: %w", call.Function.Name, err)
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

			messages = append(messages, chatMessage{
				Role:       "tool",
				Content:    newMessageContentFromText(string(payload)),
				ToolCallID: call.ID,
			})

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

			messages = append(messages, chatMessage{
				Role:       "tool",
				Content:    newMessageContentFromText(string(payload)),
				ToolCallID: call.ID,
			})

			if doc != nil {
				messages = append(messages, buildDocumentUserMessage(doc))
			}
			continue
		}

		payload, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal response for function %q: %w", call.Function.Name, err)
		}

		messages = append(messages, chatMessage{
			Role:       "tool",
			Content:    newMessageContentFromText(string(payload)),
			ToolCallID: call.ID,
		})
	}

	return messages, nil
}

func buildImageUserMessage(img *toolpayload.Payload) chatMessage {
	text := toolpayload.SanitizePath(img.Path)
	parts := []contentPart{
		{Type: "text", Text: fmt.Sprintf("Image retrieved from %s", text)},
		{Type: "image_url", ImageURL: &imageURL{URL: img.DataURL()}},
	}
	return chatMessage{
		Role:    "user",
		Content: newMessageContent(parts),
	}
}

func buildDocumentUserMessage(doc *toolpayload.Payload) chatMessage {
	parts := []contentPart{
		{Type: "text", Text: fmt.Sprintf("Document retrieved from %s (MIME: %s, %d bytes).", toolpayload.SanitizePath(doc.Path), doc.MIMEType, doc.SizeBytes)},
		{Type: "text", Text: "Inline document attachments are not supported; see tool response."},
	}
	return chatMessage{
		Role:    "user",
		Content: newMessageContent(parts),
	}
}

func (c *Client) buildChatRequest(prompt ai.Prompt, mode requestMode) (chatRequest, error) {
	modelName := c.resolveModelName(prompt.ModelName)
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
		req.Tools = mapFunctions(prompt.Functions)
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
		dataURL := c.encodeImage(img)
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
	modelCfg := c.config.GetModelConfig()

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

func (c *Client) sendChat(ctx context.Context, req chatRequest) (*chatResponse, error) {
	req.Stream = false
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + chatEndpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	c.logger.Debug("lmstudio request", "url", url, "body", string(payload))

	httpReq.Header.Set("Content-Type", "application/json")
	for key, values := range ai.DefaultHTTPHeaders() {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("lm studio chat request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading lm studio response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("lm studio chat request failed: status %s: %s", resp.Status, string(body))
	}

	var response chatResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decoding lm studio response: %w", err)
	}

	c.logger.Debug("lmstudio response", "status", resp.StatusCode, "body", string(body))

	if response.Error != nil && response.Error.Message != "" {
		return nil, fmt.Errorf("lm studio error: %s", response.Error.Message)
	}

	return &response, nil
}

func (c *Client) sendChatStream(ctx context.Context, req chatRequest, handler func(*chatStreamResponse) error) error {
	req.Stream = true
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + chatEndpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	c.logger.Debug("lmstudio stream request", "url", url, "body", string(payload))

	httpReq.Header.Set("Content-Type", "application/json")
	for key, values := range ai.DefaultHTTPHeaders() {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("lm studio chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("lm studio chat request failed: status %s: %s", resp.Status, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "data:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
		if line == "" || line == "[DONE]" {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			// LM Studio streams may include event metadata lines; skip non-JSON events.
			continue
		}
		if len(line) > 0 && line[0] != '{' && line[0] != '[' {
			continue
		}

		c.logger.Debug("lmstudio stream chunk", "chunk", line)

		var chunk chatStreamResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return fmt.Errorf("decoding lm studio stream chunk: %w", err)
		}

		if err := handler(&chunk); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading lm studio stream: %w", err)
	}

	return nil
}

func (c *Client) emitStreamChunk(ctx context.Context, ch chan<- llmshared.StreamResult, chunk *ai.StreamChunk) error {
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

func (c *Client) emitToolChunk(ctx context.Context, ch chan<- llmshared.StreamResult, calls []*ai.ToolCallChunk) error {
	if len(calls) == 0 {
		return nil
	}
	return c.emitStreamChunk(ctx, ch, &ai.StreamChunk{ToolCalls: calls})
}

type toolCallAccumulator struct {
	ID        string
	Name      string
	Type      string
	Arguments strings.Builder
}

func (c *Client) runStreamingChat(ctx context.Context, ch chan<- llmshared.StreamResult, req chatRequest) {
	defer close(ch)

	acc := make(map[string]*toolCallAccumulator)

	err := c.sendChatStream(ctx, req, func(resp *chatStreamResponse) error {
		if resp.Error != nil && resp.Error.Message != "" {
			return fmt.Errorf("lm studio error: %s", resp.Error.Message)
		}

		for _, choice := range resp.Choices {
			delta := choice.Delta

			if text := delta.Text(); text != "" {
				if err := c.emitStreamChunk(ctx, ch, &ai.StreamChunk{Text: text}); err != nil {
					return err
				}
			}

			if len(delta.ToolCalls) > 0 {
				for _, call := range delta.ToolCalls {
					entry := acc[call.ID]
					if entry == nil {
						entry = &toolCallAccumulator{ID: call.ID, Type: call.Type}
						acc[call.ID] = entry
					}
					if call.Function.Name != "" {
						entry.Name = call.Function.Name
					}
					if call.Function.Arguments != "" {
						entry.Arguments.WriteString(call.Function.Arguments)
					}
				}
			}

			if len(acc) > 0 && (choice.FinishReason == "tool_calls" || choice.FinishReason == "stop" || choice.FinishReason == "") {
				chunks := make([]*ai.ToolCallChunk, 0, len(acc))
				for _, call := range acc {
					params := map[string]any{}
					args := strings.TrimSpace(call.Arguments.String())
					if args != "" {
						if err := json.Unmarshal([]byte(args), &params); err != nil {
							params = map[string]any{
								"raw": args,
							}
						}
					}
					chunks = append(chunks, &ai.ToolCallChunk{
						ID:         call.ID,
						Name:       call.Name,
						Parameters: params,
					})
				}
				if err := c.emitToolChunk(ctx, ch, chunks); err != nil {
					return err
				}
				acc = make(map[string]*toolCallAccumulator)
			}
		}

		if resp.Usage != nil {
			tokenCount := c.buildTokenCount(resp.Usage)
			c.publishTokenCount(tokenCount)
			if tokenCount != nil {
				if err := c.emitStreamChunk(ctx, ch, &ai.StreamChunk{TokenCount: tokenCount}); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err == nil && len(acc) > 0 {
		chunks := make([]*ai.ToolCallChunk, 0, len(acc))
		for _, call := range acc {
			params := map[string]any{}
			args := strings.TrimSpace(call.Arguments.String())
			if args != "" {
				if err := json.Unmarshal([]byte(args), &params); err != nil {
					params = map[string]any{
						"raw": args,
					}
				}
			}
			chunks = append(chunks, &ai.ToolCallChunk{
				ID:         call.ID,
				Name:       call.Name,
				Parameters: params,
			})
		}
		_ = c.emitToolChunk(ctx, ch, chunks)
	}

	if err != nil && ctx.Err() == nil {
		select {
		case ch <- llmshared.StreamResult{Err: err}:
		case <-ctx.Done():
		}
	}
}

func (c *Client) renderPrompt(prompt ai.Prompt, debug bool, attrs []ai.Attr) (*ai.Prompt, error) {
	return llmshared.RenderPromptWithDebug(c.fileManager, prompt, debug, attrs)
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

func (c *Client) buildTokenCount(usage *usage) *ai.TokenCount {
	if usage == nil {
		return nil
	}

	input := usage.PromptTokens
	output := usage.CompletionTokens
	total := usage.TotalTokens
	if total == 0 {
		total = input + output
	}

	return &ai.TokenCount{
		TotalTokens:  total,
		InputTokens:  input,
		OutputTokens: output,
	}
}

func (c *Client) resolveBaseURL() string {
	if env := strings.TrimSpace(c.config.GetStringWithDefault("GENIE_LMSTUDIO_BASE_URL", "")); env != "" {
		return ensureV1Suffix(env)
	}
	if env := strings.TrimSpace(c.config.GetStringWithDefault("LMSTUDIO_BASE_URL", "")); env != "" {
		return ensureV1Suffix(env)
	}
	if env := strings.TrimSpace(c.config.GetStringWithDefault("LM_STUDIO_BASE_URL", "")); env != "" {
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

func (c *Client) resolveModelName(promptModel string) string {
	if strings.TrimSpace(promptModel) != "" {
		return promptModel
	}

	model := c.config.GetModelConfig()
	if strings.TrimSpace(model.ModelName) != "" {
		return model.ModelName
	}

	return ""
}

func (c *Client) encodeImage(img *ai.Image) string {
	if img == nil || len(img.Data) == 0 {
		return ""
	}

	mimeType := strings.TrimSpace(img.Type)
	if mimeType == "" {
		mimeType = "image/png"
	}

	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(img.Data))
}
