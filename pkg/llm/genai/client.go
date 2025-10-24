package genai

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/fileops"
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/kcaldas/genie/pkg/template"
	"google.golang.org/genai"
)

// Backend represents the GenAI backend to use
type Backend string

const (
	BackendVertexAI  Backend = "vertex"
	BackendGeminiAPI Backend = "gemini"
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
	prompt, err := g.renderPrompt(p, debug, attrs)
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

	p, err := g.renderPrompt(prompt, debug, attrs)
	if err != nil {
		return "", fmt.Errorf("error rendering prompt: %w", err)
	}

	return g.generateContentWithPrompt(ctx, *p, debug)
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

	prompt, err := g.renderPrompt(p, debug, attrs)
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

	// Create generation config
	var config *genai.GenerateContentConfig

	// Handle function declarations as Tools
	if p.Functions != nil && len(p.Functions) > 0 {
		if config == nil {
			config = &genai.GenerateContentConfig{}
		}
		functionDecls := g.mapFunctions(&p.Functions)
		config.Tools = []*genai.Tool{
			{
				FunctionDeclarations: functionDecls,
			},
		}
		// Enable function calling mode
		config.ToolConfig = &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingConfigModeAny,
			},
		}
	}

	// Set system instruction
	if p.Instruction != "" {
		if config == nil {
			config = &genai.GenerateContentConfig{}
		}
		systemParts := []*genai.Part{genai.NewPartFromText(p.Instruction)}
		config.SystemInstruction = genai.NewContentFromParts(systemParts, genai.RoleUser)
	}

	// Handle response schema and generation parameters
	if p.ResponseSchema != nil {
		if config == nil {
			config = &genai.GenerateContentConfig{}
		}
		config.ResponseMIMEType = "application/json"
		config.ResponseSchema = g.mapSchema(p.ResponseSchema)
	}

	// Set generation parameters if provided
	if p.MaxTokens > 0 || p.Temperature > 0 || p.TopP > 0 {
		if config == nil {
			config = &genai.GenerateContentConfig{}
		}

		// Set generation parameters directly in config
		if p.MaxTokens > 0 {
			maxTokens := int32(p.MaxTokens)
			config.MaxOutputTokens = maxTokens
		}

		if p.Temperature > 0 {
			temp := float32(p.Temperature)
			config.Temperature = &temp
		}

		if p.TopP > 0 {
			topP := float32(p.TopP)
			config.TopP = &topP
		}

		// Set candidate count to 1 (typical for single response generation)
		candidateCount := int32(1)
		config.CandidateCount = candidateCount
	}

	// Turn on dynamic thinking:
	thinkingBudgetVal := int32(g.Config.GetIntWithDefault("GEMINI_THINKING_BUDGET", -1))
	includeThoughts := g.Config.GetBoolWithDefault("GEMINI_INCLUDE_THOUGHTS", false)

	if includeThoughts {
		config.ThinkingConfig = &genai.ThinkingConfig{
			ThinkingBudget:  &thinkingBudgetVal,
			IncludeThoughts: includeThoughts,
			// Turn off thinking:
			// ThinkingBudget: int32(0),
			// Turn on dynamic thinking:
			// ThinkingBudget: int32(-1),
		}
	}

	// Generate content with function calling support
	result, err := g.invokeGenerateContent(ctx, p.ModelName, contents, config, p.Handlers)
	if err != nil {
		return "", fmt.Errorf("error generating content: %w", err)
	}

	// Extract text from the response
	if len(result.Candidates) == 0 {
		return "", fmt.Errorf("no response candidates")
	}

	candidate := result.Candidates[0]
	if candidate.Content == nil {
		return "", fmt.Errorf("no content in response candidate")
	}

	response := g.joinContentParts(candidate.Content)

	// If we have candidates but no usable content, treat as error
	if response == "" {
		logger := logging.NewAPILogger("genai")
		logger.Debug("empty response received despite having candidates",
			"candidates", len(result.Candidates),
			"content_parts", len(candidate.Content.Parts))
		return "", fmt.Errorf("no usable content in response candidates")
	}

	return response, nil
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

