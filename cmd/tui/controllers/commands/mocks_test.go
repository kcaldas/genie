package commands

import (
	"context"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
)

// mockSession implements the genie.Session interface for testing
type mockSession struct {
	persona string
}

func (m *mockSession) GetID() string                { return "test-id" }
func (m *mockSession) GetWorkingDirectory() string  { return "/test/dir" }
func (m *mockSession) GetCreatedAt() string         { return "test-time" }
func (m *mockSession) GetPersona() string           { 
	if m.persona == "" {
		return "test-persona"
	}
	return m.persona
}
func (m *mockSession) SetPersona(persona string)    { m.persona = persona }

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