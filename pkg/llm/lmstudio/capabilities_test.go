package lmstudio

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capabilityHTTPDoer func(*http.Request) (*http.Response, error)

func (f capabilityHTTPDoer) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestDiscoverModelCapabilitiesUsesNativeModelsAPI(t *testing.T) {
	doer := capabilityHTTPDoer(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, req.Method)
		require.Equal(t, "/api/v1/models", req.URL.Path)
		body := `{"models":[{"id":"local-vlm","type":"vlm","max_context_length":32768,"capabilities":{"tool_use":true,"reasoning":{"supported":true}},"loaded_instances":[{"config":{"context_length":8192}}]}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(body)), Header: make(http.Header)}, nil
	})
	raw, err := NewClient(&events.NoOpEventBus{}, WithBaseURL("http://lmstudio.test"), WithHTTPClient(doer))
	require.NoError(t, err)

	caps, err := raw.(*Client).DiscoverModelCapabilities(context.Background(), "local-vlm")
	require.NoError(t, err)
	assert.Equal(t, 8192, caps.InputTokenLimit)
	assert.True(t, caps.SupportsInput(ai.ModalityImage))
	assert.True(t, caps.SupportsTools)
	assert.True(t, caps.SupportsReasoning)
}
