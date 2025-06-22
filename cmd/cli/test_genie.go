package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
)

// testGenie is a lightweight implementation of the Genie interface for UI testing
type testGenie struct {
	mockResponse string
	mockError    error
	sessions     map[string]*genie.Session
	eventBus     events.EventBus
}

// newTestGenie creates a test Genie with configurable mock responses
func newTestGenie(mockResponse string, mockError error) (genie.Genie, events.EventBus) {
	eventBus := events.NewEventBus()
	testG := &testGenie{
		mockResponse: mockResponse,
		mockError:    mockError,
		sessions:     make(map[string]*genie.Session),
		eventBus:     eventBus,
	}
	return testG, eventBus
}

// Chat simulates async chat processing
func (g *testGenie) Chat(ctx context.Context, sessionID string, message string) error {
	// Publish started event immediately
	startEvent := genie.ChatStartedEvent{
		SessionID: sessionID,
		Message:   message,
	}
	g.eventBus.Publish("chat.started", startEvent)
	
	// Simulate async processing
	go func() {
		// Small delay to simulate real async behavior
		time.Sleep(10 * time.Millisecond)
		
		// Create response event
		responseEvent := genie.ChatResponseEvent{
			SessionID: sessionID,
			Message:   message,
			Response:  g.mockResponse,
			Error:     g.mockError,
		}
		g.eventBus.Publish("chat.response", responseEvent)
	}()
	
	return nil
}

// CreateSession creates a mock session with generated ID
func (g *testGenie) CreateSession() (string, error) {
	sessionID := uuid.New().String()
	g.sessions[sessionID] = &genie.Session{
		ID:           sessionID,
		CreatedAt:    time.Now().String(),
		Interactions: []genie.Interaction{},
	}
	return sessionID, nil
}

// GetSession retrieves a mock session
func (g *testGenie) GetSession(sessionID string) (*genie.Session, error) {
	if session, exists := g.sessions[sessionID]; exists {
		return session, nil
	}
	
	// Return error if session doesn't exist - don't auto-create
	return nil, fmt.Errorf("session %s not found", sessionID)
}

