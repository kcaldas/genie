package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/kcaldas/genie/pkg/llm/shared/toolpayload"
)

const (
	defaultMaxToolIterations = 200
	defaultBaseURL           = "http://127.0.0.1:11434"
	chatEndpoint             = "/api/chat"
	tokenCountPredict        = 0
)

var (
	errNoBaseURL         = errors.New("ollama base URL not configured")
	errEmptyResponse     = errors.New("ollama returned an empty response")
	errToolCallNoHandler = errors.New("model requested tool calls but no handlers were provided")

	_ ai.Gen = (*Client)(nil)
)

// Option configures the Ollama client.
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

// WithBaseURL overrides the Ollama base URL.
func WithBaseURL(baseURL string) Option {
	return func(c *llmshared.LocalClientCore) {
		if strings.TrimSpace(baseURL) != "" {
			c.BaseURL = strings.TrimRight(baseURL, "/")
		}
	}
}

// Client provides an ai.Gen implementation backed by the Ollama REST API.
type Client struct {
	llmshared.LocalClientCore
}

// NewClient creates a new Ollama-backed ai.Gen implementation.
func NewClient(eventBus events.EventBus, opts ...Option) (ai.Gen, error) {
	client := &Client{LocalClientCore: llmshared.NewLocalClientCore("ollama", eventBus)}

	for _, opt := range opts {
		opt(&client.LocalClientCore)
	}

	if strings.TrimSpace(client.BaseURL) == "" {
		client.BaseURL = client.resolveBaseURL()
	}

	if strings.TrimSpace(client.BaseURL) == "" {
		return nil, errNoBaseURL
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

func (c *Client) GenerateContentStream(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (ai.Stream, error) {
	attrs := ai.StringsToAttr(args)
	return c.GenerateContentAttrStream(ctx, prompt, debug, attrs)
}

func (c *Client) GenerateContentAttrStream(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (ai.Stream, error) {
	rendered, err := c.RenderPrompt(prompt, debug, attrs)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	turn, err := c.newTurn(*rendered)
	if err != nil {
		return nil, err
	}

	return llmshared.RunToolLoopStream(ctx, turn, turn.handlers, c.loopConfig(*rendered)), nil
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

	tokenCount := c.buildTokenCount(response)
	c.PublishTokenCount(tokenCount)

	return tokenCount, nil
}

// GetStatus reports the configured model and target endpoint.
func (c *Client) GetStatus() *ai.Status {
	model := c.Config.GetModelConfig()
	modelStr := fmt.Sprintf("%s, Temperature: %.2f, Max Tokens: %d", model.ModelName, model.Temperature, model.MaxTokens)

	message := fmt.Sprintf("Ollama configured (endpoint: %s)", c.BaseURL)
	return &ai.Status{
		Model:     modelStr,
		Backend:   "ollama",
		Connected: true,
		Message:   message,
	}
}

// loopConfig maps prompt and environment settings onto the shared
// agent-loop configuration.
func (c *Client) loopConfig(prompt ai.Prompt) llmshared.LoopConfig {
	return llmshared.NewLoopConfig(c.Config, prompt.MaxToolIterations, defaultMaxToolIterations)
}

func normalizeToolName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	name = strings.ToLower(name)

	var builder strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '_', r == '-', r == '.':
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

func buildOllamaImageMessage(img *toolpayload.Payload) chatMessage {
	text := ""
	if sanitized := toolpayload.SanitizePath(img.Path); sanitized != "" {
		text = fmt.Sprintf("Image retrieved from %s", sanitized)
	}

	return chatMessage{
		Role:    "user",
		Content: newMessageContentFromText(text),
		Images:  []string{img.Base64Data},
	}
}

func buildOllamaDocumentMessage(doc *toolpayload.Payload) chatMessage {
	parts := []messagePart{
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
		return chatRequest{}, errors.New("no Ollama model configured")
	}

	messages := c.buildMessages(prompt)

	req := chatRequest{
		Model:    modelName,
		Messages: messages,
		Stream:   false,
		Options:  c.buildOptions(prompt, mode),
	}

	if len(prompt.Functions) > 0 {
		req.Tools = llmshared.MapFunctions(prompt.Functions, schemaToMap)
	}

	if prompt.ResponseSchema != nil {
		format, err := schemaToFormat(prompt)
		if err != nil {
			return chatRequest{}, err
		}
		req.Format = format
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

	text := strings.TrimSpace(prompt.Text)

	var images []string
	for _, img := range prompt.Images {
		if img == nil || len(img.Data) == 0 {
			continue
		}
		dataURL := llmshared.EncodeImageDataURL(img)
		if dataURL != "" {
			images = append(images, dataURL)
		}
	}

	messages = append(messages, chatMessage{
		Role:    "user",
		Content: newMessageContentFromText(text),
		Images:  images,
	})

	return messages
}

func (c *Client) buildOptions(prompt ai.Prompt, mode requestMode) map[string]any {
	opts := map[string]any{}
	modelCfg := c.Config.GetModelConfig()

	maxTokens := prompt.MaxTokens
	if maxTokens <= 0 {
		maxTokens = modelCfg.MaxTokens
	}
	if maxTokens > 0 {
		opts["num_predict"] = maxTokens
	}

	temperature := prompt.Temperature
	if temperature <= 0 {
		temperature = modelCfg.Temperature
	}
	if temperature > 0 {
		opts["temperature"] = temperature
	}

	topP := prompt.TopP
	if topP <= 0 {
		topP = modelCfg.TopP
	}
	if topP > 0 && topP < 1.0 {
		opts["top_p"] = topP
	}

	if mode == countTokensMode {
		opts["num_predict"] = tokenCountPredict
	}

	if len(opts) == 0 {
		return nil
	}
	return opts
}

func (c *Client) sendChat(ctx context.Context, req chatRequest) (*chatResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.PostJSON(ctx, c.BaseURL+chatEndpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("ollama chat request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading ollama response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ollama chat request failed: status %s: %s", resp.Status, string(body))
	}

	var response chatResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decoding ollama response: %w", err)
	}

	return &response, nil
}

func (c *Client) sendChatStream(ctx context.Context, req chatRequest, handler func(*chatResponse) error) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.PostJSON(ctx, c.BaseURL+chatEndpoint, payload)
	if err != nil {
		return fmt.Errorf("ollama chat request failed: %w", err)
	}
	defer resp.Body.Close()

	return llmshared.ScanStreamLines(resp.Body, "ollama", func(line string) error {
		var chunk chatResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return fmt.Errorf("decoding ollama stream chunk: %w", err)
		}

		if err := handler(&chunk); err != nil {
			return err
		}

		if chunk.Done {
			return llmshared.ErrStopStream
		}
		return nil
	})
}

func (c *Client) buildTokenCount(response *chatResponse) *ai.TokenCount {
	if response == nil {
		return nil
	}

	input := int32(response.PromptEvalCount)
	output := int32(response.EvalCount)
	total := input + output

	return &ai.TokenCount{
		TotalTokens:  total,
		InputTokens:  input,
		OutputTokens: output,
	}
}

func (c *Client) resolveBaseURL() string {
	if env := strings.TrimSpace(c.Config.GetStringWithDefault("GENIE_OLLAMA_BASE_URL", "")); env != "" {
		return strings.TrimRight(env, "/")
	}
	if env := strings.TrimSpace(c.Config.GetStringWithDefault("OLLAMA_HOST", "")); env != "" {
		if strings.HasPrefix(env, "http://") || strings.HasPrefix(env, "https://") {
			return strings.TrimRight(env, "/")
		}
		return "http://" + strings.TrimRight(env, "/")
	}
	return defaultBaseURL
}

type requestMode int

const (
	normalMode requestMode = iota
	countTokensMode
)
