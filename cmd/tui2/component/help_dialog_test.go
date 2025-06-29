package component

import (
	"testing"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui2/controllers"
)

func TestHelpDialogComponent_CategorySelection(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}
	commandHandler := controllers.NewSlashCommandHandler()
	
	// Add some test commands
	testCommands := []*controllers.Command{
		{
			Name:        "test",
			Description: "Test command",
			Category:    "General",
			Handler:     func([]string) error { return nil },
		},
		{
			Name:        "debug",
			Description: "Debug command",
			Category:    "Debug",
			Handler:     func([]string) error { return nil },
		},
	}
	
	for _, cmd := range testCommands {
		commandHandler.RegisterCommandWithMetadata(cmd)
	}

	dialog := &HelpDialogComponent{
		DialogComponent:    NewDialogComponent("help-dialog", "help-dialog", guiCommon, nil),
		commandHandler:     commandHandler,
		selectedCategory:   0,
		categories:         []string{"General", "Chat", "Configuration", "Navigation", "Layout", "Debug", "Shortcuts"},
		commandsByCategory: commandHandler.GetRegistry().GetCommandsByCategory(),
		showingShortcuts:   false,
	}

	// Test selecting a specific category
	dialog.SelectCategory("Debug")
	
	// Find expected index for Debug category
	debugIndex := -1
	for i, cat := range dialog.categories {
		if cat == "Debug" {
			debugIndex = i
			break
		}
	}

	if debugIndex == -1 {
		t.Fatal("Debug category not found")
	}

	if dialog.selectedCategory != debugIndex {
		t.Errorf("Expected selected category index %d, got %d", debugIndex, dialog.selectedCategory)
	}

	// Test case insensitive selection
	dialog.SelectCategory("shortcuts")
	
	shortcutsIndex := -1
	for i, cat := range dialog.categories {
		if cat == "Shortcuts" {
			shortcutsIndex = i
			break
		}
	}

	if shortcutsIndex == -1 {
		t.Fatal("Shortcuts category not found")
	}

	if dialog.selectedCategory != shortcutsIndex {
		t.Errorf("Expected selected category index %d for case insensitive match, got %d", shortcutsIndex, dialog.selectedCategory)
	}
}

func TestHelpDialogComponent_NavigationHandlers(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}
	commandHandler := controllers.NewSlashCommandHandler()
	
	dialog := &HelpDialogComponent{
		DialogComponent:    NewDialogComponent("help-dialog", "help-dialog", guiCommon, nil),
		commandHandler:     commandHandler,
		selectedCategory:   0,
		categories:         []string{"General", "Chat", "Configuration", "Navigation", "Layout", "Debug", "Shortcuts"},
		commandsByCategory: commandHandler.GetRegistry().GetCommandsByCategory(),
		showingShortcuts:   false,
	}

	// Test initial state
	initialCategory := dialog.selectedCategory

	// Test handleDown
	err := dialog.handleDown(nil, nil)
	if err != nil {
		t.Errorf("handleDown failed: %v", err)
	}

	if dialog.selectedCategory != initialCategory+1 {
		t.Errorf("Expected category to increase by 1, got %d", dialog.selectedCategory)
	}

	// Test handleUp
	err = dialog.handleUp(nil, nil)
	if err != nil {
		t.Errorf("handleUp failed: %v", err)
	}

	if dialog.selectedCategory != initialCategory {
		t.Errorf("Expected category to return to initial value %d, got %d", initialCategory, dialog.selectedCategory)
	}

	// Test handleUp at boundary (should not go below 0)
	dialog.selectedCategory = 0
	err = dialog.handleUp(nil, nil)
	if err != nil {
		t.Errorf("handleUp at boundary failed: %v", err)
	}

	if dialog.selectedCategory != 0 {
		t.Errorf("Category should stay at 0 when at upper boundary, got %d", dialog.selectedCategory)
	}

	// Test handleDown at boundary (should not exceed categories length)
	dialog.selectedCategory = len(dialog.categories) - 1
	err = dialog.handleDown(nil, nil)
	if err != nil {
		t.Errorf("handleDown at boundary failed: %v", err)
	}

	if dialog.selectedCategory != len(dialog.categories)-1 {
		t.Errorf("Category should stay at max when at lower boundary, got %d", dialog.selectedCategory)
	}
}

