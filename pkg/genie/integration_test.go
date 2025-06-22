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
	
	sessionID := fixture.CreateSession()
	err := fixture.StartChat(sessionID, "integration test")
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	
	response := fixture.WaitForResponseOrFail(2 * time.Second)
	if response.Response != "integration response" {
		t.Errorf("Expected 'integration response', got %q", response.Response)
	}
}

func TestRealChainProcessing(t *testing.T) {
	fixture := genie.NewTestFixture(t, genie.WithRealChainProcessing())
	
	simpleChain := &ai.Chain{
		Name: "test-chain",
		Steps: []interface{}{
			ai.ChainStep{
				Name: "step",
				Prompt: &ai.Prompt{
					Name: "test_prompt",
					Text: "Echo: {{.message}}",
				},
				ForwardAs: "response",
			},
		},
	}
	fixture.UseChain(simpleChain)
	fixture.MockLLM.SetResponseForPrompt("test_prompt", "Echo: test")
	
	sessionID := fixture.CreateSession()
	err := fixture.StartChat(sessionID, "test")
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	
	response := fixture.WaitForResponseOrFail(2 * time.Second)
	if response.Response != "Echo: test" {
		t.Errorf("Expected 'Echo: test', got %q", response.Response)
	}
}

func TestSessionPersistence(t *testing.T) {
	fixture := genie.NewTestFixture(t)
	
	sessionID := fixture.CreateSession()
	session := fixture.GetSession(sessionID)
	
	if session.ID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, session.ID)
	}
}