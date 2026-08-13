package genai

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"
	"sync"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/fileops"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
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
	// Allows tests to intercept streaming generate content calls.
	generateContentStreamFn func(ctx context.Context, modelName string, contents []*genai.Content, config *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error]
	getModelFn              func(ctx context.Context, modelName string) (*genai.Model, error)
	countTokensFn           func(ctx context.Context, modelName string, contents []*genai.Content, config *genai.CountTokensConfig) (*genai.CountTokensResponse, error)
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
			"  Requires Google Cloud setup and authentication")
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
				"  Requires Google Cloud setup and authentication")
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

func (g *Client) CapabilityCacheKey(model string) ai.CapabilityCacheKey {
	model = g.resolveCapabilityModel(model)
	authority := "generativelanguage.googleapis.com"
	tenant := strings.TrimSpace(g.Config.GetStringWithDefault("GENIE_CAPABILITY_CACHE_NAMESPACE", ""))
	if g.Backend == BackendVertexAI {
		authority = "aiplatform.googleapis.com"
		if tenant == "" {
			tenant = strings.Join([]string{
				g.Config.GetStringWithDefault("GOOGLE_CLOUD_PROJECT", ""),
				g.Config.GetStringWithDefault("GOOGLE_CLOUD_LOCATION", "us-central1"),
			}, "/")
		}
	}
	return ai.CapabilityCacheKey{
		Provider:  "genai",
		Authority: authority,
		Tenant:    tenant,
		Model:     model,
	}
}

func (g *Client) DiscoverModelCapabilities(ctx context.Context, model string) (ai.ModelCapabilities, error) {
	if err := g.ensureInitialized(ctx); err != nil {
		return ai.ModelCapabilities{}, err
	}
	model = g.resolveCapabilityModel(model)
	var (
		metadata *genai.Model
		err      error
	)
	if g.getModelFn != nil {
		metadata, err = g.getModelFn(ctx, model)
	} else {
		metadata, err = g.Client.Models.Get(ctx, model, nil)
	}
	if err != nil {
		return ai.ModelCapabilities{}, fmt.Errorf("genai get model %q: %w", model, err)
	}
	resolvedModel := strings.TrimPrefix(metadata.Name, "models/")
	if resolvedModel == "" {
		resolvedModel = model
	}
	return ai.ModelCapabilities{
		Model:             resolvedModel,
		InputTokenLimit:   int(metadata.InputTokenLimit),
		OutputTokenLimit:  int(metadata.OutputTokenLimit),
		SupportsReasoning: metadata.Thinking,
		Source:            ai.CapabilitySourceProvider,
	}, nil
}

func (g *Client) CapabilityTTL(model string) time.Duration {
	model = strings.ToLower(model)
	if strings.Contains(model, "preview") || strings.HasSuffix(model, "-latest") {
		return time.Hour
	}
	return ai.DefaultCapabilityTTL
}

func (g *Client) resolveCapabilityModel(model string) string {
	if model = strings.TrimSpace(model); model != "" {
		return model
	}
	return strings.TrimSpace(g.Config.GetModelConfig().ModelName)
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
	turn := g.newTurn(p)
	return llmshared.RunToolLoop(ctx, turn, p.Handlers, g.loopConfig(p), nil)
}
func (g *Client) generateContentStreamWithPrompt(ctx context.Context, p ai.Prompt) (ai.Stream, error) {
	turn := g.newTurn(p)
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
		if _, err := llmshared.RunToolLoop(streamCtx, turn, p.Handlers, g.loopConfig(p), emit); err != nil {
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
func (g *Client) loopConfig(p ai.Prompt) llmshared.LoopConfig {
	return llmshared.NewLoopConfig(g.Config, g.EventBus, p, defaultMaxToolIterations)
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
		// Data event for consumers (session recording); the flagged
		// NotificationEvent above stays display-only. Synchronous so the
		// entry lands in the recording chain within the current turn.
		thinkingEvent := events.ThinkingEvent{Text: chunk.Thinking}
		g.EventBus.PublishSync(thinkingEvent.Topic(), thinkingEvent)
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

// buildGeminiBlobContent renders native tool content as inline data.
// Gemini takes images and documents through the same Blob part.
func buildGeminiBlobContent(blob ai.BlobContent) *genai.Content {
	parts := []*genai.Part{
		genai.NewPartFromText(llmshared.DescribeBlob(blob)),
		{
			InlineData: &genai.Blob{
				Data:     blob.Data,
				MIMEType: blob.MIMEType,
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
	// Add system instruction (with optional suffix) as a separate content
	if systemParts := buildSystemParts(p); len(systemParts) > 0 {
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
	// Convert the response to our TokenCount type. CountTokens estimates the
	// prompt size, so the total IS the input — there is no output yet.
	tokenCount := &ai.TokenCount{
		TotalTokens: countResp.TotalTokens,
		InputTokens: countResp.TotalTokens,
	}
	return tokenCount, nil
}

func (g *Client) publishUsageMetadata(modelName string, usage *genai.GenerateContentResponseUsageMetadata) *ai.TokenCount {
	if usage == nil {
		return nil
	}
	if strings.TrimSpace(modelName) == "" {
		modelName = g.Config.GetModelConfig().ModelName
	}
	// Gemini's PromptTokenCount INCLUDES cached content (cached is a subset).
	// Subtract so InputTokens means "uncached input" — matches Anthropic's
	// semantics so the cross-provider hit-rate math in the daemon works.
	tokenCountEvent := events.TokenCountEvent{
		Provider:             "gemini",
		Model:                modelName,
		InputTokens:          usage.PromptTokenCount - usage.CachedContentTokenCount,
		OutputTokens:         usage.CandidatesTokenCount,
		TotalTokens:          usage.TotalTokenCount,
		CachedTokens:         usage.CachedContentTokenCount,
		CacheReadInputTokens: usage.CachedContentTokenCount,
		ToolUseTokens:        usage.ToolUsePromptTokenCount,
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
		OutputTokens: usage.CandidatesTokenCount,
	}
}

// compactPriorContents replaces InlineData blobs from prior tool-call iterations
// with lightweight text placeholders to prevent unbounded context growth.
//
// A single viewImage adds ~93KB of binary data that gets re-sent every subsequent
// iteration. With 5 more iterations after viewImage, that's ~465KB of redundant data.
// This replaces those blobs with a small "[previously loaded image/jpeg, 93201 bytes]"
// placeholder while leaving all other content (FunctionResponse, text, etc.) intact.

// dedupeFunctionCallParts drops FunctionCall parts that are exact duplicates
// (same name and args) of an earlier call in the same response, keeping the
// first occurrence. A degenerate generation can repeat one complete call per
// streamed chunk — hundreds of copies in a single response — and executing
// them all fans out into duplicate tool side effects. Non-call parts are
// left untouched. Returns the number of parts dropped.
func dedupeFunctionCallParts(content *genai.Content) int {
	if content == nil || len(content.Parts) == 0 {
		return 0
	}
	seen := make(map[string]bool)
	deduped := content.Parts[:0]
	dropped := 0
	for _, part := range content.Parts {
		if part != nil && part.FunctionCall != nil {
			args, err := json.Marshal(part.FunctionCall.Args)
			if err == nil {
				key := part.FunctionCall.Name + ":" + string(args)
				if seen[key] {
					dropped++
					continue
				}
				seen[key] = true
			}
		}
		deduped = append(deduped, part)
	}
	content.Parts = deduped
	return dropped
}

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
