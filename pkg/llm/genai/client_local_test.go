package genai

import (
	"context"
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
			name: "missing handler",
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
			},
			wantErrMsg: "no handler found for function \"translate\"",
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

			initialContents := []*genai.Content{
				genai.NewContentFromParts([]*genai.Part{genai.NewPartFromText("test")}, genai.RoleUser),
			}
			configCopy := &genai.GenerateContentConfig{}

			result, toolUsed, err := client.callGenerateContent(context.Background(), "gemini-2.0-flash", initialContents, configCopy, tc.handlers, defaultMaxToolIterations)
			if tc.wantErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tc.want, strings.TrimSpace(result.Text()))
				assert.Equal(t, tc.expectToolUsage, toolUsed)
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

	initialContents := []*genai.Content{
		genai.NewContentFromParts([]*genai.Part{genai.NewPartFromText("test")}, genai.RoleUser),
	}

	result, _, err := client.callGenerateContent(context.Background(), "gemini-2.0-flash", initialContents, &genai.GenerateContentConfig{}, nil, defaultMaxToolIterations)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Here is the corrected answer.", strings.TrimSpace(result.Text()))
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
