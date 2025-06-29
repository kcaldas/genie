package component

import (
	"testing"

	"github.com/awesome-gocui/gocui"
	"github.com/jesseduffield/lazycore/pkg/boxlayout"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

// mockDialogGuiCommon implements types.IGuiCommon for testing dialogs
type mockDialogGuiCommon struct{}

func (m *mockDialogGuiCommon) GetGui() *gocui.Gui { return nil } // Won't be used in these tests
func (m *mockDialogGuiCommon) GetConfig() *types.Config {
	return &types.Config{
		ShowCursor:        true,
		MarkdownRendering: true,
		Theme:             "default",
	}
}
func (m *mockDialogGuiCommon) GetTheme() *types.Theme {
	return &types.Theme{Primary: "\033[36m"}
}
func (m *mockDialogGuiCommon) SetCurrentComponent(ctx types.Component) {}
func (m *mockDialogGuiCommon) GetCurrentComponent() types.Component    { return nil }
func (m *mockDialogGuiCommon) PostUIUpdate(fn func())                  { fn() }

func TestDialogComponent_SetInternalLayout(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}
	dialog := &DialogComponent{
		BaseComponent: NewBaseComponent("test", "test-view", guiCommon),
		dialogViews:   make(map[string]*gocui.View),
	}

	// Test setting internal layout
	layout := &boxlayout.Box{
		Direction: boxlayout.COLUMN,
		Children: []*boxlayout.Box{
			{Window: "left", Size: 20},
			{Window: "right", Weight: 1},
		},
	}

	dialog.SetInternalLayout(layout)

	if dialog.internalLayout != layout {
		t.Error("Internal layout was not set correctly")
	}
}

func TestDialogComponent_VisibilityStates(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}
	dialog := &DialogComponent{
		BaseComponent: NewBaseComponent("test", "test-view", guiCommon),
		dialogViews:   make(map[string]*gocui.View),
		isVisible:     false,
	}

	// Initially not visible
	if dialog.IsVisible() {
		t.Error("Dialog should not be visible initially")
	}

	// Test setting visible
	dialog.isVisible = true
	if !dialog.IsVisible() {
		t.Error("Dialog should be visible after setting isVisible to true")
	}

	// Test setting hidden
	dialog.isVisible = false
	if dialog.IsVisible() {
		t.Error("Dialog should not be visible after setting isVisible to false")
	}
}

func TestDialogComponent_GetInternalViewName(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}
	dialog := &DialogComponent{
		BaseComponent: NewBaseComponent("test", "test-view", guiCommon),
	}

	// Test internal view name generation
	viewName := dialog.getInternalViewName("categories")
	expected := "test-view-categories"
	
	if viewName != expected {
		t.Errorf("Expected internal view name %s, got %s", expected, viewName)
	}
}

func TestDialogComponent_CloseKeybindings(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}
	dialog := &DialogComponent{
		BaseComponent: NewBaseComponent("test", "test-view", guiCommon),
	}

	// Test close keybindings
	keybindings := dialog.GetCloseKeybindings()
	
	if len(keybindings) != 2 {
		t.Errorf("Expected 2 close keybindings, got %d", len(keybindings))
	}

	// Check that we have ESC and 'q' keybindings
	hasEsc, hasQ := false, false
	for _, kb := range keybindings {
		if kb.Key == gocui.KeyEsc {
			hasEsc = true
		}
		if kb.Key == 'q' {
			hasQ = true
		}
		if kb.View != "test-view" {
			t.Errorf("Expected keybinding view to be 'test-view', got %s", kb.View)
		}
	}

	if !hasEsc {
		t.Error("Missing ESC keybinding for dialog close")
	}
	if !hasQ {
		t.Error("Missing 'q' keybinding for dialog close")
	}
}

func TestDialogBounds_BasicCalculation(t *testing.T) {
	// Test DialogBounds struct directly
	bounds := DialogBounds{
		X:      10,
		Y:      5,
		Width:  50,
		Height: 20,
	}

	if bounds.X != 10 {
		t.Errorf("Expected X=10, got %d", bounds.X)
	}
	if bounds.Y != 5 {
		t.Errorf("Expected Y=5, got %d", bounds.Y)
	}
	if bounds.Width != 50 {
		t.Errorf("Expected Width=50, got %d", bounds.Width)
	}
	if bounds.Height != 20 {
		t.Errorf("Expected Height=20, got %d", bounds.Height)
	}
}