package shared

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunkStreamDeliversChunksThenEOF(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan StreamResult, 2)
	ch <- StreamResult{Chunk: &ai.StreamChunk{Text: "hello"}}
	ch <- StreamResult{Chunk: &ai.StreamChunk{Text: " world"}}
	close(ch)

	stream := NewChunkStream(ctx, cancel, ch)

	chunk, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "hello", chunk.Text)

	chunk, err = stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, " world", chunk.Text)

	_, err = stream.Recv()
	assert.Equal(t, io.EOF, err, "clean close with live context is a normal end of stream")
}

// A producer that stops because the request context was cancelled must
// not look like a clean end of stream: the partial text would be
// reported as a complete answer and persisted into chat history.
func TestChunkStreamSurfacesCancellationInsteadOfEOF(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan StreamResult, 1)
	ch <- StreamResult{Chunk: &ai.StreamChunk{Text: "partial"}}

	stream := NewChunkStream(ctx, cancel, ch)

	chunk, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "partial", chunk.Text)

	// Simulate user cancellation: producer observes ctx.Done() and
	// closes the channel without emitting an error.
	cancel()
	close(ch)

	_, err = stream.Recv()
	require.Error(t, err, "cancelled stream must not end with io.EOF")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestChunkStreamPropagatesEmittedErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	boom := errors.New("api exploded")
	ch := make(chan StreamResult, 1)
	ch <- StreamResult{Err: boom}
	close(ch)

	stream := NewChunkStream(ctx, cancel, ch)

	_, err := stream.Recv()
	assert.ErrorIs(t, err, boom)
}

// A consumer-initiated Close cancels the producer context by design;
// subsequent Recv calls must report EOF, not the self-inflicted
// cancellation.
// A panicking producer must surface a stream error, not kill the process.
func TestRecoverToStreamConvertsPanicToError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan StreamResult, 4)

	go func() {
		defer close(ch)
		defer RecoverToStream(ch)
		panic("nil map write in tool handler")
	}()

	stream := NewChunkStream(ctx, cancel, ch)
	_, err := stream.Recv()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil map write in tool handler")

	_, err = stream.Recv()
	assert.Equal(t, io.EOF, err, "stream must close cleanly after the panic error")
}

func TestChunkStreamCloseThenRecvIsEOF(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan StreamResult)
	stream := NewChunkStream(ctx, cancel, ch)

	go close(ch) // producer exits when it notices cancellation

	require.NoError(t, stream.Close())

	_, err := stream.Recv()
	assert.Equal(t, io.EOF, err)
}
