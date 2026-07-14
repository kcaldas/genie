package genai

import (
	"context"
	"io"
	"iter"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

func TestClientGenerateContentWithPromptImages(t *testing.T) {
	client := &Client{
		Config:   config.NewConfigManager(),
		EventBus: &events.NoOpEventBus{},
	}

	var capturedContents []*genai.Content
	client.callGenerateContentFn = func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig, handlers map[string]ai.HandlerFunc) (*genai.GenerateContentResponse, error) {
		capturedContents = append(capturedContents, contents...)
		return &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: genai.NewContentFromParts(
						[]*genai.Part{genai.NewPartFromText("processed")},
						genai.RoleModel,
					),
				},
			},
		}, nil
	}

	prompt := ai.Prompt{
		Text:      "What is in this image?",
		ModelName: "gemini-1.5-flash",
		Images: []*ai.Image{
			{
				Type:     "image/webp",
				Filename: "test.webp",
				Data:     []byte{0x0A, 0x0B, 0x0C},
			},
		},
	}

	response, err := client.generateContentWithPrompt(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.Equal(t, "processed", response)

	require.NotEmpty(t, capturedContents)
	userContent := capturedContents[0]
	require.NotNil(t, userContent)
	require.Len(t, userContent.Parts, 2)
	assert.Equal(t, "What is in this image?", userContent.Parts[0].Text)
	require.NotNil(t, userContent.Parts[1].InlineData)
	assert.Equal(t, []byte{0x0A, 0x0B, 0x0C}, userContent.Parts[1].InlineData.Data)
	assert.Equal(t, "image/webp", userContent.Parts[1].InlineData.MIMEType)
}

func TestClientJoinContentPartsWithFunctionResponse(t *testing.T) {
	client := &Client{
		Config:   config.NewConfigManager(),
		EventBus: &events.NoOpEventBus{},
	}

	content := genai.NewContentFromParts([]*genai.Part{
		genai.NewPartFromFunctionResponse("todo_update", map[string]any{"status": "complete"}),
	}, genai.RoleModel)

	result := client.joinContentParts(content)
	assert.Contains(t, result, "todo_update")
	assert.Contains(t, result, "complete")
}

func TestClientGenerateContentFunctionCallFlows(t *testing.T) {
	testCases := []struct {
		name            string
		responses       []*genai.GenerateContentResponse
		handlers        map[string]ai.HandlerFunc
		want            string
		wantErrMsg      string
		expectToolUsage bool
	}{
		{
			name: "direct response",
			responses: []*genai.GenerateContentResponse{
				{
					Candidates: []*genai.Candidate{
						{
							Content: genai.NewContentFromParts([]*genai.Part{
								genai.NewPartFromText("Hello there!"),
							}, genai.RoleModel),
						},
					},
				},
			},
			want:            "Hello there!",
			expectToolUsage: false,
		},
		{
			name: "single tool call",
			responses: []*genai.GenerateContentResponse{
				{
					Candidates: []*genai.Candidate{
						{
							Content: genai.NewContentFromParts([]*genai.Part{
								genai.NewPartFromFunctionCall("get_weather", map[string]any{"city": "Lisbon"}),
							}, genai.RoleModel),
						},
					},
				},
				{
					Candidates: []*genai.Candidate{
						{
							Content: genai.NewContentFromParts([]*genai.Part{
								genai.NewPartFromText("It is sunny and 22°C."),
							}, genai.RoleModel),
						},
					},
				},
			},
			handlers: map[string]ai.HandlerFunc{
				"get_weather": func(ctx context.Context, attr map[string]any) (map[string]any, error) {
					assert.Equal(t, map[string]any{"city": "Lisbon"}, attr)
					return map[string]any{"temperature": 22, "unit": "C"}, nil
				},
			},
			want:            "It is sunny and 22°C.",
			expectToolUsage: true,
		},
		{
			name: "missing handler returns error to model",
			responses: []*genai.GenerateContentResponse{
				{
					Candidates: []*genai.Candidate{
						{
							Content: genai.NewContentFromParts([]*genai.Part{
								genai.NewPartFromFunctionCall("translate", map[string]any{"text": "hola"}),
							}, genai.RoleModel),
						},
					},
				},
				{
					Candidates: []*genai.Candidate{
						{
							Content: genai.NewContentFromParts([]*genai.Part{
								genai.NewPartFromText("Sorry, I cannot translate that."),
							}, genai.RoleModel),
						},
					},
				},
			},
			want:            "Sorry, I cannot translate that.",
			expectToolUsage: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &Client{
				Config:   config.NewConfigManager(),
				EventBus: &events.NoOpEventBus{},
			}

			callIdx := 0
			client.callGenerateContentFn = func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig, handlers map[string]ai.HandlerFunc) (*genai.GenerateContentResponse, error) {
				require.Less(t, callIdx, len(tc.responses), "unexpected extra call to callGenerateContentFn")
				resp := tc.responses[callIdx]
				callIdx++
				return resp, nil
			}

			prompt := ai.Prompt{
				Text:      "test",
				ModelName: "gemini-2.0-flash",
				Handlers:  tc.handlers,
			}

			result, err := client.generateContentWithPrompt(context.Background(), prompt, false)
			if tc.wantErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, strings.TrimSpace(result))
			}

			assert.Equal(t, len(tc.responses), callIdx)
		})
	}
}

