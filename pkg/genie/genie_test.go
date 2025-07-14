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

	responses := make(chan events.ChatResponseEvent, 1)
	eventBus.Subscribe("chat.response", func(event any) {
		if resp, ok := event.(events.ChatResponseEvent); ok {
			responses <- resp
		}
	})

	message := "Hello, how are you?"
	err := g.Chat(context.Background(), message)

	if err != nil {
		t.Fatalf("Expected chat to start without error, got: %v", err)
	}
	select {
	case response := <-responses:
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

	started := make(chan events.ChatStartedEvent, 1)
	eventBus.Subscribe("chat.started", func(event any) {
		if start, ok := event.(events.ChatStartedEvent); ok {
			started <- start
		}
	})

	message := "Test message"
	err := g.Chat(context.Background(), message)

	if err != nil {
		t.Fatalf("Expected chat to start without error, got: %v", err)
	}
	select {
	case event := <-started:
		if event.Message != message {
			t.Errorf("Expected started event for message %s, got %s", message, event.Message)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for chat started event")
	}
}

// testGenie is a test implementation that simulates async chat processing
type testGenie struct {
	eventBus events.EventBus
}

func (g *testGenie) Chat(ctx context.Context, message string) error {
	// Publish started event immediately
	startEvent := events.ChatStartedEvent{
		Message: message,
	}
	g.eventBus.Publish("chat.started", startEvent)

	// Simulate async processing with a goroutine
	go func() {
		// Simulate some processing time
		time.Sleep(100 * time.Millisecond)

		// Create a mock response
		responseEvent := events.ChatResponseEvent{
			Message:  message,
			Response: "Mock response to: " + message,
			Error:    nil,
		}
		g.eventBus.Publish("chat.response", responseEvent)
	}()

	return nil
}

func (g *testGenie) Start(workingDir *string, persona *string) (*genie.Session, error) {
	// Mock implementation - return session with correct working directory
	actualWorkingDir := "/test/dir" // default
	if workingDir != nil {
		actualWorkingDir = *workingDir
	}

	return &genie.Session{
		ID:               uuid.New().String(),
		WorkingDirectory: actualWorkingDir,
		CreatedAt:        time.Now().String(),
	}, nil
}

func (g *testGenie) GetSession() (*genie.Session, error) {
	// Mock implementation - return empty session
	return &genie.Session{
		CreatedAt:    time.Now().String(),
	}, nil
}

func (g *testGenie) GetEventBus() events.EventBus {
	return g.eventBus
}

func (g *testGenie) GetContext(ctx context.Context) (map[string]string, error) {
	// Mock implementation - return empty context map
	return make(map[string]string), nil
}

func TestGenieWithWorkingDirectory(t *testing.T) {
	workingDir := "/test/working/dir"
	g, _ := createTestGenie(t)

	// Test that Start() returns session with correct working directory
	session, err := g.Start(&workingDir, nil)
	if err != nil {
		t.Fatalf("Expected Start to succeed, got error: %v", err)
	}

	if session.WorkingDirectory != workingDir {
		t.Errorf("Expected session working directory %s, got %s", workingDir, session.WorkingDirectory)
	}
}

func createTestGenie(t *testing.T) (genie.Genie, events.EventBus) {
	t.Helper()
	eventBus := events.NewEventBus()
	testG := &testGenie{
		eventBus: eventBus,
	}
	return testG, eventBus
}
