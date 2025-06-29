package tui2

import (
	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui2/presentation"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

type guiCommon struct {
	app *App
}

func (g *guiCommon) GetGui() *gocui.Gui {
	return g.app.gui
}

func (g *guiCommon) GetConfig() *types.Config {
	return g.app.uiState.GetConfig()
}

func (g *guiCommon) GetTheme() *types.Theme {
	themeName := g.GetConfig().Theme
	return presentation.GetTheme(themeName)
}

func (g *guiCommon) SetCurrentComponent(ctx types.Component) {
	g.app.setCurrentView(ctx.GetViewName())
}

func (g *guiCommon) GetCurrentComponent() types.Component {
	panel := g.app.uiState.GetFocusedPanel()
	return g.app.panelToComponent(panel)
}

func (g *guiCommon) PostUIUpdate(fn func()) {
	g.app.gui.Update(func(*gocui.Gui) error {
		fn()
		return nil
	})
}

// IPopupHandler implementation

func (g *guiCommon) ShowError(title, message string) error {
	return g.ShowConfirmation(title, message, nil)
}

func (g *guiCommon) ShowConfirmation(title, message string, onConfirm func() error) error {
	// Create confirmation dialog with default texts
	confirmText := "OK"
	cancelText := ""
	
	// If onConfirm is provided, show both buttons
	if onConfirm != nil {
		confirmText = "Yes"
		cancelText = "No"
	}
	
	onConfirmWrapper := func() error {
		if err := g.app.closeCurrentDialog(); err != nil {
			return err
		}
		if onConfirm != nil {
			return onConfirm()
		}
		return nil
	}
	
	onCancelWrapper := func() error {
		return g.app.closeCurrentDialog()
	}
	
	onCloseWrapper := func() error {
		return g.app.closeCurrentDialog()
	}
	
	// Show confirmation dialog
	return g.app.showConfirmationDialog(title, message, "", "", confirmText, cancelText, onConfirmWrapper, onCancelWrapper, onCloseWrapper)
}

func (g *guiCommon) ShowPrompt(title, initialValue string, onSubmit func(string) error) error {
	// TODO: Implement prompt dialog when needed
	// For now, show as error message
	return g.ShowError("Not Implemented", "Prompt dialogs not yet implemented")
}

func (g *guiCommon) ClosePopup() error {
	return g.app.closeCurrentDialog()
}