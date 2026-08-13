package multiplexer

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kcaldas/genie/pkg/ai"
	ctxregistry "github.com/kcaldas/genie/pkg/ctx"
)

// Factory creates an ai.Gen implementation for a specific provider.
type Factory func() (ai.Gen, error)

// Option configures the provider multiplexer.
type Option func(*Client)

// WithCapabilityResolver injects a resolver whose cache can outlive this
// multiplexer and the Genie instance that owns it.
func WithCapabilityResolver(resolver *ai.CapabilityResolver) Option {
	return func(c *Client) {
		if resolver != nil {
			c.capabilities = resolver
		}
	}
}

// Client routes prompt execution to multiple LLM providers based on prompt settings.
type Client struct {
	mu sync.RWMutex

	factories       map[string]Factory
	aliases         map[string]string
	clients         map[string]ai.Gen
	defaultProvider string
	lastProvider    string
	lastModel       string
	capabilities    *ai.CapabilityResolver
}

// NewClient creates a new multiplexer with lazy provider initialization.
func NewClient(defaultProvider string, factories map[string]Factory, aliases map[string]string, opts ...Option) (*Client, error) {
	if len(factories) == 0 {
		return nil, fmt.Errorf("multiplexer: no LLM factories registered")
	}

	factoriesLC := make(map[string]Factory, len(factories))
	for name, factory := range factories {
		if factory == nil {
			return nil, fmt.Errorf("multiplexer: factory for provider %q is nil", name)
		}
		factoriesLC[strings.ToLower(name)] = factory
	}

	aliasesLC := make(map[string]string, len(aliases))
	for from, to := range aliases {
		if from == "" || to == "" {
			continue
		}
		aliasesLC[strings.ToLower(from)] = strings.ToLower(to)
	}

	canonicalDefault := strings.ToLower(defaultProvider)
	if canonicalDefault == "" {
		canonicalDefault = "genai"
	}

	if _, ok := factoriesLC[canonicalDefault]; !ok {
		if alias, ok := aliasesLC[canonicalDefault]; ok {
			canonicalDefault = alias
		}
	}
	if _, ok := factoriesLC[canonicalDefault]; !ok {
		return nil, fmt.Errorf("multiplexer: unsupported default provider %q", defaultProvider)
	}

	client := &Client{
		factories:       factoriesLC,
		aliases:         aliasesLC,
		clients:         make(map[string]ai.Gen),
		defaultProvider: canonicalDefault,
		capabilities:    ai.NewCapabilityResolver(nil),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}
	return client, nil
}

// WarmUp eagerly initializes the requested provider.
func (c *Client) WarmUp(provider string) error {
	_, _, err := c.clientFor(provider)
	return err
}

// DefaultProvider returns the canonical default provider name.
func (c *Client) DefaultProvider() string {
	return c.defaultProvider
}

// GenerateContent implements ai.Gen by delegating to the selected provider.
func (c *Client) GenerateContent(ctx context.Context, p ai.Prompt, debug bool, args ...string) (string, error) {
	client, provider, err := c.clientFor(p.LLMProvider)
	if err != nil {
		return "", err
	}
	c.setLastContext(provider, p.ModelName)
	return client.GenerateContent(ctx, p, debug, args...)
}

// GenerateContentAttr implements ai.Gen by delegating to the selected provider.
func (c *Client) GenerateContentAttr(ctx context.Context, p ai.Prompt, debug bool, attrs []ai.Attr) (string, error) {
	client, provider, err := c.clientFor(p.LLMProvider)
	if err != nil {
		return "", err
	}
	c.setLastContext(provider, p.ModelName)
	return client.GenerateContentAttr(ctx, p, debug, attrs)
}

// GenerateContentStream implements ai.Gen streaming by delegating to the selected provider.
func (c *Client) GenerateContentStream(ctx context.Context, p ai.Prompt, debug bool, args ...string) (ai.Stream, error) {
	client, provider, err := c.clientFor(p.LLMProvider)
	if err != nil {
		return nil, err
	}
	c.setLastContext(provider, p.ModelName)
	return client.GenerateContentStream(ctx, p, debug, args...)
}

// GenerateContentAttrStream implements ai.Gen streaming with structured attributes.
func (c *Client) GenerateContentAttrStream(ctx context.Context, p ai.Prompt, debug bool, attrs []ai.Attr) (ai.Stream, error) {
	client, provider, err := c.clientFor(p.LLMProvider)
	if err != nil {
		return nil, err
	}
	c.setLastContext(provider, p.ModelName)
	return client.GenerateContentAttrStream(ctx, p, debug, attrs)
}

// CountTokens implements ai.Gen by delegating to the selected provider.
func (c *Client) CountTokens(ctx context.Context, p ai.Prompt, debug bool, args ...string) (*ai.TokenCount, error) {
	client, provider, err := c.clientFor(p.LLMProvider)
	if err != nil {
		return nil, err
	}
	c.setLastContext(provider, p.ModelName)
	return client.CountTokens(ctx, p, debug, args...)
}

