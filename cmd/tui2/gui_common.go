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