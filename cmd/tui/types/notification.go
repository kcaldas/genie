package types

type Notification interface {
	AddSystemMessage(string)
	AddErrorMessage(string)
}

type MockNotification struct {
	SystemMessages []string
	ErrorMessages  []string
}

func (m *MockNotification) AddSystemMessage(msg string) {
	m.SystemMessages = append(m.SystemMessages, msg)
}
func (m *MockNotification) AddErrorMessage(msg string) {
	m.ErrorMessages = append(m.ErrorMessages, msg)
}
