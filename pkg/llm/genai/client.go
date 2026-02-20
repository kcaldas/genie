package genai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/fileops"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/kcaldas/genie/pkg/llm/shared/toolpayload"
	"github.com/kcaldas/genie/pkg/template"
	"google.golang.org/genai"
)
// Backend represents the GenAI backend to use
type Backend string
const (
	BackendVertexAI          Backend    = "vertex"
	BackendGeminiAPI         Backend    = "gemini"
	roleFunctionResponse     genai.Role = "user"
	defaultMaxToolIterations            = 200
)
// Client implements the ai.Gen interface using Google's unified GenAI package
// Supports both Vertex AI and Gemini API backends
type Client struct {
	Client          *genai.Client
	FileManager     fileops.Manager
	Config          config.Manager
	TemplateManager template.Engine
	Backend         Backend
	EventBus        events.EventBus
	// Allows tests to intercept generate content calls.
	callGenerateContentFn func(ctx context.Context, modelName string, contents []*genai.Content, config *genai.GenerateContentConfig, handlers map[string]ai.HandlerFunc) (*genai.GenerateContentResponse, error)
	// Lazy initialization
	mu          sync.Mutex
	initialized bool
	initError   error
}
var _ ai.Gen = &Client{}
// NewClient creates a new unified GenAI client that will initialize lazily
func NewClient(eventBus events.EventBus) (ai.Gen, error) {
	configManager := config.NewConfigManager()
	// Determine backend preference and check basic configuration
	backend := Backend(configManager.GetStringWithDefault("GENAI_BACKEND", "gemini"))
	// Check that at least one backend has basic configuration
	hasGeminiKey := configManager.GetStringWithDefault("GEMINI_API_KEY", "") != ""
	hasVertexProject := configManager.GetStringWithDefault("GOOGLE_CLOUD_PROJECT", "") != ""
	if !hasGeminiKey && !hasVertexProject {
		return nil, fmt.Errorf("no valid AI backend configured. Please set up one of the following:\n\n" +
			"Option 1 - Gemini API (recommended):\n" +
			"  export GEMINI_API_KEY=your-api-key\n" +
			"  Get your API key from: https://aistudio.google.com/apikey\n\n" +
			"Option 2 - Vertex AI:\n" +
			"  export GOOGLE_CLOUD_PROJECT=your-project-id\n" +
			"  Requires Google Cloud setup and authentication\n")
	}
	return &Client{
		Client:          nil, // Will be created on first use
		FileManager:     fileops.NewFileOpsManager(),
		Config:          configManager,
		TemplateManager: template.NewEngine(),
		Backend:         backend,
		initialized:     false,
		EventBus:        eventBus,
	}, nil
}
// ensureInitialized initializes the GenAI client (idempotent, safe to call multiple times)
func (g *Client) ensureInitialized(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	// If already initialized (successfully or with error), return cached result
	if g.initialized {
		return g.initError
	}
	// Mark as initialized to prevent multiple attempts
	g.initialized = true
	// Try to create client based on backend preference
	client, actualBackend, err := createClientWithBackend(g.Config, g.Backend)
	if err != nil {
		// If preferred backend fails, try the other one
		var fallbackBackend Backend
		if g.Backend == BackendGeminiAPI {
			fallbackBackend = BackendVertexAI
		} else {
			fallbackBackend = BackendGeminiAPI
		}
		client, actualBackend, err = createClientWithBackend(g.Config, fallbackBackend)
		if err != nil {
			// Both backends failed - provide helpful message with both options
			g.initError = fmt.Errorf("no valid AI backend configured. Please set up one of the following:\n\n" +
				"Option 1 - Gemini API (recommended):\n" +
				"  export GEMINI_API_KEY=your-api-key\n" +
				"  Get your API key from: https://aistudio.google.com/apikey\n\n" +
				"Option 2 - Vertex AI:\n" +
				"  export GOOGLE_CLOUD_PROJECT=your-project-id\n" +
				"  Requires Google Cloud setup and authentication\n")
			return g.initError
		}
	}
	// Success - store the client and backend
	g.Client = client
	g.Backend = actualBackend
	g.initError = nil
	return nil
}
// createClientWithBackend attempts to create a client with the specified backend
func createClientWithBackend(configManager config.Manager, backend Backend) (*genai.Client, Backend, error) {
	ctx := context.Background()
	switch backend {
	case BackendGeminiAPI:
		// Try Gemini API (API key based)
		apiKey := configManager.GetStringWithDefault("GEMINI_API_KEY", "")
		if apiKey == "" {
			return nil, "", fmt.Errorf("GEMINI_API_KEY not configured")
		}
		cfg := &genai.ClientConfig{
			APIKey:  apiKey,
			Backend: genai.BackendGeminiAPI,
		}
		cfg.HTTPOptions.Headers = ai.DefaultHTTPHeaders()
		client, err := genai.NewClient(ctx, cfg)
		if err != nil {
			return nil, "", fmt.Errorf("error creating Gemini API client: %w", err)
		}
		return client, BackendGeminiAPI, nil
	case BackendVertexAI:
		// Try Vertex AI (GCP project based)
		projectID, err := configManager.GetString("GOOGLE_CLOUD_PROJECT")
		if err != nil {
			return nil, "", fmt.Errorf("GOOGLE_CLOUD_PROJECT not configured")
		}
		location := configManager.GetStringWithDefault("GOOGLE_CLOUD_LOCATION", "us-central1")
		cfg := &genai.ClientConfig{
			Project:  projectID,
			Location: location,
			Backend:  genai.BackendVertexAI,
		}
		cfg.HTTPOptions.Headers = ai.DefaultHTTPHeaders()
		client, err := genai.NewClient(ctx, cfg)
		if err != nil {
			return nil, "", fmt.Errorf("error creating Vertex AI client: %w", err)
		}
		return client, BackendVertexAI, nil
	default:
		return nil, "", fmt.Errorf("unsupported backend: %s", backend)
	}
}
func (g *Client) GenerateContent(ctx context.Context, p ai.Prompt, debug bool, args ...string) (string, error) {
	// Ensure client is initialized
	if err := g.ensureInitialized(ctx); err != nil {
		return "", err
	}
	attrs := ai.StringsToAttr(args)
	prompt, err := renderPrompt(g.FileManager, p, debug, attrs)
	if err != nil {
		return "", fmt.Errorf("error rendering prompt: %w", err)
	}
	return g.generateContentWithPrompt(ctx, *prompt, debug)
}
func (g *Client) GenerateContentAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (string, error) {
	// Ensure client is initialized
	if err := g.ensureInitialized(ctx); err != nil {
		return "", err
	}
	p, err := renderPrompt(g.FileManager, prompt, debug, attrs)
	if err != nil {
		return "", fmt.Errorf("error rendering prompt: %w", err)
	}
	return g.generateContentWithPrompt(ctx, *p, debug)
}
func (g *Client) GenerateContentStream(ctx context.Context, p ai.Prompt, debug bool, args ...string) (ai.Stream, error) {
	if err := g.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	attrs := ai.StringsToAttr(args)
	prompt, err := renderPrompt(g.FileManager, p, debug, attrs)
	if err != nil {
		return nil, fmt.Errorf("error rendering prompt: %w", err)
	}
	return g.generateContentStreamWithPrompt(ctx, *prompt)
}
func (g *Client) GenerateContentAttrStream(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (ai.Stream, error) {
	if err := g.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	rendered, err := renderPrompt(g.FileManager, prompt, debug, attrs)
	if err != nil {
		return nil, fmt.Errorf("error rendering prompt: %w", err)
	}
	return g.generateContentStreamWithPrompt(ctx, *rendered)
}
func (g *Client) CountTokens(ctx context.Context, p ai.Prompt, debug bool, args ...string) (*ai.TokenCount, error) {
	// Ensure client is initialized
	if err := g.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	attrs := ai.StringsToAttr(args)
	return g.CountTokensAttr(ctx, p, debug, attrs)
}
func (g *Client) CountTokensAttr(ctx context.Context, p ai.Prompt, debug bool, attrs []ai.Attr) (*ai.TokenCount, error) {
	// Ensure client is initialized
	if err := g.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	prompt, err := renderPrompt(g.FileManager, p, debug, attrs)
	if err != nil {
		return nil, fmt.Errorf("error rendering prompt: %w", err)
	}
	return g.countTokensWithPrompt(ctx, *prompt)
}
// GetStatus returns the connection status and backend information
func (g *Client) GetStatus() *ai.Status {
	model := g.Config.GetModelConfig()
	modelStr := fmt.Sprintf("%s, Temperature: %.2f, Max Tokens: %d", model.ModelName, model.Temperature, model.MaxTokens)
	// Check if we have the required configuration for our current backend
	switch g.Backend {
	case BackendGeminiAPI:
		apiKey := g.Config.GetStringWithDefault("GEMINI_API_KEY", "")
		if apiKey == "" {
			return &ai.Status{Model: modelStr, Connected: false, Backend: "gemini", Message: "GEMINI_API_KEY not configured"}
		}
		return &ai.Status{Model: modelStr, Connected: true, Backend: "gemini", Message: "Gemini API configured"}
	case BackendVertexAI:
		projectID := g.Config.GetStringWithDefault("GOOGLE_CLOUD_PROJECT", "")
		if projectID == "" {
			return &ai.Status{Model: modelStr, Connected: false, Backend: "vertex", Message: "GOOGLE_CLOUD_PROJECT not configured"}
		}
		location := g.Config.GetStringWithDefault("GOOGLE_CLOUD_LOCATION", "us-central1")
		return &ai.Status{Model: modelStr, Connected: true, Backend: "vertex", Message: fmt.Sprintf("Vertex AI configured (project: %s, location: %s)", projectID, location)}
	default:
		return &ai.Status{Model: modelStr, Connected: false, Backend: "unknown", Message: fmt.Sprintf("Unknown backend: %s", g.Backend)}
	}
}
func (g *Client) generateContentWithPrompt(ctx context.Context, p ai.Prompt, debug bool) (string, error) {
	contents := g.buildInitialContents(p)
	config := g.buildGenerateConfig(p)
	limit := normalizeToolIterations(p.MaxToolIterations)
	// Generate content with function calling support
	result, toolUsed, err := g.invokeGenerateContent(ctx, p.ModelName, contents, config, p.Handlers, limit)
	if err != nil {
		return "", fmt.Errorf("error generating content: %w", err)
	}
	// Extract text from the response
	if len(result.Candidates) == 0 {
		if toolUsed {
			return "", nil
		}
		return "", fmt.Errorf("no response candidates")
	}
	candidate := result.Candidates[0]
	if candidate.Content == nil {
		if toolUsed {
			return "", nil
		}
		return "", fmt.Errorf("no content in response candidate")
	}
	response := g.joinContentParts(candidate.Content)
	if strings.TrimSpace(response) == "" {
		if toolUsed {
			return "", nil
		}
		return "", fmt.Errorf("no usable content in response candidates")
	}
	return response, nil
}
func (g *Client) generateContentStreamWithPrompt(ctx context.Context, p ai.Prompt) (ai.Stream, error) {
	contents := g.buildInitialContents(p)
	config := g.buildGenerateConfig(p)
	limit := normalizeToolIterations(p.MaxToolIterations)
	streamCtx, cancel := context.WithCancel(ctx)
	ch := make(chan llmshared.StreamResult, 1)
	go g.runContentStream(streamCtx, ch, p.ModelName, contents, config, p.Handlers, limit)
	return llmshared.NewChunkStream(cancel, ch), nil
}
func (g *Client) runContentStream(ctx context.Context, ch chan<- llmshared.StreamResult, modelName string, contents []*genai.Content, config *genai.GenerateContentConfig, handlers map[string]ai.HandlerFunc, maxIterations int) {
	defer close(ch)
	currentContents := make([]*genai.Content, len(contents))
	copy(currentContents, contents)
	currentConfig := config
	for iteration := 0; ; iteration++ {
		done, updatedContents, err := g.streamGenerationStep(ctx, ch, modelName, currentContents, currentConfig, handlers)
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
		if iteration+1 >= maxIterations {
			select {
			case ch <- llmshared.StreamResult{Err: fmt.Errorf("exceeded maximum tool call iterations (%d) without completion", maxIterations)}:
			case <-ctx.Done():
			}
			return
		}
		currentContents = updatedContents
		if currentConfig != nil {
			currentConfig.ToolConfig = nil
		}
	}
}
func (g *Client) streamGenerationStep(ctx context.Context, ch chan<- llmshared.StreamResult, modelName string, contents []*genai.Content, config *genai.GenerateContentConfig, handlers map[string]ai.HandlerFunc) (bool, []*genai.Content, error) {
	stream := g.Client.Models.GenerateContentStream(ctx, modelName, contents, config)
	var lastResp *genai.GenerateContentResponse
	debug := g.Config.GetBoolWithDefault("GENIE_DEBUG", false)
	chunkCount := 0

	// Accumulate all parts across chunks to build the complete response
	var allParts []*genai.Part
	var lastUsageMetadata *genai.GenerateContentResponseUsageMetadata
	var lastFinishReason genai.FinishReason
	var lastFinishMessage string

	for resp, err := range stream {
		if err != nil {
			return true, nil, fmt.Errorf("error generating streamed content: %w", err)
		}
		lastResp = resp
		chunkCount++

		// Accumulate non-empty parts from each chunk
		if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
			for _, part := range resp.Candidates[0].Content.Parts {
				if !isEmptyPart(part) {
					// Debug: log function call parts being accumulated
					if part.FunctionCall != nil {
						log.Printf("DEBUG streaming: accumulating FunctionCall name=%s args=%v (chunk %d, total parts so far: %d)",
							part.FunctionCall.Name, part.FunctionCall.Args, chunkCount, len(allParts)+1)
					}
					allParts = append(allParts, part)
				}
			}
		}
		// Track usage metadata and finish reason from each chunk
		if resp.UsageMetadata != nil {
			lastUsageMetadata = resp.UsageMetadata
		}
		if len(resp.Candidates) > 0 {
			if resp.Candidates[0].FinishReason != "" {
				lastFinishReason = resp.Candidates[0].FinishReason
				lastFinishMessage = resp.Candidates[0].FinishMessage
			}
		}

		if chunk := g.responseToStreamChunk(resp); chunk != nil {
			if err := g.emitStreamChunk(ctx, ch, chunk); err != nil {
				return true, nil, err
			}
		}
	}
	if debug {
		notification := events.NotificationEvent{
			Message:     fmt.Sprintf("Stream: %d chunks, %d parts", chunkCount, len(allParts)),
			ContentType: "debug",
		}
		g.EventBus.Publish(notification.Topic(), notification)
	}
	if err := ctx.Err(); err != nil {
		return true, nil, err
	}
	if lastResp == nil {
		return true, nil, fmt.Errorf("no response received from model")
	}
	// Check for blocking reasons
	if err := g.checkFinishReason(lastFinishReason, lastFinishMessage); err != nil {
		return true, nil, err
	}
	if tc := g.publishUsageMetadata(lastUsageMetadata); tc != nil {
		if err := g.emitStreamChunk(ctx, ch, &ai.StreamChunk{TokenCount: tc}); err != nil {
			return true, nil, err
		}
	}

	// Build accumulated response with all parts for function call handling
	accumulatedResp := g.buildAccumulatedResponse(allParts, lastUsageMetadata)
	fnCalls := accumulatedResp.FunctionCalls()
	if len(fnCalls) == 0 {
		return true, nil, nil
	}
	updatedContents, err := g.handleFunctionCalls(ctx, accumulatedResp, contents, handlers, false)
	if err != nil {
		return false, nil, err
	}
	return false, updatedContents, nil
}

