package genie_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/tools"
)

// mockPersona implements the Persona interface for testing
type mockPersona struct {
	id     string
	name   string
	source string
}

func (m *mockPersona) GetID() string     { return m.id }
func (m *mockPersona) GetName() string   { return m.name }
func (m *mockPersona) GetSource() string { return m.source }

// mockSession implements the Session interface for testing
type mockSession struct {
	id            string
	workingDir    string
	genieHomeDir  string
	createdAt     string
	persona       genie.Persona
}

func (m *mockSession) GetID() string {
	return m.id
}

func (m *mockSession) GetWorkingDirectory() string {
	return m.workingDir
}

func (m *mockSession) GetGenieHomeDirectory() string {
	if m.genieHomeDir == "" {
		return "/test/home"  // default for backward compatibility
	}
	return m.genieHomeDir
}

func (m *mockSession) GetCreatedAt() string {
	return m.createdAt
}

func (m *mockSession) GetPersona() genie.Persona {
	return m.persona
}

func (m *mockSession) SetPersona(persona genie.Persona) {
	m.persona = persona
}

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

func (g *testGenie) Chat(ctx context.Context, message string, _ ...genie.ChatOption) error {
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

func (g *testGenie) Start(workingDir *string, persona *string, _ ...genie.StartOption) (genie.Session, error) {
	// Mock implementation - return session with correct working directory
	actualWorkingDir := "/test/dir" // default
	if workingDir != nil {
		actualWorkingDir = *workingDir
	}

	var actualPersona genie.Persona
	if persona != nil && *persona != "" {
		actualPersona = &mockPersona{
			id:     *persona,
			name:   *persona,
			source: "test",
		}
	}

	return &mockSession{
		id:         uuid.New().String(),
		workingDir: actualWorkingDir,
		createdAt:  time.Now().String(),
		persona:    actualPersona,
	}, nil
}

func (g *testGenie) GetEventBus() events.EventBus {
	return g.eventBus
}

func (g *testGenie) GetContext(ctx context.Context) (map[string]string, error) {
	// Mock implementation - return empty context map
	return make(map[string]string), nil
}

func (g *testGenie) GetStatus() *genie.Status {
	// Mock implementation - return mock status
	return &genie.Status{
		Connected: true,
		Backend:   "test-mock",
		Message:   "Test mock is connected",
	}
}

func (g *testGenie) ListPersonas(ctx context.Context) ([]genie.Persona, error) {
	// Mock implementation - return empty list
	return []genie.Persona{}, nil
}

func (g *testGenie) GetSession() (genie.Session, error) {
	// Mock implementation - return a mock session
	return &mockSession{
		id:         "test-session",
		workingDir: "/test/dir",
		createdAt:  "test-time",
		persona:    nil, // No persona set initially
	}, nil
}

func (g *testGenie) GetToolsRegistry() (tools.Registry, error) {
	// Mock implementation - return empty registry
	return tools.NewRegistry(), nil
}

func (g *testGenie) RecalculateContextBudget(ctx context.Context) error {
	return nil
}

func TestGenieWithWorkingDirectory(t *testing.T) {
	workingDir := "/test/working/dir"
	g, _ := createTestGenie(t)

	// Test that Start() returns session with correct working directory
	session, err := g.Start(&workingDir, nil)
	if err != nil {
		t.Fatalf("Expected Start to succeed, got error: %v", err)
	}

	if session.GetWorkingDirectory() != workingDir {
		t.Errorf("Expected session working directory %s, got %s", workingDir, session.GetWorkingDirectory())
	}
}

func TestSessionSetPersona(t *testing.T) {
	g, _ := createTestGenie(t)

	// Start with no persona
	session, err := g.Start(nil, nil)
	if err != nil {
		t.Fatalf("Expected Start to succeed, got error: %v", err)
	}

	// Verify initial persona is nil
	if session.GetPersona() != nil {
		t.Errorf("Expected initial persona to be nil, got %v", session.GetPersona())
	}

	// Set a persona
	testPersona := &mockPersona{
		id:     "test-persona",
		name:   "Test Persona",
		source: "test",
	}
	session.SetPersona(testPersona)

	// Verify persona was set
	if session.GetPersona() == nil {
		t.Error("Expected persona to be set, got nil")
	} else if session.GetPersona().GetID() != "test-persona" {
		t.Errorf("Expected persona ID %s, got %s", "test-persona", session.GetPersona().GetID())
	}

	// Change persona
	newPersona := &mockPersona{
		id:     "new-persona",
		name:   "New Persona",
		source: "test",
	}
	session.SetPersona(newPersona)

	// Verify persona was changed
	if session.GetPersona() == nil {
		t.Error("Expected persona to be set, got nil")
	} else if session.GetPersona().GetID() != "new-persona" {
		t.Errorf("Expected persona ID %s, got %s", "new-persona", session.GetPersona().GetID())
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
