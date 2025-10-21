package genai

import (
	"context"
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
