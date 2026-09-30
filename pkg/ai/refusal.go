package ai

import (
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

var (
	finalMu         sync.RWMutex
	finalClassifier []func(error) bool
)

// RegisterFinalError adds a check that recognises a provider SDK's error for
// a response that must not be retried. Provider packages register one for
// their SDK's error type; IsRetryable consults them all.
func RegisterFinalError(isFinal func(error) bool) {
	finalMu.Lock()
	defer finalMu.Unlock()
	finalClassifier = append(finalClassifier, isFinal)
}

func isRegisteredFinal(err error) bool {
	finalMu.RLock()
	defer finalMu.RUnlock()
	for _, isFinal := range finalClassifier {
		if isFinal(err) {
			return true
		}
	}
	return false
}
