package cloudchat

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/logging"
)

func TestMaritaca_DefaultEndpoint(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, textAnswer("Olá!"))
	client := newTestClient(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "owner-key"})

	resp, err := client.GenerateContent(context.Background(), ai.Prompt{
		Instruction: "Você é um assistente.",
		Text:        "Diga olá.",
		ModelName:   "sabia-4",
	}, false)
	require.NoError(t, err)
	assert.Equal(t, "Olá!", resp)

	require.Len(t, mockHTTP.urls, 1)
	assert.Equal(t, "https://chat.maritaca.ai/api/chat/completions", mockHTTP.urls[0])
	assert.Equal(t, "Bearer owner-key", mockHTTP.headers[0].Get("Authorization"))
	assert.Equal(t, "sabia-4", mockHTTP.requests[0].Model)
	require.Len(t, mockHTTP.requests[0].Messages, 2)
	assert.Equal(t, "system", mockHTTP.requests[0].Messages[0].Role)
}

// A base URL override (the Mutiro LLM proxy) replaces the default; the key
// is whatever the environment carries, the agent key behind a proxy.
func TestMaritaca_BaseURLOverride(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, textAnswer("ok"))
	client := newTestClient(t, mockHTTP, map[string]string{
		"MARITACA_API_KEY":  "agent-key",
		"MARITACA_BASE_URL": "https://llm.example.com/mutiro/maritaca/",
	})

	_, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi", ModelName: "sabiazinho-4"}, false)
	require.NoError(t, err)
	assert.Equal(t, "https://llm.example.com/mutiro/maritaca/chat/completions", mockHTTP.urls[0])
	assert.Equal(t, "Bearer agent-key", mockHTTP.headers[0].Get("Authorization"))
}

func TestMaritaca_MissingKey(t *testing.T) {
	client := newTestClient(t, newMockHTTPClient(t), map[string]string{})

	_, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi", ModelName: "sabia-4"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARITACA_API_KEY")

	status := client.GetStatus()
	assert.False(t, status.Connected)
	assert.Equal(t, "maritaca", status.Backend)
	assert.True(t, strings.HasPrefix(status.Model, "sabia-4,"), status.Model)
}

// An image type the model does not take becomes a note instead of a part
// the endpoint would reject.
func TestMaritaca_UnsupportedImageTypeBecomesANote(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, textAnswer("ok"))
	client := newTestClient(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "k"})

	_, err := client.GenerateContent(context.Background(), ai.Prompt{
		Text:      "O que é isto?",
		ModelName: "sabia-4",
		Images:    []*ai.Image{{Type: "image/gif", Data: []byte{1, 2, 3}}},
	}, false)
	require.NoError(t, err)

	user := mockHTTP.requests[0].Messages[len(mockHTTP.requests[0].Messages)-1]
	require.Len(t, user.Content.Parts, 1)
	assert.Contains(t, user.Content.Parts[0].Text, "does not accept image/gif")
}

// Usage events carry the provider name and the cache split.
func TestMaritaca_PublishesCachedUsage(t *testing.T) {
	bus := events.NewEventBus()
	received := make(chan events.TokenCountEvent, 1)
	bus.Subscribe(events.TokenCountEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.TokenCountEvent); ok {
			received <- event
		}
	})
	mockHTTP := newMockHTTPClient(t, func(int, chatRequest) chatResponse {
		answer := textAnswer("ok")(0, chatRequest{})
		answer.Usage = &usage{PromptTokens: 4000, CompletionTokens: 5, TotalTokens: 4005, PromptTokensDetails: &promptTokensDetail{CachedTokens: 3968}}
		return answer
	})
	client := newTestClientWithBus(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "k"}, bus)

	_, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi", ModelName: "sabia-4"}, false)
	require.NoError(t, err)

	select {
	case event := <-received:
		assert.Equal(t, "maritaca", event.Provider)
		assert.Equal(t, "sabia-4", event.Model)
		assert.Equal(t, int32(32), event.InputTokens)
		assert.Equal(t, int32(3968), event.CachedTokens)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for token count event")
	}
}

