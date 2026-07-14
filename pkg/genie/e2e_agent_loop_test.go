package genie_test

// True end-to-end agent-loop tests: a user message goes through the real
// DefaultPromptRunner into the real Ollama client (HTTP faked with an
// httptest server speaking the /api/chat wire format), through the shared
// tool loop (llm/shared.RunToolLoop), into a REAL tools.Registry executing
// a REAL file read, whose result is fed back to the "model" before the
// final answer is produced.
//
// Nothing between RunPrompt and the HTTP wire is mocked: the prompt is
// loaded through prompts.NewPromptLoader exactly the way production binds
// tools (including the event-publishing handler wrapper), and the readFile
// handler resolves paths against the session cwd taken from context.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/llm/ollama"
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/kcaldas/genie/pkg/prompts"
	"github.com/kcaldas/genie/pkg/toolctx"
	"github.com/kcaldas/genie/pkg/tools"
)

// e2ePromptYAML is loaded through the production prompt loader, which
// attaches the readFile declaration + handler from the real registry and
// wraps the handler to publish tool.starting / tool.executed events.
const e2ePromptYAML = `
name: e2e-agent-loop
model_name: llama3
max_tokens: 200
temperature: 0.1
instruction: You are a concise test assistant. Use tools when needed.
text: "{{.message}}"
required_tools:
  - readFile
`

const e2eSecret = "octopus-tentacle-9f27"

// --- Ollama /api/chat wire shapes (mirrors pkg/llm/ollama/types.go) ---

type wireChatRequest struct {
	Model    string            `json:"model"`
	Messages []wireChatMessage `json:"messages"`
	Tools    []wireToolDecl    `json:"tools"`
	Stream   bool              `json:"stream"`
	Options  map[string]any    `json:"options"`
}

type wireChatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCallID string         `json:"tool_call_id"`
	ToolCalls  []wireToolCall `json:"tool_calls"`
}

type wireToolDecl struct {
	Type     string `json:"type"`
	Function struct {
		Name string `json:"name"`
	} `json:"function"`
}

type wireToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}

// fakeOllama is an httptest handler that replays scripted /api/chat
// response bodies (one per request, in order) and records every request
// it received so tests can assert on the exact wire traffic.
type fakeOllama struct {
	t     *testing.T
	mu    sync.Mutex
	steps []string
	reqs  []wireChatRequest
}

func newFakeOllama(t *testing.T, steps ...string) *fakeOllama {
	return &fakeOllama{t: t, steps: steps}
}

func (f *fakeOllama) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handler runs off the test goroutine: use t.Errorf (never FailNow).
	if r.URL.Path != "/api/chat" || r.Method != http.MethodPost {
		f.t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		f.t.Errorf("reading request body: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var req wireChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		f.t.Errorf("decoding request body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	call := len(f.reqs)
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()

	if call >= len(f.steps) {
		f.t.Errorf("fake ollama received unscripted request #%d", call)
		http.Error(w, "no scripted response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, f.steps[call])
}

func (f *fakeOllama) requestCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reqs)
}

func (f *fakeOllama) request(i int) wireChatRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reqs[i]
}

// --- scripted model responses ---

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return string(data)
}

// readFileToolCallResponse scripts the model requesting a readFile call
// for secret.txt (relative path, resolved against the session cwd).
func readFileToolCallResponse(done bool) map[string]any {
	return map[string]any{
		"model": "llama3",
		"message": map[string]any{
			"role":    "assistant",
			"content": "",
			"tool_calls": []any{
				map[string]any{
					"id":   "call_1",
					"type": "function",
					"function": map[string]any{
						"name": "readFile",
						"arguments": map[string]any{
							"file_path":        "secret.txt",
							"_display_message": "reading the secret file",
						},
					},
				},
			},
		},
		"done": done,
	}
}

func assistantTextResponse(text string, done bool) map[string]any {
	return map[string]any{
		"model": "llama3",
		"message": map[string]any{
			"role":    "assistant",
			"content": text,
		},
		"done": done,
	}
}