func TestClientGenerateContentWithPrompt_AllToolsNoFinalText(t *testing.T) {
	client := &Client{
		Config:   config.NewConfigManager(),
		EventBus: &events.NoOpEventBus{},
	}

	responses := []*genai.GenerateContentResponse{
		{
			Candidates: []*genai.Candidate{
				{
					Content: genai.NewContentFromParts([]*genai.Part{
						genai.NewPartFromFunctionCall("noop", map[string]any{"task": "run"}),
					}, genai.RoleModel),
				},
			},
		},
		{
			Candidates: []*genai.Candidate{
				{
					Content: nil,
				},
			},
		},
	}

	callIdx := 0
	client.callGenerateContentFn = func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig, handlers map[string]ai.HandlerFunc) (*genai.GenerateContentResponse, error) {
		require.Less(t, callIdx, len(responses), "unexpected extra call to callGenerateContentFn")
		resp := responses[callIdx]
		callIdx++
		return resp, nil
	}

	prompt := ai.Prompt{
		Text: "do something",
		Handlers: map[string]ai.HandlerFunc{
			"noop": func(ctx context.Context, attr map[string]any) (map[string]any, error) {
				return map[string]any{"status": "ok"}, nil
			},
		},
	}

	resp, err := client.generateContentWithPrompt(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.Equal(t, "", resp)
	assert.Equal(t, len(responses), callIdx)
}

func TestMalformedFunctionCallRetry_NonStreaming(t *testing.T) {
	client := &Client{
		Config:   config.NewConfigManager(),
		EventBus: &events.NoOpEventBus{},
	}

	responses := []*genai.GenerateContentResponse{
		{
			Candidates: []*genai.Candidate{
				{
					Content: genai.NewContentFromParts([]*genai.Part{
						genai.NewPartFromText("Let me call that tool"),
					}, genai.RoleModel),
					FinishReason:  genai.FinishReasonMalformedFunctionCall,
					FinishMessage: "could not parse function call arguments as JSON",
				},
			},
		},
		{
			Candidates: []*genai.Candidate{
				{
					Content: genai.NewContentFromParts([]*genai.Part{
						genai.NewPartFromText("Here is the corrected answer."),
					}, genai.RoleModel),
				},
			},
		},
	}

	callIdx := 0
	var capturedContents [][]*genai.Content
	client.callGenerateContentFn = func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig, handlers map[string]ai.HandlerFunc) (*genai.GenerateContentResponse, error) {
		require.Less(t, callIdx, len(responses), "unexpected extra call")
		capturedContents = append(capturedContents, contents)
		resp := responses[callIdx]
		callIdx++
		return resp, nil
	}

	prompt := ai.Prompt{Text: "test", ModelName: "gemini-2.0-flash"}

	result, err := client.generateContentWithPrompt(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.Equal(t, "Here is the corrected answer.", strings.TrimSpace(result))
	assert.Equal(t, 2, callIdx, "should have made exactly 2 calls")

	// Verify the second call includes the error feedback message
	require.Len(t, capturedContents, 2)
	secondCallContents := capturedContents[1]
	lastContent := secondCallContents[len(secondCallContents)-1]
	assert.Equal(t, "user", string(lastContent.Role))
	assert.Contains(t, lastContent.Parts[0].Text, "malformed")
	assert.Contains(t, lastContent.Parts[0].Text, "could not parse function call arguments as JSON")
}

func TestMalformedFunctionCallRetry_Streaming(t *testing.T) {
	client := &Client{
		Config:   config.NewConfigManager(),
		EventBus: &events.NoOpEventBus{},
	}

	// We need a real genai.Client with a Models field to call GenerateContentStream,
	// but we can test the streamGenerationStep logic indirectly via runContentStream.
	// Instead, test the non-streaming path which uses callGenerateContentFn,
	// and verify the streaming path logic via the checkFinishReason removal.

	// Test that checkFinishReason no longer returns an error for MalformedFunctionCall
	err := client.checkFinishReason(genai.FinishReasonMalformedFunctionCall, "bad args")
	assert.NoError(t, err, "checkFinishReason should no longer error on MalformedFunctionCall")

	// Test that other blocking reasons still error
	err = client.checkFinishReason(genai.FinishReasonSafety, "unsafe")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "safety filters")
}