// A prompt naming no model uses the configured one, else the provider default.
func TestMaritaca_DefaultModel(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, textAnswer("ok"), textAnswer("ok"))
	client := newTestClient(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "k"})
	_, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi"}, false)
	require.NoError(t, err)
	assert.Equal(t, "sabia-4", mockHTTP.requests[0].Model)

	configured := newTestClient(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "k", "GENIE_MODEL_NAME": "sabiazinho-4"})
	_, err = configured.GenerateContent(context.Background(), ai.Prompt{Text: "hi"}, false)
	require.NoError(t, err)
	assert.Equal(t, "sabiazinho-4", mockHTTP.requests[1].Model)
}

// max_tokens above the model's output limit is clamped: Maritaca rejects
// the request (422) instead of capping it, and Genie's default is higher
// than the Sabiá limit.
func TestMaritaca_MaxTokensClampedToModelLimit(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, textAnswer("ok"))
	client := newTestClient(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "k"})

	_, err := client.GenerateContent(context.Background(), ai.Prompt{
		Text:              "hi",
		ModelName:         "sabia-4",
		MaxTokens:         65535,
		ModelCapabilities: &ai.ModelCapabilities{OutputTokenLimit: 32000},
	}, false)
	require.NoError(t, err)
	require.NotNil(t, mockHTTP.requests[0].MaxTokens)
	assert.Equal(t, int32(32000), *mockHTTP.requests[0].MaxTokens)
}

// Callers that build prompts directly (decide.Model) send no capabilities;
// the limit then comes from the model registry.
func TestMaritaca_MaxTokensClampedWithoutCapabilities(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, textAnswer("ok"))
	client := newTestClient(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "k"})

	_, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi", ModelName: "sabia-4"}, false)
	require.NoError(t, err)
	require.NotNil(t, mockHTTP.requests[0].MaxTokens)
	assert.Equal(t, int32(32000), *mockHTTP.requests[0].MaxTokens)
}

// Local token counts are labeled with the model actually used, not the
// configuration manager's Gemini fallback.
func TestMaritaca_CountTokensNamesTheModel(t *testing.T) {
	bus := events.NewEventBus()
	received := make(chan events.TokenCountEvent, 1)
	bus.Subscribe(events.TokenCountEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.TokenCountEvent); ok {
			received <- event
		}
	})
	client := newTestClientWithBus(t, newMockHTTPClient(t), map[string]string{"MARITACA_API_KEY": "k"}, bus)

	count, err := client.CountTokens(context.Background(), ai.Prompt{Text: "olá"}, false)
	require.NoError(t, err)
	assert.Positive(t, count.InputTokens)
	select {
	case event := <-received:
		assert.Equal(t, "sabia-4", event.Model)
		assert.Equal(t, "maritaca", event.Provider)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for token count event")
	}
}

// --- helpers ---

func textAnswer(text string) func(int, chatRequest) chatResponse {
	return func(int, chatRequest) chatResponse {
		return chatResponse{
			Choices: []chatChoice{{
				Message: responseMessage{
					Role:    "assistant",
					Content: responseContent{Parts: []contentPart{{Type: "text", Text: text}}},
				},
				FinishReason: "stop",
			}},
		}
	}
}

func newTestClient(t *testing.T, mockHTTP *mockHTTPClient, values map[string]string) *Client {
	t.Helper()
	return newTestClientWithBus(t, mockHTTP, values, &events.NoOpEventBus{})
}

func newTestClientWithBus(t *testing.T, mockHTTP *mockHTTPClient, values map[string]string, bus events.EventBus) *Client {
	t.Helper()
	rawClient, err := NewClient(
		Maritaca,
		bus,
		WithConfigManager(&stubConfig{values: values}),
		WithHTTPClient(mockHTTP),
		WithLogger(logging.NewDisabledLogger()),
	)
	require.NoError(t, err)
	return rawClient.(*Client)
}

// stubConfig returns only the configured values, ignoring process env.
type stubConfig struct {
	values map[string]string
}

func (s *stubConfig) GetString(key string) (string, error) { return s.values[key], nil }

func (s *stubConfig) GetStringWithDefault(key, defaultValue string) string {
	if v, ok := s.values[key]; ok && v != "" {
		return v
	}
	return defaultValue
}

