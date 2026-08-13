package ai

import "context"

// Modality is a model input category recorded in the static model registry.
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
	CapabilitySourceRegistry CapabilitySource = "registry"
	CapabilitySourceProvider CapabilitySource = "provider"
)

// ModelCapabilities is provider-neutral metadata about one model. Zero token
// limits and a nil modality map mean the registry does not know those fields.
type ModelCapabilities struct {
	Model               string            `json:"model"`
	InputTokenLimit     int               `json:"input_token_limit"`
	OutputTokenLimit    int               `json:"output_token_limit"`
	SharedContextWindow bool              `json:"shared_context_window,omitempty"`
	InputModalities     map[Modality]bool `json:"input_modalities,omitempty"`
	SupportsTools       bool              `json:"supports_tools,omitempty"`
	SupportsReasoning   bool              `json:"supports_reasoning,omitempty"`
	Source              CapabilitySource  `json:"source"`
}

// SupportsInput reports whether the registry explicitly enables a modality.
func (c ModelCapabilities) SupportsInput(modality Modality) bool {
	return c.InputModalities != nil && c.InputModalities[modality]
}

// CapabilityCacheKey identifies the provider connection and model used by a
// maintenance discoverer. The offline updater records it as provenance; it is
// not a runtime cache key.
type CapabilityCacheKey struct {
	Provider  string `json:"provider"`
	Authority string `json:"authority"`
	Tenant    string `json:"tenant,omitempty"`
	Model     string `json:"model"`
}

// ModelCapabilityDiscoverer is the command-time fetch surface used by the
// offline registry updater. Request execution never invokes this interface.
type ModelCapabilityDiscoverer interface {
	CapabilityCacheKey(model string) CapabilityCacheKey
	DiscoverModelCapabilities(ctx context.Context, model string) (ModelCapabilities, error)
}

// ModelCatalogDiscoverer lists models visible to one configured provider.
type ModelCatalogDiscoverer interface {
	ModelCapabilityDiscoverer
	DiscoverableModels(ctx context.Context) ([]string, error)
}
