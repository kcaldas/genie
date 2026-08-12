package tools

import "github.com/kcaldas/genie/pkg/ai"

func resultOutput(details map[string]any) ai.ToolOutput {
	return ai.JSONToolOutput(details)
}

func failedOutput(details map[string]any) ai.ToolOutput {
	return ai.ErrorToolOutput(details)
}

func failResult(msg string) ai.ToolOutput {
	return failedOutput(map[string]any{
		"success": false,
		"results": "",
		"error":   msg,
	})
}
