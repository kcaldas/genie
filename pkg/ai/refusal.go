package ai

import (
	"errors"
	"net/http"
	"strings"
	"sync"
)

// RefusesRetry reports a response that asks its client not to retry
// (`x-should-retry: false`, the header the Anthropic and OpenAI SDKs honour).
// Budget and entitlement refusals from an LLM proxy use it: repeating them
// cannot succeed and only multiplies requests.
func RefusesRetry(h http.Header) bool {
	return strings.EqualFold(strings.TrimSpace(h.Get("X-Should-Retry")), "false")
}

// Refusal is a response that refused its call for good (RefusesRetry), such
// as an LLM proxy's budget refusal. It keeps the response's status and
// headers, where such an endpoint says why, so the application can tell the
// person what happened rather than report a generic failure.
type Refusal struct {
	StatusCode int
	Header     http.Header
	Err        error
}

func (r *Refusal) Error() string { return r.Err.Error() }
func (r *Refusal) Unwrap() error { return r.Err }

var (
	responseMu sync.RWMutex
	responseOf []func(error) *http.Response
)

// RegisterResponseError adds a lookup for the HTTP response behind a provider
// SDK's error. Provider packages register one for their SDK's error type, so
// AsRefusal recognises a refusal the SDK reported.
func RegisterResponseError(lookup func(error) *http.Response) {
	responseMu.Lock()
	defer responseMu.Unlock()
	responseOf = append(responseOf, lookup)
}

// AsRefusal finds the refusal in err's chain: a Refusal, or a provider SDK's
// error for a response that refuses retry.
func AsRefusal(err error) (*Refusal, bool) {
	if err == nil {
		return nil, false
	}
	var r *Refusal
	if errors.As(err, &r) {
		return r, true
	}
	responseMu.RLock()
	defer responseMu.RUnlock()
	for _, lookup := range responseOf {
		if resp := lookup(err); resp != nil && RefusesRetry(resp.Header) {
			return &Refusal{StatusCode: resp.StatusCode, Header: resp.Header, Err: err}, true
		}
	}
	return nil, false
}
