package component

import (
	"testing"

	"github.com/kcaldas/genie/cmd/tui2/controllers"
)

func TestHelpDialogComponent_CategorySelection(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}

	dialog := &HelpDialogComponent{
		DialogComponent: NewDialogComponent("help-dialog", "help-dialog", guiCommon, nil),
		categories:      []string{"General", "Debug"},
	}

	dialog.SelectCategory("Debug")
	if dialog.selectedCategory != 1 {
		t.Errorf("Expected Debug category index 1, got %d", dialog.selectedCategory)
	}

	dialog.SelectCategory("general") // case insensitive
	if dialog.selectedCategory != 0 {
		t.Errorf("Expected General category index 0, got %d", dialog.selectedCategory)
	}
}

func TestHelpDialogComponent_NavigationHandlers(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}
	dialog := &HelpDialogComponent{
		DialogComponent:  NewDialogComponent("help-dialog", "help-dialog", guiCommon, nil),
		selectedCategory: 1,
		categories:       []string{"General", "Debug"},
	}

	// Test navigation
	dialog.handleDown(nil, nil)
	if dialog.selectedCategory != 1 {
		t.Errorf("Expected category 1, got %d", dialog.selectedCategory)
	}

	dialog.handleUp(nil, nil)
	if dialog.selectedCategory != 0 {
		t.Errorf("Expected category 0, got %d", dialog.selectedCategory)
	}

	// Test boundaries
	dialog.selectedCategory = 0
	dialog.handleUp(nil, nil)
	if dialog.selectedCategory != 0 {
		t.Errorf("Should stay at 0, got %d", dialog.selectedCategory)
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
}