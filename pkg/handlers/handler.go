package handlers

import "github.com/kcaldas/genie/pkg/ai"

// Re-export types from ai package for convenience
type ResponseHandler = ai.ResponseHandler
type HandlerRegistry = ai.HandlerRegistry

// ProcessingResult represents the result of processing a response
type ProcessingResult struct {
	Success      bool
	Message      string
	FilesCreated []string
	Errors       []string
}