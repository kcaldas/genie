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
	defaultMaxImageBytes = 5 * 1024 * 1024 // 5 MiB
)

var (
	// allowList of MIME types we consider safe to send back to the model.
	allowedImageMIMETypes = map[string]struct{}{
		"image/png":     {},
		"image/jpeg":    {},
		"image/jpg":     {},
		"image/gif":     {},
		"image/webp":    {},
		"image/bmp":     {},
		"image/svg+xml": {},
	}
)

// ViewImageTool makes images available to the LLM.
type ViewImageTool struct {
	publisher events.Publisher
	maxBytes  int64
}

// ViewImageOption configures the behaviour of the view image tool.
type ViewImageOption func(*ViewImageTool)

// WithMaxImageBytes sets an upper bound for images that can be returned.
func WithMaxImageBytes(max int64) ViewImageOption {
	return func(tool *ViewImageTool) {
		if max > 0 {
			tool.maxBytes = max
		}
	}
}

// NewViewImageTool creates a new view image tool.
func NewViewImageTool(publisher events.Publisher, opts ...ViewImageOption) Tool {
	tool := &ViewImageTool{
		publisher: publisher,
		maxBytes:  defaultMaxImageBytes,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(tool)
		}
	}
	return tool
}

// Declaration returns the function declaration for the view image tool.
func (v *ViewImageTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "viewImage",
		Description: "Reads an image from the workspace and returns it as base64 so the model can inspect it.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters required to view an image",
			Properties: map[string]*ai.Schema{
				"file_path": {
					Type:        ai.TypeString,
					Description: "Path to the image relative to the workspace root.",
					MinLength:   1,
					MaxLength:   500,
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Explain to the user why this image is being inspected.",
					MinLength:   5,
					MaxLength:   200,
				},
			},
			Required: []string{"file_path", "_display_message"},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Image payload returned to the model",
			Properties: map[string]*ai.Schema{
				"success": {
					Type:        ai.TypeBoolean,
					Description: "Whether the image was loaded successfully.",
				},
				"mime_type": {
					Type:        ai.TypeString,
					Description: "Detected MIME type for the image.",
				},
				"size_bytes": {
					Type:        ai.TypeInteger,
					Description: "Size of the image in bytes.",
				},
				"data_base64": {
					Type:        ai.TypeString,
					Description: "Image data encoded in base64. Present when success is true.",
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
					Description: "Relative path to the resolved image.",
				},
			},
			Required: []string{"success"},
		},
	}
}

// Handler returns the function handler for the view image tool.
func (v *ViewImageTool) Handler() ai.HandlerFunc {
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

		content, err := v.loadImage(resolvedPath)
		if err != nil {
			return v.failure(err.Error())
		}

		relativePath := ConvertToRelativePath(ctx, resolvedPath)

		return map[string]any{
			"success":     true,
			"mime_type":   content.mimeType,
			"size_bytes":  content.size,
			"data_base64": content.base64,
			"data_url":    fmt.Sprintf("data:%s;base64,%s", content.mimeType, content.base64),
			"path":        relativePath,
		}, nil
	}
}

// FormatOutput keeps consistent formatting with other tools.
func (v *ViewImageTool) FormatOutput(result map[string]interface{}) string {
	success, _ := result["success"].(bool)
	path, _ := result["path"].(string)
	errorMsg, _ := result["error"].(string)
	size, _ := result["size_bytes"].(int64)

	if !success {
		if errorMsg == "" {
			errorMsg = "unknown error"
		}
		return fmt.Sprintf("**Failed to attach image** (%s)", errorMsg)
	}

	if path == "" {
		path = "image"
	}

	if size > 0 {
		return fmt.Sprintf("Attached image `%s` (%d bytes)", path, size)
	}

	return fmt.Sprintf("Attached image `%s`", path)
}

type imagePayload struct {
	base64   string
	mimeType string
	size     int64
}

func (v *ViewImageTool) loadImage(path string) (*imagePayload, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	size := int64(len(data))
	if size == 0 {
		return nil, fmt.Errorf("image file is empty")
	}
	if size > v.maxBytes {
		return nil, fmt.Errorf("image exceeds maximum supported size (%d bytes)", v.maxBytes)
	}

	mimeType := v.detectMIME(path, data)
	if _, allowed := allowedImageMIMETypes[mimeType]; !allowed {
		return nil, fmt.Errorf("unsupported image type: %s", mimeType)
	}

	return &imagePayload{
		base64:   base64.StdEncoding.EncodeToString(data),
		mimeType: mimeType,
		size:     size,
	}, nil
}

func (v *ViewImageTool) detectMIME(path string, data []byte) string {
	if ext := strings.ToLower(filepath.Ext(path)); ext != "" {
		if typ := mime.TypeByExtension(ext); typ != "" {
			return typ
		}
	}

	// http.DetectContentType works for many common image formats.
	return http.DetectContentType(data)
}

func (v *ViewImageTool) publishMessageIfPresent(params map[string]any) error {
	if v.publisher == nil {
		return nil
	}

	msg, _ := params["_display_message"].(string)
	if strings.TrimSpace(msg) == "" {
		return fmt.Errorf("_display_message parameter is required")
	}

	v.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
		ToolName: "viewImage",
		Message:  msg,
	})
	return nil
}

func (v *ViewImageTool) failure(message string) (map[string]any, error) {
	return map[string]any{
		"success": false,
		"error":   strings.TrimSpace(message),
	}, nil
}
