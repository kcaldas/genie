package genie

import (
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
)

const defaultImageMIMEType = "image/png"

// ChatImage represents an image attachment in a chat request.
type ChatImage struct {
	// Data contains the raw image bytes that will be forwarded to the LLM.
	Data []byte
	// MIMEType is the content type for the image (e.g. "image/png").
	// When left empty, image/png is assumed.
	MIMEType string
	// Filename is optional metadata surfaced to providers that support it.
	Filename string
}

type chatRequestOptions struct {
	images     []ChatImage
	promptData map[string]string
}

// ChatOption configures a chat request. Options are optional – existing
// callers can continue invoking Chat with just a message.
type ChatOption func(*chatRequestOptions)

// WithImages attaches one or more images to the chat request.
// Empty payloads are ignored.
func WithImages(images ...ChatImage) ChatOption {
	return func(opts *chatRequestOptions) {
		for _, img := range images {
			if len(img.Data) == 0 {
				continue
			}
			opts.images = append(opts.images, ChatImage{
				Data:     append([]byte(nil), img.Data...),
				MIMEType: normalizeMIMEType(img.MIMEType),
				Filename: img.Filename,
			})
		}
	}
}

// WithAIImages attaches pre-built ai.Image values to the chat request.
// The image bytes are copied to avoid mutation after the call.
func WithAIImages(images ...*ai.Image) ChatOption {
	return func(opts *chatRequestOptions) {
		for _, img := range images {
			if img == nil || len(img.Data) == 0 {
				continue
			}
			opts.images = append(opts.images, ChatImage{
				Data:     append([]byte(nil), img.Data...),
				MIMEType: normalizeMIMEType(img.Type),
				Filename: img.Filename,
			})
		}
	}
}

// WithPromptData injects additional key/value pairs into the prompt data map.
// Values override existing keys derived from context if they collide.
func WithPromptData(data map[string]string) ChatOption {
	return func(opts *chatRequestOptions) {
		if len(data) == 0 {
			return
		}
		if opts.promptData == nil {
			opts.promptData = make(map[string]string, len(data))
		}
		for key, value := range data {
			opts.promptData[key] = value
		}
	}
}

func applyChatOptions(optionFns ...ChatOption) chatRequestOptions {
	request := chatRequestOptions{
		promptData: make(map[string]string),
	}
	for _, opt := range optionFns {
		if opt == nil {
			continue
		}
		opt(&request)
	}
	return request
}

func normalizeMIMEType(mime string) string {
	mime = strings.TrimSpace(mime)
	if mime == "" {
		return defaultImageMIMEType
	}
	return mime
}

func mergePromptImages(base []*ai.Image, extras []ChatImage) []*ai.Image {
	if len(base) == 0 && len(extras) == 0 {
		return nil
	}

	merged := make([]*ai.Image, 0, len(base)+len(extras))

	for _, img := range base {
		if img == nil || len(img.Data) == 0 {
			continue
		}
		merged = append(merged, &ai.Image{
			Type:     normalizeMIMEType(img.Type),
			Filename: img.Filename,
			Data:     append([]byte(nil), img.Data...),
		})
	}

	for _, img := range extras {
		if len(img.Data) == 0 {
			continue
		}
		merged = append(merged, &ai.Image{
			Type:     normalizeMIMEType(img.MIMEType),
			Filename: img.Filename,
			Data:     append([]byte(nil), img.Data...),
		})
	}

	if len(merged) == 0 {
		return nil
	}
	return merged
}
