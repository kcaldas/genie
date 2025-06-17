package vertex

import (
	"os"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	// Skip this test if required environment variables are not set
	if os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		t.Skip("Skipping test: GOOGLE_CLOUD_PROJECT environment variable not set")
	}

	client := NewClient()

	// Verify it implements the Gen interface
	var _ ai.Gen = client

	assert.NotNil(t, client)
}

func TestClient_GenerateContent(t *testing.T) {
	// Skip this test if required environment variables are not set
	if os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		t.Skip("Skipping test: GOOGLE_CLOUD_PROJECT environment variable not set")
	}

	client := NewClient()

	prompt := ai.Prompt{
		Text:      "Hello {{.name}}",
		ModelName: "gemini-pro",
	}

	// We can't actually test the real API call without credentials
	// but we can test that the method exists and has the right signature
	result, err := client.GenerateContent(prompt, false, "name", "World")

	// For now, just check the method signature works
	_ = result
	_ = err
}
