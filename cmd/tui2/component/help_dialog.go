package component

import (
	"fmt"
	"sort"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/jesseduffield/lazycore/pkg/boxlayout"
	"github.com/kcaldas/genie/cmd/tui2/controllers"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

type HelpDialogComponent struct {
	*DialogComponent
	commandHandler     *controllers.SlashCommandHandler
	selectedCategory   int
	categories         []string
	commandsByCategory map[string][]*controllers.Command
	showingShortcuts   bool
}

func NewHelpDialogComponent(guiCommon types.IGuiCommon, commandHandler *controllers.SlashCommandHandler, onClose func() error) *HelpDialogComponent {
	dialog := NewDialogComponent("help-dialog", "help-dialog", guiCommon, onClose)
	dialog.SetTitle(" Help ")

	// Get command data dynamically from registry
	registry := commandHandler.GetRegistry()
	commandsByCategory := registry.GetCommandsByCategory()
	
	// Get categories dynamically from registry, with "Shortcuts" always first
	categories := []string{"Shortcuts"} // Start with Shortcuts as first category
	for category := range commandsByCategory {
		if category != "Shortcuts" {
			categories = append(categories, category)
		}
	}
	// Sort the non-Shortcuts categories alphabetically for consistent ordering
	otherCategories := categories[1:] // Skip "Shortcuts" which is at index 0
	sort.Strings(otherCategories)
	categories = append([]string{"Shortcuts"}, otherCategories...)

	component := &HelpDialogComponent{
		DialogComponent:    dialog,
		commandHandler:     commandHandler,
		selectedCategory:   0,
		categories:         categories,
		commandsByCategory: commandsByCategory,
		showingShortcuts:   false,
	}

	// Set up internal layout using boxlayout
	component.setupInternalLayout()

	return component
}

func (h *HelpDialogComponent) setupInternalLayout() {
	layout := &boxlayout.Box{
		Direction: boxlayout.ROW, // Vertical split (top to bottom)
		Children: []*boxlayout.Box{
			{
				Direction: boxlayout.COLUMN, // Horizontal split for main content
				Weight:    1,                // Takes most space
				Children: []*boxlayout.Box{
					{
						Window: "categories", // Left panel
						Size:   22,          // Width for categories
					},
					{
						Window: "content", // Right panel
						Weight: 1,         // Takes remaining horizontal space
					},
				},
			},
			{
				Window: "tips", // Bottom panel for navigation tips
				Size:   4,      // Height for tips panel
			},
		},
	}
	
	h.SetInternalLayout(layout)
}

func (h *HelpDialogComponent) GetKeybindings() []*types.KeyBinding {
	// Start with base dialog keybindings (Esc, q)
	keybindings := h.GetCloseKeybindings()
	
	// Add help-specific keybindings for main dialog view
	helpBindings := []*types.KeyBinding{
		{
			View:    h.viewName,
			Key:     gocui.KeyArrowUp,
			Mod:     gocui.ModNone,
			Handler: h.handleUp,
		},
		{
			View:    h.viewName,
			Key:     gocui.KeyArrowDown,
			Mod:     gocui.ModNone,
			Handler: h.handleDown,
		},
		{
			View:    h.viewName,
			Key:     gocui.KeyEnter,
			Mod:     gocui.ModNone,
			Handler: h.handleSelect,
		},
		{
			View:    h.viewName,
			Key:     'h',
			Mod:     gocui.ModNone,
			Handler: h.handleToggleShortcuts,
		},
		{
			View:    h.viewName,
			Key:     gocui.KeyTab,
			Mod:     gocui.ModNone,
			Handler: h.handleToggleShortcuts,
		},
	}
	
	// Also bind to internal views for better focus handling
	internalViews := []string{
		h.getInternalViewName("categories"),
		h.getInternalViewName("content"),
		h.getInternalViewName("tips"),
	}
	
	for _, viewName := range internalViews {
		helpBindings = append(helpBindings, []*types.KeyBinding{
			{
				View:    viewName,
				Key:     gocui.KeyArrowUp,
				Mod:     gocui.ModNone,
				Handler: h.handleUp,
			},
			{
				View:    viewName,
				Key:     gocui.KeyArrowDown,
				Mod:     gocui.ModNone,
				Handler: h.handleDown,
			},
			{
				View:    viewName,
				Key:     gocui.KeyEnter,
				Mod:     gocui.ModNone,
				Handler: h.handleSelect,
			},
			{
				View:    viewName,
				Key:     'h',
				Mod:     gocui.ModNone,
				Handler: h.handleToggleShortcuts,
			},
			{
				View:    viewName,
				Key:     gocui.KeyEsc,
				Mod:     gocui.ModNone,
				Handler: func(g *gocui.Gui, v *gocui.View) error { return h.Close() },
			},
			{
				View:    viewName,
				Key:     'q',
				Mod:     gocui.ModNone,
				Handler: func(g *gocui.Gui, v *gocui.View) error { return h.Close() },
			},
		}...)
	}
	
	return append(keybindings, helpBindings...)
}

func (h *HelpDialogComponent) handleUp(g *gocui.Gui, v *gocui.View) error {
	if h.selectedCategory > 0 {
		h.selectedCategory--
		return h.Render()
	}
	return nil
}

func (h *HelpDialogComponent) handleDown(g *gocui.Gui, v *gocui.View) error {
	if h.selectedCategory < len(h.categories)-1 {
		h.selectedCategory++
		return h.Render()
	}
	return nil
}

func (h *HelpDialogComponent) handleSelect(g *gocui.Gui, v *gocui.View) error {
	// Toggle to shortcuts view if "Shortcuts" is selected
	if h.categories[h.selectedCategory] == "Shortcuts" {
		h.showingShortcuts = true
		return h.Render()
	}
	return nil
}

func (h *HelpDialogComponent) handleToggleShortcuts(g *gocui.Gui, v *gocui.View) error {
	h.showingShortcuts = !h.showingShortcuts
	if h.showingShortcuts {
		// Set to shortcuts category
		for i, cat := range h.categories {
			if cat == "Shortcuts" {
				h.selectedCategory = i
				break
			}
		}
	}
	return h.Render()
}

func (h *HelpDialogComponent) getInternalViewName(windowName string) string {
	return h.viewName + "-" + windowName
}

func (h *HelpDialogComponent) Show() error {
	// Calculate dialog bounds (80% of screen, with min/max limits)
	bounds := h.CalculateDialogBounds(80, 80, 60, 20, 120, 40)
	return h.DialogComponent.Show(bounds)
}

func (h *HelpDialogComponent) Render() error {
	// Render left panel (categories)
	if err := h.renderCategoriesPanel(); err != nil {
		return err
	}
	
	// Render right panel (content)
	if err := h.renderContentPanel(); err != nil {
		return err
	}
	
	// Render bottom panel (navigation tips)
	if err := h.renderTipsPanel(); err != nil {
		return err
	}
	
	return nil
}

func (h *HelpDialogComponent) renderCategoriesPanel() error {
	view := h.GetInternalView("categories")
	if view == nil {
		return nil
	}
	
	view.Clear()
	
	// Title with visual boundaries to help debug positioning
	fmt.Fprintln(view, "Categories")
	fmt.Fprintln(view, strings.Repeat("-", 20))
	
	// Render categories with selection highlight
	for i, category := range h.categories {
		if i == h.selectedCategory {
			// Highlight selected category with visual indicator
			fmt.Fprintf(view, "\033[7m► %-19s\033[0m\n", category)
		} else {
			fmt.Fprintf(view, "  %-20s\n", category)
		}
	}
	
	return nil
}

func (h *HelpDialogComponent) renderContentPanel() error {
	view := h.GetInternalView("content")
	if view == nil {
		return nil
	}
	
	view.Clear()
	
	selectedCat := h.categories[h.selectedCategory]
	
	if selectedCat == "Shortcuts" || h.showingShortcuts {
		return h.renderShortcutsContent(view)
	} else {
		return h.renderCommandsContent(view, selectedCat)
	}
}

func (h *HelpDialogComponent) renderShortcutsContent(view *gocui.View) error {
	shortcuts := []struct {
		key         string
		description string
	}{
		{"Tab", "Switch between panels"},
		{"Ctrl+C", "Exit application"},
		{"↑↓", "Navigate in panels / categories"},
		{"PgUp/PgDn", "Scroll messages"},
		{"Enter", "Select category"},
		{"h / Tab", "Toggle shortcuts view"},
		{"y", "Copy selected message (in messages)"},
		{"Y", "Copy all messages (in messages)"},
		{"Esc / q", "Close dialogs"},
	}

	fmt.Fprintln(view, "Keyboard Shortcuts")
	fmt.Fprintln(view, strings.Repeat("=", 30))
	fmt.Fprintln(view, "")
	
	for _, shortcut := range shortcuts {
		fmt.Fprintf(view, "%-12s %s\n", shortcut.key, shortcut.description)
	}
	
	return nil
}

func (h *HelpDialogComponent) renderCommandsContent(view *gocui.View, category string) error {
	commands, exists := h.commandsByCategory[category]
	if !exists || len(commands) == 0 {
		fmt.Fprintf(view, "%s Commands\n", category)
		fmt.Fprintln(view, strings.Repeat("=", len(category)+9))
		fmt.Fprintln(view, "")
		fmt.Fprintln(view, "No commands in this category.")
		return nil
	}

	title := fmt.Sprintf("%s Commands", category)
	fmt.Fprintln(view, title)
	fmt.Fprintln(view, strings.Repeat("=", len(title)))
	fmt.Fprintln(view, "")
	
	for _, cmd := range commands {
		// Command name and aliases
		fmt.Fprintf(view, "/%s", cmd.Name)
		if len(cmd.Aliases) > 0 {
			aliasStr := strings.Join(cmd.Aliases, ", ")
			fmt.Fprintf(view, " (%s)", aliasStr)
		}
		fmt.Fprintln(view, "")
		
		// Description with indentation
		if cmd.Description != "" {
			fmt.Fprintf(view, "  %s\n", cmd.Description)
		}
		
		// Usage with indentation (Unix style)
		if cmd.Usage != "" {
			fmt.Fprintf(view, "  Usage: %s\n", cmd.Usage)
		}
		
		fmt.Fprintln(view, "")
	}
	
	return nil
}

func (h *HelpDialogComponent) renderTipsPanel() error {
	view := h.GetInternalView("tips")
	if view == nil {
		return nil
	}
	
	view.Clear()
	
	// Simple navigation tips
	fmt.Fprintln(view, "↑↓ Navigate | Enter Select | h/Tab Shortcuts | Esc/q Close")
	
	return nil
}

// SelectCategory allows external code to jump to a specific category
func (h *HelpDialogComponent) SelectCategory(categoryName string) {
	for i, cat := range h.categories {
		if strings.EqualFold(cat, categoryName) {
			h.selectedCategory = i
			h.showingShortcuts = false
			break
		}
	}
}