package genai

import (
	"context"
	"fmt"
	"strings"
)

// ListModels returns the model names the backend serves, without the
// "models/" resource prefix so they match what prompts pin.
func (g *Client) ListModels(ctx context.Context) ([]string, error) {
	if err := g.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	g.mu.Lock()
	client := g.Client
	g.mu.Unlock()
	if client == nil {
		return nil, fmt.Errorf("genai: client not initialised")
	}
	var names []string
	for model, err := range client.Models.All(ctx) {
		if err != nil {
			return nil, fmt.Errorf("genai: list models: %w", err)
		}
		names = append(names, strings.TrimPrefix(model.Name, "models/"))
	}
	return names, nil
}
