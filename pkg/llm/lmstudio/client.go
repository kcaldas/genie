package lmstudio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/kcaldas/genie/pkg/llm/shared/toolpayload"
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

// Option configures the LM Studio client.
type Option = llmshared.LocalOption

// Shared functional options operating on the embedded local-client core.
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

// Client provides an ai.Gen implementation backed by the LM Studio REST API.
type Client struct {
	llmshared.LocalClientCore
}

// NewClient creates a new LM Studio-backed ai.Gen implementation.
func NewClient(eventBus events.EventBus, opts ...Option) (ai.Gen, error) {
	client := &Client{LocalClientCore: llmshared.NewLocalClientCore("lmstudio", eventBus)}

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

	return llmshared.RunToolLoop(ctx, turn, turn.handlers, c.loopConfig(*rendered), nil)
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
		return c.runBlockingLoopAsStream(ctx, *rendered)
	}

	request, err := c.buildChatRequest(*rendered, normalMode)
	if err != nil {
		return nil, err
	}

	streamCtx, cancel := context.WithCancel(ctx)
	ch := make(chan llmshared.StreamResult, 1)

	go c.runStreamingChat(streamCtx, ch, request)

	return llmshared.NewChunkStream(streamCtx, cancel, ch), nil
}

// runBlockingLoopAsStream executes the shared tool loop without
// streaming and emits the final text as one chunk.
func (c *Client) runBlockingLoopAsStream(ctx context.Context, prompt ai.Prompt) (ai.Stream, error) {
	turn, err := c.newTurn(prompt)
	if err != nil {
		return nil, err
	}

	streamCtx, cancel := context.WithCancel(ctx)
	ch := make(chan llmshared.StreamResult, 1)

	go func() {
		defer close(ch)
		defer llmshared.RecoverToStream(ch)

		resp, err := llmshared.RunToolLoop(streamCtx, turn, turn.handlers, c.loopConfig(prompt), nil)
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

	return llmshared.NewChunkStream(streamCtx, cancel, ch), nil
}

// CountTokens renders the prompt, estimates token usage using string attributes, and returns the result.
func (c *Client) CountTokens(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (*ai.TokenCount, error) {
	attrs := ai.StringsToAttr(args)
	return c.CountTokensAttr(ctx, prompt, debug, attrs)
}

// CountTokensAttr renders the prompt, estimates token usage using structured attributes, and returns the result.
func (c *Client) CountTokensAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (*ai.TokenCount, error) {
	rendered, err := c.RenderPrompt(prompt, debug, attrs)
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
	c.PublishTokenCount(tokenCount)

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
	return llmshared.NewLoopConfig(c.Config, prompt.MaxToolIterations, defaultMaxToolIterations)
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

func (c *Client) sendChat(ctx context.Context, req chatRequest) (*chatResponse, error) {
	req.Stream = false
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.BaseURL + chatEndpoint
	c.Logger.Debug("lmstudio request", "url", url, "body", string(payload))

	resp, err := c.PostJSON(ctx, url, payload)
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

	c.Logger.Debug("lmstudio response", "status", resp.StatusCode, "body", string(body))

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

	url := c.BaseURL + chatEndpoint
	c.Logger.Debug("lmstudio stream request", "url", url, "body", string(payload))

	resp, err := c.PostJSON(ctx, url, payload)
	if err != nil {
		return fmt.Errorf("lm studio chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("lm studio chat request failed: status %s: %s", resp.Status, string(body))
	}

	return llmshared.ScanStreamLines(resp.Body, "lm studio", func(line string) error {
		if strings.HasPrefix(line, "data:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
		if line == "" || line == "[DONE]" {
			return nil
		}
		if strings.HasPrefix(line, "event:") {
			// LM Studio streams may include event metadata lines; skip non-JSON events.
			return nil
		}
		if line[0] != '{' && line[0] != '[' {
			return nil
		}

		c.Logger.Debug("lmstudio stream chunk", "chunk", line)

		var chunk chatStreamResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return fmt.Errorf("decoding lm studio stream chunk: %w", err)
		}

		return handler(&chunk)
	})
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

type toolCallAccumulator struct {
	ID        string
	Name      string
	Type      string
	Arguments strings.Builder
}

// flushToolCallChunks converts accumulated tool-call deltas into
// consumer-facing chunks. Unparseable argument payloads are surfaced
// raw rather than dropped.
func flushToolCallChunks(acc map[string]*toolCallAccumulator) []*ai.ToolCallChunk {
	if len(acc) == 0 {
		return nil
	}
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
	return chunks
}

// runStreamingChat streams a single tool-free generation. Tool-call
// deltas are accumulated and reported to the consumer as chunks, but
// never executed (tool execution uses the blocking loop).
func (c *Client) runStreamingChat(ctx context.Context, ch chan<- llmshared.StreamResult, req chatRequest) {
	defer close(ch)
	defer llmshared.RecoverToStream(ch)

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
				if err := c.emitStreamChunk(ctx, ch, &ai.StreamChunk{ToolCalls: flushToolCallChunks(acc)}); err != nil {
					return err
				}
				acc = make(map[string]*toolCallAccumulator)
			}
		}

		if resp.Usage != nil {
			tokenCount := c.buildTokenCount(resp.Usage)
			c.PublishTokenCount(tokenCount)
			if tokenCount != nil {
				if err := c.emitStreamChunk(ctx, ch, &ai.StreamChunk{TokenCount: tokenCount}); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err == nil && len(acc) > 0 {
		_ = c.emitStreamChunk(ctx, ch, &ai.StreamChunk{ToolCalls: flushToolCallChunks(acc)})
	}

	if err != nil && ctx.Err() == nil {
		select {
		case ch <- llmshared.StreamResult{Err: err}:
		case <-ctx.Done():
		}
	}
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
