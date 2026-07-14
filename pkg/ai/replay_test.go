package ai

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReplayRoundTripFromCaptureFile is the core contract: a session recorded
// through CaptureMiddleware can be replayed from the capture file, serving the
// same responses (and errors) for the same calls.
func TestReplayRoundTripFromCaptureFile(t *testing.T) {
	ctx := context.Background()
	gen := &scriptedCaptureGen{script: []scriptedResult{
		{text: "answer one"},
		{text: "answer two"},
		{text: "summary"},
		{err: errors.New("provider exploded")},
	}}
	file := filepath.Join(t.TempDir(), "capture.json")
	mw := newCaptureForTest(t, gen, file)

	chat := Prompt{Name: "chat", Text: "hello {{.q}}"}
	summarize := Prompt{Name: "summarize"}
	attrs := []Attr{{Key: "lang", Value: "go"}}

	// Record a session.
	resp, err := mw.GenerateContent(ctx, chat, false, "q", "one")
	require.NoError(t, err)
	require.Equal(t, "answer one", resp)

	resp, err = mw.GenerateContent(ctx, chat, false, "q", "two")
	require.NoError(t, err)
	require.Equal(t, "answer two", resp)

	resp, err = mw.GenerateContentAttr(ctx, summarize, false, attrs)
	require.NoError(t, err)
	require.Equal(t, "summary", resp)

	_, err = mw.GenerateContent(ctx, Prompt{Name: "broken"}, false)
	require.Error(t, err)

	// Replay it from the file.
	replay, err := NewReplayGen(file)
	require.NoError(t, err)

	got, err := replay.GenerateContent(ctx, chat, false, "q", "one")
	require.NoError(t, err)
	assert.Equal(t, "answer one", got)

	got, err = replay.GenerateContent(ctx, chat, false, "q", "two")
	require.NoError(t, err)
	assert.Equal(t, "answer two", got)

	got, err = replay.GenerateContentAttr(ctx, summarize, false, attrs)
	require.NoError(t, err)
	assert.Equal(t, "summary", got)

	_, err = replay.GenerateContent(ctx, Prompt{Name: "broken"}, false)
	require.Error(t, err)
	assert.EqualError(t, err, "provider exploded", "recorded errors must replay with the captured message")
}

func TestReplayMatchesByArgsNotJustOrder(t *testing.T) {
	interactions := []Interaction{
		{ID: "1", Prompt: CapturedPrompt{Name: "chat"}, Args: []string{"q", "one"}, Response: "answer one"},
		{ID: "2", Prompt: CapturedPrompt{Name: "chat"}, Args: []string{"q", "two"}, Response: "answer two"},
	}
	file := filepath.Join(t.TempDir(), "capture.json")
	require.NoError(t, SaveInteractionsToFile(interactions, file))

	replay, err := NewReplayGen(file)
	require.NoError(t, err)
	ctx := context.Background()

	// Ask for the second recording first: args, not order, select it.
	got, err := replay.GenerateContent(ctx, Prompt{Name: "chat"}, false, "q", "two")
	require.NoError(t, err)
	assert.Equal(t, "answer two", got)

	got, err = replay.GenerateContent(ctx, Prompt{Name: "chat"}, false, "q", "one")
	require.NoError(t, err)
	assert.Equal(t, "answer one", got)
}

