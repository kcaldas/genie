package genai

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
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
}

var _ ai.Gen = &Client{}

// NewClient creates a new unified GenAI client with automatic backend detection
func NewClient() ai.Gen {
	client, err := NewClientWithError()
	if err != nil {
		panic(fmt.Sprintf("error creating GenAI client: %s", err.Error()))
	}
	return client
}

// NewClientWithError creates a new unified GenAI client and returns an error if configuration is missing
func NewClientWithError() (ai.Gen, error) {
	configManager := config.NewConfigManager()

	// Determine backend preference
	backend := Backend(configManager.GetStringWithDefault("GENAI_BACKEND", "gemini"))

	// Try to create client based on backend preference
	client, actualBackend, err := createClientWithBackend(configManager, backend)
	if err != nil {
		// If preferred backend fails, try the other one
		var fallbackBackend Backend
		if backend == BackendGeminiAPI {
			fallbackBackend = BackendVertexAI
		} else {
			fallbackBackend = BackendGeminiAPI
		}

		client, actualBackend, err = createClientWithBackend(configManager, fallbackBackend)
		if err != nil {
			// Both backends failed - provide helpful message with both options
			return nil, fmt.Errorf("no valid AI backend configured. Please set up one of the following:\n\n" +
				"Option 1 - Gemini API (recommended):\n" +
				"  export GEMINI_API_KEY=your-api-key\n" +
				"  Get your API key from: https://aistudio.google.com/apikey\n\n" +
				"Option 2 - Vertex AI:\n" +
				"  export GOOGLE_CLOUD_PROJECT=your-project-id\n" +
				"  Requires Google Cloud setup and authentication\n")
		}
	}

	return &Client{
		Client:          client,
		FileManager:     fileops.NewFileOpsManager(),
		Config:          configManager,
		TemplateManager: template.NewEngine(),
		Backend:         actualBackend,
	}, nil
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

		client, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  apiKey,
			Backend: genai.BackendGeminiAPI,
		})
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

		client, err := genai.NewClient(ctx, &genai.ClientConfig{
			Project:  projectID,
			Location: location,
			Backend:  genai.BackendVertexAI,
		})
		if err != nil {
			return nil, "", fmt.Errorf("error creating Vertex AI client: %w", err)
		}

		return client, BackendVertexAI, nil

	default:
		return nil, "", fmt.Errorf("unsupported backend: %s", backend)
	}
}

func (g *Client) GenerateContent(ctx context.Context, p ai.Prompt, debug bool, args ...string) (string, error) {
	attrs := ai.StringsToAttr(args)
	prompt, err := g.renderPrompt(p, debug, attrs)
	if err != nil {
		return "", fmt.Errorf("error rendering prompt: %w", err)
	}

	return g.generateContentWithPrompt(ctx, *prompt, debug)
}

func (g *Client) GenerateContentAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (string, error) {
	p, err := g.renderPrompt(prompt, debug, attrs)
	if err != nil {
		return "", fmt.Errorf("error rendering prompt: %w", err)
	}

	return g.generateContentWithPrompt(ctx, *p, debug)
}

