package ollama

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

func TestDiscoverModelCapabilitiesUsesShowAndAllocatedContext(t *testing.T) {
	doer := capabilityHTTPDoer(func(req *http.Request) (*http.Response, error) {
		var body string
		switch req.URL.Path {
		case "/api/show":
			require.Equal(t, http.MethodPost, req.Method)
			body = `{"capabilities":["completion","vision","tools"],"model_info":{"gemma3.context_length":131072}}`
		case "/api/ps":
			require.Equal(t, http.MethodGet, req.Method)
			body = `{"models":[{"name":"gemma3","model":"gemma3","context_length":4096}]}`
		default:
			t.Fatalf("unexpected path %s", req.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(body)), Header: make(http.Header)}, nil
	})
	raw, err := NewClient(&events.NoOpEventBus{}, WithBaseURL("http://ollama.test"), WithHTTPClient(doer))
	require.NoError(t, err)

	caps, err := raw.(*Client).DiscoverModelCapabilities(context.Background(), "gemma3")
	require.NoError(t, err)
	assert.Equal(t, 4096, caps.InputTokenLimit)
	assert.True(t, caps.SharedContextWindow)
	assert.True(t, caps.SupportsInput(ai.ModalityImage))
	assert.True(t, caps.SupportsTools)
	assert.Equal(t, ai.CapabilitySourceProvider, caps.Source)
}
