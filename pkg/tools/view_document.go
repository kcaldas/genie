package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

const (
	defaultMaxDocumentBytes = 20 * 1024 * 1024 // 20 MiB
)

var allowedDocumentMIMETypes = map[string]struct{}{
	"application/pdf": {},
}

// ViewDocumentTool exposes documents (currently PDFs) to the LLM.
type ViewDocumentTool struct {
	publisher events.Publisher
	maxBytes  int64
}

// ViewDocumentOption configures the view document tool.
type ViewDocumentOption func(*ViewDocumentTool)

// WithMaxDocumentBytes overrides the maximum document size.
func WithMaxDocumentBytes(max int64) ViewDocumentOption {
	return func(tool *ViewDocumentTool) {
		if max > 0 {
			tool.maxBytes = max
		}
	}
}

// NewViewDocumentTool creates a new document tool instance.
func NewViewDocumentTool(publisher events.Publisher, opts ...ViewDocumentOption) Tool {
	tool := &ViewDocumentTool{
		publisher: publisher,
		maxBytes:  defaultMaxDocumentBytes,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(tool)
		}
	}
	return tool
}

func (v *ViewDocumentTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "viewDocument",
		Description: "Reads a document from the workspace (currently only PDF) and returns it for inspection.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters required to view a document",
			Properties: map[string]*ai.Schema{
				"file_path": {
					Type:        ai.TypeString,
					Description: "Path to the document relative to the workspace root.",
					MinLength:   1,
					MaxLength:   500,
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Explain to the user why this document is being inspected.",
					MinLength:   5,
					MaxLength:   200,
				},
			},
			Required: []string{"file_path", "_display_message"},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Document payload returned to the model",
			Properties: map[string]*ai.Schema{
				"success": {
					Type:        ai.TypeBoolean,
					Description: "Whether the document was loaded successfully.",
				},
				"mime_type": {
					Type:        ai.TypeString,
					Description: "Detected MIME type.",
				},
				"size_bytes": {
					Type:        ai.TypeInteger,
					Description: "Size of the document in bytes.",
				},
				"data_base64": {
					Type:        ai.TypeString,
					Description: "Document data encoded in base64. Present when success is true.",
				},
				"data_url": {
					Type:        ai.TypeString,
					Description: "Convenience data URL prefixed with the MIME type.",
				},
				"error": {
					Type:        ai.TypeString,
					Description: "Reason for failure when success is false.",
				},
				"path": {
					Type:        ai.TypeString,
					Description: "Relative path to the resolved document.",
				},
			},
			Required: []string{"success"},
		},
	}
}

func (v *ViewDocumentTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		filePath, ok := params["file_path"].(string)
		if !ok || strings.TrimSpace(filePath) == "" {
			return v.failure("file_path parameter is required")
		}

		resolvedPath, valid := ResolvePathWithWorkingDirectory(ctx, filePath)
		if !valid {
			return v.failure("file path is outside the working directory")
		}

		if err := v.publishMessageIfPresent(params); err != nil {
			return nil, err
		}

		payload, err := v.loadDocument(resolvedPath)
		if err != nil {
			return v.failure(err.Error())
		}

		relativePath := ConvertToRelativePath(ctx, resolvedPath)

		return map[string]any{
			"success":     true,
			"mime_type":   payload.mimeType,
			"size_bytes":  payload.size,
			"data_base64": payload.base64,
			"data_url":    fmt.Sprintf("data:%s;base64,%s", payload.mimeType, payload.base64),
			"path":        relativePath,
		}, nil
	}
}

func (v *ViewDocumentTool) FormatOutput(result map[string]interface{}) string {
	success, _ := result["success"].(bool)
	path, _ := result["path"].(string)
	errorMsg, _ := result["error"].(string)
	size, _ := result["size_bytes"].(int64)

	if !success {
		if strings.TrimSpace(errorMsg) == "" {
			return "**Failed to attach document**"
		}
		return fmt.Sprintf("**Failed to attach document**: %s", errorMsg)
	}

	if path == "" {
		path = "document"
	}

	if size > 0 {
		return fmt.Sprintf("Attached document `%s` (%d bytes)", path, size)
	}

	return fmt.Sprintf("Attached document `%s`", path)
}

func (v *ViewDocumentTool) loadDocument(path string) (*documentPayload, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read document: %w", err)
	}

	size := int64(len(data))
	if size == 0 {
		return nil, fmt.Errorf("document file is empty")
	}
	if size > v.maxBytes {
		return nil, fmt.Errorf("document exceeds maximum supported size (%d bytes)", v.maxBytes)
	}

	mimeType := detectDocumentMIME(path, data)
	if _, ok := allowedDocumentMIMETypes[mimeType]; !ok {
		return nil, fmt.Errorf("unsupported document type: %s", mimeType)
	}

	return &documentPayload{
		base64:   base64.StdEncoding.EncodeToString(data),
		mimeType: mimeType,
		size:     size,
	}, nil
}

func detectDocumentMIME(path string, data []byte) string {
	if ext := strings.ToLower(filepath.Ext(path)); ext != "" {
		if typ := mime.TypeByExtension(ext); typ != "" {
			return typ
		}
	}
	return http.DetectContentType(data)
}

func (v *ViewDocumentTool) publishMessageIfPresent(params map[string]any) error {
	if v.publisher == nil {
		return nil
	}

	msg, _ := params["_display_message"].(string)
	if strings.TrimSpace(msg) == "" {
		return fmt.Errorf("_display_message parameter is required")
	}

	v.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
		ToolName: "viewDocument",
		Message:  msg,
	})
	return nil
}

func (v *ViewDocumentTool) failure(message string) (map[string]any, error) {
	return map[string]any{
		"success": false,
		"error":   strings.TrimSpace(message),
	}, nil
}

type documentPayload struct {
	base64   string
	mimeType string
	size     int64
}
