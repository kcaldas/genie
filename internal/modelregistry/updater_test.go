package modelregistry

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	geniectx "github.com/kcaldas/genie/pkg/ctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDiscoverer struct {
	models []string
	caps   map[string]ai.ModelCapabilities
	err    error
}

func (f fakeDiscoverer) CapabilityCacheKey(model string) ai.CapabilityCacheKey {
	return ai.CapabilityCacheKey{Provider: "fake", Model: model}
}
func (f fakeDiscoverer) DiscoverableModels(context.Context) ([]string, error) {
	return f.models, f.err
}
func (f fakeDiscoverer) DiscoverModelCapabilities(_ context.Context, model string) (ai.ModelCapabilities, error) {
	if f.err != nil {
		return ai.ModelCapabilities{}, f.err
	}
	return f.caps[model], nil
}

func initialSnapshot(t *testing.T, providers map[string]geniectx.ProviderRegistry) []byte {
	t.Helper()
	snapshot := geniectx.RegistrySnapshot{
		SchemaVersion: 1, GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		HandMaintained: []string{"openai"}, Providers: providers,
	}
	data, err := jsonMarshal(snapshot)
	require.NoError(t, err)
	return data
}

func TestUpdateIsByteIdenticalOnSecondRun(t *testing.T) {
	fake := fakeDiscoverer{models: []string{"model-b", "model-a"}, caps: map[string]ai.ModelCapabilities{
		"model-a": {Model: "model-a", InputTokenLimit: 1000},
		"model-b": {Model: "model-b", InputTokenLimit: 2000},
	}}
	source := Source{Name: "google", Provenance: "models.list/models.get", Discoverer: fake}
	first, changed, err := Update(context.Background(), initialSnapshot(t, map[string]geniectx.ProviderRegistry{}), []Source{source}, func() time.Time { return time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC) })
	require.NoError(t, err)
	assert.True(t, changed)

	second, changed, err := Update(context.Background(), first, []Source{source}, func() time.Time { return time.Date(2030, 2, 1, 0, 0, 0, 0, time.UTC) })
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, first, second)
}

func TestUpdateRetainsDisappearedModelsAsStale(t *testing.T) {
	existing := initialSnapshot(t, map[string]geniectx.ProviderRegistry{"google": {
		Source: "old", Entries: map[string]geniectx.RegistryEntry{"gone": {ModelInfo: geniectx.ModelInfo{ContextWindow: 99, InputLimit: geniectx.InputLimitShared}}},
	}})
	fake := fakeDiscoverer{models: []string{"current"}, caps: map[string]ai.ModelCapabilities{
		"current": {Model: "current", InputTokenLimit: 1000},
	}}
	data, _, err := Update(context.Background(), existing, []Source{{Name: "google", Provenance: "new", Discoverer: fake}}, time.Now)
	require.NoError(t, err)
	snapshot, err := geniectx.ParseRegistrySnapshot(data)
	require.NoError(t, err)
	assert.True(t, snapshot.Providers["google"].Entries["gone"].Stale)
	assert.Contains(t, snapshot.Providers["google"].Entries["gone"].Note, "human review")
}

func TestUpdateFailureReturnsNoPartialOutput(t *testing.T) {
	data, changed, err := Update(context.Background(), initialSnapshot(t, nil), []Source{{
		Name: "google", Discoverer: fakeDiscoverer{err: errors.New("network down")},
	}}, time.Now)
	assert.ErrorContains(t, err, "network down")
	assert.False(t, changed)
	assert.Nil(t, data)
}

func TestUpdateKeepsUnknownModalitiesNilAndInputOnlySemantic(t *testing.T) {
	fake := fakeDiscoverer{models: []string{"model"}, caps: map[string]ai.ModelCapabilities{
		"model": {Model: "model", InputTokenLimit: 1000, InputModalities: nil},
	}}
	data, _, err := Update(context.Background(), initialSnapshot(t, nil), []Source{{Name: "google", Discoverer: fake}}, time.Now)
	require.NoError(t, err)
	snapshot, err := geniectx.ParseRegistrySnapshot(data)
	require.NoError(t, err)
	entry := snapshot.Providers["google"].Entries["model"]
	assert.Nil(t, entry.InputModalities)
	assert.Equal(t, geniectx.InputLimitInputOnly, entry.InputLimit)
}

func jsonMarshal(snapshot geniectx.RegistrySnapshot) ([]byte, error) {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	return append(data, '\n'), err
}
