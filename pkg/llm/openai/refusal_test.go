package openai

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	openai "github.com/openai/openai-go/v3"
)

func TestSDKErrorThatRefusesRetryIsFinal(t *testing.T) {
	final := fmt.Errorf("turn: %w", &openai.Error{StatusCode: 429, Response: &http.Response{StatusCode: 429, Header: http.Header{"X-Should-Retry": []string{"false"}, "X-Reason": []string{"spend"}}}})
	if ai.IsRetryable(final) {
		t.Fatal("a 429 with x-should-retry: false must not be retried")
	}
	if r, ok := ai.AsRefusal(final); !ok || r.StatusCode != 429 || r.Header.Get("X-Reason") != "spend" {
		t.Fatalf("refusal %+v, %v", r, ok)
	}
	plain := fmt.Errorf("turn: %w", &openai.Error{StatusCode: 503, Response: &http.Response{StatusCode: 503, Header: http.Header{}}})
	if !ai.IsRetryable(plain) {
		t.Fatal("an ordinary 503 stays retryable")
	}
}

// The SDK's own text is only the status; the provider's description is
// what tells a context overflow from a rate limit.
func TestAPIErrorKeepsTheProvidersMessage(t *testing.T) {
	sdk := &openai.Error{StatusCode: 400, Message: "maximum context length exceeded"}
	err := apiError(sdk)
	if !strings.Contains(err.Error(), "maximum context length exceeded") || !errors.Is(err, sdk) {
		t.Fatalf("%v", err)
	}
	if plain := errors.New("dial tcp: refused"); apiError(plain) != plain {
		t.Fatal("other errors pass through unchanged")
	}
}
