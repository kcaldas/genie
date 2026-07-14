package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptedResult is one queued outcome for scriptedCaptureGen.
type scriptedResult struct {
	text    string
	err     error          // returned directly (non-stream) or as a setup error (stream)
	chunks  []*StreamChunk // stream calls only; defaults to a single chunk containing text
	recvErr error          // stream calls only; error Recv returns after chunks (default io.EOF)
}

// genCall records the arguments a scriptedCaptureGen method was invoked with.
type genCall struct {
	method string
	prompt Prompt
	args   []string
	attrs  []Attr
	debug  bool
}

// scriptedCaptureGen serves queued results in order and records every call.
// It follows the scriptedGen pattern from retry_test.go but also scripts
// responses and streams so capture behavior can be asserted end to end.
type scriptedCaptureGen struct {
	Gen    // panics if an unstubbed method is called
	script []scriptedResult
	calls  []genCall
	status *Status
}

func (s *scriptedCaptureGen) next() scriptedResult {
	if len(s.script) == 0 {
		return scriptedResult{text: "default"}
	}
	r := s.script[0]
	s.script = s.script[1:]
	return r
}

func (s *scriptedCaptureGen) GenerateContent(ctx context.Context, p Prompt, debug bool, args ...string) (string, error) {
	s.calls = append(s.calls, genCall{method: "GenerateContent", prompt: p, args: args, debug: debug})
	r := s.next()
	return r.text, r.err
}

func (s *scriptedCaptureGen) GenerateContentAttr(ctx context.Context, p Prompt, debug bool, attrs []Attr) (string, error) {
	s.calls = append(s.calls, genCall{method: "GenerateContentAttr", prompt: p, attrs: attrs, debug: debug})
	r := s.next()
	return r.text, r.err
}

func (s *scriptedCaptureGen) GenerateContentStream(ctx context.Context, p Prompt, debug bool, args ...string) (Stream, error) {
	s.calls = append(s.calls, genCall{method: "GenerateContentStream", prompt: p, args: args, debug: debug})
	r := s.next()
	if r.err != nil {
		return nil, r.err
	}
	return newScriptedStream(r), nil
}

func (s *scriptedCaptureGen) GenerateContentAttrStream(ctx context.Context, p Prompt, debug bool, attrs []Attr) (Stream, error) {
	s.calls = append(s.calls, genCall{method: "GenerateContentAttrStream", prompt: p, attrs: attrs, debug: debug})
	r := s.next()
	if r.err != nil {
		return nil, r.err
	}
	return newScriptedStream(r), nil
}

func (s *scriptedCaptureGen) CountTokens(ctx context.Context, p Prompt, debug bool, args ...string) (*TokenCount, error) {
	s.calls = append(s.calls, genCall{method: "CountTokens", prompt: p, args: args, debug: debug})
	return &TokenCount{TotalTokens: 42}, nil
}

func (s *scriptedCaptureGen) CountTokensAttr(ctx context.Context, p Prompt, debug bool, attrs []Attr) (*TokenCount, error) {
	s.calls = append(s.calls, genCall{method: "CountTokensAttr", prompt: p, attrs: attrs, debug: debug})
	return &TokenCount{TotalTokens: 43}, nil
}

func (s *scriptedCaptureGen) GetStatus() *Status {
	return s.status
}

// scriptedStream yields the scripted chunks, then recvErr (io.EOF by default).
type scriptedStream struct {
	chunks  []*StreamChunk
	recvErr error
	idx     int
}

func newScriptedStream(r scriptedResult) *scriptedStream {
	chunks := r.chunks
	if chunks == nil {
		chunks = []*StreamChunk{{Text: r.text}}
	}
	recvErr := r.recvErr
	if recvErr == nil {
		recvErr = io.EOF
	}
	return &scriptedStream{chunks: chunks, recvErr: recvErr}
}

func (s *scriptedStream) Recv() (*StreamChunk, error) {
	if s.idx < len(s.chunks) {
		c := s.chunks[s.idx]
		s.idx++
		return c, nil
	}
	return nil, s.recvErr
}

func (s *scriptedStream) Close() error { return nil }