func (s *stubConfig) RequireString(key string) string { return s.values[key] }

func (s *stubConfig) GetInt(key string) (int, error) { return 0, nil }

func (s *stubConfig) GetIntWithDefault(key string, defaultValue int) int { return defaultValue }

func (s *stubConfig) GetBoolWithDefault(key string, defaultValue bool) bool { return defaultValue }

func (s *stubConfig) GetDurationWithDefault(key string, defaultValue time.Duration) time.Duration {
	return defaultValue
}

// GetModelConfig mirrors the real manager: an unset GENIE_MODEL_NAME
// still yields the global Gemini default, and max tokens default to 65535.
func (s *stubConfig) GetModelConfig() config.ModelConfig {
	return config.ModelConfig{ModelName: s.GetStringWithDefault("GENIE_MODEL_NAME", "gemini-3.7-flash"), MaxTokens: 65535}
}

type mockHTTPClient struct {
	t         *testing.T
	mu        sync.Mutex
	handlers  []func(call int, req chatRequest) chatResponse
	requests  []chatRequest
	rawBodies [][]byte
	headers   []http.Header
	urls      []string
	callCount int
}

func newMockHTTPClient(t *testing.T, handlers ...func(call int, req chatRequest) chatResponse) *mockHTTPClient {
	return &mockHTTPClient{t: t, handlers: handlers}
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	body, err := io.ReadAll(req.Body)
	require.NoError(m.t, err)
	_ = req.Body.Close()

	var parsed chatRequest
	require.NoError(m.t, json.Unmarshal(body, &parsed))
	m.requests = append(m.requests, parsed)
	m.rawBodies = append(m.rawBodies, body)
	m.headers = append(m.headers, req.Header.Clone())
	m.urls = append(m.urls, req.URL.String())

	if m.callCount >= len(m.handlers) {
		require.FailNow(m.t, "mock HTTP client received more calls than handlers configured")
	}
	response := m.handlers[m.callCount](m.callCount, parsed)
	m.callCount++

	payload, err := json.Marshal(response)
	require.NoError(m.t, err)
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(payload)), Header: make(http.Header)}, nil
}

// One client serves concurrent calls: the key is set once and only read
// afterwards (run with -race).
func TestConcurrentCallsShareTheKeySafely(t *testing.T) {
	const calls = 8
	handlers := make([]func(int, chatRequest) chatResponse, calls)
	for i := range handlers {
		handlers[i] = textAnswer("ok")
	}
	mockHTTP := newMockHTTPClient(t, handlers...)
	client := newTestClient(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "k"})

	var wg sync.WaitGroup
	for range calls {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi", ModelName: "sabia-4"}, false)
			assert.NoError(t, err)
		}()
		go func() {
			defer wg.Done()
			assert.True(t, client.GetStatus().Connected)
		}()
	}
	wg.Wait()
	for _, h := range mockHTTP.headers {
		assert.Equal(t, "Bearer k", h.Get("Authorization"))
	}
}

// A key exported after a failed call is picked up by the next one.
func TestKeyExportedAfterAFailedCall(t *testing.T) {
	values := map[string]string{}
	client := newTestClient(t, newMockHTTPClient(t, textAnswer("ok")), values)

	_, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi", ModelName: "sabia-4"}, false)
	require.ErrorIs(t, err, errMissingAPIKey)

	values["MARITACA_API_KEY"] = "late-key"
	_, err = client.GenerateContent(context.Background(), ai.Prompt{Text: "hi", ModelName: "sabia-4"}, false)
	require.NoError(t, err)
}

