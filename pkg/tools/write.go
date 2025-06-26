package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/fileops"
)

// WriteTool implements file writing with diff preview and confirmation
type WriteTool struct {
	fileManager         fileops.Manager
	diffGenerator       *DiffGenerator
	eventBus            events.EventBus
	publisher           events.Publisher
	confirmationEnabled bool
}

// NewWriteTool creates a new write tool with diff preview capabilities
func NewWriteTool(eventBus events.EventBus, publisher events.Publisher, confirmationEnabled bool) Tool {
	fileManager := fileops.NewFileOpsManager()
	diffGenerator := NewDiffGenerator(fileManager)

	return &WriteTool{
		fileManager:         fileManager,
		diffGenerator:       diffGenerator,
		eventBus:            eventBus,
		publisher:           publisher,
		confirmationEnabled: confirmationEnabled,
	}
}

// Declaration returns the function declaration for this tool
func (w *WriteTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "writeFile",
		Description: "Write content to a file with diff preview and user confirmation. Always reads existing file content first to show changes, creates directories as needed, and requires confirmation before applying changes.",
		Parameters: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"path": {
					Type:        ai.TypeString,
					Description: "The file path to write to (relative to current directory)",
				},
				"content": {
					Type:        ai.TypeString,
					Description: "The content to write to the file",
				},
				"mode": {
					Type:        ai.TypeString,
					Description: "File permissions in octal format (optional, defaults to '0644')",
				},
				"backup": {
					Type:        ai.TypeString,
					Description: "Whether to create a backup of existing file ('true' or 'false', optional)",
				},
			},
			Required: []string{"path", "content"},
		},
		Response: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"success": {
					Type:        ai.TypeBoolean,
					Description: "Whether the operation was successful",
				},
				"results": {
					Type:        ai.TypeString,
					Description: "Description of what was done",
				},
				"diff": {
					Type:        ai.TypeString,
					Description: "The diff showing changes made",
				},
				"backup_path": {
					Type:        ai.TypeString,
					Description: "Path to backup file if created",
				},
			},
		},
	}
}

// Handler returns the function handler for this tool
func (w *WriteTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, args map[string]any) (map[string]any, error) {
		// Extract and validate arguments
		filePath, ok := args["path"].(string)
		if !ok || filePath == "" {
			return map[string]any{
				"success": false,
				"results": "Error: 'path' parameter is required and must be a non-empty string",
			}, nil
		}

		content, ok := args["content"].(string)
		if !ok {
			return map[string]any{
				"success": false,
				"results": "Error: 'content' parameter is required and must be a string",
			}, nil
		}

		// Handle optional parameters
		backupRequested := false
		if backupStr, ok := args["backup"].(string); ok {
			backupRequested = backupStr == "true"
		}

		// Clean and validate file path using utility function
		filePath = filepath.Clean(filePath)
		resolvedPath, isValid := ResolvePathWithWorkingDirectory(ctx, filePath)
		if !isValid {
			return map[string]any{
				"success": false,
				"results": "Error: file path is outside working directory or invalid",
			}, nil
		}
		filePath = resolvedPath

		// Generate diff to show what will change
		diffContent, err := w.diffGenerator.GenerateUnifiedDiff(filePath, content)
		if err != nil {
			// If error is about no changes, return early
			if err.Error() == "no changes detected - file content is identical" {
				return map[string]any{
					"success": false,
					"results": "No changes needed - file content is already identical",
				}, nil
			}
			return map[string]any{
				"success": false,
				"results": fmt.Sprintf("Error generating diff: %v", err),
			}, nil
		}

		// If confirmation is enabled, request user approval
		if w.confirmationEnabled {
			confirmed, err := w.requestDiffConfirmation(ctx, filePath, diffContent)
			if err != nil {
				return map[string]any{
					"success": false,
					"results": fmt.Sprintf("Error during confirmation: %v", err),
				}, nil
			}

			if !confirmed {
				return map[string]any{
					"success": false,
					"results": "File write operation cancelled by user",
					"diff":    diffContent,
				}, nil
			}
		}

		// Create backup if requested and file exists
		var backupPath string
		if backupRequested && w.fileManager.FileExists(filePath) {
			backupPath, err = w.createBackup(filePath)
			if err != nil {
				return map[string]any{
					"success": false,
					"results": fmt.Sprintf("Error creating backup: %v", err),
				}, nil
			}
		}

		// Write the file
		err = w.fileManager.WriteFile(filePath, []byte(content))
		if err != nil {
			return map[string]any{
				"success": false,
				"results": fmt.Sprintf("Error writing file: %v", err),
			}, nil
		}

		// Prepare success response
		result := map[string]any{
			"success": true,
			"results": fmt.Sprintf("Successfully wrote file: %s", filePath),
			"diff":    diffContent,
		}

		if backupPath != "" {
			result["backup_path"] = backupPath
		}

		return result, nil
	}
}