func newCaptureForTest(t *testing.T, gen Gen, outputFile string) *CaptureMiddleware {
	t.Helper()
	return NewCaptureMiddleware(gen, CaptureConfig{
		Enabled:      true,
		ProviderName: "test-provider",
		OutputFile:   outputFile,
	}).(*CaptureMiddleware)
}

// drainStream consumes a stream until EOF (or error) and returns the chunks seen.
func drainStream(s Stream) ([]*StreamChunk, error) {
	var chunks []*StreamChunk
	for {
		chunk, err := s.Recv()
		if chunk != nil {
			chunks = append(chunks, chunk)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return chunks, nil
			}
			return chunks, err
		}
	}
}

func streamText(chunks []*StreamChunk) string {
	var b strings.Builder
	for _, c := range chunks {
		b.WriteString(c.Text)
	}
	return b.String()
}

func TestCaptureRecordsGenerateContent(t *testing.T) {
	gen := &scriptedCaptureGen{script: []scriptedResult{{text: "hi there"}}}
	mw := newCaptureForTest(t, gen, "")

	prompt := Prompt{
		Name:        "chat",
		Text:        "hello {{.name}}",
		Instruction: "be nice",
		Functions: []*FunctionDeclaration{
			{Name: "readFile", Description: "reads a file"},
			{Name: "bash", Description: "runs a command"},
		},
	}

	resp, err := mw.GenerateContent(context.Background(), prompt, true, "name", "world")
	require.NoError(t, err)
	assert.Equal(t, "hi there", resp)

	// The call was forwarded unchanged to the underlying Gen.
	require.Len(t, gen.calls, 1)
	assert.Equal(t, "GenerateContent", gen.calls[0].method)
	assert.Equal(t, "chat", gen.calls[0].prompt.Name)
	assert.Equal(t, []string{"name", "world"}, gen.calls[0].args)
	assert.True(t, gen.calls[0].debug)

	// The interaction was recorded with prompt, args, response, and metadata.
	last := mw.GetLastInteraction()
	require.NotNil(t, last)
	assert.NotEmpty(t, last.ID)
	assert.Equal(t, "chat", last.Prompt.Name)
	assert.Equal(t, "hello {{.name}}", last.Prompt.Text)
	assert.Equal(t, "be nice", last.Prompt.Instruction)
	assert.Equal(t, []string{"name", "world"}, last.Args)
	assert.Equal(t, "hi there", last.Response)
	assert.Equal(t, "test-provider", last.LLMProvider)
	assert.Equal(t, []string{"readFile", "bash"}, last.Tools)
	assert.True(t, last.Debug)
	assert.Nil(t, last.Error)
	assert.GreaterOrEqual(t, last.Duration, time.Duration(0))
}

func TestCaptureRecordsAndPropagatesErrorUnchanged(t *testing.T) {
	boom := errors.New("provider exploded")
	gen := &scriptedCaptureGen{script: []scriptedResult{{err: boom}}}
	mw := newCaptureForTest(t, gen, "")

	resp, err := mw.GenerateContent(context.Background(), Prompt{Name: "chat"}, false)
	assert.Empty(t, resp)
	require.Error(t, err)
	assert.Same(t, boom, err, "the underlying error must be propagated unchanged")

	last := mw.GetLastInteraction()
	require.NotNil(t, last)
	require.NotNil(t, last.Error)
	assert.Equal(t, "provider exploded", last.Error.Message)
	assert.Equal(t, "*errors.errorString", last.Error.Type)
	assert.Empty(t, last.Response)
}

func TestCaptureDisabledIsPurePassThrough(t *testing.T) {
	gen := &scriptedCaptureGen{script: []scriptedResult{
		{text: "raw"},
		{text: "raw-attr"},
		{text: "raw-stream"},
	}}
	outputFile := filepath.Join(t.TempDir(), "capture.json")
	mw := NewCaptureMiddleware(gen, CaptureConfig{
		Enabled:      false,
		ProviderName: "test-provider",
		OutputFile:   outputFile,
	}).(*CaptureMiddleware)
	ctx := context.Background()

	resp, err := mw.GenerateContent(ctx, Prompt{Name: "chat"}, false, "k", "v")
	require.NoError(t, err)
	assert.Equal(t, "raw", resp)

	resp, err = mw.GenerateContentAttr(ctx, Prompt{Name: "chat"}, false, []Attr{{Key: "k", Value: "v"}})
	require.NoError(t, err)
	assert.Equal(t, "raw-attr", resp)

	stream, err := mw.GenerateContentStream(ctx, Prompt{Name: "chat"}, false)
	require.NoError(t, err)
	chunks, err := drainStream(stream)
	require.NoError(t, err)
	assert.Equal(t, "raw-stream", streamText(chunks))

	assert.Len(t, gen.calls, 3, "all calls must reach the underlying Gen")
	assert.Empty(t, mw.GetCapturedInteractions())
	assert.Nil(t, mw.GetLastInteraction())

	_, statErr := os.Stat(outputFile)
	assert.True(t, os.IsNotExist(statErr), "disabled capture must not write a file")
}

