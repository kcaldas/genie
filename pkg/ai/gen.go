package ai

import (
	"context"
)

type Gen interface {
	GenerateContent(ctx context.Context, p Prompt, debug bool, args ...string) (string, error)
	GenerateContentAttr(ctx context.Context, prompt Prompt, debug bool, attrs []Attr) (string, error)
	CountTokens(ctx context.Context, p Prompt, debug bool, args ...string) (*TokenCount, error)
	CountTokensAttr(ctx context.Context, p Prompt, debug bool, attrs []Attr) (*TokenCount, error)
	GetStatus() *Status
}

type Status struct {
	Connected bool
	Backend   string
	Message   string
}

type TokenCount struct {
	TotalTokens  int32
	InputTokens  int32
	OutputTokens int32
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
	RequiredTools  []string `yaml:"required_tools"`
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
