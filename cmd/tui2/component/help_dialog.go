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

// HelpDialogComponent provides a comprehensive help system with dynamic categories
// and keyboard shortcuts. It displays commands organized by category with Unix-style
// usage information and maintains a cursor-free clean appearance.
type HelpDialogComponent struct {
	*DialogComponent
	commandHandler     *controllers.SlashCommandHandler
	selectedCategory   int
	categories         []string                             // Dynamically generated from registry
	commandsByCategory map[string][]*controllers.CommandWrapper   // Commands grouped by category
}

func NewHelpDialogComponent(guiCommon types.IGuiCommon, commandHandler *controllers.SlashCommandHandler, onClose func() error) *HelpDialogComponent {
	dialog := NewDialogComponent("help-dialog", "help-dialog", guiCommon, onClose)
	dialog.SetTitle(" Help ")

	// Get command data dynamically from registry
	registry := commandHandler.GetRegistry()
	commandsByCategory := registry.GetCommandsByCategory()
	
	// Get categories dynamically from registry
	var categories []string
	for category := range commandsByCategory {
		categories = append(categories, category)
	}
	// Sort categories alphabetically for consistent ordering
	sort.Strings(categories)

	component := &HelpDialogComponent{
		DialogComponent:    dialog,
		commandHandler:     commandHandler,
		selectedCategory:   0,
		categories:         categories,
		commandsByCategory: commandsByCategory,
	}

	// Set up internal layout using boxlayout
	component.setupInternalLayout()

	return component
}

// setupInternalLayout configures a 3-panel layout:
// - Left: Categories list with arrow selection indicators
// - Right: Command/shortcuts content 
// - Bottom: Navigation tips panel
// All panels are non-editable with highlighting disabled for clean appearance.
func (h *HelpDialogComponent) setupInternalLayout() {
	layout := &boxlayout.Box{
		Direction: boxlayout.ROW, // Vertical split (top to bottom)
		Children: []*boxlayout.Box{
			{
				Direction: boxlayout.COLUMN, // Horizontal split for main content
				Weight:    1,                // Takes most space
				Children: []*boxlayout.Box{
					{
						Window: "categories", // Left panel: category list
						Size:   22,          // Fixed width for categories
					},
					{
						Window: "content", // Right panel: commands/shortcuts
						Weight: 1,         // Takes remaining horizontal space
					},
				},
			},
			{
				Window: "tips", // Bottom panel: navigation tips
				Size:   4,      // Fixed height for tips
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
			Key:     'k',
			Mod:     gocui.ModNone,
			Handler: h.handleUp,
		},
		{
			View:    h.viewName,
			Key:     'j',
			Mod:     gocui.ModNone,
			Handler: h.handleDown,
		},
		{
			View:    h.viewName,
			Key:     gocui.KeyEnter,
			Mod:     gocui.ModNone,
			Handler: h.handleSelect,
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
				Key:     'k',
				Mod:     gocui.ModNone,
				Handler: h.handleUp,
			},
			{
				View:    viewName,
				Key:     'j',
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
	// Category selection - could be expanded for future functionality
	return nil
}


func (h *HelpDialogComponent) getInternalViewName(windowName string) string {
	return h.viewName + "-" + windowName
}

// Show displays the help dialog with cursor disabled for clean appearance.
// The cursor is globally disabled while the dialog is open to prevent visual
// distractions, since internal views are non-editable and use arrow indicators.
func (h *HelpDialogComponent) Show() error {
	// Calculate dialog bounds (80% of screen, with min/max limits)
	bounds := h.CalculateDialogBounds(80, 80, 60, 20, 120, 40)
	err := h.DialogComponent.Show(bounds)
	if err != nil {
		return err
	}
	
	// Hide cursor globally for clean appearance since dialog is non-interactive
	// This prevents cursor artifacts in internal views while maintaining navigation
	gui := h.BaseComponent.gui.GetGui()
	gui.Update(func(g *gocui.Gui) error {
		g.Cursor = false // Disable cursor globally while dialog is open
		return nil
	})
	
	return nil
}

// Close restores the cursor and closes the help dialog.
// This ensures the cursor is re-enabled for normal application use after
// the dialog is dismissed via ESC, 'q', or other close triggers.
func (h *HelpDialogComponent) Close() error {
	// Restore cursor for normal application use
	gui := h.BaseComponent.gui.GetGui()
	gui.Update(func(g *gocui.Gui) error {
		g.Cursor = true // Re-enable cursor globally
		return nil
	})
	
	// Call parent close to handle view cleanup and callbacks
	return h.DialogComponent.Close()
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
	view.Highlight = false // Disable highlighting
	view.Editable = false  // Disable editing
	
	// Render categories with arrow indicator
	for i, category := range h.categories {
		if i == h.selectedCategory {
			fmt.Fprintf(view, "► %-20s\n", category)
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
	view.Highlight = false // Disable highlighting
	view.Editable = false  // No cursor needed
	
	selectedCat := h.categories[h.selectedCategory]
	
	return h.renderCommandsContent(view, selectedCat)
}


func (h *HelpDialogComponent) renderCommandsContent(view *gocui.View, category string) error {
	commands, exists := h.commandsByCategory[category]
	if !exists || len(commands) == 0 {
		fmt.Fprintln(view, "No commands in this category.")
		return nil
	}
	
	for _, cmd := range commands {
		// Command name and aliases
		fmt.Fprintf(view, ":%s", cmd.GetName())
		if len(cmd.GetAliases()) > 0 {
			aliasStr := strings.Join(cmd.GetAliases(), ", ")
			fmt.Fprintf(view, " (%s)", aliasStr)
		}
		fmt.Fprintln(view, "")
		
		// Description with indentation
		if cmd.GetDescription() != "" {
			fmt.Fprintf(view, "  %s\n", cmd.GetDescription())
		}
		
		// Usage with indentation (Unix style)
		if cmd.GetUsage() != "" {
			fmt.Fprintf(view, "  Usage: %s\n", cmd.GetUsage())
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
	view.Highlight = false // Disable highlighting
	view.Editable = false  // No cursor needed
	
	// Simple navigation tips
	fmt.Fprintln(view, "↑↓ Navigate | Enter Select | h/Tab Shortcuts | Esc/q Close")
	
	return nil
}

// SelectCategory allows external code to jump to a specific category
func (h *HelpDialogComponent) SelectCategory(categoryName string) {
	for i, cat := range h.categories {
		if strings.EqualFold(cat, categoryName) {
			h.selectedCategory = i
			break
		}
	}
}