// CountTokensAttr implements ai.Gen by delegating to the selected provider.
func (c *Client) CountTokensAttr(ctx context.Context, p ai.Prompt, debug bool, attrs []ai.Attr) (*ai.TokenCount, error) {
	client, provider, err := c.clientFor(p.LLMProvider)
	if err != nil {
		return nil, err
	}
	c.setLastContext(provider, p.ModelName)
	return client.CountTokensAttr(ctx, p, debug, attrs)
}

// ModelCapabilities resolves live metadata for the provider selected by the
// prompt. Providers without discovery support (currently OpenAI) retain the
// conservative static context-window fallback.
func (c *Client) ModelCapabilities(ctx context.Context, p ai.Prompt) (ai.ModelCapabilities, error) {
	client, provider, err := c.clientFor(p.LLMProvider)
	if err != nil {
		return fallbackCapabilities(p.ModelName), err
	}
	discoverer, ok := client.(ai.ModelCapabilityDiscoverer)
	if !ok {
		return fallbackCapabilities(p.ModelName), nil
	}

	model := strings.TrimSpace(p.ModelName)
	key := discoverer.CapabilityCacheKey(model)
	if key.Provider == "" {
		key.Provider = provider
	}
	if key.Model == "" {
		key.Model = model
	}
	ttl := ai.DefaultCapabilityTTL
	if providerTTL, ok := client.(ai.CapabilityTTLProvider); ok {
		ttl = providerTTL.CapabilityTTL(model)
	}
	caps, err := c.capabilities.Resolve(ctx, key, ttl, func(ctx context.Context) (ai.ModelCapabilities, error) {
		return discoverer.DiscoverModelCapabilities(ctx, model)
	})
	if caps.Model == "" {
		caps.Model = model
	}
	if caps.InputTokenLimit <= 0 {
		fallback := fallbackCapabilities(model)
		caps.InputTokenLimit = fallback.InputTokenLimit
		if caps.Source == "" {
			caps.Source = fallback.Source
		}
	}
	return caps, err
}

func fallbackCapabilities(model string) ai.ModelCapabilities {
	return ai.ModelCapabilities{
		Model:               strings.TrimSpace(model),
		InputTokenLimit:     ctxregistry.LookupContextWindow(model),
		SharedContextWindow: true,
		Source:              ai.CapabilitySourceFallback,
	}
}

// GetStatus returns the status from the default provider.

func (c *Client) GetStatus() *ai.Status {
	provider := c.getStatusProvider()
	client, _, err := c.clientFor(provider)
	if err != nil {
		return &ai.Status{
			Connected: false,
			Backend:   provider,
			Message:   err.Error(),
		}
	}
	status := client.GetStatus()
	if status == nil {
		status = &ai.Status{}
	}
	status.Backend = provider
	if model := c.getLastModel(); model != "" {
		status.Model = fmt.Sprintf("%s (persona)", model)
	}
	return status
}

func (c *Client) clientFor(provider string) (ai.Gen, string, error) {
	canonical, err := c.canonicalizeProvider(provider)
	if err != nil {
		return nil, "", err
	}

	c.mu.RLock()
	if existing := c.clients[canonical]; existing != nil {
		c.mu.RUnlock()
		return existing, canonical, nil
	}
	c.mu.RUnlock()

	factory := c.factories[canonical]
	client, err := factory()
	if err != nil {
		return nil, "", err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if existing := c.clients[canonical]; existing != nil {
		return existing, canonical, nil
	}
	c.clients[canonical] = client
	return client, canonical, nil
}

func (c *Client) canonicalizeProvider(provider string) (string, error) {
	name := strings.TrimSpace(provider)
	if name == "" {
		name = c.defaultProvider
	}
	key := strings.ToLower(name)

	if _, ok := c.factories[key]; ok {
		return key, nil
	}

	if alias, ok := c.aliases[key]; ok {
		if _, ok := c.factories[alias]; ok {
			return alias, nil
		}
	}

	return "", fmt.Errorf("multiplexer: unsupported LLM provider %q", provider)
}

func (c *Client) setLastContext(provider, model string) {
	c.mu.Lock()
	c.lastProvider = provider
	if trimmed := strings.TrimSpace(model); trimmed != "" {
		c.lastModel = trimmed
	}
	c.mu.Unlock()
}

func (c *Client) getStatusProvider() string {
	c.mu.RLock()
	if c.lastProvider != "" {
		provider := c.lastProvider
		c.mu.RUnlock()
		return provider
	}
	c.mu.RUnlock()
	return c.defaultProvider
}

func (c *Client) getLastModel() string {
	c.mu.RLock()
	model := c.lastModel
	c.mu.RUnlock()
	return model
}
