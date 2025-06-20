package vertex

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"cloud.google.com/go/vertexai/genai"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/fileops"
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/kcaldas/genie/pkg/template"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Client implements the ai.Gen interface using Google's Vertex AI
type Client struct {
	Client          *genai.Client
	FileManager     fileops.Manager
	Config          config.Manager
	TemplateManager template.Engine
}

var _ ai.Gen = &Client{}

// NewClient creates a new Vertex AI client
func NewClient() ai.Gen {
	client, err := NewClientWithError()
	if err != nil {
		panic(fmt.Sprintf("error creating Vertex AI client: %s", err.Error()))
	}
	return client
}

// NewClientWithError creates a new Vertex AI client and returns an error if configuration is missing
func NewClientWithError() (ai.Gen, error) {
	configManager := config.NewConfigManager()

	projectID, err := configManager.GetString("GOOGLE_CLOUD_PROJECT")
	if err != nil {
		return nil, fmt.Errorf("missing required environment variable GOOGLE_CLOUD_PROJECT\n\nTo use Vertex AI, please set your Google Cloud project ID:\n  export GOOGLE_CLOUD_PROJECT=your-project-id\n\nOr run with the environment variable:\n  GOOGLE_CLOUD_PROJECT=your-project-id genie ask \"your question\"")
	}

	location := configManager.GetStringWithDefault("GOOGLE_CLOUD_LOCATION", "us-central1")

	// Check for service account credentials
	var opts []option.ClientOption
	serviceAccountPath := configManager.GetStringWithDefault("GOOGLE_APPLICATION_CREDENTIALS", "")
	if serviceAccountPath != "" {
		opts = append(opts, option.WithCredentialsFile(serviceAccountPath))
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, projectID, location, opts...)
	//, option.WithGRPCDialOption(grpc.WithUnaryInterceptor(LoggingInterceptor)))
	if err != nil {
		return nil, fmt.Errorf("error creating Vertex AI client: %w", err)
	}

	return &Client{
		Client:          client,
		FileManager:     fileops.NewFileOpsManager(),
		Config:          configManager,
		TemplateManager: template.NewEngine(),
	}, nil
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

func (g *Client) generateContentWithPrompt(ctx context.Context, p ai.Prompt, debug bool) (string, error) {
	gemini := g.Client.GenerativeModel(p.ModelName)

	candidates := int32(1)

	// Set the generation configuration
	gemini.GenerationConfig = genai.GenerationConfig{
		MaxOutputTokens: &p.MaxTokens,
		Temperature:     &p.Temperature,
		CandidateCount:  &candidates,
		TopP:            &p.TopP,
	}

	if p.ResponseSchema != nil {
		gemini.GenerationConfig.ResponseMIMEType = "application/json"
		gemini.GenerationConfig.ResponseSchema = g.mapSchema(p.ResponseSchema)
	}

	// Set the safety settings
	gemini.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockMediumAndAbove,
		},
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockMediumAndAbove,
		},
		{
			Category:  genai.HarmCategorySexuallyExplicit,
			Threshold: genai.HarmBlockMediumAndAbove,
		},
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockMediumAndAbove,
		},
	}

	if p.Functions != nil {
		// Map prompt functions to model functions
		functionDecls := g.mapFunctions(&p.Functions)

		// Add the weather function to our model toolbox.
		gemini.Tools = []*genai.Tool{
			{
				FunctionDeclarations: functionDecls,
			},
		}
	}

	gemini.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(p.Instruction),
		},
	}

	parts := []genai.Part{
		genai.Text(p.Text),
	}

	for _, img := range p.Images {
		parts = append(parts, genai.ImageData(img.Type, img.Data))
	}

	// Generate the text response from the model
	resp, err := g.callGemini(ctx, gemini, p.Handlers, parts...)
	if err != nil {
		return "", fmt.Errorf("error generating content: %w", err)
	}

	// Extract only text content from the final response
	// The resp should be the final response with no function calls
	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no response candidates")
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil {
		return "", nil
	}

	// Verify this is indeed a final response (no function calls)
	if len(candidate.FunctionCalls()) > 0 {
		return "", fmt.Errorf("unexpected: final response still contains function calls")
	}

	// Extract only text parts from this final response
	var textParts []string
	for _, part := range candidate.Content.Parts {
		if textPart, ok := part.(genai.Text); ok {
			text := string(textPart)
			if strings.TrimSpace(text) != "" {
				textParts = append(textParts, text)
			}
		}
	}

	return strings.Join(textParts, ""), nil
}

