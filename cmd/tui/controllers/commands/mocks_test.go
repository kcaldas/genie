package commands

import (
	"context"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
)

// MockPersona implements genie.Persona for testing
type MockPersona struct {
	id     string
	name   string
	source string
}

func (m *MockPersona) GetID() string     { return m.id }
func (m *MockPersona) GetName() string   { return m.name }
func (m *MockPersona) GetSource() string { return m.source }

// mockSession implements the genie.Session interface for testing
type mockSession struct {
	persona genie.Persona
}

func (m *mockSession) GetID() string                { return "test-id" }
func (m *mockSession) GetWorkingDirectory() string  { return "/test/dir" }
func (m *mockSession) GetCreatedAt() string         { return "test-time" }
func (m *mockSession) GetPersona() genie.Persona    { 
	if m.persona == nil {
		return &MockPersona{id: "test-persona", name: "Test Persona", source: "test"}
	}
	return m.persona
}
func (m *mockSession) SetPersona(persona genie.Persona) { m.persona = persona }

// MockGenieService implements genie.Genie for testing
type MockGenieService struct {
	mockStatus         *genie.Status
	mockPersonas       []genie.Persona
	mockPersonasError  error
	mockSession        genie.Session
}

func (m *MockGenieService) Start(workingDir *string, persona *string) (genie.Session, error) {
	return &mockSession{}, nil
}

func (m *MockGenieService) Chat(ctx context.Context, message string) error {
	return nil
}

func (m *MockGenieService) GetContext(ctx context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (m *MockGenieService) GetStatus() *genie.Status {
	return m.mockStatus
}

func (m *MockGenieService) GetEventBus() events.EventBus {
	return events.NewEventBus()
}

func (m *MockGenieService) ListPersonas(ctx context.Context) ([]genie.Persona, error) {
	if m.mockPersonasError != nil {
		return nil, m.mockPersonasError
	}
	return m.mockPersonas, nil
}

func (m *MockGenieService) GetSession() (genie.Session, error) {
	if m.mockSession != nil {
		return m.mockSession, nil
	}
	return &mockSession{}, nil
}