package ollama

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/fileops"
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/kcaldas/genie/pkg/template"
)

const (
	maxToolIterations = 8
	defaultBaseURL    = "http://127.0.0.1:11434"
	chatEndpoint      = "/api/chat"
	tokenCountPredict = 0
)

var (
	errNoBaseURL         = errors.New("ollama base URL not configured")
	errEmptyResponse     = errors.New("ollama returned an empty response")
	errToolCallNoHandler = errors.New("model requested tool calls but no handlers were provided")

	_ ai.Gen = (*Client)(nil)
)

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Option configures the Ollama client.
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

// WithBaseURL overrides the Ollama base URL.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		if strings.TrimSpace(baseURL) != "" {
			c.baseURL = strings.TrimRight(baseURL, "/")
		}
	}
}

// Client provides an ai.Gen implementation backed by the Ollama REST API.
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

// NewClient creates a new Ollama-backed ai.Gen implementation.
func NewClient(eventBus events.EventBus, opts ...Option) (ai.Gen, error) {
	client := &Client{
		config:      config.NewConfigManager(),
		fileManager: fileops.NewFileOpsManager(),
		template:    template.NewEngine(),
		eventBus:    eventBus,
		logger:      logging.NewAPILogger("ollama"),
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
		client.logger = logging.NewAPILogger("ollama")
	}

	if strings.TrimSpace(client.baseURL) == "" {
		client.baseURL = client.resolveBaseURL()
	}

	if strings.TrimSpace(client.baseURL) == "" {
		return nil, errNoBaseURL
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

	tokenCount := c.buildTokenCount(response)
	c.publishTokenCount(tokenCount)

	return tokenCount, nil
}

// GetStatus reports the configured model and target endpoint.
func (c *Client) GetStatus() *ai.Status {
	model := c.config.GetModelConfig()
	modelStr := fmt.Sprintf("%s, Temperature: %.2f, Max Tokens: %d", model.ModelName, model.Temperature, model.MaxTokens)

	message := fmt.Sprintf("Ollama configured (endpoint: %s)", c.baseURL)
	return &ai.Status{
		Model:     modelStr,
		Backend:   "ollama",
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

	for iteration := 0; iteration < maxToolIterations; iteration++ {
		request.Messages = messages

		response, err := c.sendChat(ctx, request)
		if err != nil {
			return "", err
		}

		c.publishTokenCount(c.buildTokenCount(response))

		assistant := response.Message
		if content := strings.TrimSpace(assistant.Content.Text()); content != "" {
			notification := events.NotificationEvent{Message: content}
			c.eventBus.Publish(notification.Topic(), notification)
		}

		messages = append(messages, assistant.toChatMessage())

		if len(assistant.ToolCalls) == 0 {
			responseText := strings.TrimSpace(assistant.Content.Text())
			if responseText == "" {
				return "", errEmptyResponse
			}
			return responseText, nil
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

	return "", fmt.Errorf("exceeded maximum tool call iterations (%d) without completion", maxToolIterations)
}

func (c *Client) executeToolCalls(ctx context.Context, calls []toolCall, handlers map[string]ai.HandlerFunc) ([]chatMessage, error) {
	messages := make([]chatMessage, 0, len(calls))

	normalizedHandlers := make(map[string]ai.HandlerFunc, len(handlers))
	for name, handler := range handlers {
		if handler == nil {
			continue
		}
		if normalized := normalizeToolName(name); normalized != "" {
			if _, exists := normalizedHandlers[normalized]; !exists {
				normalizedHandlers[normalized] = handler
			}
		}
	}

	for _, call := range calls {
		handler := handlers[call.Function.Name]
		if handler == nil {
			if normalized := normalizeToolName(call.Function.Name); normalized != "" {
				handler = normalizedHandlers[normalized]
			}
		}
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

func (c *Client) buildChatRequest(prompt ai.Prompt, mode requestMode) (chatRequest, error) {
	modelName := c.resolveModelName(prompt.ModelName)
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
		req.Tools = mapFunctions(prompt.Functions)
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
		messages = append(messages, chatMessage{
			Role:    "system",
			Content: newMessageContentFromText(instruction),
		})
	}

	parts := []messagePart{}
	if text := strings.TrimSpace(prompt.Text); text != "" {
		parts = append(parts, messagePart{
			Type: "text",
			Text: text,
		})
	}

	for _, img := range prompt.Images {
		if img == nil || len(img.Data) == 0 {
			continue
		}
		dataURL := c.encodeImage(img)
		if dataURL == "" {
			continue
		}
		parts = append(parts, messagePart{
			Type:  "image",
			Image: dataURL,
		})
	}

	if len(parts) == 0 {
		messages = append(messages, chatMessage{
			Role:    "user",
			Content: newMessageContentFromText(""),
		})
		return messages
	}

	messages = append(messages, chatMessage{
		Role:    "user",
		Content: newMessageContent(parts),
	})

	return messages
}

func (c *Client) buildOptions(prompt ai.Prompt, mode requestMode) map[string]any {
	opts := map[string]any{}
	modelCfg := c.config.GetModelConfig()

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

	url := c.baseURL + chatEndpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	for key, values := range ai.DefaultHTTPHeaders() {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	resp, err := c.httpClient.Do(httpReq)
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

func (c *Client) saveObjectToTmpFile(object interface{}, filename string) error {
	filePath := filepath.Join("tmp", filename)
	return c.fileManager.WriteObjectAsYAML(filePath, object)
}

func (c *Client) resolveBaseURL() string {
	if env := strings.TrimSpace(c.config.GetStringWithDefault("GENIE_OLLAMA_BASE_URL", "")); env != "" {
		return strings.TrimRight(env, "/")
	}
	if env := strings.TrimSpace(c.config.GetStringWithDefault("OLLAMA_HOST", "")); env != "" {
		if strings.HasPrefix(env, "http://") || strings.HasPrefix(env, "https://") {
			return strings.TrimRight(env, "/")
		}
		return "http://" + strings.TrimRight(env, "/")
	}
	return defaultBaseURL
}

func (c *Client) encodeImage(img *ai.Image) string {
	if img == nil || len(img.Data) == 0 {
		return ""
	}

	mimeType := strings.TrimSpace(img.Type)
	if mimeType == "" {
		mimeType = "image/png"
	}
	data := base64.StdEncoding.EncodeToString(img.Data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, data)
}

func (c *Client) mapAttr(attrs []ai.Attr) map[string]string {
	result := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		result[attr.Key] = attr.Value
	}
	return result
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

type requestMode int

const (
	normalMode requestMode = iota
	countTokensMode
)
