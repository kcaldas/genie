package component

import (
	"testing"

	"github.com/awesome-gocui/gocui"
	"github.com/jesseduffield/lazycore/pkg/boxlayout"
	"github.com/kcaldas/genie/cmd/tui/types"
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
	}

	layout := &boxlayout.Box{Direction: boxlayout.COLUMN}
	dialog.SetInternalLayout(layout)

	if dialog.internalLayout != layout {
		t.Error("Internal layout not set")
	}
}

func TestDialogComponent_VisibilityStates(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}
	dialog := &DialogComponent{
		BaseComponent: NewBaseComponent("test", "test-view", guiCommon),
		isVisible:     false,
	}

	if dialog.IsVisible() {
		t.Error("Dialog should not be visible initially")
	}

	dialog.isVisible = true
	if !dialog.IsVisible() {
		t.Error("Dialog should be visible when set")
	}
}

func TestDialogComponent_CloseKeybindings(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}
	dialog := &DialogComponent{
		BaseComponent: NewBaseComponent("test", "test-view", guiCommon),
	}

	keybindings := dialog.GetCloseKeybindings()
	if len(keybindings) != 2 {
		t.Errorf("Expected 2 keybindings, got %d", len(keybindings))
	}

	hasEsc, hasQ := false, false
	for _, kb := range keybindings {
		if kb.Key == gocui.KeyEsc {
			hasEsc = true
		}
		if kb.Key == 'q' {
			hasQ = true
		}
	}

	if !hasEsc || !hasQ {
		t.Error("Missing ESC or Q keybinding")
	}
}

