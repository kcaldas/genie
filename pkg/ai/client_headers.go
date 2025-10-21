package ai

import "net/http"

const (
	// ClientHeaderName is the header we send to downstream LLM providers identifying Genie.
	ClientHeaderName = "X-Genie-Client"
	// ClientHeaderValue is the value Genie uses when identifying itself to LLM providers.
	ClientHeaderValue = "genie"
)

// DefaultHTTPHeaders returns a copy of the standard Genie headers for outbound LLM requests.
func DefaultHTTPHeaders() http.Header {
	h := make(http.Header)
	h.Add(ClientHeaderName, ClientHeaderValue)
	return h
}