func (g *Client) callGemini(ctx context.Context, gemini *genai.GenerativeModel, handlers map[string]ai.HandlerFunc, parts ...genai.Part) (*genai.GenerateContentResponse, error) {
	resp, err := gemini.GenerateContent(ctx, parts...)
	if err != nil {
		return nil, fmt.Errorf("error generating content: %w", err)
	}

	// Check if we have any candidates
	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates generated")
	}

	fnCalls := resp.Candidates[0].FunctionCalls()
	if len(fnCalls) == 0 {
		// base case: no function calls => done
		return resp, nil
	}

	maxCalls := ctx.Value("maxCalls")
	if maxCalls == nil {
		ctx = context.WithValue(ctx, "maxCalls", 3)
	}

	ctxCalls := ctx.Value("calls")
	if ctxCalls == nil {
		ctx = context.WithValue(ctx, "calls", 0)
	}

	callsSoFar := ctx.Value("calls").(int)
	if callsSoFar >= ctx.Value("maxCalls").(int) {
		logger := logging.NewAPILogger("vertex")
		logger.Warn("maximum function calls reached", "maxCalls", maxCalls)
		return resp, nil
	}
	ctx = context.WithValue(ctx, "calls", callsSoFar+1)

	for _, fnCall := range resp.Candidates[0].FunctionCalls() {
		handler := handlers[fnCall.Name]
		if handler == nil {
			return nil, fmt.Errorf("no handler found for function %q", fnCall.Name)
		}
		handlerResp, err := handler(ctx, fnCall.Args)
		if err != nil {
			return nil, fmt.Errorf("error handling function %q: %w", fnCall.Name, err)
		}
		fResp := genai.FunctionResponse{
			Name: fnCall.Name,
			Response: handlerResp,
		}
		parts = append(parts, fResp)
	}

	return g.callGemini(ctx, gemini, handlers, parts...)
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
		err := g.saveObjectToTmpFile(prompt, fmt.Sprintf("%s-initital-prompt.yaml", prompt.Name))
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

func (g *Client) mapSchema(schema *ai.Schema) *genai.Schema {
	if schema == nil {
		return nil
	}
	return &genai.Schema{
		Type:          g.mapType(schema.Type),
		Format:        schema.Format,
		Title:         schema.Title,
		Description:   schema.Description,
		Nullable:      schema.Nullable,
		Items:         g.mapSchema(schema.Items),
		MinItems:      schema.MinItems,
		MaxItems:      schema.MaxItems,
		Enum:          schema.Enum,
		Properties:    g.mapSchemaMap(schema.Properties),
		Required:      schema.Required,
		MinProperties: schema.MinProperties,
		MaxProperties: schema.MaxProperties,
		Minimum:       schema.Minimum,
		Maximum:       schema.Maximum,
		MinLength:     schema.MinLength,
		MaxLength:     schema.MaxLength,
		Pattern:       schema.Pattern,
	}
}

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

// LoggingInterceptor is a unary client interceptor that logs the request (and optionally the reply).
func LoggingInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	logger := logging.NewAPILogger("vertex-grpc")

	// Convert the request to a proto.Message if possible.
	if pbReq, ok := req.(proto.Message); ok {
		// For JSON logging, use protojson.Marshal():
		jsonReq, _ := protojson.Marshal(pbReq)
		logger.Debug("grpc request", "method", method, "request", string(jsonReq))
	} else {
		// If it's not a proto message, just log the struct.
		logger.Debug("grpc request", "method", method, "request", fmt.Sprintf("%+v", req))
	}

	// Make the actual gRPC call.
	err := invoker(ctx, method, req, reply, cc, opts...)

	// Optionally log the response if needed
	if pbReply, ok := reply.(proto.Message); ok {
		jsonReply, _ := protojson.Marshal(pbReply)
		logger.Debug("grpc response", "method", method, "response", string(jsonReply))
	}
	if err != nil {
		logger.Error("grpc error", "method", method, "error", err)
	}

	return err
}
