package genie

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cancellingStreamGen simulates a provider whose producer goroutine
// stops mid-stream because the request context was cancelled.
type cancellingStreamGen struct {
	ai.Gen
	chunks []string
}

func (g *cancellingStreamGen) GenerateContentAttrStream(ctx context.Context, p ai.Prompt, debug bool, attrs []ai.Attr) (ai.Stream, error) {
	streamCtx, cancel := context.WithCancel(ctx)
	ch := make(chan llmshared.StreamResult, len(g.chunks))
	for _, text := range g.chunks {
		ch <- llmshared.StreamResult{Chunk: &ai.StreamChunk{Text: text}}
	}
	// Producer observes cancellation and exits without emitting an error.
	cancel()
	close(ch)
	return llmshared.NewChunkStream(streamCtx, cancel, ch), nil
}

// A turn cancelled mid-stream must surface as an error: reporting the
// accumulated partial text as a successful response would display a
// truncated answer as final and persist it into chat history.
func TestRunPromptStreamDoesNotReportCancelledPartialAsSuccess(t *testing.T) {
	runner := NewDefaultPromptRunner(&cancellingStreamGen{chunks: []string{"partial ", "answer"}}, false)

	_, err := runner.RunPromptStream(context.Background(), &ai.Prompt{}, map[string]string{}, events.NewEventBus())
	require.Error(t, err, "a cancelled stream must not be treated as a clean completion")
	assert.ErrorIs(t, err, context.Canceled)
}