func TestClientGenerateContentWithPrompt_EmptyWithoutToolsErrors(t *testing.T) {
	client := &Client{
		Config:   config.NewConfigManager(),
		EventBus: &events.NoOpEventBus{},
	}

	client.callGenerateContentFn = func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig, handlers map[string]ai.HandlerFunc) (*genai.GenerateContentResponse, error) {
		return &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: nil,
				},
			},
		}, nil
	}

	prompt := ai.Prompt{Text: "ping"}

	resp, err := client.generateContentWithPrompt(context.Background(), prompt, false)
	require.Error(t, err)
	assert.Empty(t, resp)
	assert.Contains(t, err.Error(), "no content in response candidate")
}

func TestDedupeFunctionCallParts(t *testing.T) {
	assert.Equal(t, 0, dedupeFunctionCallParts(nil))

	callA := func() *genai.Part { return genai.NewPartFromFunctionCall("a", map[string]any{"x": 1}) }
	content := genai.NewContentFromParts([]*genai.Part{
		callA(),
		genai.NewPartFromText("keep me"),
		callA(),
		genai.NewPartFromFunctionCall("b", map[string]any{"x": 1}),
		callA(),
	}, genai.RoleModel)

	dropped := dedupeFunctionCallParts(content)
	assert.Equal(t, 2, dropped)
	require.Len(t, content.Parts, 3)
	assert.Equal(t, "a", content.Parts[0].FunctionCall.Name)
	assert.Equal(t, "keep me", content.Parts[1].Text)
	assert.Equal(t, "b", content.Parts[2].FunctionCall.Name)

	// Same name with different args is a distinct call, not a dupe.
	distinctArgs := genai.NewContentFromParts([]*genai.Part{
		genai.NewPartFromFunctionCall("a", map[string]any{"x": 1}),
		genai.NewPartFromFunctionCall("a", map[string]any{"x": 2}),
	}, genai.RoleModel)
	assert.Equal(t, 0, dedupeFunctionCallParts(distinctArgs))
	assert.Len(t, distinctArgs.Parts, 2)
}

func TestDuplicateFunctionCallsDedupedWithinResponse(t *testing.T) {
	client := &Client{
		Config:   config.NewConfigManager(),
		EventBus: &events.NoOpEventBus{},
	}

	// A degenerate response repeating one complete call five times, plus one
	// distinct call — mirrors a model repetition loop within a single response.
	dupeArgs := map[string]any{"thought": "Explaining the phrase objectively.", "thoughtNumber": 1}
	parts := make([]*genai.Part, 0, 6)
	for i := 0; i < 5; i++ {
		parts = append(parts, genai.NewPartFromFunctionCall("thinking", dupeArgs))
	}
	parts = append(parts, genai.NewPartFromFunctionCall("get_weather", map[string]any{"city": "Lisbon"}))

	responses := []*genai.GenerateContentResponse{
		{
			Candidates: []*genai.Candidate{
				{Content: genai.NewContentFromParts(parts, genai.RoleModel)},
			},
		},
		{
			Candidates: []*genai.Candidate{
				{Content: genai.NewContentFromParts([]*genai.Part{genai.NewPartFromText("done")}, genai.RoleModel)},
			},
		},
	}

	executions := map[string]int{}
	handlers := map[string]ai.HandlerFunc{
		"thinking": func(ctx context.Context, attr map[string]any) (map[string]any, error) {
			executions["thinking"]++
			return map[string]any{"ok": true}, nil
		},
		"get_weather": func(ctx context.Context, attr map[string]any) (map[string]any, error) {
			executions["get_weather"]++
			return map[string]any{"temperature": 22}, nil
		},
	}

	callIdx := 0
	var capturedContents [][]*genai.Content
	client.callGenerateContentFn = func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig, hs map[string]ai.HandlerFunc) (*genai.GenerateContentResponse, error) {
		require.Less(t, callIdx, len(responses), "unexpected extra call")
		capturedContents = append(capturedContents, contents)
		resp := responses[callIdx]
		callIdx++
		return resp, nil
	}

	prompt := ai.Prompt{Text: "test", ModelName: "gemini-2.0-flash", Handlers: handlers}

	result, err := client.generateContentWithPrompt(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.Equal(t, "done", strings.TrimSpace(result))
	assert.Equal(t, 1, executions["thinking"], "identical duplicate calls must execute once")
	assert.Equal(t, 1, executions["get_weather"])

	// The second call's history must carry only the deduped call parts and a
	// function-response turn matching them one-to-one.
	require.Len(t, capturedContents, 2)
	second := capturedContents[1]
	require.GreaterOrEqual(t, len(second), 3)
	modelTurn := second[len(second)-2]
	responseTurn := second[len(second)-1]
	assert.Len(t, modelTurn.Parts, 2)
	assert.Len(t, responseTurn.Parts, 2)
}

