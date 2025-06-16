package ai

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"cloud.google.com/go/vertexai/genai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/fileops"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Gen interface {
	GenerateContent(p Prompt, debug bool, args ...string) (string, error)
	GenerateContentAttr(prompt Prompt, debug bool, attrs []Attr) (string, error)
}

// An Attr is a key-value pair.
type Attr struct {
	Key   string
	Value string
}

type Image struct {
	Type     string `yaml:"type"`
	Filename string `yaml:"filename"`
	Data     []byte `yaml:"data"`
}

type Prompt struct {
	Name           string   `yaml:"name"`
	Instruction    string   `yaml:"instruction"`
	Text           string   `yaml:"text"`
	Images         []*Image `yaml:"images"`
	Functions      []*FunctionDeclaration
	ResponseSchema *Schema                `yaml:"response_schema"`
	Handlers       map[string]HandlerFunc `yaml:"-"`
	ModelName      string                 `yaml:"model_name"`
	MaxTokens      int32                  `yaml:"max_tokens"`
	Temperature    float32                `yaml:"temperature"`
	TopP           float32                `yaml:"top_p"`
}

type FunctionDeclaration struct {
	Name        string
	Description string
	Parameters  *Schema
	Response    *Schema
}

type Type int32

const (
	TypeString  Type = 1
	TypeNumber  Type = 2
	TypeInteger Type = 3
	TypeBoolean Type = 4
	TypeArray   Type = 5
	TypeObject  Type = 6
)

type Schema struct {
	Type          Type               `yaml:"type"`
	Format        string             `yaml:"format"`
	Title         string             `yaml:"title"`
	Description   string             `yaml:"description"`
	Nullable      bool               `yaml:"nullable"`
	Items         *Schema            `yaml:"items"`
	MinItems      int64              `yaml:"min_items"`
	MaxItems      int64              `yaml:"max_items"`
	Enum          []string           `yaml:"enum"`
	Properties    map[string]*Schema `yaml:"properties"`
	Required      []string           `yaml:"required"`
	MinProperties int64              `yaml:"min_properties"`
	MaxProperties int64              `yaml:"max_properties"`
	Minimum       float64            `yaml:"minimum"`
	Maximum       float64            `yaml:"maximum"`
	MinLength     int64              `yaml:"min_length"`
	MaxLength     int64              `yaml:"max_length"`
	Pattern       string             `yaml:"pattern"`
}

type FunctionResponse struct {
	Name     string
	Response map[string]any
}

type HandlerFunc func(ctx context.Context, attr map[string]any) (map[string]any, error)

type VertexGen struct {
	Client      *genai.Client
	FileManager fileops.Manager
	Config      config.Manager
}

var _ Gen = &VertexGen{}

func NewVertexGen() Gen {
	configManager := config.NewManager()
	
	projectID := configManager.RequireString("GOOGLE_CLOUD_PROJECT")
	location := configManager.GetStringWithDefault("GOOGLE_CLOUD_LOCATION", "us-central1")

	ctx := context.Background()
	client, err := genai.NewClient(ctx, projectID, location)
	//, option.WithGRPCDialOption(grpc.WithUnaryInterceptor(LoggingInterceptor)))
	if err != nil {
		panic(fmt.Sprintf("error creating client: %s", err.Error()))
	}

	return &VertexGen{
		Client:      client,
		FileManager: fileops.NewManager(),
		Config:      configManager,
	}
}

func (g *VertexGen) GenerateContent(p Prompt, debug bool, args ...string) (string, error) {
	attrs := StringsToAttr(args)
	prompt, err := g.renderPrompt(p, debug, attrs)
	if err != nil {
		return "", fmt.Errorf("error rendering prompt: %w", err)
	}

	return g.generateContent(prompt)
}