func TestReplayServesIdenticalCallsSequentially(t *testing.T) {
	interactions := []Interaction{
		{ID: "1", Prompt: CapturedPrompt{Name: "chat"}, Args: []string{"q", "same"}, Response: "first"},
		{ID: "2", Prompt: CapturedPrompt{Name: "chat"}, Args: []string{"q", "same"}, Response: "second"},
	}
	replay := NewReplayGenFromInteractions(interactions)
	ctx := context.Background()

	got, err := replay.GenerateContent(ctx, Prompt{Name: "chat"}, false, "q", "same")
	require.NoError(t, err)
	assert.Equal(t, "first", got)

	got, err = replay.GenerateContent(ctx, Prompt{Name: "chat"}, false, "q", "same")
	require.NoError(t, err)
	assert.Equal(t, "second", got)

	// A third identical call has nothing left to replay.
	_, err = replay.GenerateContent(ctx, Prompt{Name: "chat"}, false, "q", "same")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"chat"`)
	assert.Contains(t, err.Error(), "already replayed")
}

func TestReplayFallsBackToPromptNameMatch(t *testing.T) {
	interactions := []Interaction{
		{ID: "1", Prompt: CapturedPrompt{Name: "chat"}, Args: []string{"session", "recorded-id"}, Response: "fallback answer"},
	}
	replay := NewReplayGenFromInteractions(interactions)

	// Args differ (e.g. a fresh session id), but the prompt name matches.
	got, err := replay.GenerateContent(context.Background(), Prompt{Name: "chat"}, false, "session", "new-id")
	require.NoError(t, err)
	assert.Equal(t, "fallback answer", got)
}

func TestReplayUnmatchedPromptReturnsDescriptiveError(t *testing.T) {
	interactions := []Interaction{
		{ID: "1", Prompt: CapturedPrompt{Name: "chat"}, Response: "hi"},
	}
	replay := NewReplayGenFromInteractions(interactions)

	_, err := replay.GenerateContent(context.Background(), Prompt{Name: "unknown"}, false, "a", "b")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"unknown"`, "the error must name the unmatched prompt")
	assert.Contains(t, err.Error(), `"chat"`, "the error must list the interactions still available")
	assert.Contains(t, err.Error(), "[a b]", "the error must include the call args")
}

func TestReplayStreamsRecordedResponse(t *testing.T) {
	interactions := []Interaction{
		{ID: "1", Prompt: CapturedPrompt{Name: "chat"}, Args: []string{"q", "hi"}, Response: "streamed answer"},
		{ID: "2", Prompt: CapturedPrompt{Name: "summarize"}, Attrs: []CapturedAttr{{Key: "lang", Value: "go"}}, Response: "attr answer"},
		{ID: "3", Prompt: CapturedPrompt{Name: "broken"}, Error: &CapturedError{Message: "boom", Type: "*errors.errorString"}},
	}
	replay := NewReplayGenFromInteractions(interactions)
	ctx := context.Background()

	stream, err := replay.GenerateContentStream(ctx, Prompt{Name: "chat"}, false, "q", "hi")
	require.NoError(t, err)
	chunks, err := drainStream(stream)
	require.NoError(t, err)
	assert.Equal(t, "streamed answer", streamText(chunks))

	stream, err = replay.GenerateContentAttrStream(ctx, Prompt{Name: "summarize"}, false, []Attr{{Key: "lang", Value: "go"}})
	require.NoError(t, err)
	chunks, err = drainStream(stream)
	require.NoError(t, err)
	assert.Equal(t, "attr answer", streamText(chunks))

	// A recorded error surfaces when the stream is requested.
	stream, err = replay.GenerateContentStream(ctx, Prompt{Name: "broken"}, false)
	assert.Nil(t, stream)
	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestNewReplayGenErrors(t *testing.T) {
	_, err := NewReplayGen(filepath.Join(t.TempDir(), "missing.json"))
	require.Error(t, err, "a missing capture file must be reported")

	badFile := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(badFile, []byte("not json"), 0o644))
	_, err = NewReplayGen(badFile)
	require.Error(t, err, "an unparsable capture file must be reported")
}

func TestReplayStatusAndTokenCounts(t *testing.T) {
	file := filepath.Join(t.TempDir(), "capture.json")
	require.NoError(t, SaveInteractionsToFile([]Interaction{}, file))

	replay, err := NewReplayGen(file)
	require.NoError(t, err)

	status := replay.GetStatus()
	require.NotNil(t, status)
	assert.True(t, status.Connected)
	assert.Equal(t, "replay", status.Backend)
	assert.Contains(t, status.Message, file)

	ctx := context.Background()
	tc, err := replay.CountTokens(ctx, Prompt{Name: "chat"}, false)
	require.NoError(t, err)
	assert.Equal(t, int32(0), tc.TotalTokens)

	tc, err = replay.CountTokensAttr(ctx, Prompt{Name: "chat"}, false, nil)
	require.NoError(t, err)
	assert.Equal(t, int32(0), tc.TotalTokens)
}
