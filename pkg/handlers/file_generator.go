package handlers

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/fileops"
	"github.com/pmezard/go-difflib/difflib"
)

// FileSpec represents a file to be created
type FileSpec struct {
	Path    string
	Content string
}

// FileGenerationHandler processes LLM responses to create files
type FileGenerationHandler struct {
	fileManager fileops.Manager
	eventBus    events.EventBus
	publisher   events.Publisher
}

// NewFileGenerationHandler creates a new file generation handler
func NewFileGenerationHandler(eventBus events.EventBus, publisher events.Publisher) ai.ResponseHandler {
	fileManager := fileops.NewFileOpsManager()

	return &FileGenerationHandler{
		fileManager: fileManager,
		eventBus:    eventBus,
		publisher:   publisher,
	}
}

// Name returns the handler name
func (h *FileGenerationHandler) Name() string {
	return "file_generator"
}

// CanHandle checks if the response contains file generation instructions
func (h *FileGenerationHandler) CanHandle(response string) bool {
	// Look for FILE: pattern in the response
	filePattern := regexp.MustCompile(`(?i)FILE:\s*([^\n\r]+)`)
	return filePattern.MatchString(response)
}

// Process extracts file specifications and creates files with confirmation
func (h *FileGenerationHandler) Process(ctx context.Context, response string) (string, error) {
	// Parse files from response
	files, err := h.parseFilesFromResponse(response)
	if err != nil {
		return "", fmt.Errorf("error parsing files from response: %w", err)
	}

	if len(files) == 0 {
		return "No files found in response", nil
	}

	// Process each file
	var results []string
	var errors []string

	for _, file := range files {
		result, err := h.processFile(ctx, file)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", file.Path, err))
		} else {
			results = append(results, result)
		}
	}

	// Build final response
	response_text := ""
	if len(results) > 0 {
		response_text += "Successfully processed:\n" + strings.Join(results, "\n")
	}
	if len(errors) > 0 {
		if response_text != "" {
			response_text += "\n\n"
		}
		response_text += "Errors:\n" + strings.Join(errors, "\n")
	}

	return response_text, nil
}

// parseFilesFromResponse extracts file specifications from LLM response
func (h *FileGenerationHandler) parseFilesFromResponse(response string) ([]FileSpec, error) {
	var files []FileSpec

	// Pattern to match FILE: path and CONTENT: sections
	// This regex captures multi-line content between CONTENT: and END_FILE (or end of string)
	pattern := regexp.MustCompile(`(?is)FILE:\s*([^\n\r]+).*?CONTENT:\s*\n(.*?)(?:END_FILE|$)`)

	matches := pattern.FindAllStringSubmatch(response, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			path := strings.TrimSpace(match[1])
			content := strings.TrimSpace(match[2])

			// Clean and validate file path
			path = filepath.Clean(path)
			if filepath.IsAbs(path) {
				return nil, fmt.Errorf("absolute paths are not allowed for security reasons: %s", path)
			}

			files = append(files, FileSpec{
				Path:    path,
				Content: content,
			})
		}
	}

	return files, nil
}

// processFile handles a single file creation with confirmation
func (h *FileGenerationHandler) processFile(ctx context.Context, file FileSpec) (string, error) {
	// Generate diff to show what will change
	diffContent, err := h.generateUnifiedDiff(file.Path, file.Content)
	if err != nil {
		// If error is about no changes, return early
		if err.Error() == "no changes detected - file content is identical" {
			return fmt.Sprintf("%s: No changes needed - content is identical", file.Path), nil
		}
		return "", fmt.Errorf("error generating diff: %w", err)
	}

	// Request user confirmation with diff preview
	confirmed, err := h.requestDiffConfirmation(ctx, file.Path, diffContent)
	if err != nil {
		return "", fmt.Errorf("error during confirmation: %w", err)
	}

	if !confirmed {
		return fmt.Sprintf("%s: Creation cancelled by user", file.Path), nil
	}

	// Write the file
	err = h.fileManager.WriteFile(file.Path, []byte(file.Content))
	if err != nil {
		return "", fmt.Errorf("error writing file: %w", err)
	}

	return fmt.Sprintf("%s: Created successfully", file.Path), nil
}

// requestDiffConfirmation requests user confirmation with diff preview
func (h *FileGenerationHandler) requestDiffConfirmation(ctx context.Context, filePath, diffContent string) (bool, error) {
	// Generate unique execution ID
	executionID := uuid.New().String()

	// Create confirmation request event
	request := events.UserConfirmationRequest{
		ExecutionID: executionID,
		Title:       "file_generator",
		FilePath:    filePath,
		Content:     diffContent,
		ContentType: "diff",
		Message:     fmt.Sprintf("Create file %s", filePath),
	}

	// Set up response channel
	responseChan := make(chan events.UserConfirmationResponse, 1)

	// Subscribe to confirmation responses for this execution
	h.eventBus.Subscribe("user.confirmation.response", func(event any) {
		if response, ok := event.(events.UserConfirmationResponse); ok {
			if response.ExecutionID == executionID {
				responseChan <- response
			}
		}
	})

	// Publish the confirmation request
	h.publisher.Publish(request.Topic(), request)

	// Wait for response without timeout
	select {
	case response := <-responseChan:
		return response.Confirmed, nil
	case <-ctx.Done():
		return false, fmt.Errorf("context cancelled during confirmation")
	}
}

// generateUnifiedDiff creates a unified diff showing changes to a file
func (h *FileGenerationHandler) generateUnifiedDiff(filePath, newContent string) (string, error) {
	// Read existing content if file exists
	var oldContent string
	if h.fileManager.FileExists(filePath) {
		oldBytes, err := h.fileManager.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("error reading existing file: %w", err)
		}
		oldContent = string(oldBytes)
	}

	// Check if content is identical
	if oldContent == newContent {
		return "", fmt.Errorf("no changes detected - file content is identical")
	}

	// Generate unified diff
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(oldContent),
		B:        difflib.SplitLines(newContent),
		FromFile: filePath,
		ToFile:   filePath,
		Context:  3,
	}

	diffText, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return "", fmt.Errorf("error generating unified diff: %w", err)
	}

	return diffText, nil
}
