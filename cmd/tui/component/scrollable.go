package component

import (
	"strings"

	"github.com/awesome-gocui/gocui"
)

// Scrollable defines the interface for components that support scrolling
type Scrollable interface {
	ScrollUp() error
	ScrollDown() error
	PageUp() error
	PageDown() error
	ScrollToTop() error
	ScrollToBottom() error
}

// ScrollableBase provides default scroll implementations for components
type ScrollableBase struct {
	getView func() *gocui.View
}

// NewScrollableBase creates a new ScrollableBase with a view getter function
func NewScrollableBase(viewGetter func() *gocui.View) *ScrollableBase {
	return &ScrollableBase{
		getView: viewGetter,
	}
}

// ScrollUp scrolls the view up by one line
func (s *ScrollableBase) ScrollUp() error {
	v := s.getView()
	if v == nil {
		return nil
	}
	ox, oy := v.Origin()
	if oy > 0 {
		v.SetOrigin(ox, oy-1)
	}
	return nil
}

// ScrollDown scrolls the view down by one line
func (s *ScrollableBase) ScrollDown() error {
	v := s.getView()
	if v == nil {
		return nil
	}
	ox, oy := v.Origin()
	v.SetOrigin(ox, oy+1)
	return nil
}

// PageUp scrolls the view up by one page
func (s *ScrollableBase) PageUp() error {
	v := s.getView()
	if v == nil {
		return nil
	}
	ox, oy := v.Origin()
	_, height := v.Size()
	newY := oy - height
	if newY < 0 {
		newY = 0
	}
	v.SetOrigin(ox, newY)
	return nil
}

// PageDown scrolls the view down by one page
func (s *ScrollableBase) PageDown() error {
	v := s.getView()
	if v == nil {
		return nil
	}
	ox, oy := v.Origin()
	_, height := v.Size()
	v.SetOrigin(ox, oy+height)
	return nil
}

// ScrollToTop scrolls the view to the top
func (s *ScrollableBase) ScrollToTop() error {
	v := s.getView()
	if v == nil {
		return nil
	}
	v.SetOrigin(0, 0)
	return nil
}

// ScrollToBottom scrolls the view to the bottom using the view's buffer
func (s *ScrollableBase) ScrollToBottom() error {
	v := s.getView()
	if v == nil {
		return nil
	}

	// Get the content from the view's buffer
	content := v.ViewBuffer()
	lines := strings.Count(content, "\n")
	_, height := v.Size()

	// Calculate the target Y position to show the bottom
	targetY := lines - height + 1
	if targetY < 0 {
		targetY = 0
	}

	v.SetOrigin(0, targetY)
	return nil
}