package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var ErrCapabilityDiscoveryUnsupported = errors.New("model capability discovery is not supported")

// Modality is a model input category reported by a provider.
type Modality string

const (
	ModalityText     Modality = "text"
	ModalityImage    Modality = "image"
	ModalityAudio    Modality = "audio"
	ModalityVideo    Modality = "video"
	ModalityDocument Modality = "document"
)

// CapabilitySource identifies where model metadata came from.
type CapabilitySource string

const (
	CapabilitySourceProvider CapabilitySource = "provider"
	CapabilitySourceFallback CapabilitySource = "fallback"
)

// ModelCapabilities is provider-neutral metadata about one model. Zero token
// limits and a nil modality map mean the provider did not report those fields.
// Support booleans are true only when the provider explicitly advertises them.
type ModelCapabilities struct {
	Model             string            `json:"model"`
	InputTokenLimit   int               `json:"input_token_limit"`
	OutputTokenLimit  int               `json:"output_token_limit"`
	InputModalities   map[Modality]bool `json:"input_modalities,omitempty"`
	SupportsTools     bool              `json:"supports_tools,omitempty"`
	SupportsReasoning bool              `json:"supports_reasoning,omitempty"`
	Source            CapabilitySource  `json:"source"`
	Cached            bool              `json:"cached,omitempty"`
	Stale             bool              `json:"stale,omitempty"`
}

// SupportsInput reports whether a provider explicitly advertised a modality.
func (c ModelCapabilities) SupportsInput(modality Modality) bool {
	return c.InputModalities != nil && c.InputModalities[modality]
}

// CapabilityCacheKey scopes cached metadata to a provider connection. Tenant
// is a non-secret host namespace, project/location, or equivalent account
// boundary. SchemaVersion permits incompatible cache entries to be invalidated.
type CapabilityCacheKey struct {
	SchemaVersion int    `json:"schema_version"`
	Provider      string `json:"provider"`
	Authority     string `json:"authority"`
	Tenant        string `json:"tenant,omitempty"`
	Model         string `json:"model"`
}

const CapabilityCacheSchemaVersion = 1

func (k CapabilityCacheKey) normalized() CapabilityCacheKey {
	if k.SchemaVersion == 0 {
		k.SchemaVersion = CapabilityCacheSchemaVersion
	}
	k.Provider = strings.ToLower(strings.TrimSpace(k.Provider))
	k.Authority = strings.TrimSpace(k.Authority)
	k.Tenant = strings.TrimSpace(k.Tenant)
	k.Model = strings.TrimSpace(k.Model)
	return k
}

func (k CapabilityCacheKey) flightKey() string {
	k = k.normalized()
	return fmt.Sprintf("%d|%q|%q|%q|%q", k.SchemaVersion, k.Provider, k.Authority, k.Tenant, k.Model)
}

// CapabilityEntry is the serializable value stored by CapabilityStore.
type CapabilityEntry struct {
	Capabilities ModelCapabilities `json:"capabilities"`
	FetchedAt    time.Time         `json:"fetched_at"`
	ExpiresAt    time.Time         `json:"expires_at"`
}

// CapabilityStore lets hosts retain model metadata beyond a Genie instance.
// Mutiro can provide an agent-wide or durable implementation.
type CapabilityStore interface {
	Get(ctx context.Context, key CapabilityCacheKey) (CapabilityEntry, bool, error)
	Put(ctx context.Context, key CapabilityCacheKey, entry CapabilityEntry) error
}

// MemoryCapabilityStore is the default process-local capability store.
type MemoryCapabilityStore struct {
	mu      sync.RWMutex
	entries map[CapabilityCacheKey]CapabilityEntry
}

func NewMemoryCapabilityStore() *MemoryCapabilityStore {
	return &MemoryCapabilityStore{entries: make(map[CapabilityCacheKey]CapabilityEntry)}
}

func (s *MemoryCapabilityStore) Get(_ context.Context, key CapabilityCacheKey) (CapabilityEntry, bool, error) {
	if s == nil {
		return CapabilityEntry{}, false, nil
	}
	s.mu.RLock()
	entry, ok := s.entries[key.normalized()]
	s.mu.RUnlock()
	entry.Capabilities = cloneCapabilities(entry.Capabilities)
	return entry, ok, nil
}

func (s *MemoryCapabilityStore) Put(_ context.Context, key CapabilityCacheKey, entry CapabilityEntry) error {
	if s == nil {
		return nil
	}
	entry.Capabilities = cloneCapabilities(entry.Capabilities)
	s.mu.Lock()
	s.entries[key.normalized()] = entry
	s.mu.Unlock()
	return nil
}

