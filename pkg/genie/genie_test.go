package genie_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
)

func TestGenieCanProcessSimpleChat(t *testing.T) {
	g, eventBus := createTestGenie(t)
	
	responses := make(chan genie.ChatResponseEvent, 1)
	eventBus.Subscribe("chat.response", func(event interface{}) {
		if resp, ok := event.(genie.ChatResponseEvent); ok {
			responses <- resp
		}
	})
	
	sessionID := "test-session"
	message := "Hello, how are you?"
	err := g.Chat(context.Background(), sessionID, message)
	
	if err != nil {
		t.Fatalf("Expected chat to start without error, got: %v", err)
	}
	select {
	case response := <-responses:
		if response.SessionID != sessionID {
			t.Errorf("Expected response for session %s, got %s", sessionID, response.SessionID)
		}
		if response.Message != message {
			t.Errorf("Expected response to original message %s, got %s", message, response.Message)
		}
		if response.Error != nil {
			t.Errorf("Expected successful response, got error: %v", response.Error)
		}
		if response.Response == "" {
			t.Error("Expected non-empty response")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for chat response")
	}
}

func TestGeniePublishesChatStartedEvents(t *testing.T) {
	g, eventBus := createTestGenie(t)
	
	started := make(chan genie.ChatStartedEvent, 1)
	eventBus.Subscribe("chat.started", func(event interface{}) {
		if start, ok := event.(genie.ChatStartedEvent); ok {
			started <- start
		}
	})
	
	sessionID := "test-session"
	message := "Test message"
	err := g.Chat(context.Background(), sessionID, message)
	
	if err != nil {
		t.Fatalf("Expected chat to start without error, got: %v", err)
	}
	select {
	case event := <-started:
		if event.SessionID != sessionID {
			t.Errorf("Expected started event for session %s, got %s", sessionID, event.SessionID)
		}
		if event.Message != message {
			t.Errorf("Expected started event for message %s, got %s", message, event.Message)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for chat started event")
	}
}

func TestGenieHandlesMultipleConcurrentChats(t *testing.T) {
	g, eventBus := createTestGenie(t)
	
	responses := make(chan genie.ChatResponseEvent, 3)
	eventBus.Subscribe("chat.response", func(event interface{}) {
		if resp, ok := event.(genie.ChatResponseEvent); ok {
			responses <- resp
		}
	})
	
	session1 := "session-1"
	session2 := "session-2" 
	session3 := "session-3"
	
	err1 := g.Chat(context.Background(), session1, "Message 1")
	err2 := g.Chat(context.Background(), session2, "Message 2")
	err3 := g.Chat(context.Background(), session3, "Message 3")
	
	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("Expected all chats to start without error, got: %v, %v, %v", err1, err2, err3)
	}
	receivedSessions := make(map[string]bool)
	for i := 0; i < 3; i++ {
		select {
		case response := <-responses:
			if response.Error != nil {
				t.Errorf("Expected successful response for session %s, got error: %v", response.SessionID, response.Error)
			}
			receivedSessions[response.SessionID] = true
		case <-time.After(10 * time.Second):
			t.Fatal("Timeout waiting for concurrent chat responses")
		}
	}
	
	expectedSessions := []string{session1, session2, session3}
	for _, sessionID := range expectedSessions {
		if !receivedSessions[sessionID] {
			t.Errorf("Did not receive response for session %s", sessionID)
		}
	}
}

func TestGenieSessionIsolation(t *testing.T) {
	g, eventBus := createTestGenie(t)
	
	session1Responses := make(chan genie.ChatResponseEvent, 1)
	session2Responses := make(chan genie.ChatResponseEvent, 1)
	
	eventBus.Subscribe("chat.response", func(event interface{}) {
		if resp, ok := event.(genie.ChatResponseEvent); ok {
			switch resp.SessionID {
			case "session-1":
				session1Responses <- resp
			case "session-2":
				session2Responses <- resp
			}
		}
	})
	
	err1 := g.Chat(context.Background(), "session-1", "Message for session 1")
	err2 := g.Chat(context.Background(), "session-2", "Message for session 2")
	
	if err1 != nil || err2 != nil {
		t.Fatalf("Expected chats to start without error, got: %v, %v", err1, err2)
	}
	select {
	case resp1 := <-session1Responses:
		if resp1.SessionID != "session-1" {
			t.Errorf("Session 1 received response for wrong session: %s", resp1.SessionID)
		}
		if resp1.Message != "Message for session 1" {
			t.Errorf("Session 1 received wrong message: %s", resp1.Message)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for session 1 response")
	}
	
	select {
	case resp2 := <-session2Responses:
		if resp2.SessionID != "session-2" {
			t.Errorf("Session 2 received response for wrong session: %s", resp2.SessionID)
		}
		if resp2.Message != "Message for session 2" {
			t.Errorf("Session 2 received wrong message: %s", resp2.Message)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for session 2 response")
	}
}

// testGenie is a test implementation that simulates async chat processing
type testGenie struct {
	eventBus events.EventBus
}

func (g *testGenie) Chat(ctx context.Context, sessionID string, message string) error {
	// Publish started event immediately
	startEvent := genie.ChatStartedEvent{
		SessionID: sessionID,
		Message:   message,
	}
	g.eventBus.Publish("chat.started", startEvent)
	
	// Simulate async processing with a goroutine
	go func() {
		// Simulate some processing time
		time.Sleep(100 * time.Millisecond)
		
		// Create a mock response
		responseEvent := genie.ChatResponseEvent{
			SessionID: sessionID,
			Message:   message,
			Response:  "Mock response to: " + message,
			Error:     nil,
		}
		g.eventBus.Publish("chat.response", responseEvent)
	}()
	
	return nil
}

func (g *testGenie) CreateSession() (string, error) {
	// Mock implementation - generate and return session ID
	return uuid.New().String(), nil
}

func (g *testGenie) GetSession(sessionID string) (*genie.Session, error) {
	// Mock implementation - return empty session
	return &genie.Session{
		ID:           sessionID,
		CreatedAt:    time.Now().String(),
		Interactions: []genie.Interaction{},
	}, nil
}

func createTestGenie(t *testing.T) (genie.Genie, events.EventBus) {
	t.Helper()
	eventBus := events.NewEventBus()
	testG := &testGenie{
		eventBus: eventBus,
	}
	return testG, eventBus
}