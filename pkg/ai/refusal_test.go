package ai

import (
	"errors"
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

type sdkError struct{ header http.Header }

func (e *sdkError) Error() string { return "429 Too Many Requests" }

func TestRegisteredClassifierMakesAnErrorFinal(t *testing.T) {
	err := errors.New("wrapped: " + (&sdkError{}).Error())
	wrapped := &sdkError{header: http.Header{"X-Should-Retry": []string{"false"}}}
	RegisterFinalError(func(err error) bool {
		var e *sdkError
		return errors.As(err, &e) && RefusesRetry(e.header)
	})
	if !IsRetryable(err) {
		t.Fatal("an unrelated error stays retryable")
	}
	if IsRetryable(wrapped) {
		t.Fatal("a response that refuses retry must not be retried")
	}
}