type capabilityFlight struct {
	done chan struct{}
	caps ModelCapabilities
	err  error
}

// CapabilityResolver owns refresh policy and request coalescing independently
// of provider-client lifetimes. Share one resolver across cycled Genie instances
// to avoid one discovery request per instance.
type CapabilityResolver struct {
	store CapabilityStore

	mu      sync.Mutex
	flights map[string]*capabilityFlight
	now     func() time.Time
}

func NewCapabilityResolver(store CapabilityStore) *CapabilityResolver {
	if store == nil {
		store = NewMemoryCapabilityStore()
	}
	return &CapabilityResolver{
		store:   store,
		flights: make(map[string]*capabilityFlight),
		now:     time.Now,
	}
}

const DefaultCapabilityTTL = 6 * time.Hour

// Resolve returns fresh cached metadata when available, otherwise coalesces
// concurrent discovery calls. An expired value is used if refresh fails.
// Cache read/write failures never prevent provider discovery.
func (r *CapabilityResolver) Resolve(
	ctx context.Context,
	key CapabilityCacheKey,
	ttl time.Duration,
	discover func(context.Context) (ModelCapabilities, error),
) (ModelCapabilities, error) {
	if discover == nil {
		return ModelCapabilities{}, fmt.Errorf("capability discovery function is nil")
	}
	if r == nil {
		return discover(ctx)
	}
	if ttl <= 0 {
		ttl = DefaultCapabilityTTL
	}
	key = key.normalized()
	now := r.now()
	stale, hasStale, _ := r.store.Get(ctx, key)
	if hasStale && stale.ExpiresAt.After(now) {
		caps := cloneCapabilities(stale.Capabilities)
		caps.Cached = true
		caps.Stale = false
		return caps, nil
	}

	flightKey := key.flightKey()
	r.mu.Lock()
	if existing := r.flights[flightKey]; existing != nil {
		r.mu.Unlock()
		select {
		case <-existing.done:
			return cloneCapabilities(existing.caps), existing.err
		case <-ctx.Done():
			return ModelCapabilities{}, ctx.Err()
		}
	}
	flight := &capabilityFlight{done: make(chan struct{})}
	r.flights[flightKey] = flight
	r.mu.Unlock()

	// Another resolver sharing the store may have refreshed between the first
	// read and this process-local flight acquisition.
	if entry, ok, _ := r.store.Get(ctx, key); ok && entry.ExpiresAt.After(r.now()) {
		flight.caps = cloneCapabilities(entry.Capabilities)
		flight.caps.Cached = true
	} else {
		flight.caps, flight.err = discover(ctx)
		if flight.err == nil {
			flight.caps = cloneCapabilities(flight.caps)
			flight.caps.Cached = false
			flight.caps.Stale = false
			fetchedAt := r.now()
			_ = r.store.Put(ctx, key, CapabilityEntry{
				Capabilities: flight.caps,
				FetchedAt:    fetchedAt,
				ExpiresAt:    fetchedAt.Add(ttl),
			})
		} else if hasStale {
			flight.caps = cloneCapabilities(stale.Capabilities)
			flight.caps.Cached = true
			flight.caps.Stale = true
			flight.err = nil
		}
	}

	r.mu.Lock()
	delete(r.flights, flightKey)
	close(flight.done)
	r.mu.Unlock()
	return cloneCapabilities(flight.caps), flight.err
}

func cloneCapabilities(c ModelCapabilities) ModelCapabilities {
	if c.InputModalities != nil {
		c.InputModalities = make(map[Modality]bool, len(c.InputModalities))
		for modality, supported := range c.InputModalities {
			c.InputModalities[modality] = supported
		}
	}
	return c
}

// PromptCapabilityProvider resolves metadata for the provider and model selected
// by a prompt. It is optional; ai.Gen remains source-compatible for embedders.
type PromptCapabilityProvider interface {
	ModelCapabilities(ctx context.Context, prompt Prompt) (ModelCapabilities, error)
}

// ModelCapabilityDiscoverer is implemented by concrete provider clients and
// consumed by the multiplexer-backed resolver.
type ModelCapabilityDiscoverer interface {
	CapabilityCacheKey(model string) CapabilityCacheKey
	DiscoverModelCapabilities(ctx context.Context, model string) (ModelCapabilities, error)
}

// CapabilityTTLProvider optionally varies cache lifetime by model or backend.
type CapabilityTTLProvider interface {
	CapabilityTTL(model string) time.Duration
}
