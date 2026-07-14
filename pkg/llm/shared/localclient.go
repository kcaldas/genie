package shared

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/fileops"
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/kcaldas/genie/pkg/template"
)

// HTTPDoer is the minimal HTTP client surface the local providers
// depend on; tests inject fakes through WithHTTPClient.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// LocalClientCore bundles the dependencies, configuration and shared
// behavior of the local-server providers (Ollama, LM Studio). Providers
// embed it and keep only their wire formats and base-URL resolution.
type LocalClientCore struct {
	Provider    string
	Config      config.Manager
	FileManager fileops.Manager
	Template    template.Engine
	EventBus    events.EventBus
	Logger      logging.Logger
	HTTPClient  HTTPDoer
	BaseURL     string
}

// NewLocalClientCore builds a core with the default dependency set for
// the named provider. A nil event bus is replaced with a no-op bus.
func NewLocalClientCore(provider string, eventBus events.EventBus) LocalClientCore {
	if eventBus == nil {
		eventBus = &events.NoOpEventBus{}
	}
	return LocalClientCore{
		Provider:    provider,
		Config:      config.NewConfigManager(),
		FileManager: fileops.NewFileOpsManager(),
		Template:    template.NewEngine(),
		EventBus:    eventBus,
		Logger:      logging.NewAPILogger(provider),
		HTTPClient:  &http.Client{},
	}
}

// LocalOption configures the shared core of a local provider client.
// Providers alias this as their exported Option type.
type LocalOption func(*LocalClientCore)

// WithConfigManager injects a custom configuration manager.
func WithConfigManager(manager config.Manager) LocalOption {
	return func(c *LocalClientCore) {
		if manager != nil {
			c.Config = manager
		}
	}
}

// WithFileManager injects a custom file manager (useful for tests).
func WithFileManager(manager fileops.Manager) LocalOption {
	return func(c *LocalClientCore) {
		if manager != nil {
			c.FileManager = manager
		}
	}
}

// WithTemplateEngine injects a custom template engine.
func WithTemplateEngine(engine template.Engine) LocalOption {
	return func(c *LocalClientCore) {
		if engine != nil {
			c.Template = engine
		}
	}
}

// WithLogger injects a custom logger implementation.
func WithLogger(logger logging.Logger) LocalOption {
	return func(c *LocalClientCore) {
		if logger != nil {
			c.Logger = logger
		}
	}
}

// WithHTTPClient injects a custom HTTP client.
func WithHTTPClient(client HTTPDoer) LocalOption {
	return func(c *LocalClientCore) {
		if client != nil {
			c.HTTPClient = client
		}
	}
}

// RenderPrompt renders the prompt with the given attributes, honoring
// the debug dump behavior shared by all providers.
func (c *LocalClientCore) RenderPrompt(prompt ai.Prompt, debug bool, attrs []ai.Attr) (*ai.Prompt, error) {
	return RenderPromptWithDebug(c.FileManager, prompt, debug, attrs)
}

// ResolveModelName picks the prompt's model when set, falling back to
// the configured default model.
func (c *LocalClientCore) ResolveModelName(promptModel string) string {
	if strings.TrimSpace(promptModel) != "" {
		return promptModel
	}
	model := c.Config.GetModelConfig()
	if strings.TrimSpace(model.ModelName) != "" {
		return model.ModelName
	}
	return ""
}

// PublishTokenCount emits a TokenCountEvent for this provider. A nil
// token count is ignored.
func (c *LocalClientCore) PublishTokenCount(tokenCount *ai.TokenCount) {
	if tokenCount == nil {
		return
	}
	event := events.TokenCountEvent{
		Provider:     c.Provider,
		Model:        c.ResolveModelName(""),
		InputTokens:  tokenCount.InputTokens,
		OutputTokens: tokenCount.OutputTokens,
		TotalTokens:  tokenCount.TotalTokens,
	}
	c.EventBus.Publish(event.Topic(), event)
}

// PostJSON sends a JSON payload with the default headers applied and
// returns the raw response for the provider to decode.
func (c *LocalClientCore) PostJSON(ctx context.Context, url string, payload []byte) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for key, values := range ai.DefaultHTTPHeaders() {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}
	return c.HTTPClient.Do(httpReq)
}

// ErrStopStream stops ScanStreamLines early without reporting an error
// (e.g. when the provider marks a chunk as final).
var ErrStopStream = errors.New("stop scanning stream")

// ScanStreamLines reads a line-oriented streaming response body,
// passing each non-empty trimmed line to handle. The label names the
// provider in read-error messages.
func ScanStreamLines(r io.Reader, label string, handle func(line string) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := handle(line); err != nil {
			if errors.Is(err, ErrStopStream) {
				return nil
			}
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading %s stream: %w", label, err)
	}
	return nil
}

// EncodeImageDataURL converts a prompt image into a data URL, defaulting
// the MIME type to image/png.
func EncodeImageDataURL(img *ai.Image) string {
	if img == nil || len(img.Data) == 0 {
		return ""
	}
	mimeType := strings.TrimSpace(img.Type)
	if mimeType == "" {
		mimeType = "image/png"
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(img.Data))
}
