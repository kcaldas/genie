package openai

import (
	"context"
	"fmt"
)

// ListModels returns the model ids the endpoint serves. A custom endpoint
// (OPENAI_BASE_URL, such as a proxy) answers with its own catalogue.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	api := c.apiClient
	c.mu.Unlock()
	if api == nil {
		return nil, fmt.Errorf("openai: client not initialised")
	}
	var ids []string
	pager := api.Models.ListAutoPaging(ctx)
	for pager.Next() {
		ids = append(ids, pager.Current().ID)
	}
	if err := pager.Err(); err != nil {
		return nil, fmt.Errorf("openai: list models: %w", err)
	}
	return ids, nil
}
