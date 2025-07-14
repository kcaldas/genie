package genai

import (
	"context"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
)

func TestClient_CountTokens(t *testing.T) {
	// This test will skip if no API key is configured
	client, err := NewClient(nil)
	if err != nil {
		t.Skip("Skipping CountTokens test - no genai client available:", err)
	}

	// Test basic token counting
	prompt := ai.Prompt{
		Text:      "The quick brown fox jumps over the lazy dog.",
		ModelName: "gemini-2.0-flash",
	}

	ctx := context.Background()
	tokenCount, err := client.CountTokens(ctx, prompt, false)

	// If we get an authentication error, skip the test
	if err != nil && (containsError(err, "API") || containsError(err, "authentication") || containsError(err, "credential")) {
		t.Skip("Skipping CountTokens test - API credentials not configured:", err)
	}

	assert.NoError(t, err)
	assert.NotNil(t, tokenCount)
	assert.Greater(t, tokenCount.TotalTokens, int32(0), "Should have counted some tokens")

	t.Logf("Token count for test prompt: %d total tokens", tokenCount.TotalTokens)
}

func TestClient_CountTokens_WithInstruction(t *testing.T) {
	// This test will skip if no API key is configured
	client, err := NewClient(nil)
	if err != nil {
		t.Skip("Skipping CountTokens test - no genai client available:", err)
	}

	// Test token counting with system instruction
	prompt := ai.Prompt{
		Instruction: "You are a helpful assistant.",
		Text:        "Hello, how are you?",
		ModelName:   "gemini-2.0-flash",
	}

	ctx := context.Background()
	tokenCount, err := client.CountTokens(ctx, prompt, false)

	// If we get an authentication error, skip the test
	if err != nil && (containsError(err, "API") || containsError(err, "authentication") || containsError(err, "credential")) {
		t.Skip("Skipping CountTokens test - API credentials not configured:", err)
	}

	assert.NoError(t, err)
	assert.NotNil(t, tokenCount)
	assert.Greater(t, tokenCount.TotalTokens, int32(0), "Should have counted some tokens")

	t.Logf("Token count for prompt with instruction: %d total tokens", tokenCount.TotalTokens)
}

func containsError(err error, substr string) bool {
	return err != nil && strings.Contains(err.Error(), substr)
}

