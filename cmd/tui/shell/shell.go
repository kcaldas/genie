package shell

import (
	"github.com/awesome-gocui/gocui"
)

// Command represents a parsed command from the shell input.
// This is a minimal representation for now, focusing on the raw text.
type Command struct {
	Text string
}

// Shell defines the interface for our custom shell abstraction.
// It embeds gocui.Editor to handle direct input to a gocui.View.
type Shell interface {
	gocui.Editor // Embed gocui.Editor to handle character input and basic cursor movements

	// GetInputBuffer returns the current content of the input buffer.
	GetInputBuffer() string

	// SetInputBuffer sets the content of the input buffer and updates the view.
	SetInputBuffer(s string, v *gocui.View)

	// ClearInput clears the input buffer and resets the view.
	ClearInput(v *gocui.View)

	// NavigateHistoryUp navigates to the previous command in history.
	NavigateHistoryUp(v *gocui.View)

	// NavigateHistoryDown navigates to the next command in history.
	NavigateHistoryDown(v *gocui.View)

	// ResetHistoryNavigation resets the history navigation state.
	ResetHistoryNavigation()
}
