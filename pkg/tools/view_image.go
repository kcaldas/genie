package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"math"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

const (
	defaultMaxImageBytes  = 5 * 1024 * 1024 // 5 MiB
	maxImageDimension     = 1024            // cap longest edge when shrinking
	minShrinkQuality      = 40              // lowest JPEG quality to try
	startShrinkQuality    = 90              // starting JPEG quality
	shrinkQualityStep     = 10              // quality reduction per attempt
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

	if len(data) == 0 {
		return nil, fmt.Errorf("image file is empty")
	}

	mimeType := v.detectMIME(path, data)
	if _, allowed := allowedImageMIMETypes[mimeType]; !allowed {
		return nil, fmt.Errorf("unsupported image type: %s", mimeType)
	}

	// Scale down raster images that are too large (dimensions or bytes).
	if isScalableImage(mimeType) {
		scaled, scaledMIME, err := shrinkImageIfNeeded(data, v.maxBytes)
		if err == nil && scaled != nil {
			data = scaled
			mimeType = scaledMIME
		}
		// If scaling fails (e.g. corrupt image) or isn't needed, use original data.
	}

	size := int64(len(data))
	if size > v.maxBytes {
		return nil, fmt.Errorf("image exceeds maximum supported size (%d bytes)", v.maxBytes)
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

// isScalableImage returns true for raster formats Go's stdlib can decode.
func isScalableImage(mimeType string) bool {
	switch mimeType {
	case "image/png", "image/jpeg", "image/jpg", "image/gif":
		return true
	}
	return false
}

// shrinkImageIfNeeded decodes a raster image and scales it down if its longest
// edge exceeds maxImageDimension or its byte size exceeds maxBytes. The result
// is re-encoded as JPEG.
func shrinkImageIfNeeded(data []byte, maxBytes int64) ([]byte, string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	needsScale := int64(len(data)) > maxBytes ||
		width > maxImageDimension ||
		height > maxImageDimension

	if !needsScale {
		return nil, "", nil // signal: no scaling needed
	}

	scale := computeImageScale(width, height, maxImageDimension)
	if scale < 1.0 {
		newWidth := intMax(1, int(math.Round(float64(width)*scale)))
		newHeight := intMax(1, int(math.Round(float64(height)*scale)))
		img = resizeNearestNeighbor(img, newWidth, newHeight)
	}

	// Re-encode as JPEG, reducing quality until within maxBytes.
	for quality := startShrinkQuality; quality >= minShrinkQuality; quality -= shrinkQualityStep {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, "", fmt.Errorf("encode jpeg: %w", err)
		}
		if int64(buf.Len()) <= maxBytes {
			return buf.Bytes(), "image/jpeg", nil
		}
	}

	return nil, "", fmt.Errorf("unable to reduce image below %d bytes", maxBytes)
}

func computeImageScale(width, height, maxDim int) float64 {
	longest := float64(intMax(width, height))
	if longest <= float64(maxDim) {
		return 1.0
	}
	return float64(maxDim) / longest
}

func resizeNearestNeighbor(src image.Image, width, height int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	bounds := src.Bounds()
	scaleX := float64(bounds.Dx()) / float64(width)
	scaleY := float64(bounds.Dy()) / float64(height)

	for y := 0; y < height; y++ {
		srcY := bounds.Min.Y + int(math.Floor(float64(y)*scaleY))
		if srcY >= bounds.Max.Y {
			srcY = bounds.Max.Y - 1
		}
		for x := 0; x < width; x++ {
			srcX := bounds.Min.X + int(math.Floor(float64(x)*scaleX))
			if srcX >= bounds.Max.X {
				srcX = bounds.Max.X - 1
			}
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}

	return dst
}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (v *ViewImageTool) failure(message string) (map[string]any, error) {
	return map[string]any{
		"success": false,
		"error":   strings.TrimSpace(message),
	}, nil
}
