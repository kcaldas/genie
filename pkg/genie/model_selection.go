package genie

import (
	"context"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/ctx"
)

// ModelSelection names a provider and a model. Either part may be empty:
// an empty provider keeps the persona's (or the configured default), an
// empty model keeps the persona's pin.
type ModelSelection struct {
	Provider string
	Model    string
}

// ParseModelSpec reads "model" or "provider/model". The provider is
// lower-cased; a model may itself contain '/' (as "org/model" does).
func ParseModelSpec(spec string) ModelSelection {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return ModelSelection{}
	}
	provider, model, found := strings.Cut(spec, "/")
	if !found {
		return ModelSelection{Model: spec}
	}
	return ModelSelection{Provider: strings.ToLower(strings.TrimSpace(provider)), Model: strings.TrimSpace(model)}
}

// String renders the selection as "provider/model", or whichever part is set.
func (s ModelSelection) String() string {
	switch {
	case s.Provider != "" && s.Model != "":
		return s.Provider + "/" + s.Model
	case s.Provider != "":
		return s.Provider
	default:
		return s.Model
	}
}

// IsZero reports an empty selection.
func (s ModelSelection) IsZero() bool { return s.Provider == "" && s.Model == "" }

// SetModel overrides the model used for the following turns, on top of
// whatever the persona pins. The provider must be one the backend knows
// when it can tell; an empty selection clears the override.
func (g *core) SetModel(sel ModelSelection) error {
	sel.Provider = strings.ToLower(strings.TrimSpace(sel.Provider))
	sel.Model = strings.TrimSpace(sel.Model)
	if sel.Provider != "" && sel.Model == "" {
		return fmt.Errorf("a model is required with a provider: use <provider>/<model>")
	}
	if sel.Provider != "" {
		if c, ok := g.catalog(); ok {
			canonical, err := c.Canonical(sel.Provider)
			if err != nil {
				return fmt.Errorf("unknown LLM provider %q; configured providers: %s", sel.Provider, strings.Join(c.Providers(), ", "))
			}
			sel.Provider = canonical
		}
	}
	g.modelMu.Lock()
	g.modelOverride = sel
	g.modelMu.Unlock()
	if g.started {
		g.initContextBudget(g.sessionContext(context.Background()))
	}
	return nil
}

// sessionContext attaches the current session (home, cwd, persona) to ctx
// so persona resolution sees the active persona, not the default one.
func (g *core) sessionContext(ctx context.Context) context.Context {
	if g.sessionMgr == nil {
		return ctx
	}
	sess, err := g.sessionMgr.GetSession()
	if err != nil || sess == nil {
		return ctx
	}
	return applySessionContext(ctx, sess)
}

func (g *core) override() ModelSelection {
	g.modelMu.RLock()
	defer g.modelMu.RUnlock()
	return g.modelOverride
}

// Model reports what the next turn will use: the override where set, else
// the persona's provider and model.
func (g *core) Model(ctx context.Context) ModelSelection {
	sel := g.override()
	if sel.Provider != "" && sel.Model != "" {
		return sel
	}
	if g.personaManager != nil {
		if prompt, err := g.personaManager.GetPrompt(g.sessionContext(ctx)); err == nil {
			if sel.Provider == "" {
				sel.Provider = strings.ToLower(prompt.LLMProvider)
			}
			if sel.Model == "" {
				sel.Model = prompt.ModelName
			}
		}
	}
	if sel.Provider == "" && g.configMgr != nil {
		sel.Provider = strings.ToLower(g.configMgr.GetStringWithDefault("GENIE_LLM_PROVIDER", "genai"))
	}
	return sel
}

// applyModelOverride rewrites a turn's prompt copy with the override and
// refreshes the registry capabilities for the new model.
func (g *core) applyModelOverride(prompt *ai.Prompt) {
	sel := g.override()
	if sel.IsZero() {
		return
	}
	if sel.Provider != "" {
		prompt.LLMProvider = sel.Provider
	}
	if sel.Model != "" && sel.Model != prompt.ModelName {
		prompt.ModelName = sel.Model
		prompt.ModelCapabilities = ctx.ModelCapabilities(sel.Model)
	}
}

// catalog finds the model catalogue behind the prompt runner. A host that
// wraps the PromptRunner must forward Providers, Canonical and ListModels
// (or implement Unwrap) for the catalogue to stay reachable.
func (g *core) catalog() (ai.ModelCatalog, bool) {
	var runner any = g.promptRunner
	for runner != nil {
		if c, ok := runner.(ai.ModelCatalog); ok {
			return c, true
		}
		u, ok := runner.(interface{ Unwrap() PromptRunner })
		if !ok {
			return nil, false
		}
		next := u.Unwrap()
		if any(next) == runner {
			return nil, false
		}
		runner = next
	}
	return nil, false
}

// Providers lists the configured LLM providers, or nil when the backend
// cannot tell.
func (g *core) Providers() []string {
	if c, ok := g.catalog(); ok {
		return c.Providers()
	}
	return nil
}

// ListModels asks provider (empty for the default) for its models.
func (g *core) ListModels(ctx context.Context, provider string) ([]string, error) {
	if c, ok := g.catalog(); ok {
		return c.ListModels(ctx, provider)
	}
	return nil, ai.ErrNoModelList
}

// Canonical exposes the LLM client's catalogue, looking through middleware.
func (r *DefaultPromptRunner) Canonical(provider string) (string, error) {
	if c, ok := ai.CatalogOf(r.llmClient); ok {
		return c.Canonical(provider)
	}
	return strings.ToLower(strings.TrimSpace(provider)), nil
}

// Providers exposes the LLM client's catalogue, looking through middleware.
func (r *DefaultPromptRunner) Providers() []string {
	if c, ok := ai.CatalogOf(r.llmClient); ok {
		return c.Providers()
	}
	return nil
}

// ListModels exposes the LLM client's catalogue, looking through middleware.
func (r *DefaultPromptRunner) ListModels(ctx context.Context, provider string) ([]string, error) {
	if c, ok := ai.CatalogOf(r.llmClient); ok {
		return c.ListModels(ctx, provider)
	}
	return nil, ai.ErrNoModelList
}
