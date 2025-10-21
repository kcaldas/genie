package multiplexer

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kcaldas/genie/pkg/ai"
)

// Factory creates an ai.Gen implementation for a specific provider.
type Factory func() (ai.Gen, error)

// Client routes prompt execution to multiple LLM providers based on prompt settings.
type Client struct {
	mu sync.RWMutex

	factories       map[string]Factory
	aliases         map[string]string
	clients         map[string]ai.Gen
	defaultProvider string
	lastProvider    string
}

// NewClient creates a new multiplexer with lazy provider initialization.
func NewClient(defaultProvider string, factories map[string]Factory, aliases map[string]string) (*Client, error) {
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

	return &Client{
		factories:       factoriesLC,
		aliases:         aliasesLC,
		clients:         make(map[string]ai.Gen),
		defaultProvider: canonicalDefault,
	}, nil
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
	c.setLastProvider(provider)
	return client.GenerateContent(ctx, p, debug, args...)
}

// GenerateContentAttr implements ai.Gen by delegating to the selected provider.
func (c *Client) GenerateContentAttr(ctx context.Context, p ai.Prompt, debug bool, attrs []ai.Attr) (string, error) {
	client, provider, err := c.clientFor(p.LLMProvider)
	if err != nil {
		return "", err
	}
	c.setLastProvider(provider)
	return client.GenerateContentAttr(ctx, p, debug, attrs)
}

// CountTokens implements ai.Gen by delegating to the selected provider.
func (c *Client) CountTokens(ctx context.Context, p ai.Prompt, debug bool, args ...string) (*ai.TokenCount, error) {
	client, provider, err := c.clientFor(p.LLMProvider)
	if err != nil {
		return nil, err
	}
	c.setLastProvider(provider)
	return client.CountTokens(ctx, p, debug, args...)
}

// CountTokensAttr implements ai.Gen by delegating to the selected provider.
func (c *Client) CountTokensAttr(ctx context.Context, p ai.Prompt, debug bool, attrs []ai.Attr) (*ai.TokenCount, error) {
	client, provider, err := c.clientFor(p.LLMProvider)
	if err != nil {
		return nil, err
	}
	c.setLastProvider(provider)
	return client.CountTokensAttr(ctx, p, debug, attrs)
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
	return client.GetStatus()
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

func (c *Client) setLastProvider(provider string) {
	c.mu.Lock()
	c.lastProvider = provider
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