// A document a tool returns (viewDocument) reaches Maritaca as a file part
// after the tool result; Maritaca extracts its text.
func TestMaritaca_ToolResultDocumentIsAFilePart(t *testing.T) {
	for _, tc := range []struct{ name, mime string }{
		{"contrato.pdf", "application/pdf"},
		{"relatorio.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"planilha.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"dados.csv", "text/csv"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mockHTTP := newMockHTTPClient(t, toolCallAnswer("viewDocument"), func(_ int, req chatRequest) chatResponse {
				require.Len(t, req.Messages, 4) // user, assistant, tool, file
				msg := req.Messages[3]
				assert.Equal(t, "user", msg.Role)
				require.Len(t, msg.Content.Parts, 2)
				assert.Equal(t, "text", msg.Content.Parts[0].Type)
				file := msg.Content.Parts[1]
				assert.Equal(t, "file", file.Type)
				require.NotNil(t, file.File)
				assert.Equal(t, tc.name, file.File.Filename)
				assert.True(t, strings.HasPrefix(file.File.FileData, "data:"+tc.mime+";base64,"), file.File.FileData)
				return textAnswer("lido")(0, req)
			})
			client := newTestClient(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "k"})
			runToolWithBlob(t, client, ai.BlobContent{Name: tc.name, MIMEType: tc.mime, Data: []byte("conteudo")})
		})
	}
}

// Images reach Maritaca as image_url parts, which it reads by OCR, from a
// tool and in the current turn.
func TestMaritaca_ImagesAreImageParts(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, toolCallAnswer("viewDocument"), func(_ int, req chatRequest) chatResponse {
		require.Len(t, req.Messages, 4)
		img := req.Messages[3].Content.Parts[1]
		assert.Equal(t, "image_url", img.Type)
		require.NotNil(t, img.ImageURL)
		assert.True(t, strings.HasPrefix(img.ImageURL.URL, "data:image/png;base64,"))
		return textAnswer("ok")(0, req)
	})
	client := newTestClient(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "k"})
	runToolWithBlob(t, client, ai.BlobContent{Name: "print.png", MIMEType: "image/png", Data: []byte{1, 2, 3}})

	current := newMockHTTPClient(t, func(_ int, req chatRequest) chatResponse {
		parts := req.Messages[len(req.Messages)-1].Content.Parts
		require.Len(t, parts, 2)
		assert.Equal(t, "image_url", parts[1].Type)
		return textAnswer("ok")(0, req)
	})
	client = newTestClient(t, current, map[string]string{"MARITACA_API_KEY": "k"})
	_, err := client.GenerateContent(context.Background(), ai.Prompt{
		Text: "que bicho é esse?", ModelName: "sabia-4",
		Images: []*ai.Image{{Type: "image/png", Data: []byte{1, 2, 3}}},
	}, false)
	require.NoError(t, err)
}

// An attachment Maritaca does not take stays a text description.
func TestMaritaca_UnsupportedAttachmentStaysText(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, toolCallAnswer("viewDocument"), func(_ int, req chatRequest) chatResponse {
		require.Len(t, req.Messages, 3) // user, assistant, tool
		assert.Contains(t, req.Messages[2].Content.Parts[0].Text, "arquivo.zip")
		return textAnswer("ok")(0, req)
	})
	client := newTestClient(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "k"})
	runToolWithBlob(t, client, ai.BlobContent{Name: "arquivo.zip", MIMEType: "application/zip", Data: []byte{1}})
}

func toolCallAnswer(name string) func(int, chatRequest) chatResponse {
	return func(int, chatRequest) chatResponse {
		return chatResponse{Choices: []chatChoice{{
			Message: responseMessage{
				Role:    "assistant",
				Content: responseContent{Parts: []contentPart{{Type: "text", Text: ""}}},
				ToolCalls: []toolCall{{ID: "call_1", Type: "function",
					Function: toolCallFunction{Name: name, Arguments: json.RawMessage(`{}`)}}},
			},
			FinishReason: "tool_calls",
		}}}
	}
}

func runToolWithBlob(t *testing.T, client *Client, blob ai.BlobContent) {
	t.Helper()
	_, err := client.GenerateContent(context.Background(), ai.Prompt{
		Text: "abre o arquivo", ModelName: "sabia-4",
		Functions: []*ai.FunctionDeclaration{{Name: "viewDocument", Parameters: &ai.Schema{Type: ai.TypeObject}}},
		Handlers: map[string]ai.HandlerFunc{"viewDocument": func(context.Context, map[string]any) (ai.ToolOutput, error) {
			return ai.ToolOutput{Content: []ai.ToolContent{ai.TextContent{Text: "arquivo carregado"}, blob}}, nil
		}},
	}, false)
	require.NoError(t, err)
}
