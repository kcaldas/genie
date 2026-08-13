package ai

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

const CapabilitySourceRegistry CapabilitySource = "registry"

// ModelCapabilities is provider-neutral metadata about one model. Zero token
// limits and a nil modality map mean the registry does not know those fields.
type ModelCapabilities struct {
	Model               string            `json:"model"`
	InputTokenLimit     int               `json:"input_token_limit"`
	OutputTokenLimit    int               `json:"output_token_limit"`
	SharedContextWindow bool              `json:"shared_context_window,omitempty"`
	InputModalities     map[Modality]bool `json:"input_modalities,omitempty"`
	Source              CapabilitySource  `json:"source"`
}

// SupportsInput reports whether the registry explicitly enables a modality.
func (c ModelCapabilities) SupportsInput(modality Modality) bool {
	return c.InputModalities != nil && c.InputModalities[modality]
}