func (g *VertexGen) generateContent(p *Prompt) (string, error) {
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
	ctx := context.Background()
	resp, err := g.callGemini(ctx, gemini, p.Handlers, parts...)
	if err != nil {
		return "", fmt.Errorf("error generating content: %w", err)
	}

	// Get the best candidate (for now, just the first one)
	bestCandidate := resp.Candidates[0]

	// Assemble the parts into a single string
	var summaryParts []string
	for _, part := range bestCandidate.Content.Parts {
		summaryParts = append(summaryParts, fmt.Sprintf("%s", part))
	}
	summary := strings.Join(summaryParts, " ")

	return summary, nil
}

func (g *VertexGen) callGemini(ctx context.Context, gemini *genai.GenerativeModel, handlers map[string]HandlerFunc, parts ...genai.Part) (*genai.GenerateContentResponse, error) {
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
		log.Printf("Max calls %d reached.", maxCalls)
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
			Response: map[string]any{
				"content": handlerResp,
			},
		}
		parts = append(parts, fResp)
	}

	return g.callGemini(ctx, gemini, handlers, parts...)
}

func (g *VertexGen) GenerateContentAttr(prompt Prompt, debug bool, attrs []Attr) (string, error) {
	p, err := g.renderPrompt(prompt, debug, attrs)
	if err != nil {
		return "", fmt.Errorf("error rendering prompt: %w", err)
	}

	return g.generateContent(p)
}

// mapAttr converts a slice of Attr to a map
func (g *VertexGen) mapAttr(attrs []Attr) map[string]string {
	m := make(map[string]string)
	for _, attr := range attrs {
		m[attr.Key] = attr.Value
	}
	return m
}

// renderPromt renders the prompt into another prompt with the given attributes
func (g *VertexGen) renderPrompt(prompt Prompt, debug bool, attrs []Attr) (*Prompt, error) {
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
	p, err := RenderPrompt(prompt, g.mapAttr(attrs))
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

func (g *VertexGen) saveObjectToTmpFile(object interface{}, filename string) error {
	filePath := filepath.Join("tmp", filename)
	return g.FileManager.WriteObjectAsYAML(filePath, object)
}

func (g *VertexGen) mapFunctions(functions *[]*FunctionDeclaration) []*genai.FunctionDeclaration {
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

func (g *VertexGen) mapSchema(schema *Schema) *genai.Schema {
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

func (g *VertexGen) mapSchemaMap(schemaMap map[string]*Schema) map[string]*genai.Schema {
	if schemaMap == nil {
		return nil
	}
	genSchemaMap := make(map[string]*genai.Schema)
	for k, v := range schemaMap {
		genSchemaMap[k] = g.mapSchema(v)
	}
	return genSchemaMap
}

func (g *VertexGen) mapType(t Type) genai.Type {
	switch t {
	case TypeString:
		return genai.TypeString
	case TypeNumber:
		return genai.TypeNumber
	case TypeInteger:
		return genai.TypeInteger
	case TypeBoolean:
		return genai.TypeBoolean
	case TypeArray:
		return genai.TypeArray
	case TypeObject:
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
	// Convert the request to a proto.Message if possible.
	if pbReq, ok := req.(proto.Message); ok {
		// For JSON logging, use protojson.Marshal():
		jsonReq, _ := protojson.Marshal(pbReq)
		log.Printf("[GRPC] >>> Method: %s\n>>> Request: %s\n", method, string(jsonReq))
	} else {
		// If it’s not a proto message, just log the struct.
		log.Printf("[GRPC] >>> Method: %s\n>>> Request: %+v\n", method, req)
	}

	// Make the actual gRPC call.
	err := invoker(ctx, method, req, reply, cc, opts...)

	// Optionally log the response if needed
	if pbReply, ok := reply.(proto.Message); ok {
		jsonReply, _ := protojson.Marshal(pbReply)
		log.Printf("[GRPC] <<< Reply: %s\n", string(jsonReply))
	}
	if err != nil {
		log.Printf("[GRPC] <<< Error: %v\n", err)
	}

	return err
}