// --- shared fixture ---

type e2eFixture struct {
	workDir string
	bus     events.EventBus
	runner  genie.PromptRunner
	prompt  *ai.Prompt
	fake    *fakeOllama

	mu       sync.Mutex
	executed []events.ToolExecutedEvent
}

// newE2EFixture builds the full production chain against the fake server:
// real registry -> real prompt loader (event wrapping included) -> real
// Ollama client -> real DefaultPromptRunner.
func newE2EFixture(t *testing.T, fake *fakeOllama) *e2eFixture {
	t.Helper()

	workDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(workDir, "secret.txt"),
		[]byte("the secret ingredient is "+e2eSecret),
		0o644,
	))

	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	bus := events.NewEventBus()

	fx := &e2eFixture{workDir: workDir, bus: bus, fake: fake}
	unsubscribe := bus.Subscribe("tool.executed", func(e interface{}) {
		if evt, ok := e.(events.ToolExecutedEvent); ok {
			fx.mu.Lock()
			fx.executed = append(fx.executed, evt)
			fx.mu.Unlock()
		}
	})
	t.Cleanup(unsubscribe)

	registry := tools.NewDefaultRegistry(bus, tools.NewTodoManager(), nil, nil)
	t.Cleanup(registry.Shutdown)

	loader := prompts.NewPromptLoader(bus, registry)
	prompt, err := loader.LoadPromptFromBytes([]byte(e2ePromptYAML))
	require.NoError(t, err)
	require.Empty(t, prompt.MissingTools, "readFile must resolve from the real registry")
	require.Contains(t, prompt.Handlers, "readFile")

	client, err := ollama.NewClient(
		bus,
		ollama.WithBaseURL(server.URL),
		ollama.WithLogger(logging.NewDisabledLogger()),
	)
	require.NoError(t, err)

	fx.runner = genie.NewDefaultPromptRunner(client, false)
	fx.prompt = &prompt
	return fx
}

// sessionContext mimics genie's applySessionContext for the values the
// readFile handler consumes: the session working directory (path
// resolution) and the execution id (tool event correlation).
func (fx *e2eFixture) sessionContext() context.Context {
	ctx := toolctx.WithWorkingDir(context.Background(), fx.workDir)
	return toolctx.WithExecutionID(ctx, "e2e-agent-loop")
}

func (fx *e2eFixture) toolExecutedEvents() []events.ToolExecutedEvent {
	fx.mu.Lock()
	defer fx.mu.Unlock()
	return append([]events.ToolExecutedEvent(nil), fx.executed...)
}

// assertToolLoopWireTraffic asserts on the two captured requests: the
// first declared the readFile tool and carried the rendered user message;
// the second fed the REAL file content back to the model as a tool result.
func (fx *e2eFixture) assertToolLoopWireTraffic(t *testing.T) {
	t.Helper()

	require.Equal(t, 2, fx.fake.requestCount())

	first := fx.fake.request(0)
	assert.Equal(t, "llama3", first.Model)
	require.Len(t, first.Messages, 2)
	assert.Equal(t, "system", first.Messages[0].Role)
	assert.Equal(t, "user", first.Messages[1].Role)
	assert.Equal(t, "What is in secret.txt?", first.Messages[1].Content,
		"prompt template must render the user message")

	toolNames := make([]string, 0, len(first.Tools))
	for _, tool := range first.Tools {
		toolNames = append(toolNames, tool.Function.Name)
	}
	assert.Contains(t, toolNames, "readFile",
		"registry-declared tool must be sent to the model")

	second := fx.fake.request(1)
	require.Len(t, second.Messages, 4,
		"second request must carry system, user, assistant tool-call and tool result")

	assistant := second.Messages[2]
	assert.Equal(t, "assistant", assistant.Role)
	require.Len(t, assistant.ToolCalls, 1)
	assert.Equal(t, "readFile", assistant.ToolCalls[0].Function.Name)

	toolMsg := second.Messages[3]
	assert.Equal(t, "tool", toolMsg.Role)
	assert.Equal(t, "call_1", toolMsg.ToolCallID)
	assert.Contains(t, toolMsg.Content, e2eSecret,
		"tool result fed back to the model must contain the real file content")
	assert.Contains(t, toolMsg.Content, `"success":true`)
}

