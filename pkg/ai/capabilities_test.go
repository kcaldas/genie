package ai

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapabilityResolverSharesStoreAcrossInstances(t *testing.T) {
	store := NewMemoryCapabilityStore()
	first := NewCapabilityResolver(store)
	second := NewCapabilityResolver(store)
	key := CapabilityCacheKey{Provider: "genai", Authority: "example.test", Model: "model-a"}

	var calls atomic.Int32
	discover := func(context.Context) (ModelCapabilities, error) {
		calls.Add(1)
		return ModelCapabilities{Model: "model-a", InputTokenLimit: 1_000_000, Source: CapabilitySourceProvider}, nil
	}

	caps, err := first.Resolve(context.Background(), key, time.Hour, discover)
	require.NoError(t, err)
	assert.False(t, caps.Cached)

	caps, err = second.Resolve(context.Background(), key, time.Hour, discover)
	require.NoError(t, err)
	assert.True(t, caps.Cached)
	assert.Equal(t, int32(1), calls.Load())
}

func TestCapabilityResolverCoalescesConcurrentDiscovery(t *testing.T) {
	resolver := NewCapabilityResolver(nil)
	key := CapabilityCacheKey{Provider: "anthropic", Authority: "example.test", Model: "model-a"}

	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	discover := func(context.Context) (ModelCapabilities, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return ModelCapabilities{Model: "model-a", InputTokenLimit: 200_000}, nil
	}

	const callers = 8
	var wg sync.WaitGroup
	wg.Add(callers)
	errs := make(chan error, callers)
	for range callers {
		go func() {
			defer wg.Done()
			_, err := resolver.Resolve(context.Background(), key, time.Hour, discover)
			errs <- err
		}()
	}
	<-started
	close(release)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, int32(1), calls.Load())
}

func TestCapabilityResolverUsesStaleEntryWhenRefreshFails(t *testing.T) {
	store := NewMemoryCapabilityStore()
	resolver := NewCapabilityResolver(store)
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	resolver.now = func() time.Time { return now }
	key := CapabilityCacheKey{Provider: "genai", Authority: "example.test", Model: "model-a"}
	require.NoError(t, store.Put(context.Background(), key, CapabilityEntry{
		Capabilities: ModelCapabilities{Model: "model-a", InputTokenLimit: 128_000},
		FetchedAt:    now.Add(-2 * time.Hour),
		ExpiresAt:    now.Add(-time.Hour),
	}))

	caps, err := resolver.Resolve(context.Background(), key, time.Hour, func(context.Context) (ModelCapabilities, error) {
		return ModelCapabilities{}, errors.New("provider unavailable")
	})
	require.NoError(t, err)
	assert.Equal(t, 128_000, caps.InputTokenLimit)
	assert.True(t, caps.Cached)
	assert.True(t, caps.Stale)
}

func TestCapabilityCacheKeySeparatesAuthoritiesAndTenants(t *testing.T) {
	store := NewMemoryCapabilityStore()
	resolver := NewCapabilityResolver(store)
	var calls atomic.Int32
	discover := func(context.Context) (ModelCapabilities, error) {
		return ModelCapabilities{InputTokenLimit: int(calls.Add(1))}, nil
	}

	keys := []CapabilityCacheKey{
		{Provider: "openai", Authority: "api.example-a.test", Tenant: "agent-1", Model: "same-model"},
		{Provider: "openai", Authority: "api.example-b.test", Tenant: "agent-1", Model: "same-model"},
		{Provider: "openai", Authority: "api.example-a.test", Tenant: "agent-2", Model: "same-model"},
	}
	for _, key := range keys {
		_, err := resolver.Resolve(context.Background(), key, time.Hour, discover)
		require.NoError(t, err)
	}
	assert.Equal(t, int32(len(keys)), calls.Load())
}