func TestCaptureWritesOutputFileAfterEachInteraction(t *testing.T) {
	gen := &scriptedCaptureGen{script: []scriptedResult{{text: "first"}, {text: "second"}}}
	outputFile := filepath.Join(t.TempDir(), "capture.json")
	mw := newCaptureForTest(t, gen, outputFile)
	ctx := context.Background()

	_, err := mw.GenerateContent(ctx, Prompt{Name: "chat", Text: "one"}, false, "n", "1")
	require.NoError(t, err)

	// The file is (re)written as soon as each interaction completes.
	data, err := os.ReadFile(outputFile)
	require.NoError(t, err, "capture file must exist after the first interaction")

	var interactions []Interaction
	require.NoError(t, json.Unmarshal(data, &interactions), "capture file must be valid JSON")
	require.Len(t, interactions, 1)
	assert.Equal(t, "chat", interactions[0].Prompt.Name)
	assert.Equal(t, "one", interactions[0].Prompt.Text)
	assert.Equal(t, []string{"n", "1"}, interactions[0].Args)
	assert.Equal(t, "first", interactions[0].Response)
	assert.Equal(t, "test-provider", interactions[0].LLMProvider)

	_, err = mw.GenerateContent(ctx, Prompt{Name: "chat", Text: "two"}, false, "n", "2")
	require.NoError(t, err)

	data, err = os.ReadFile(outputFile)
	require.NoError(t, err)
	interactions = nil
	require.NoError(t, json.Unmarshal(data, &interactions))
	require.Len(t, interactions, 2)
	assert.Equal(t, "second", interactions[1].Response)
}

func TestCaptureRecordsAttrs(t *testing.T) {
	gen := &scriptedCaptureGen{script: []scriptedResult{{text: "attr response"}}}
	mw := newCaptureForTest(t, gen, "")

	attrs := []Attr{{Key: "task", Value: "summarize"}, {Key: "lang", Value: "go"}}
	resp, err := mw.GenerateContentAttr(context.Background(), Prompt{Name: "summarize"}, false, attrs)
	require.NoError(t, err)
	assert.Equal(t, "attr response", resp)

	require.Len(t, gen.calls, 1)
	assert.Equal(t, "GenerateContentAttr", gen.calls[0].method)
	assert.Equal(t, attrs, gen.calls[0].attrs)

	last := mw.GetLastInteraction()
	require.NotNil(t, last)
	assert.Equal(t, []CapturedAttr{
		{Key: "task", Value: "summarize"},
		{Key: "lang", Value: "go"},
	}, last.Attrs)
	assert.Equal(t, "attr response", last.Response)
}

func TestCaptureStreamRecordsStreamedText(t *testing.T) {
	gen := &scriptedCaptureGen{script: []scriptedResult{
		{chunks: []*StreamChunk{{Text: "Hel"}, {Text: "lo"}, {Text: " world"}}},
	}}
	mw := newCaptureForTest(t, gen, "")

	stream, err := mw.GenerateContentStream(context.Background(), Prompt{Name: "chat"}, false, "q", "hi")
	require.NoError(t, err)

	chunks, err := drainStream(stream)
	require.NoError(t, err)
	require.Len(t, chunks, 3, "chunks must be forwarded unchanged")
	assert.Equal(t, "Hello world", streamText(chunks))

	last := mw.GetLastInteraction()
	require.NotNil(t, last)
	assert.Equal(t, "Hello world", last.Response, "the concatenated stream text must be recorded")
	assert.Equal(t, []string{"q", "hi"}, last.Args)
	assert.Nil(t, last.Error)
}

