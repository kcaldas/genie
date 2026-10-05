package anthropic

import (
	"fmt"
	"net/http"
	"testing"

	anthropic_sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/kcaldas/genie/pkg/ai"
)

func TestSDKErrorThatRefusesRetryIsFinal(t *testing.T) {
	final := fmt.Errorf("turn: %w", &anthropic_sdk.Error{StatusCode: 429, Response: &http.Response{StatusCode: 429, Header: http.Header{"X-Should-Retry": []string{"false"}, "X-Reason": []string{"spend"}}}})
	if ai.IsRetryable(final) {
		t.Fatal("a 429 with x-should-retry: false must not be retried")
	}
	if r, ok := ai.AsRefusal(final); !ok || r.StatusCode != 429 || r.Header.Get("X-Reason") != "spend" {
		t.Fatalf("refusal %+v, %v", r, ok)
	}
	plain := fmt.Errorf("turn: %w", &anthropic_sdk.Error{StatusCode: 429, Response: &http.Response{StatusCode: 429, Header: http.Header{}}})
	if !ai.IsRetryable(plain) {
		t.Fatal("an ordinary 429 stays retryable")
	}
}
