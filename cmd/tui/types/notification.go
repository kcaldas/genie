package types

type Notification interface {
	AddSystemMessage(string)
	AddErrorMessage(string)
	AddAssistantMessage(string)
	ShowWelcomeMessage()
}

type MockNotification struct {
	SystemMessages    []string
	ErrorMessages     []string
	AssistantMessages []string
}

func (m *MockNotification) AddSystemMessage(msg string) {
	m.SystemMessages = append(m.SystemMessages, msg)
}
func (m *MockNotification) AddErrorMessage(msg string) {
	m.ErrorMessages = append(m.ErrorMessages, msg)
}
func (m *MockNotification) AddAssistantMessage(msg string) {
	m.AssistantMessages = append(m.AssistantMessages, msg)
}
func (m *MockNotification) ShowWelcomeMessage() {
	m.AddSystemMessage("Welcome to Genie! Type :? for help.")
}