func TestCaptureStreamMidstreamErrorRecordsPartialTextAndError(t *testing.T) {
	recvErr := errors.New("stream broke")
	gen := &scriptedCaptureGen{script: []scriptedResult{
		{chunks: []*StreamChunk{{Text: "par"}, {Text: "tial"}}, recvErr: recvErr},
	}}
	mw := newCaptureForTest(t, gen, "")

	stream, err := mw.GenerateContentStream(context.Background(), Prompt{Name: "chat"}, false)
	require.NoError(t, err)

	chunks, err := drainStream(stream)
	require.Error(t, err)
	assert.Same(t, recvErr, err, "the stream error must be propagated unchanged")
	assert.Equal(t, "partial", streamText(chunks))

	last := mw.GetLastInteraction()
	require.NotNil(t, last)
	require.NotNil(t, last.Error)
	assert.Equal(t, "stream broke", last.Error.Message)
	assert.Equal(t, "partial", last.Response, "text received before the failure must be recorded")
}

func TestCaptureStreamSetupErrorRecordsAndPropagates(t *testing.T) {
	setupErr := errors.New("no connection")
	gen := &scriptedCaptureGen{script: []scriptedResult{{err: setupErr}}}
	mw := newCaptureForTest(t, gen, "")

	stream, err := mw.GenerateContentStream(context.Background(), Prompt{Name: "chat"}, false)
	assert.Nil(t, stream)
	require.Error(t, err)
	assert.Same(t, setupErr, err)

	last := mw.GetLastInteraction()
	require.NotNil(t, last)
	require.NotNil(t, last.Error)
	assert.Equal(t, "no connection", last.Error.Message)
	assert.Empty(t, last.Response)
}

func TestCaptureStreamCloseRecordsPartialText(t *testing.T) {
	gen := &scriptedCaptureGen{script: []scriptedResult{
		{chunks: []*StreamChunk{{Text: "Hel"}, {Text: "lo"}}},
	}}
	mw := newCaptureForTest(t, gen, "")

	stream, err := mw.GenerateContentStream(context.Background(), Prompt{Name: "chat"}, false)
	require.NoError(t, err)

	chunk, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "Hel", chunk.Text)

	require.NoError(t, stream.Close())

	last := mw.GetLastInteraction()
	require.NotNil(t, last)
	assert.Equal(t, "Hel", last.Response, "closing early must record what was received so far")
	assert.Nil(t, last.Error)

	// Close is safe to call again and must not record a second interaction.
	require.NoError(t, stream.Close())
	assert.Len(t, mw.GetCapturedInteractions(), 1)
}

func TestCaptureAttrStreamRecordsAttrsAndText(t *testing.T) {
	gen := &scriptedCaptureGen{script: []scriptedResult{
		{chunks: []*StreamChunk{{Text: "summar"}, {Text: "ized"}}},
	}}
	mw := newCaptureForTest(t, gen, "")

	attrs := []Attr{{Key: "lang", Value: "go"}}
	stream, err := mw.GenerateContentAttrStream(context.Background(), Prompt{Name: "summarize"}, false, attrs)
	require.NoError(t, err)

	chunks, err := drainStream(stream)
	require.NoError(t, err)
	assert.Equal(t, "summarized", streamText(chunks))

	last := mw.GetLastInteraction()
	require.NotNil(t, last)
	assert.Equal(t, []CapturedAttr{{Key: "lang", Value: "go"}}, last.Attrs)
	assert.Equal(t, "summarized", last.Response)
}

func TestCaptureAttrStreamSetupErrorRecordsAndPropagates(t *testing.T) {
	setupErr := errors.New("attr stream refused")
	gen := &scriptedCaptureGen{script: []scriptedResult{{err: setupErr}}}
	mw := newCaptureForTest(t, gen, "")

	stream, err := mw.GenerateContentAttrStream(context.Background(), Prompt{Name: "summarize"}, false, nil)
	assert.Nil(t, stream)
	require.Error(t, err)
	assert.Same(t, setupErr, err)

	last := mw.GetLastInteraction()
	require.NotNil(t, last)
	require.NotNil(t, last.Error)
	assert.Equal(t, "attr stream refused", last.Error.Message)
}

