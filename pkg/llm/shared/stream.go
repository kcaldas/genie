package shared

import (
	"context"
	"io"
	"sync"

	"github.com/kcaldas/genie/pkg/ai"
)

// StreamResult represents a single item emitted by a streaming provider.
// Exactly one of Chunk or Err should be set.
type StreamResult struct {
	Chunk *ai.StreamChunk
	Err   error
}

// NewChunkStream wraps a channel of StreamResult values and exposes it as an ai.Stream.
// The provided cancel function is called when the stream is closed to allow callers
// to stop upstream work early.
func NewChunkStream(cancel context.CancelFunc, ch <-chan StreamResult) ai.Stream {
	return &chunkStream{
		cancel: cancel,
		ch:     ch,
	}
}

type chunkStream struct {
	cancel context.CancelFunc

	mu     sync.Mutex
	ch     <-chan StreamResult
	closed bool
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
		s.mu.Unlock()
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
