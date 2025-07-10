package types

type Notification interface {
	AddSystemMessage(string)
	AddErrorMessage(string)
}
