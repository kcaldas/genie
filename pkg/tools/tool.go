package tools

import "github.com/kcaldas/genie/pkg/ai"

// Tool represents a tool that can be called by the AI
type Tool interface {
	// Declaration returns the function declaration for this tool
	Declaration() *ai.FunctionDeclaration
	
	// Handler returns the function handler for this tool
	Handler() ai.HandlerFunc
	
	// FormatOutput formats the tool's execution result for user display
	// The result parameter should match the tool's response schema
	FormatOutput(result map[string]interface{}) string
}