package ai

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptedGen returns the queued errors in order, then succeeds.
type scriptedGen struct {
	Gen   // panics if an unstubbed method is called
	errs  []error
	calls int
}

func (s *scriptedGen) GenerateContentAttr(ctx context.Context, p Prompt, debug bool, attrs []Attr) (string, error) {
	s.calls++
	if len(s.errs) > 0 {
		err := s.errs[0]
		s.errs = s.errs[1:]
		if err != nil {
			return "", err
		}
	}
	return "success", nil
}

func (s *scriptedGen) GenerateContentAttrStream(ctx context.Context, p Prompt, debug bool, attrs []Attr) (Stream, error) {
	s.calls++
	if len(s.errs) > 0 {
		err := s.errs[0]
		s.errs = s.errs[1:]
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func newTestRetry(underlying Gen) *RetryMiddleware {
	return NewRetryMiddleware(underlying, RetryConfig{
		Enabled:        true,
		MaxRetries:     3,
		InitialBackoff: time.Millisecond,
	})
}

func TestRetryRecoversFromTransientError(t *testing.T) {
	gen := &scriptedGen{errs: []error{errors.New("http 503: overloaded")}}
	mw := newTestRetry(gen)

	resp, err := mw.GenerateContentAttr(context.Background(), Prompt{}, false, nil)
	require.NoError(t, err)
	assert.Equal(t, "success", resp)
	assert.Equal(t, 2, gen.calls)
}

// Cancellation is a user decision, not a transient fault. Retrying it
// wastes time (and, once retry wraps single requests, re-executes work
// the user explicitly abandoned).
func TestRetryDoesNotRetryCancellation(t *testing.T) {
	gen := &scriptedGen{errs: []error{context.Canceled, context.Canceled, context.Canceled}}
	mw := newTestRetry(gen)

	_, err := mw.GenerateContentAttr(context.Background(), Prompt{}, false, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, gen.calls, "a cancelled attempt must not be retried")
}

func TestRetryDoesNotRetryNonRetryableErrors(t *testing.T) {
	permanent := NonRetryable(errors.New("ANTHROPIC_API_KEY not configured"))
	gen := &scriptedGen{errs: []error{permanent, permanent}}
	mw := newTestRetry(gen)

	_, err := mw.GenerateContentAttr(context.Background(), Prompt{}, false, nil)
	require.Error(t, err)
	assert.Equal(t, 1, gen.calls, "a permanent error must not be retried")
}

// Backoff must respect context cancellation instead of sleeping blindly
// against a dead request.
func TestRetryBackoffAbortsOnContextCancellation(t *testing.T) {
	gen := &scriptedGen{errs: []error{
		errors.New("transient one"),
		errors.New("transient two"),
		errors.New("transient three"),
	}}
	mw := NewRetryMiddleware(gen, RetryConfig{
		Enabled:        true,
		MaxRetries:     3,
		InitialBackoff: 30 * time.Second, // would block for ~a minute if ignored
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := mw.GenerateContentAttr(ctx, Prompt{}, false, nil)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, elapsed, 5*time.Second, "backoff must abort when the context is cancelled")
	assert.Equal(t, 1, gen.calls)
}

func TestRetryStreamDoesNotRetryCancellation(t *testing.T) {
	gen := &scriptedGen{errs: []error{context.Canceled}}
	mw := newTestRetry(gen)

	_, err := mw.GenerateContentAttrStream(context.Background(), Prompt{}, false, nil)
	require.Error(t, err)
	assert.Equal(t, 1, gen.calls)
}