func TestHelpDialogComponent_ShortcutsToggle(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}
	commandHandler := controllers.NewSlashCommandHandler()
	
	dialog := &HelpDialogComponent{
		DialogComponent:    NewDialogComponent("help-dialog", "help-dialog", guiCommon, nil),
		commandHandler:     commandHandler,
		selectedCategory:   0,
		categories:         []string{"General", "Chat", "Configuration", "Navigation", "Layout", "Debug", "Shortcuts"},
		commandsByCategory: commandHandler.GetRegistry().GetCommandsByCategory(),
		showingShortcuts:   false,
	}

	// Initially should not be showing shortcuts
	if dialog.showingShortcuts {
		t.Error("Should not be showing shortcuts initially")
	}

	// Toggle shortcuts
	err := dialog.handleToggleShortcuts(nil, nil)
	if err != nil {
		t.Errorf("handleToggleShortcuts failed: %v", err)
	}

	if !dialog.showingShortcuts {
		t.Error("Should be showing shortcuts after toggle")
	}

	// Check that it switched to shortcuts category
	shortcutsIndex := -1
	for i, cat := range dialog.categories {
		if cat == "Shortcuts" {
			shortcutsIndex = i
			break
		}
	}

	if shortcutsIndex != -1 && dialog.selectedCategory != shortcutsIndex {
		t.Errorf("Expected to switch to shortcuts category %d, got %d", shortcutsIndex, dialog.selectedCategory)
	}

	// Toggle back
	err = dialog.handleToggleShortcuts(nil, nil)
	if err != nil {
		t.Errorf("handleToggleShortcuts back failed: %v", err)
	}

	if dialog.showingShortcuts {
		t.Error("Should not be showing shortcuts after toggle back")
	}
}

func TestHelpDialogComponent_KeybindingsSetup(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}
	commandHandler := controllers.NewSlashCommandHandler()
	
	dialog := &HelpDialogComponent{
		DialogComponent:    NewDialogComponent("help-dialog", "help-dialog", guiCommon, nil),
		commandHandler:     commandHandler,
		selectedCategory:   0,
		categories:         []string{"General", "Chat", "Configuration", "Navigation", "Layout", "Debug", "Shortcuts"},
		commandsByCategory: commandHandler.GetRegistry().GetCommandsByCategory(),
		showingShortcuts:   false,
	}

	// Test that keybindings are set up
	keybindings := dialog.GetKeybindings()
	
	if len(keybindings) == 0 {
		t.Error("Help dialog should have keybindings set up")
	}

	// Check for essential keybindings
	hasArrowKeys := false
	hasToggleShortcuts := false
	hasClose := false

	for _, kb := range keybindings {
		switch kb.Key {
		case gocui.KeyArrowUp, gocui.KeyArrowDown:
			hasArrowKeys = true
		case 'h', gocui.KeyTab:
			hasToggleShortcuts = true
		case gocui.KeyEsc, 'q':
			hasClose = true
		}
	}

	if !hasArrowKeys {
		t.Error("Missing arrow key bindings for navigation")
	}
	if !hasToggleShortcuts {
		t.Error("Missing toggle shortcuts keybindings")
	}
	if !hasClose {
		t.Error("Missing close keybindings")
	}
}

func TestHelpDialogComponent_BasicFunctionality(t *testing.T) {
	guiCommon := &mockDialogGuiCommon{}
	commandHandler := controllers.NewSlashCommandHandler()

	// Create dialog using the constructor
	dialog := NewHelpDialogComponent(guiCommon, commandHandler, nil)

	if dialog == nil {
		t.Fatal("Failed to create help dialog component")
	}

	// Test that it has the expected categories
	expectedCategories := []string{"General", "Chat", "Configuration", "Navigation", "Layout", "Debug", "Shortcuts"}
	if len(dialog.categories) != len(expectedCategories) {
		t.Errorf("Expected %d categories, got %d", len(expectedCategories), len(dialog.categories))
	}

	// Test that internal layout is set up
	if dialog.internalLayout == nil {
		t.Error("Internal layout should be set up")
	}

	// Test initial state
	if dialog.selectedCategory != 0 {
		t.Errorf("Initial selected category should be 0, got %d", dialog.selectedCategory)
	}

	if dialog.showingShortcuts {
		t.Error("Should not be showing shortcuts initially")
	}
}