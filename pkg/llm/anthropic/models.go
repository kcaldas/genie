package anthropic

import (
	"context"
	"fmt"

	anthropic_sdk "github.com/anthropics/anthropic-sdk-go"
)

// ListModels returns the model ids the endpoint serves.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	api := c.apiClient
	c.mu.Unlock()
	if api == nil {
		return nil, fmt.Errorf("anthropic: client not initialised")
	}
	var ids []string
	pager := api.Models.ListAutoPaging(ctx, anthropic_sdk.ModelListParams{})
	for pager.Next() {
		ids = append(ids, pager.Current().ID)
	}
	if err := pager.Err(); err != nil {
		return nil, fmt.Errorf("anthropic: list models: %w", err)
	}
	return ids, nil
}