// buildAccumulatedResponse creates a response with all accumulated parts
func (g *Client) buildAccumulatedResponse(parts []*genai.Part, usage *genai.GenerateContentResponseUsageMetadata) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: parts,
					Role:  "model",
				},
			},
		},
		UsageMetadata: usage,
	}
}

// isEmptyPart checks if a part has no meaningful content
func isEmptyPart(part *genai.Part) bool {
	if part == nil {
		return true
	}
	// Check all possible content fields
	hasContent := part.Text != "" ||
		part.FunctionCall != nil ||
		part.FunctionResponse != nil ||
		part.ExecutableCode != nil ||
		part.CodeExecutionResult != nil ||
		(part.InlineData != nil && len(part.InlineData.Data) > 0) ||
		(part.FileData != nil && part.FileData.FileURI != "")
	return !hasContent
}

// checkFinishReason returns an error if the model stopped for a blocking reason
func (g *Client) checkFinishReason(reason genai.FinishReason, message string) error {
	switch reason {
	case genai.FinishReasonSafety:
		return fmt.Errorf("response blocked by safety filters: %s", message)
	case genai.FinishReasonRecitation:
		return fmt.Errorf("response blocked due to potential recitation: %s", message)
	case genai.FinishReasonBlocklist:
		return fmt.Errorf("response blocked due to forbidden terms: %s", message)
	case genai.FinishReasonProhibitedContent:
		return fmt.Errorf("response blocked due to prohibited content: %s", message)
	case genai.FinishReasonSPII:
		return fmt.Errorf("response blocked due to sensitive personal information: %s", message)
	case genai.FinishReasonMalformedFunctionCall:
		return fmt.Errorf("model generated invalid function call: %s", message)
	case genai.FinishReasonMaxTokens:
		// Not an error, but worth logging
		notification := events.NotificationEvent{
			Message:     "Response truncated: reached maximum output tokens",
			ContentType: "warning",
		}
		g.EventBus.Publish(notification.Topic(), notification)
	}
	return nil
}

