package openai

import (
	"testing"
)

// Reasoning is on by default from gpt-5 onward, and a tool-carrying
// /v1/chat/completions request is refused while it is. The rule tests the
// generation rather than the model name, so a family released tomorrow is
// covered and an older one is left alone.
func TestGptGeneration(t *testing.T) {
	for _, tc := range []struct {
		model string
		want  float64
		isGPT bool
	}{
		{"gpt-5.6-luna", 5.6, true},
		{"gpt-5", 5, true},
		{"gpt-6-something", 6, true},
		{"gpt-4o", 4, true},
		{"gpt-4.1-mini", 4.1, true},
		{"o4-mini", 0, false},
		{"claude-sonnet-5", 0, false},
	} {
		got, ok := gptGeneration(tc.model)
		if ok != tc.isGPT || got != tc.want {
			t.Errorf("gptGeneration(%q) = %v,%v; want %v,%v", tc.model, got, ok, tc.want, tc.isGPT)
		}
	}
}

func TestUseResponsesAPI(t *testing.T) {
	for _, tc := range []struct {
		model string
		want  bool
	}{
		{"gpt-5.6-luna", true},
		{"gpt-5", true},
		{"gpt-6-something", true},
		{"gpt-4o", false},
		{"gpt-4.1-mini", false},
		{"o4-mini", false},
		{"claude-sonnet-5", false},
	} {
		if got := useResponsesAPI(tc.model); got != tc.want {
			t.Errorf("useResponsesAPI(%q) = %v, want %v", tc.model, got, tc.want)
		}
	}
}

// gpt-5.6 answers a request carrying temperature with
// 400 "Unsupported parameter: 'temperature' is not supported with this model",
// on both /v1/responses and chat/completions. The gate keys on the generation
// rather than a name list, and deliberately leaves non-OpenAI models routed
// through this client (OpenAI-compatible endpoints) alone.
func TestAllowsSamplingParamsByGeneration(t *testing.T) {
	for _, tc := range []struct {
		model string
		want  bool
	}{
		{"gpt-5.6-luna", false},
		{"gpt-5", false},
		{"gpt-6-mini", false},
		{"gpt-4o", true},
		{"gpt-4o-mini", true},
		{"gpt-4.1-mini", true},
		{"gpt-3.5-turbo", true},
		{"o1-preview", false},
		{"o4-mini", false},
		// Third-party models reached through OPENAI_BASE_URL keep whatever
		// they had; this change is about OpenAI's own generations.
		{"deepseek-v4-flash", true},
	} {
		if got := allowsSamplingParams(tc.model); got != tc.want {
			t.Errorf("allowsSamplingParams(%q) = %v, want %v", tc.model, got, tc.want)
		}
	}
}
