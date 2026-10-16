package ai

import (
	"context"
	"errors"
)

// ErrNoModelList reports a provider that cannot enumerate its models.
var ErrNoModelList = errors.New("provider does not list models")

// ModelLister is implemented by provider clients that can enumerate the
// models their endpoint serves.
type ModelLister interface {
	ListModels(ctx context.Context) ([]string, error)
}

// ModelCatalog knows the configured providers and, when a provider can
// enumerate them, the models each one serves.
type ModelCatalog interface {
	// Providers returns the canonical provider names, sorted.
	Providers() []string
	// ListModels returns the models provider serves, sorted. An empty
	// provider means the default one. It returns ErrNoModelList when the
	// provider cannot enumerate its models.
	ListModels(ctx context.Context, provider string) ([]string, error)
}

// Unwrapper is implemented by middleware so callers can reach the Gen
// underneath for optional interfaces such as ModelCatalog.
type Unwrapper interface {
	Unwrap() Gen
}

// CatalogOf finds the ModelCatalog behind gen, looking through middleware.
func CatalogOf(gen Gen) (ModelCatalog, bool) {
	for gen != nil {
		if c, ok := gen.(ModelCatalog); ok {
			return c, true
		}
		u, ok := gen.(Unwrapper)
		if !ok {
			return nil, false
		}
		gen = u.Unwrap()
	}
	return nil, false
}
