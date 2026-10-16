package ai

import (
	"context"
	"testing"
)

// nopGen is the smallest Gen; it answers nothing.
type nopGen struct{}

func (nopGen) GenerateContent(context.Context, Prompt, bool, ...string) (string, error) {
	return "", nil
}
func (nopGen) GenerateContentAttr(context.Context, Prompt, bool, []Attr) (string, error) {
	return "", nil
}
func (nopGen) GenerateContentStream(context.Context, Prompt, bool, ...string) (Stream, error) {
	return nil, nil
}
func (nopGen) GenerateContentAttrStream(context.Context, Prompt, bool, []Attr) (Stream, error) {
	return nil, nil
}
func (nopGen) CountTokens(context.Context, Prompt, bool, ...string) (*TokenCount, error) {
	return nil, nil
}
func (nopGen) CountTokensAttr(context.Context, Prompt, bool, []Attr) (*TokenCount, error) {
	return nil, nil
}
func (nopGen) GetStatus() *Status { return &Status{} }

type catalogGen struct {
	nopGen
	providers []string
}

func (c *catalogGen) Providers() []string                { return c.providers }
func (c *catalogGen) Canonical(p string) (string, error) { return p, nil }
func (c *catalogGen) ListModels(context.Context, string) ([]string, error) {
	return nil, ErrNoModelList
}

func TestCatalogOfLooksThroughMiddleware(t *testing.T) {
	inner := &catalogGen{providers: []string{"openai"}}
	wrapped := NewRetryMiddleware(NewCaptureMiddleware(inner, CaptureConfig{}), RetryConfig{})
	cat, ok := CatalogOf(wrapped)
	if !ok {
		t.Fatal("expected a catalog behind the middleware")
	}
	if got := cat.Providers(); len(got) != 1 || got[0] != "openai" {
		t.Fatalf("Providers() = %v", got)
	}
	if _, ok := CatalogOf(nopGen{}); ok {
		t.Fatal("a plain Gen has no catalog")
	}
}