func (g *Client) emitStreamChunk(ctx context.Context, ch chan<- llmshared.StreamResult, chunk *ai.StreamChunk) error {
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
func (g *Client) responseToStreamChunk(resp *genai.GenerateContentResponse) *ai.StreamChunk {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil
	}
	chunk := &ai.StreamChunk{}
	var textParts []string
	var thoughtParts []string
	showThoughts := g.Config.GetBoolWithDefault("GEMINI_SHOW_THOUGHTS", false)
	for _, part := range resp.Candidates[0].Content.Parts {
		switch {
		case part.Text != "":
			if part.Thought {
				thoughtParts = append(thoughtParts, part.Text)
				if showThoughts {
					notification := events.NotificationEvent{
						Message:     fmt.Sprintf("Thinking: %s", part.Text),
						ContentType: "thought",
					}
					g.EventBus.Publish(notification.Topic(), notification)
				}
			} else {
				textParts = append(textParts, part.Text)
			}
		case part.FunctionCall != nil:
			if call := g.convertFunctionCall(part.FunctionCall); call != nil {
				chunk.ToolCalls = append(chunk.ToolCalls, call)
			}
		case part.FunctionResponse != nil && part.FunctionResponse.Response != nil:
			payload, err := json.Marshal(part.FunctionResponse.Response)
			if err != nil {
				textParts = append(textParts, fmt.Sprintf("%s: <unserializable function response>", part.FunctionResponse.Name))
				continue
			}
			label := part.FunctionResponse.Name
			if label == "" {
				textParts = append(textParts, string(payload))
			} else {
				textParts = append(textParts, fmt.Sprintf("%s: %s", label, string(payload)))
			}
		case part.CodeExecutionResult != nil:
			resultText := strings.TrimSpace(part.CodeExecutionResult.Output)
			if resultText != "" {
				textParts = append(textParts, resultText)
			}
		case part.ExecutableCode != nil:
			code := strings.TrimSpace(part.ExecutableCode.Code)
			if code != "" {
				textParts = append(textParts, fmt.Sprintf("Executable code (%s):\n%s", part.ExecutableCode.Language, code))
			}
		case part.InlineData != nil && len(part.InlineData.Data) > 0:
			textParts = append(textParts, fmt.Sprintf("[model returned %d bytes of %s data]", len(part.InlineData.Data), part.InlineData.MIMEType))
		case part.FileData != nil && part.FileData.FileURI != "":
			textParts = append(textParts, fmt.Sprintf("[model referenced file: %s]", part.FileData.FileURI))
		}
	}
	if len(textParts) > 0 {
		chunk.Text = strings.Join(textParts, "")
	}
	if len(thoughtParts) > 0 {
		chunk.Thinking = strings.Join(thoughtParts, "")
	}
	if chunk.Text == "" && chunk.Thinking == "" && len(chunk.ToolCalls) == 0 {
		return nil
	}
	return chunk
}
func (g *Client) convertFunctionCall(call *genai.FunctionCall) *ai.ToolCallChunk {
	if call == nil {
		return nil
	}
	parameters := make(map[string]any, len(call.Args))
	for k, v := range call.Args {
		parameters[k] = v
	}
	return &ai.ToolCallChunk{
		ID:         call.ID,
		Name:       call.Name,
		Parameters: parameters,
	}
}
func normalizeToolIterations(value int32) int {
	if value <= 0 {
		return defaultMaxToolIterations
	}
	return int(value)
}
func buildGeminiImageContent(img *toolpayload.Payload) *genai.Content {
	var parts []*genai.Part
	if text := toolpayload.SanitizePath(img.Path); text != "" {
		parts = append(parts, genai.NewPartFromText(fmt.Sprintf("Image retrieved from %s", text)))
	}
	parts = append(parts, &genai.Part{
		InlineData: &genai.Blob{
			Data:     img.Data,
			MIMEType: img.MIMEType,
		},
	})
	return genai.NewContentFromParts(parts, genai.RoleUser)
}
func buildGeminiDocumentContent(doc *toolpayload.Payload) *genai.Content {
	parts := []*genai.Part{
		genai.NewPartFromText(fmt.Sprintf("Document retrieved from %s (MIME: %s, %d bytes)", toolpayload.SanitizePath(doc.Path), doc.MIMEType, doc.SizeBytes)),
		{
			InlineData: &genai.Blob{
				Data:     doc.Data,
				MIMEType: doc.MIMEType,
			},
		},
	}
	return genai.NewContentFromParts(parts, genai.RoleUser)
}
func (g *Client) joinContentParts(content *genai.Content) string {
	var (
		textParts    []string
		thoughtParts []string
		extraParts   []string
		showThoughts = g.Config.GetBoolWithDefault("GEMINI_SHOW_THOUGHTS", false)
	)
	for _, part := range content.Parts {
		switch {
		case part.Text != "":
			if part.Thought {
				thoughtParts = append(thoughtParts, part.Text)
				if showThoughts {
					notification := events.NotificationEvent{
						Message:     fmt.Sprintf("Thinking: %s", part.Text),
						ContentType: "thought",
					}
					g.EventBus.Publish(notification.Topic(), notification)
				}
			} else {
				textParts = append(textParts, part.Text)
			}
		case part.FunctionResponse != nil && part.FunctionResponse.Response != nil:
			payload, err := json.Marshal(part.FunctionResponse.Response)
			if err != nil {
				extraParts = append(extraParts, fmt.Sprintf("%s: <unserializable function response>", part.FunctionResponse.Name))
				continue
			}
			label := part.FunctionResponse.Name
			if label == "" {
				extraParts = append(extraParts, string(payload))
			} else {
				extraParts = append(extraParts, fmt.Sprintf("%s: %s", label, string(payload)))
			}
		case part.FunctionCall != nil:
			payload, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				extraParts = append(extraParts, fmt.Sprintf("Function call %s(<unserializable args>)", part.FunctionCall.Name))
				continue
			}
			extraParts = append(extraParts, fmt.Sprintf("Function call %s(%s)", part.FunctionCall.Name, string(payload)))
		case part.CodeExecutionResult != nil:
			resultText := strings.TrimSpace(part.CodeExecutionResult.Output)
			if resultText != "" {
				extraParts = append(extraParts, resultText)
			}
		case part.ExecutableCode != nil:
			code := strings.TrimSpace(part.ExecutableCode.Code)
			if code != "" {
				extraParts = append(extraParts, fmt.Sprintf("Executable code (%s):\n%s", part.ExecutableCode.Language, code))
			}
		case part.InlineData != nil && len(part.InlineData.Data) > 0:
			extraParts = append(extraParts, fmt.Sprintf("[model returned %d bytes of %s data]", len(part.InlineData.Data), part.InlineData.MIMEType))
		case part.FileData != nil && part.FileData.FileURI != "":
			extraParts = append(extraParts, fmt.Sprintf("[model referenced file: %s]", part.FileData.FileURI))
		}
	}
	if len(textParts) > 0 {
		return strings.Join(textParts, "")
	}
	if len(extraParts) > 0 {
		return strings.Join(extraParts, "\n")
	}
	if len(thoughtParts) > 0 {
		return thoughtParts[len(thoughtParts)-1]
	}
	return ""
}
func (g *Client) invokeGenerateContent(ctx context.Context, modelName string, contents []*genai.Content, config *genai.GenerateContentConfig, handlers map[string]ai.HandlerFunc, maxIterations int) (result *genai.GenerateContentResponse, toolUsed bool, err error) {
	return g.callGenerateContent(ctx, modelName, contents, config, handlers, maxIterations)
}
func (g *Client) countTokensWithPrompt(ctx context.Context, p ai.Prompt) (*ai.TokenCount, error) {
	// Build the content parts for the user message
	parts := []*genai.Part{
		genai.NewPartFromText(p.Text),
	}
	// Add images if present
	for _, img := range p.Images {
		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{
				Data:     img.Data,
				MIMEType: img.Type,
			},
		})
	}
	// Create the user content with proper role
	userContent := genai.NewContentFromParts(parts, genai.RoleUser)
	// Build contents array
	var contents []*genai.Content
	// Add system instruction as a separate content if provided
	if p.Instruction != "" {
		// System instructions are typically handled as user content in the unified API
		systemParts := []*genai.Part{genai.NewPartFromText(p.Instruction)}
		systemContent := genai.NewContentFromParts(systemParts, genai.RoleUser)
		contents = []*genai.Content{systemContent, userContent}
	} else {
		contents = []*genai.Content{userContent}
	}
	// Use model name from prompt, or fallback to default
	modelName := p.ModelName
	if modelName == "" {
		modelName = "gemini-2.0-flash" // Default model
	}
	// Count tokens using the Models.CountTokens method
	countResp, err := g.Client.Models.CountTokens(ctx, modelName, contents, nil)
	if err != nil {
		return nil, fmt.Errorf("error counting tokens: %w", err)
	}
	// Convert the response to our TokenCount type
	tokenCount := &ai.TokenCount{
		TotalTokens: countResp.TotalTokens,
	}
	// Check if the response has detailed token counts (may not be available in all backends)
	if countResp.CachedContentTokenCount != 0 {
		tokenCount.InputTokens = countResp.CachedContentTokenCount
	}
	return tokenCount, nil
}
// mapAttr converts a slice of Attr to a map
func (g *Client) mapAttr(attrs []ai.Attr) map[string]string {
	m := make(map[string]string)
	for _, attr := range attrs {
		m[attr.Key] = attr.Value
	}
	return m
}
// callGenerateContent executes the generation loop until the model returns a response without tool calls.
func (g *Client) callGenerateContent(ctx context.Context, modelName string, contents []*genai.Content, config *genai.GenerateContentConfig, handlers map[string]ai.HandlerFunc, maxIterations int) (result *genai.GenerateContentResponse, toolUsed bool, err error) {
	currentContents := make([]*genai.Content, len(contents))
	copy(currentContents, contents)
	currentConfig := config
	var (
		updatedContents []*genai.Content
		done            bool
		stepUsedTool    bool
	)
	limit := maxIterations
	if limit <= 0 {
		limit = defaultMaxToolIterations
	}
	for iteration := 0; ; iteration++ {
		result, updatedContents, done, stepUsedTool, err = g.executeGenerationStep(ctx, modelName, currentContents, currentConfig, handlers)
		if err != nil {
			return nil, false, err
		}
		toolUsed = toolUsed || stepUsedTool
		if done {
			return result, toolUsed, nil
		}
		if iteration+1 >= limit {
			return nil, toolUsed, fmt.Errorf("exceeded maximum tool call iterations (%d) without completion", limit)
		}
		currentContents = updatedContents
	}
}
func (g *Client) executeGenerationStep(ctx context.Context, modelName string, contents []*genai.Content, config *genai.GenerateContentConfig, handlers map[string]ai.HandlerFunc) (result *genai.GenerateContentResponse, updatedContents []*genai.Content, done bool, toolUsed bool, err error) {
	if g.callGenerateContentFn != nil {
		result, err = g.callGenerateContentFn(ctx, modelName, contents, config, handlers)
	} else {
		result, err = g.Client.Models.GenerateContent(ctx, modelName, contents, config)
	}
	if err != nil {
		return nil, nil, false, false, fmt.Errorf("error generating content: %w", err)
	}
	g.publishUsageMetadata(result.UsageMetadata)
	fnCalls := result.FunctionCalls()
	if len(fnCalls) == 0 {
		if len(result.Candidates) == 0 {
			return nil, nil, false, false, fmt.Errorf("no candidates generated")
		}
		return result, nil, true, false, nil
	}
	updatedContents, err = g.handleFunctionCalls(ctx, result, contents, handlers, true)
	if err != nil {
		return nil, nil, false, true, err
	}
	if config != nil {
		config.ToolConfig = nil
	}
	return nil, updatedContents, false, true, nil
}
func (g *Client) publishUsageMetadata(usage *genai.GenerateContentResponseUsageMetadata) *ai.TokenCount {
	if usage == nil {
		return nil
	}
	tokenCountEvent := events.TokenCountEvent{
		InputTokens:   usage.PromptTokenCount,
		OutputTokens:  usage.CachedContentTokenCount,
		TotalTokens:   usage.TotalTokenCount,
		CachedTokens:  usage.CachedContentTokenCount,
		ToolUseTokens: usage.ToolUsePromptTokenCount,
	}
	g.EventBus.Publish(tokenCountEvent.Topic(), tokenCountEvent)
	if g.Config.GetBoolWithDefault("GENIE_TOKEN_DEBUG", false) {
		if usageMetadata, err := json.MarshalIndent(usage, "", "  "); err == nil {
			notification := events.NotificationEvent{
				Message: string(usageMetadata),
			}
			g.EventBus.Publish(notification.Topic(), notification)
		}
	}
	return &ai.TokenCount{
		TotalTokens:  usage.TotalTokenCount,
		InputTokens:  usage.PromptTokenCount,
		OutputTokens: usage.CachedContentTokenCount,
	}
}
func (g *Client) handleFunctionCalls(ctx context.Context, result *genai.GenerateContentResponse, contents []*genai.Content, handlers map[string]ai.HandlerFunc, emitNotification bool) ([]*genai.Content, error) {
	fnCalls := result.FunctionCalls()

	// Debug: log function calls to diagnose duplicate execution issues
	if len(fnCalls) > 0 {
		log.Printf("DEBUG handleFunctionCalls: received %d function call(s)", len(fnCalls))
		for i, fc := range fnCalls {
			log.Printf("DEBUG handleFunctionCalls: [%d] name=%s args=%v", i, fc.Name, fc.Args)
		}
	}

	updatedContents := make([]*genai.Content, len(contents))
	copy(updatedContents, contents)
	// Compact prior iteration contents to prevent unbounded context growth.
	// Replaces heavy InlineData blobs and truncates large FunctionResponse payloads.
	compactPriorContents(updatedContents)
	if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
		if emitNotification {
			contentStr := strings.TrimSpace(g.joinContentParts(result.Candidates[0].Content))
			if contentStr != "" {
				notification := events.NotificationEvent{
					Message: contentStr,
				}
				g.EventBus.Publish(notification.Topic(), notification)
			}
		}
		updatedContents = append(updatedContents, result.Candidates[0].Content)
	}
	responseParts := make([]*genai.Part, 0, len(fnCalls))
	// Collect media contents (images, documents) to append AFTER the function response.
	// Inserting them between the model's function call and the function response
	// breaks the Gemini function calling protocol and causes the API to hang.
	var mediaContents []*genai.Content
	for _, fnCall := range fnCalls {
		handler := handlers[fnCall.Name]
		if handler == nil {
			return nil, fmt.Errorf("no handler found for function %q", fnCall.Name)
		}
		handlerResp, err := handler(ctx, fnCall.Args)
		if err != nil {
			return nil, fmt.Errorf("error handling function %q: %w", fnCall.Name, err)
		}
		switch fnCall.Name {
		case "viewImage":
			img, sanitized, err := toolpayload.Extract(handlerResp)
			if err != nil {
				return nil, fmt.Errorf("invalid viewImage response: %w", err)
			}
			handlerResp = sanitized
			if img != nil {
				mediaContents = append(mediaContents, buildGeminiImageContent(img))
			}
		case "viewDocument":
			doc, sanitized, err := toolpayload.Extract(handlerResp)
			if err != nil {
				return nil, fmt.Errorf("invalid viewDocument response: %w", err)
			}
			handlerResp = sanitized
			if doc != nil {
				mediaContents = append(mediaContents, buildGeminiDocumentContent(doc))
			}
		}
		part := genai.NewPartFromFunctionResponse(fnCall.Name, handlerResp)
		// Echo back the FunctionCall ID so the model can match responses to calls
		if fnCall.ID != "" {
			part.FunctionResponse.ID = fnCall.ID
		}
		responseParts = append(responseParts, part)
	}
	// Combine all function responses into a single Content to match API expectations
	if len(responseParts) > 0 {
		updatedContents = append(updatedContents, &genai.Content{
			Parts: responseParts,
			Role:  string(roleFunctionResponse),
		})
	}
	// Append media contents AFTER the function response
	updatedContents = append(updatedContents, mediaContents...)
	return updatedContents, nil
}

// compactPriorContents replaces InlineData blobs from prior tool-call iterations
// with lightweight text placeholders to prevent unbounded context growth.
//
// A single viewImage adds ~93KB of binary data that gets re-sent every subsequent
// iteration. With 5 more iterations after viewImage, that's ~465KB of redundant data.
// This replaces those blobs with a small "[previously loaded image/jpeg, 93201 bytes]"
// placeholder while leaving all other content (FunctionResponse, text, etc.) intact.
func compactPriorContents(contents []*genai.Content) {
	for _, content := range contents {
		if content == nil {
			continue
		}
		for i, part := range content.Parts {
			if part == nil {
				continue
			}
			if part.InlineData != nil && len(part.InlineData.Data) > 0 {
				placeholder := fmt.Sprintf("[previously loaded %s, %d bytes]",
					part.InlineData.MIMEType, len(part.InlineData.Data))
				content.Parts[i] = genai.NewPartFromText(placeholder)
			}
		}
	}
}
