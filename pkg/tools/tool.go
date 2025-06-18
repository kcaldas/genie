package tools

import "github.com/kcaldas/genie/pkg/ai"

// Tool represents a tool that can be called by the AI
type Tool interface {
	// Declaration returns the function declaration for this tool
	Declaration() *ai.FunctionDeclaration
	
	// Handler returns the function handler for this tool
	Handler() ai.HandlerFunc
}