// assertToolExecutedEvent asserts the loader's event wrapping was in the
// execution path: a successful tool.executed event for readFile carrying
// the real file content, with underscore params filtered out.
func (fx *e2eFixture) assertToolExecutedEvent(t *testing.T) {
	t.Helper()

	executed := fx.toolExecutedEvents()
	require.Len(t, executed, 1)
	evt := executed[0]
	assert.Equal(t, "readFile", evt.ToolName)
	assert.True(t, evt.Success)
	assert.Equal(t, "Executed", evt.Message)
	assert.Equal(t, "e2e-agent-loop", evt.ExecutionID)
	assert.Equal(t, "secret.txt", evt.Parameters["file_path"])
	assert.NotContains(t, evt.Parameters, "_display_message",
		"underscore-prefixed params must be filtered from events")

	results, _ := evt.Result["results"].(string)
	assert.Contains(t, results, e2eSecret,
		"event must carry the real file content read by the registry handler")
}

// TestEndToEndAgentLoop_BlockingReadFile drives the blocking path:
// RunPrompt -> ollama client -> shared RunToolLoop -> real readFile.
func TestEndToEndAgentLoop_BlockingReadFile(t *testing.T) {
	t.Parallel()

	finalAnswer := "The secret ingredient is " + e2eSecret + "."
	fake := newFakeOllama(t)
	fx := newE2EFixture(t, fake)
	fake.steps = []string{
		mustJSON(t, readFileToolCallResponse(true)),
		mustJSON(t, assistantTextResponse(finalAnswer, true)),
	}

	answer, err := fx.runner.RunPrompt(
		fx.sessionContext(),
		fx.prompt,
		map[string]string{"message": "What is in secret.txt?"},
		fx.bus,
	)
	require.NoError(t, err)
	assert.Equal(t, finalAnswer, answer)
	assert.Contains(t, answer, e2eSecret)

	fx.assertToolLoopWireTraffic(t)
	fx.assertToolExecutedEvent(t)

	// Both requests were blocking chat calls.
	assert.False(t, fx.fake.request(0).Stream)
	assert.False(t, fx.fake.request(1).Stream)
}

// TestEndToEndAgentLoop_StreamingReadFile drives the streaming path:
// RunPromptStream -> RunToolLoopStream, with the fake server emitting
// newline-delimited chunks; the final answer arrives in three separate
// chunks that the runner must reassemble.
func TestEndToEndAgentLoop_StreamingReadFile(t *testing.T) {
	t.Parallel()

	chunks := []string{"The secret ingredient is ", e2eSecret, "."}
	fake := newFakeOllama(t)
	fx := newE2EFixture(t, fake)
	fake.steps = []string{
		strings.Join([]string{
			mustJSON(t, readFileToolCallResponse(false)),
			mustJSON(t, assistantTextResponse("", true)),
		}, "\n"),
		strings.Join([]string{
			mustJSON(t, assistantTextResponse(chunks[0], false)),
			mustJSON(t, assistantTextResponse(chunks[1], false)),
			mustJSON(t, assistantTextResponse(chunks[2], true)),
		}, "\n"),
	}

	answer, err := fx.runner.RunPromptStream(
		fx.sessionContext(),
		fx.prompt,
		map[string]string{"message": "What is in secret.txt?"},
		fx.bus,
	)
	require.NoError(t, err)
	assert.Equal(t, strings.Join(chunks, ""), answer,
		"streamed chunks must be reassembled into the final answer")

	fx.assertToolLoopWireTraffic(t)
	fx.assertToolExecutedEvent(t)

	// Both requests were streaming chat calls.
	assert.True(t, fx.fake.request(0).Stream)
	assert.True(t, fx.fake.request(1).Stream)
}
