package shared

import (
	"context"
	"fmt"
	"io"
	"runtime/debug"
	"sync"

	"github.com/kcaldas/genie/pkg/ai"
)

// RecoverToStream converts a panic in a streaming producer goroutine
// into a stream error instead of crashing the whole process. Tool
// handlers and response parsing run inside producer goroutines, where
// an unrecovered panic would otherwise take down the TUI mid-session.
//
// Defer it AFTER `defer close(ch)` so it runs first (LIFO) and the
// error is sent before the channel closes.
func RecoverToStream(ch chan<- StreamResult) {
	if r := recover(); r != nil {
		ch <- StreamResult{Err: fmt.Errorf("internal error in streaming producer: %v\n%s", r, debug.Stack())}
	}
}

// StreamResult represents a single item emitted by a streaming provider.
// Exactly one of Chunk or Err should be set.
type StreamResult struct {
	Chunk *ai.StreamChunk
	Err   error
}

// NewChunkStream wraps a channel of StreamResult values and exposes it as an ai.Stream.
// ctx is the (derived) context the producer goroutine runs under; when the
// producer stops because ctx was cancelled, Recv reports the cancellation
// instead of a clean end of stream, so partial output is never mistaken
// for a complete response. The provided cancel function is called when the
// stream is closed to allow callers to stop upstream work early.
func NewChunkStream(ctx context.Context, cancel context.CancelFunc, ch <-chan StreamResult) ai.Stream {
	return &chunkStream{
		ctx:    ctx,
		cancel: cancel,
		ch:     ch,
	}
}

type chunkStream struct {
	ctx    context.Context
	cancel context.CancelFunc

	mu             sync.Mutex
	ch             <-chan StreamResult
	closed         bool
	consumerClosed bool
}

func (s *chunkStream) Recv() (*ai.StreamChunk, error) {
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return nil, io.EOF
	}

	result, ok := <-s.ch
	if !ok {
		s.mu.Lock()
		s.closed = true
		consumerClosed := s.consumerClosed
		s.mu.Unlock()
		// The channel closed without an explicit error. If the producer
		// stopped because the request context was cancelled — and the
		// cancellation was not this consumer closing the stream itself —
		// surface it instead of pretending the stream completed.
		if !consumerClosed && s.ctx != nil && s.ctx.Err() != nil {
			return nil, s.ctx.Err()
		}
		return nil, io.EOF
	}

	if result.Err != nil {
		s.Close()
		return nil, result.Err
	}

	if result.Chunk == nil {
		return nil, nil
	}
	return result.Chunk, nil
}

func (s *chunkStream) Close() error {
	s.mu.Lock()
	s.consumerClosed = true
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	// Drain any remaining results to allow the producer goroutine to exit.
	for range s.ch {
	}
	return nil
}
