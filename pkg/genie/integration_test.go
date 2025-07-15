package genie_test

import (
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/genie"
)

func TestIntegrationBasic(t *testing.T) {
	fixture := genie.NewTestFixture(t)
	fixture.ExpectSimpleMessage("integration test", "integration response")

	fixture.StartAndGetSession()
	err := fixture.StartChat("integration test")
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	response := fixture.WaitForResponseOrFail(2 * time.Second)
	if response.Response != "integration response" {
		t.Errorf("Expected 'integration response', got %q", response.Response)
	}
}

func TestRealPromptProcessing(t *testing.T) {
	fixture := genie.NewTestFixture(t, genie.WithRealPromptProcessing())

	simplePrompt := &ai.Prompt{
		Name: "test_prompt",
		Text: "Echo: {{.message}}",
	}
	fixture.UsePrompt(simplePrompt)

	// Note: GetMockLLM() access is appropriate here since we're using WithRealPromptProcessing()
	// which tests actual prompt execution with real LLM calls (but mocked LLM responses)
	fixture.GetMockLLM().SetResponseForPrompt("test_prompt", "Echo: test")

	fixture.StartAndGetSession()
	err := fixture.StartChat("test")
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	response := fixture.WaitForResponseOrFail(2 * time.Second)
	if response.Response != "Echo: test" {
		t.Errorf("Expected 'Echo: test', got %q", response.Response)
	}
}