func TestModelLoopGuardBreaksIdenticalIterations(t *testing.T) {
	client := &Client{
		Config:   config.NewConfigManager(),
		EventBus: &events.NoOpEventBus{},
	}

	callIdx := 0
	client.callGenerateContentFn = func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig, hs map[string]ai.HandlerFunc) (*genai.GenerateContentResponse, error) {
		callIdx++
		return &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: genai.NewContentFromParts([]*genai.Part{
						genai.NewPartFromFunctionCall("thinking", map[string]any{"thought": "same"}),
					}, genai.RoleModel),
				},
			},
		}, nil
	}

	handlers := map[string]ai.HandlerFunc{
		"thinking": func(ctx context.Context, attr map[string]any) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		},
	}

	prompt := ai.Prompt{Text: "test", ModelName: "gemini-2.0-flash", Handlers: handlers}

	_, err := client.generateContentWithPrompt(context.Background(), prompt, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stuck in loop")
	assert.Equal(t, 3, callIdx, "guard should break on the third identical iteration")
}

func thinkingCallResponse() *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: genai.NewContentFromParts([]*genai.Part{
					genai.NewPartFromFunctionCall("thinking", map[string]any{"thought": "same"}),
				}, genai.RoleModel),
			},
		},
	}
}

func TestStreamingLoopGuardBreaksIdenticalIterations(t *testing.T) {
	client := &Client{
		Config:   config.NewConfigManager(),
		EventBus: &events.NoOpEventBus{},
	}

	streamCalls := 0
	client.generateContentStreamFn = func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error] {
		streamCalls++
		return func(yield func(*genai.GenerateContentResponse, error) bool) {
			yield(thinkingCallResponse(), nil)
		}
	}

	handlers := map[string]ai.HandlerFunc{
		"thinking": func(ctx context.Context, attr map[string]any) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		},
	}

	prompt := ai.Prompt{Text: "test", ModelName: "gemini-2.0-flash", Handlers: handlers, MaxToolIterations: 10}
	stream, err := client.generateContentStreamWithPrompt(context.Background(), prompt)
	require.NoError(t, err)

	streamErr := drainStream(t, stream)
	require.Error(t, streamErr)
	assert.Contains(t, streamErr.Error(), "stuck in loop")
	assert.Equal(t, 3, streamCalls, "guard should break on the third identical iteration")
}

func TestStreamingDuplicateFunctionCallsExecuteOnce(t *testing.T) {
	// Repro of a degenerate generation observed with gemini-3.1-pro-preview:
	// the model streams the same complete function call once per chunk —
	// 107 copies accumulated in a single response.
	client := &Client{
		Config:   config.NewConfigManager(),
		EventBus: &events.NoOpEventBus{},
	}

	streamCalls := 0
	client.generateContentStreamFn = func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error] {
		streamCalls++
		call := streamCalls
		return func(yield func(*genai.GenerateContentResponse, error) bool) {
			if call == 1 {
				for i := 0; i < 107; i++ {
					if !yield(thinkingCallResponse(), nil) {
						return
					}
				}
				return
			}
			yield(&genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{Content: genai.NewContentFromParts([]*genai.Part{genai.NewPartFromText("done")}, genai.RoleModel)},
				},
			}, nil)
		}
	}

	executions := 0
	handlers := map[string]ai.HandlerFunc{
		"thinking": func(ctx context.Context, attr map[string]any) (map[string]any, error) {
			executions++
			return map[string]any{"ok": true}, nil
		},
	}

	prompt := ai.Prompt{Text: "test", ModelName: "gemini-2.0-flash", Handlers: handlers, MaxToolIterations: 10}
	stream, err := client.generateContentStreamWithPrompt(context.Background(), prompt)
	require.NoError(t, err)

	streamErr := drainStream(t, stream)
	require.NoError(t, streamErr)
	assert.Equal(t, 1, executions, "107 identical calls in one response must execute once")
	assert.Equal(t, 2, streamCalls)
}

// drainStream consumes a stream to completion and returns its terminal
// error (nil for a clean EOF).
func drainStream(t *testing.T, stream ai.Stream) error {
	t.Helper()
	defer stream.Close()
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