// requestDiffConfirmation requests user confirmation with diff preview
func (w *WriteTool) requestDiffConfirmation(ctx context.Context, filePath, diffContent string) (bool, error) {
	// Generate unique execution ID
	executionID := uuid.New().String()

	// Create confirmation request event
	request := events.UserConfirmationRequest{
		ExecutionID: executionID,
		Title:       "writeFile",
		FilePath:    filePath,
		Content:     diffContent,
		ContentType: "diff",
		Message:     fmt.Sprintf("Write changes to %s", filePath),
	}

	// Set up response channel
	responseChan := make(chan events.UserConfirmationResponse, 1)

	// Subscribe to confirmation responses for this execution
	w.eventBus.Subscribe("user.confirmation.response", func(event interface{}) {
		if response, ok := event.(events.UserConfirmationResponse); ok {
			if response.ExecutionID == executionID {
				responseChan <- response
			}
		}
	})

	// Publish the confirmation request
	w.publisher.Publish(request.Topic(), request)

	// Wait for response with timeout
	select {
	case response := <-responseChan:
		return response.Confirmed, nil
	case <-time.After(5 * time.Minute): // 5 minute timeout
		return false, fmt.Errorf("confirmation timeout - no response received")
	case <-ctx.Done():
		return false, fmt.Errorf("context cancelled during confirmation")
	}
}

// createBackup creates a backup of the existing file
func (w *WriteTool) createBackup(filePath string) (string, error) {
	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	ext := filepath.Ext(filePath)
	base := filePath[:len(filePath)-len(ext)]
	backupPath := fmt.Sprintf("%s.backup.%s%s", base, timestamp, ext)

	// Read existing content
	content, err := w.fileManager.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file for backup: %w", err)
	}

	// Write backup
	err = w.fileManager.WriteFile(backupPath, content)
	if err != nil {
		return "", fmt.Errorf("error writing backup file: %w", err)
	}

	return backupPath, nil
}

// FormatOutput formats the tool's execution result for user display
func (w *WriteTool) FormatOutput(result map[string]interface{}) string {
	success, _ := result["success"].(bool)
	message, _ := result["results"].(string)
	diffContent, _ := result["diff"].(string)
	backupPath, _ := result["backup_path"].(string)

	output := message

	// Add diff information if available
	if diffContent != "" && success {
		// Parse diff to show summary
		summary := w.diffGenerator.AnalyzeDiff(diffContent)
		if summary.IsNewFile {
			output += fmt.Sprintf("\n📄 Created new file with %d lines", summary.LinesAdded)
		} else if summary.IsModified {
			output += fmt.Sprintf("\n✏️  Modified file: +%d -%d lines", summary.LinesAdded, summary.LinesRemoved)
		}
	}

	// Add backup information if available
	if backupPath != "" {
		output += fmt.Sprintf("\n💾 Backup created: %s", backupPath)
	}

	return output
}

