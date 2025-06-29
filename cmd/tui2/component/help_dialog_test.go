package component

import (
	"testing"

	"github.com/kcaldas/genie/cmd/tui2/controllers"
)

func TestHelpDialogComponent_CategorySelection(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}

	dialog := &HelpDialogComponent{
		DialogComponent: NewDialogComponent("help-dialog", "help-dialog", guiCommon, nil),
		categories:      []string{"General", "Debug", "Shortcuts"},
	}

	dialog.SelectCategory("Debug")
	if dialog.selectedCategory != 1 {
		t.Errorf("Expected Debug category index 1, got %d", dialog.selectedCategory)
	}

	dialog.SelectCategory("shortcuts") // case insensitive
	if dialog.selectedCategory != 2 {
		t.Errorf("Expected Shortcuts category index 2, got %d", dialog.selectedCategory)
	}
}

func TestHelpDialogComponent_NavigationHandlers(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}
	dialog := &HelpDialogComponent{
		DialogComponent:  NewDialogComponent("help-dialog", "help-dialog", guiCommon, nil),
		selectedCategory: 1,
		categories:       []string{"General", "Debug", "Shortcuts"},
	}

	// Test navigation
	dialog.handleDown(nil, nil)
	if dialog.selectedCategory != 2 {
		t.Errorf("Expected category 2, got %d", dialog.selectedCategory)
	}

	dialog.handleUp(nil, nil)
	if dialog.selectedCategory != 1 {
		t.Errorf("Expected category 1, got %d", dialog.selectedCategory)
	}

	// Test boundaries
	dialog.selectedCategory = 0
	dialog.handleUp(nil, nil)
	if dialog.selectedCategory != 0 {
		t.Errorf("Should stay at 0, got %d", dialog.selectedCategory)
	}
}

func TestHelpDialogComponent_ShortcutsToggle(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}
	dialog := &HelpDialogComponent{
		DialogComponent:  NewDialogComponent("help-dialog", "help-dialog", guiCommon, nil),
		categories:       []string{"General", "Debug", "Shortcuts"},
		showingShortcuts: false,
	}

	dialog.handleToggleShortcuts(nil, nil)
	if !dialog.showingShortcuts {
		t.Error("Should be showing shortcuts after toggle")
	}

	dialog.handleToggleShortcuts(nil, nil)
	if dialog.showingShortcuts {
		t.Error("Should not be showing shortcuts after toggle back")
	}
}


func TestHelpDialogComponent_BasicFunctionality(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}
	commandHandler := controllers.NewSlashCommandHandler()

	dialog := NewHelpDialogComponent(guiCommon, commandHandler, nil)
	if dialog == nil {
		t.Fatal("Failed to create dialog")
	}
	if dialog.selectedCategory != 0 {
		t.Errorf("Expected initial category 0, got %d", dialog.selectedCategory)
	}
	if dialog.showingShortcuts {
		t.Error("Should not show shortcuts initially")
	}
}