// GetStatus returns the connection status and backend information
func (g *Client) GetStatus() (connected bool, backend string, message string) {
	// Check if we have the required configuration for our current backend
	switch g.Backend {
	case BackendGeminiAPI:
		apiKey := g.Config.GetStringWithDefault("GEMINI_API_KEY", "")
		if apiKey == "" {
			return false, "gemini", "GEMINI_API_KEY not configured"
		}
		return true, "gemini", "Gemini API configured"
	
	case BackendVertexAI:
		projectID := g.Config.GetStringWithDefault("GOOGLE_CLOUD_PROJECT", "")
		if projectID == "" {
			return false, "vertex", "GOOGLE_CLOUD_PROJECT not configured"
		}
		location := g.Config.GetStringWithDefault("GOOGLE_CLOUD_LOCATION", "us-central1")
		return true, "vertex", fmt.Sprintf("Vertex AI configured (project: %s, location: %s)", projectID, location)
	
	default:
		return false, "unknown", fmt.Sprintf("Unknown backend: %s", g.Backend)
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
	if p.Functions != nil {
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

	// Generate content with function calling support
	result, err := g.callGenerateContent(ctx, p.ModelName, contents, config, p.Handlers)
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

	// Extract text parts from the response
	var textParts []string
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			textParts = append(textParts, part.Text)
		}
	}

	response := strings.Join(textParts, "")

	// Debug logging for empty responses
	if response == "" {
		logger := logging.NewAPILogger("genai")
		logger.Debug("empty response received",
			"candidates", len(result.Candidates),
			"content_parts", len(candidate.Content.Parts))
	}

	return response, nil
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

// callGenerateContent handles the recursive function calling
func (g *Client) callGenerateContent(ctx context.Context, modelName string, contents []*genai.Content, config *genai.GenerateContentConfig, handlers map[string]ai.HandlerFunc) (*genai.GenerateContentResponse, error) {
	// Generate content using the Models.GenerateContent method
	result, err := g.Client.Models.GenerateContent(ctx, modelName, contents, config)
	if err != nil {
		return nil, fmt.Errorf("error generating content: %w", err)
	}

	// Check if we have any candidates
	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates generated")
	}

	// Check for function calls
	if len(result.FunctionCalls()) == 0 {
		// Base case: no function calls => done
		return result, nil
	}
	fnCalls := result.FunctionCalls()

	// Handle max calls limit
	maxCalls := ctx.Value("maxCalls")
	if maxCalls == nil {
		ctx = context.WithValue(ctx, "maxCalls", 8)
	}

	ctxCalls := ctx.Value("calls")
	if ctxCalls == nil {
		ctx = context.WithValue(ctx, "calls", 0)
	}

	callsSoFar := ctx.Value("calls").(int)
	if callsSoFar >= ctx.Value("maxCalls").(int) {
		logger := logging.NewAPILogger("genai")
		logger.Warn("maximum function calls reached, making final call without tools", "maxCalls", maxCalls)

		// Make final call without tools to force a text-only response
		finalConfig := &genai.GenerateContentConfig{}
		if config != nil && config.SystemInstruction != nil {
			finalConfig.SystemInstruction = config.SystemInstruction
		}
		// Deliberately omit Tools and ToolConfig to force a text-only response

		finalResult, err := g.Client.Models.GenerateContent(ctx, modelName, contents, finalConfig)
		if err != nil {
			return nil, fmt.Errorf("error in final call without tools: %w", err)
		}

		return finalResult, nil
	}
	ctx = context.WithValue(ctx, "calls", callsSoFar+1)

	// Execute function calls and build new content
	newContents := make([]*genai.Content, len(contents))
	copy(newContents, contents)

	// CRITICAL: Append the model's response content (with function calls) first
	if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
		newContents = append(newContents, result.Candidates[0].Content)
	}

	// Process each function call and create proper function response parts
	var functionResponseParts []*genai.Part
	for _, fnCall := range fnCalls {
		handler := handlers[fnCall.Name]
		if handler == nil {
			return nil, fmt.Errorf("no handler found for function %q", fnCall.Name)
		}
		handlerResp, err := handler(ctx, fnCall.Args)
		if err != nil {
			return nil, fmt.Errorf("error handling function %q: %w", fnCall.Name, err)
		}

		// Create function response part using the proper method
		fRespPart := &genai.Part{
			FunctionResponse: &genai.FunctionResponse{
				Name:     fnCall.Name,
				Response: handlerResp,
			},
		}
		functionResponseParts = append(functionResponseParts, fRespPart)
	}

	// Add function responses as user content
	if len(functionResponseParts) > 0 {
		fRespContent := &genai.Content{
			Role:  genai.RoleUser,
			Parts: functionResponseParts,
		}
		newContents = append(newContents, fRespContent)
	}

	// Recursive call with updated contents
	return g.callGenerateContent(ctx, modelName, newContents, config, handlers)
}
