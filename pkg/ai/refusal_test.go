package ai

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestRefusesRetryReadsTheHeader(t *testing.T) {
	h := http.Header{}
	if RefusesRetry(h) {
		t.Fatal("no header is not a refusal")
	}
	h.Set("X-Should-Retry", "false")
	if !RefusesRetry(h) {
		t.Fatal("x-should-retry: false is a refusal")
	}
	h.Set("X-Should-Retry", "true")
	if RefusesRetry(h) {
		t.Fatal("x-should-retry: true is not a refusal")
	}
}

type sdkError struct{ resp *http.Response }

func (e *sdkError) Error() string { return "429 Too Many Requests" }

func refusingResponse(reason string) *http.Response {
	h := http.Header{}
	h.Set("X-Should-Retry", "false")
	h.Set("X-Reason", reason)
	return &http.Response{StatusCode: http.StatusTooManyRequests, Header: h}
}

func TestRefusalIsFinalAndKeepsItsResponse(t *testing.T) {
	err := fmt.Errorf("turn: %w", &Refusal{StatusCode: 429, Header: refusingResponse("spend").Header, Err: errors.New("refused")})
	if IsRetryable(err) {
		t.Fatal("a refusal must not be retried")
	}
	r, ok := AsRefusal(err)
	if !ok || r.StatusCode != 429 || r.Header.Get("X-Reason") != "spend" {
		t.Fatalf("refusal %+v, %v", r, ok)
	}
}

func TestRegisteredResponseMakesAnSDKErrorARefusal(t *testing.T) {
	RegisterResponseError(func(err error) *http.Response {
		var e *sdkError
		if errors.As(err, &e) {
			return e.resp
		}
		return nil
	})
	plain := fmt.Errorf("turn: %w", &sdkError{resp: &http.Response{StatusCode: 429, Header: http.Header{}}})
	if !IsRetryable(plain) {
		t.Fatal("an ordinary 429 stays retryable")
	}
	if _, ok := AsRefusal(plain); ok {
		t.Fatal("an ordinary 429 is not a refusal")
	}
	refused := fmt.Errorf("turn: %w", &sdkError{resp: refusingResponse("spend")})
	if IsRetryable(refused) {
		t.Fatal("a response that refuses retry must not be retried")
	}
	r, ok := AsRefusal(refused)
	if !ok || r.StatusCode != 429 || r.Header.Get("X-Reason") != "spend" || !errors.Is(r, r.Err) {
		t.Fatalf("refusal %+v, %v", r, ok)
	}
}
