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

			result, toolUsed, err := client.callGenerateContent(context.Background(), "gemini-2.0-flash", initialContents, configCopy, tc.handlers)
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
