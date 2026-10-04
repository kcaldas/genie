package openai

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	openai "github.com/openai/openai-go/v3"
)

func TestSDKErrorThatRefusesRetryIsFinal(t *testing.T) {
	final := fmt.Errorf("turn: %w", &openai.Error{StatusCode: 429, Response: &http.Response{StatusCode: 429, Header: http.Header{"X-Should-Retry": []string{"false"}}}})
	if ai.IsRetryable(final) {
		t.Fatal("a 429 with x-should-retry: false must not be retried")
	}
	plain := fmt.Errorf("turn: %w", &openai.Error{StatusCode: 503, Response: &http.Response{StatusCode: 503, Header: http.Header{}}})
	if !ai.IsRetryable(plain) {
		t.Fatal("an ordinary 503 stays retryable")
	}
}
