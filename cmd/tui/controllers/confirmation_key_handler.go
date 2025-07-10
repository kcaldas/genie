package controllers

import "github.com/awesome-gocui/gocui"

// ConfirmationKeyHandler provides common key interpretation logic for confirmation dialogs
type ConfirmationKeyHandler struct{}

// NewConfirmationKeyHandler creates a new confirmation key handler
func NewConfirmationKeyHandler() *ConfirmationKeyHandler {
	return &ConfirmationKeyHandler{}
}

// InterpretKey determines if a key press is a confirmation response
// Returns (confirmed, handled) where:
// - confirmed: true for "yes" keys, false for "no" keys
// - handled: true if the key was recognized as a confirmation key, false otherwise
func (c *ConfirmationKeyHandler) InterpretKey(key interface{}) (confirmed bool, handled bool) {
	switch key {
	case '1', 'y', 'Y':
		return true, true
	case '2', 'n', 'N', gocui.KeyEsc:
		return false, true
	default:
		return false, false
	}
}