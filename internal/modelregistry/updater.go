package modelregistry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	geniectx "github.com/kcaldas/genie/pkg/ctx"
)

// Source is one authoritative provider catalog used during maintenance.
type Source struct {
	Name       string
	Provenance string
	Discoverer ai.ModelCatalogDiscoverer
}

// Update refreshes selected provider blocks in an existing snapshot. It does
// not mutate the input and returns no bytes when any provider fails.
func Update(ctx context.Context, existing []byte, sources []Source, now func() time.Time) ([]byte, bool, error) {
	snapshot, err := geniectx.ParseRegistrySnapshot(existing)
	if err != nil {
		return nil, false, fmt.Errorf("parse existing registry: %w", err)
	}
	if now == nil {
		now = time.Now
	}

	for _, source := range sources {
		if strings.TrimSpace(source.Name) == "" || source.Discoverer == nil {
			return nil, false, fmt.Errorf("invalid empty model-registry source")
		}
		provider, err := discoverProvider(ctx, source, snapshot.Providers[source.Name])
		if err != nil {
			return nil, false, fmt.Errorf("refresh %s: %w", source.Name, err)
		}
		snapshot.Providers[source.Name] = provider
	}

	snapshot.SchemaVersion = 1
	snapshot.HandMaintained = handMaintainedPrefixes()
	if unchangedSnapshot(existing, snapshot) {
		return append([]byte(nil), existing...), false, nil
	}
	snapshot.GeneratedAt = now().UTC().Truncate(time.Second)
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, false, fmt.Errorf("encode generated registry: %w", err)
	}
	return append(data, '\n'), true, nil
}

func handMaintainedPrefixes() []string {
	models := geniectx.HandMaintainedModelPrefixes()
	prefixes := make([]string, 0, len(models))
	for model := range models {
		prefixes = append(prefixes, model)
	}
	sort.Strings(prefixes)
	return prefixes
}

func discoverProvider(ctx context.Context, source Source, previous geniectx.ProviderRegistry) (geniectx.ProviderRegistry, error) {
	models, err := source.Discoverer.DiscoverableModels(ctx)
	if err != nil {
		return geniectx.ProviderRegistry{}, err
	}
	models = uniqueSorted(models)
	if len(models) == 0 {
		return geniectx.ProviderRegistry{}, fmt.Errorf("provider returned no discoverable models")
	}

	entries := make(map[string]geniectx.RegistryEntry, len(models)+len(previous.Entries))
	for _, model := range models {
		caps, err := source.Discoverer.DiscoverModelCapabilities(ctx, model)
		if err != nil {
			return geniectx.ProviderRegistry{}, err
		}
		if caps.InputTokenLimit <= 0 {
			return geniectx.ProviderRegistry{}, fmt.Errorf("model %q returned no usable input limit", model)
		}
		resolved := strings.TrimSpace(caps.Model)
		if resolved == "" {
			resolved = model
		}
		semantics := geniectx.InputLimitInputOnly
		if caps.SharedContextWindow {
			semantics = geniectx.InputLimitShared
		}
		entries[resolved] = geniectx.RegistryEntry{ModelInfo: geniectx.ModelInfo{
			ContextWindow: caps.InputTokenLimit, MaxOutputTokens: caps.OutputTokenLimit,
			InputModalities: cloneModalities(caps.InputModalities), InputLimit: semantics,
		}}
	}
	for model, old := range previous.Entries {
		if _, ok := entries[model]; ok {
			continue
		}
		old.Stale = true
		old.Note = "not returned by provider; retained pending human review"
		entries[model] = old
	}
	return geniectx.ProviderRegistry{Source: source.Provenance, Entries: entries}, nil
}

func uniqueSorted(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	result := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		result = append(result, model)
	}
	sort.Strings(result)
	return result
}

func cloneModalities(modalities map[ai.Modality]bool) map[ai.Modality]bool {
	if modalities == nil {
		return nil
	}
	clone := make(map[ai.Modality]bool, len(modalities))
	for modality, supported := range modalities {
		clone[modality] = supported
	}
	return clone
}

func unchangedSnapshot(existing []byte, candidate geniectx.RegistrySnapshot) bool {
	current, err := geniectx.ParseRegistrySnapshot(existing)
	if err != nil {
		return false
	}
	current.GeneratedAt = time.Time{}
	candidate.GeneratedAt = time.Time{}
	a, _ := json.Marshal(current)
	b, _ := json.Marshal(candidate)
	return bytes.Equal(a, b)
}