func (g *Client) invokeGenerateContent(ctx context.Context, modelName string, contents []*genai.Content, config *genai.GenerateContentConfig, handlers map[string]ai.HandlerFunc) (*genai.GenerateContentResponse, error) {
	if g.callGenerateContentFn != nil {
		return g.callGenerateContentFn(ctx, modelName, contents, config, handlers)
	}
	return g.callGenerateContent(ctx, modelName, contents, config, handlers)
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

// renderPrompt renders the prompt into another prompt with the given attributes
func (g *Client) renderPrompt(prompt ai.Prompt, debug bool, attrs []ai.Attr) (*ai.Prompt, error) {
	if debug {
		err := g.saveObjectToTmpFile(prompt, fmt.Sprintf("%s-initial-prompt.yaml", prompt.Name))
		if err != nil {
			return nil, err
		}
		err = g.saveObjectToTmpFile(attrs, fmt.Sprintf("%s-attrs.yaml", prompt.Name))
		if err != nil {
			return nil, err
		}
	}
	p, err := ai.RenderPrompt(prompt, g.mapAttr(attrs))
	if err != nil {
		return nil, fmt.Errorf("error rendering prompt: %w", err)
	}
	if debug {
		err := g.saveObjectToTmpFile(p, fmt.Sprintf("%s-final-prompt.yaml", p.Name))
		if err != nil {
			return nil, err
		}
	}
	return &p, nil
}

func (g *Client) saveObjectToTmpFile(object interface{}, filename string) error {
	filePath := filepath.Join("tmp", filename)
	return g.FileManager.WriteObjectAsYAML(filePath, object)
}

// mapFunctions converts ai.FunctionDeclaration to genai.FunctionDeclaration
func (g *Client) mapFunctions(functions *[]*ai.FunctionDeclaration) []*genai.FunctionDeclaration {
	var genFunctions []*genai.FunctionDeclaration
	for _, f := range *functions {
		genFunction := &genai.FunctionDeclaration{
			Name:        f.Name,
			Description: f.Description,
			Parameters:  g.mapSchema(f.Parameters),
			Response:    g.mapSchema(f.Response),
		}
		genFunctions = append(genFunctions, genFunction)
	}
	return genFunctions
}

// mapSchema converts ai.Schema to genai.Schema
func (g *Client) mapSchema(schema *ai.Schema) *genai.Schema {
	if schema == nil {
		return nil
	}

	genSchema := &genai.Schema{
		Type:        g.mapType(schema.Type),
		Format:      schema.Format,
		Title:       schema.Title,
		Description: schema.Description,
		Items:       g.mapSchema(schema.Items),
		Enum:        schema.Enum,
		Properties:  g.mapSchemaMap(schema.Properties),
		Required:    schema.Required,
		Pattern:     schema.Pattern,
	}

	// Convert basic types to pointers where needed
	if schema.Nullable {
		genSchema.Nullable = &schema.Nullable
	}
	if schema.MinItems > 0 {
		genSchema.MinItems = &schema.MinItems
	}
	if schema.MaxItems > 0 {
		genSchema.MaxItems = &schema.MaxItems
	}
	if schema.MinProperties > 0 {
		genSchema.MinProperties = &schema.MinProperties
	}
	if schema.MaxProperties > 0 {
		genSchema.MaxProperties = &schema.MaxProperties
	}
	if schema.Minimum != 0 {
		genSchema.Minimum = &schema.Minimum
	}
	if schema.Maximum != 0 {
		genSchema.Maximum = &schema.Maximum
	}
	if schema.MinLength > 0 {
		genSchema.MinLength = &schema.MinLength
	}
	if schema.MaxLength > 0 {
		genSchema.MaxLength = &schema.MaxLength
	}

	return genSchema
}

// mapSchemaMap converts map of ai.Schema to map of genai.Schema
func (g *Client) mapSchemaMap(schemaMap map[string]*ai.Schema) map[string]*genai.Schema {
	if schemaMap == nil {
		return nil
	}
	genSchemaMap := make(map[string]*genai.Schema)
	for k, v := range schemaMap {
		genSchemaMap[k] = g.mapSchema(v)
	}
	return genSchemaMap
}

// mapType converts ai.Type to genai.Type
func (g *Client) mapType(t ai.Type) genai.Type {
	switch t {
	case ai.TypeString:
		return genai.TypeString
	case ai.TypeNumber:
		return genai.TypeNumber
	case ai.TypeInteger:
		return genai.TypeInteger
	case ai.TypeBoolean:
		return genai.TypeBoolean
	case ai.TypeArray:
		return genai.TypeArray
	case ai.TypeObject:
		return genai.TypeObject
	default:
		return genai.TypeUnspecified
	}
}

// callGenerateContent executes the generation loop until the model returns a response without tool calls.
func (g *Client) callGenerateContent(ctx context.Context, modelName string, contents []*genai.Content, config *genai.GenerateContentConfig, handlers map[string]ai.HandlerFunc) (*genai.GenerateContentResponse, error) {
	currentContents := make([]*genai.Content, len(contents))
	copy(currentContents, contents)
	currentConfig := config

	for {
		result, updatedContents, done, err := g.executeGenerationStep(ctx, modelName, currentContents, currentConfig, handlers)
		if err != nil {
			return nil, err
		}
		if done {
			return result, nil
		}
		currentContents = updatedContents
	}
}

func (g *Client) executeGenerationStep(ctx context.Context, modelName string, contents []*genai.Content, config *genai.GenerateContentConfig, handlers map[string]ai.HandlerFunc) (*genai.GenerateContentResponse, []*genai.Content, bool, error) {
	var (
		result *genai.GenerateContentResponse
		err    error
	)

	if g.callGenerateContentFn != nil {
		result, err = g.callGenerateContentFn(ctx, modelName, contents, config, handlers)
	} else {
		result, err = g.Client.Models.GenerateContent(ctx, modelName, contents, config)
	}
	if err != nil {
		return nil, nil, false, fmt.Errorf("error generating content: %w", err)
	}

	if usage := result.UsageMetadata; usage != nil {
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
	}

	fnCalls := result.FunctionCalls()
	if len(fnCalls) == 0 {
		if len(result.Candidates) == 0 {
			return nil, nil, false, fmt.Errorf("no candidates generated")
		}
		return result, nil, true, nil
	}

	newContents := make([]*genai.Content, len(contents))
	copy(newContents, contents)

	if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
		contentStr := strings.TrimSpace(g.joinContentParts(result.Candidates[0].Content))
		if contentStr != "" {
			notification := events.NotificationEvent{
				Message: contentStr,
			}
			g.EventBus.Publish(notification.Topic(), notification)
		}
		newContents = append(newContents, result.Candidates[0].Content)
	}

	functionResponseParts := make([]*genai.Part, 0, len(fnCalls))
	for _, fnCall := range fnCalls {
		handler := handlers[fnCall.Name]
		if handler == nil {
			return nil, nil, false, fmt.Errorf("no handler found for function %q", fnCall.Name)
		}

		handlerResp, err := handler(ctx, fnCall.Args)
		if err != nil {
			return nil, nil, false, fmt.Errorf("error handling function %q: %w", fnCall.Name, err)
		}

		functionResponseParts = append(functionResponseParts, &genai.Part{
			FunctionResponse: &genai.FunctionResponse{
				Name:     fnCall.Name,
				Response: handlerResp,
			},
		})
	}

	if len(functionResponseParts) > 0 {
		fRespContent := &genai.Content{
			Role:  genai.RoleUser,
			Parts: functionResponseParts,
		}
		newContents = append(newContents, fRespContent)
	}

	if config != nil {
		config.ToolConfig = nil
	}

	return nil, newContents, false, nil
}
