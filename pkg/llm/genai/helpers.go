package genai

import (
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/fileops"
	"github.com/kcaldas/genie/pkg/llm/shared"
	"google.golang.org/genai"
)

// buildSystemInstruction returns the stable system blocks as one part, or
// nil when the prompt has none. Volatile context never goes here: Gemini's
// implicit cache matches the request prefix, and the system instruction
// is the head of it.
func buildSystemInstruction(layout shared.Conversation) *genai.Content {
	if layout.System == "" {
		return nil
	}
	return genai.NewContentFromParts([]*genai.Part{genai.NewPartFromText(layout.System)}, genai.RoleUser)
}

// buildInitialContents lays the conversation out the way Gemini caches it:
// one content per side of each past turn, then the current turn as the
// final user content carrying the volatile context, the message and any
// images. Gemini merges adjacent same-role contents and matches its
// implicit cache on whole contents, so the system blocks are NOT repeated
// here and nothing that changes per turn precedes the history.
func (g *Client) buildInitialContents(p ai.Prompt) []*genai.Content {
	return buildContents(shared.LayoutConversation(p))
}

func buildContents(layout shared.Conversation) []*genai.Content {
	contents := make([]*genai.Content, 0, 2*len(layout.Turns)+1)
	for _, turn := range layout.Turns {
		if user := strings.TrimSpace(turn.User); user != "" {
			contents = append(contents, genai.NewContentFromText(user, genai.RoleUser))
		}
		if assistant := shared.FormatAssistantTurn(turn); assistant != "" {
			contents = append(contents, genai.NewContentFromText(assistant, genai.RoleModel))
		}
	}
	return append(contents, buildTailContent(layout))
}

// countTokensRequest is the layout CountTokens sends, matching what the
// generate path bills. Vertex takes the system instruction in the count
// config; the Gemini API rejects it there, so it is counted as a leading
// user content instead.
func countTokensRequest(backend Backend, p ai.Prompt) ([]*genai.Content, *genai.CountTokensConfig) {
	layout := shared.LayoutConversation(p)
	contents := buildContents(layout)
	system := buildSystemInstruction(layout)
	if system == nil {
		return contents, nil
	}
	if backend == BackendGeminiAPI {
		return append([]*genai.Content{system}, contents...), nil
	}
	return contents, &genai.CountTokensConfig{SystemInstruction: system}
}

// buildTailContent is the current turn: volatile context and message as
// one text part, followed by the images.
func buildTailContent(layout shared.Conversation) *genai.Content {
	parts := []*genai.Part{genai.NewPartFromText(layout.TailText())}
	for _, img := range layout.Images {
		if img == nil {
			continue
		}
		parts = append(parts, &genai.Part{InlineData: &genai.Blob{Data: img.Data, MIMEType: img.Type}})
	}
	return genai.NewContentFromParts(parts, genai.RoleUser)
}

func (g *Client) buildGenerateConfig(p ai.Prompt) *genai.GenerateContentConfig {
	var cfg genai.GenerateContentConfig
	used := false

	if len(p.Functions) > 0 {
		// No ToolConfig: the default (AUTO) lets the model decide whether a
		// tool call is warranted. Forcing calls (ANY) made Gemini return
		// empty or malformed candidates when no tool fit the request.
		cfg.Tools = []*genai.Tool{{FunctionDeclarations: g.mapFunctions(&p.Functions)}}
		used = true
	}

	if system := buildSystemInstruction(shared.LayoutConversation(p)); system != nil {
		cfg.SystemInstruction = system
		used = true
	}

	if p.ResponseSchema != nil {
		cfg.ResponseMIMEType = "application/json"
		cfg.ResponseSchema = g.mapSchema(p.ResponseSchema)
		used = true
	}

	if p.MaxTokens > 0 {
		cfg.MaxOutputTokens = int32(p.MaxTokens)
		used = true
	}
	if p.Temperature > 0 {
		temp := float32(p.Temperature)
		cfg.Temperature = &temp
		used = true
	}
	if p.TopP > 0 {
		topP := float32(p.TopP)
		cfg.TopP = &topP
		used = true
	}
	if p.MaxTokens > 0 || p.Temperature > 0 || p.TopP > 0 {
		count := int32(1)
		cfg.CandidateCount = count
		used = true
	}

	includeThoughts := g.Config.GetBoolWithDefault("GEMINI_INCLUDE_THOUGHTS", false)
	isGemini3 := strings.Contains(strings.ToLower(p.ModelName), "gemini-3")

	// Gemini 3 requires ThinkingLevel (cannot disable thinking)
	// Gemini 2.5 uses ThinkingBudget
	if isGemini3 {
		thinkingLevel := genai.ThinkingLevel(g.Config.GetStringWithDefault("GEMINI_THINKING_LEVEL", "LOW"))
		cfg.ThinkingConfig = &genai.ThinkingConfig{
			ThinkingLevel:   thinkingLevel,
			IncludeThoughts: includeThoughts,
		}
		used = true
	} else if includeThoughts {
		thinkingBudgetVal := int32(g.Config.GetIntWithDefault("GEMINI_THINKING_BUDGET", -1))
		cfg.ThinkingConfig = &genai.ThinkingConfig{ThinkingBudget: &thinkingBudgetVal, IncludeThoughts: includeThoughts}
		used = true
	}

	if !used {
		return nil
	}
	return &cfg
}

func renderPrompt(fileManager fileops.Manager, prompt ai.Prompt, debug bool, attrs []ai.Attr) (*ai.Prompt, error) {
	return shared.RenderPromptWithDebug(fileManager, prompt, debug, attrs)
}

func (g *Client) mapFunctions(functions *[]*ai.FunctionDeclaration) []*genai.FunctionDeclaration {
	var genFunctions []*genai.FunctionDeclaration
	for _, f := range *functions {
		if f == nil {
			continue
		}
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

func (g *Client) mapSchemaMap(schemaMap map[string]*ai.Schema) map[string]*genai.Schema {
	if schemaMap == nil {
		return nil
	}
	genSchemaMap := make(map[string]*genai.Schema, len(schemaMap))
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
