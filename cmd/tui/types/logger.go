package types

// Logger defines the interface for debug logging
type Logger interface {
	Debug(message string)
}