func TestCaptureDelegatesTokenCountingAndStatus(t *testing.T) {
	gen := &scriptedCaptureGen{status: &Status{Connected: true, Backend: "scripted"}}
	mw := newCaptureForTest(t, gen, "")
	ctx := context.Background()

	tc, err := mw.CountTokens(ctx, Prompt{Name: "chat"}, false)
	require.NoError(t, err)
	assert.Equal(t, int32(42), tc.TotalTokens)

	tc, err = mw.CountTokensAttr(ctx, Prompt{Name: "chat"}, false, nil)
	require.NoError(t, err)
	assert.Equal(t, int32(43), tc.TotalTokens)

	status := mw.GetStatus()
	require.NotNil(t, status)
	assert.Equal(t, "scripted", status.Backend)

	assert.Empty(t, mw.GetCapturedInteractions(), "token counting must not be captured")
}

func TestGetCaptureConfigFromEnv(t *testing.T) {
	t.Setenv("GENIE_CAPTURE_LLM", "")
	t.Setenv("GENIE_DEBUG", "")
	t.Setenv("GENIE_CAPTURE_FILE", "")

	cfg := GetCaptureConfigFromEnv("claude")
	assert.False(t, cfg.Enabled)
	assert.False(t, cfg.DebugMode)
	assert.Empty(t, cfg.OutputFile)
	assert.Equal(t, "claude", cfg.ProviderName)

	t.Setenv("GENIE_CAPTURE_LLM", "true")
	t.Setenv("GENIE_CAPTURE_FILE", "out.json")
	cfg = GetCaptureConfigFromEnv("claude")
	assert.True(t, cfg.Enabled)
	assert.False(t, cfg.DebugMode)
	assert.Equal(t, "out.json", cfg.OutputFile)

	t.Setenv("GENIE_CAPTURE_FILE", "")
	cfg = GetCaptureConfigFromEnv("claude")
	assert.True(t, cfg.Enabled)
	assert.True(t, strings.HasPrefix(cfg.OutputFile, "genie-capture-claude-"), "default output file must include the provider name, got %q", cfg.OutputFile)
	assert.True(t, strings.HasSuffix(cfg.OutputFile, ".json"))

	t.Setenv("GENIE_CAPTURE_LLM", "")
	t.Setenv("GENIE_DEBUG", "true")
	cfg = GetCaptureConfigFromEnv("claude")
	assert.True(t, cfg.Enabled, "GENIE_DEBUG=true must also enable capture")
	assert.True(t, cfg.DebugMode)
}

func TestInteractionCaptureStoreRoundTrip(t *testing.T) {
	capture := NewInteractionCapture()

	first := capture.StartInteraction(Prompt{Name: "chat", Text: "hi"}, []string{"a", "b"})
	capture.CompleteInteraction(first, "resp", nil, 5*time.Millisecond)

	second := capture.StartInteraction(Prompt{Name: "chat"}, nil)
	capture.CompleteInteraction(second, "", errors.New("bad"), time.Millisecond)

	file := filepath.Join(t.TempDir(), "interactions.json")
	require.NoError(t, capture.SaveToFile(file))

	loaded := NewInteractionCapture()
	require.NoError(t, loaded.LoadFromFile(file))

	got := loaded.GetInteractions()
	require.Len(t, got, 2)
	assert.Equal(t, first.ID, got[0].ID)
	assert.Equal(t, []string{"a", "b"}, got[0].Args)
	assert.Equal(t, "resp", got[0].Response)
	assert.Equal(t, 5*time.Millisecond, got[0].Duration)
	require.NotNil(t, got[1].Error)
	assert.Equal(t, "bad", got[1].Error.Message)

	byID := loaded.GetInteractionByID(first.ID)
	require.NotNil(t, byID)
	assert.Equal(t, "resp", byID.Response)
	assert.Nil(t, loaded.GetInteractionByID("missing"))

	assert.Contains(t, loaded.GetSummary(), "Captured 2 interactions")

	loaded.Clear()
	assert.Nil(t, loaded.GetLastInteraction())
	assert.Equal(t, "No interactions captured", loaded.GetSummary())
}
