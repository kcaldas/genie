package layout

import (
	"github.com/awesome-gocui/gocui"
	"github.com/jesseduffield/lazycore/pkg/boxlayout"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

type LayoutManager struct {
	windowManager *WindowManager
	screenManager *ScreenManager
	config        *LayoutConfig
	gui           *gocui.Gui
}

type LayoutConfig struct {
	ChatPanelWidth    float64
	ShowSidebar       bool
	CompactMode       bool
	ResponsePanelMode string
	MinPanelWidth     int
	MinPanelHeight    int
}

type LayoutArgs struct {
	Width         int
	Height        int
	FocusedWindow string
	ScreenMode    ScreenMode
	Config        *LayoutConfig
}

func NewLayoutManager(gui *gocui.Gui, config *LayoutConfig) *LayoutManager {
	return &LayoutManager{
		windowManager: NewWindowManager(gui),
		screenManager: NewScreenManager(),
		config:        config,
		gui:           gui,
	}
}

func (lm *LayoutManager) GetDefaultConfig() *LayoutConfig {
	return &LayoutConfig{
		ChatPanelWidth:    0.7,
		ShowSidebar:       true,
		CompactMode:       false,
		ResponsePanelMode: "split",
		MinPanelWidth:     20,
		MinPanelHeight:    3,
	}
}

func (lm *LayoutManager) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	lm.screenManager.SetDimensions(maxX, maxY)
	
	args := LayoutArgs{
		Width:         maxX,
		Height:        maxY,
		FocusedWindow: lm.screenManager.GetFocusedWindow(),
		ScreenMode:    lm.screenManager.GetMode(),
		Config:        lm.config,
	}
	
	rootBox := lm.buildLayoutTree(args)
	windowDimensions := boxlayout.ArrangeWindows(rootBox, 0, 0, maxX, maxY)
	
	for windowName, dims := range windowDimensions {
		lm.windowManager.UpdateWindowDimensions(windowName, dims)
	}
	
	return lm.createViews(args)
}

func (lm *LayoutManager) buildLayoutTree(args LayoutArgs) *boxlayout.Box {
	return &boxlayout.Box{
		Direction: boxlayout.ROW,
		Children: []*boxlayout.Box{
			{
				Direction: boxlayout.COLUMN,
				Weight:    1,
				ConditionalChildren: func(width, height int) []*boxlayout.Box {
					return lm.getMainLayoutChildren(args)
				},
			},
			{
				Direction: boxlayout.COLUMN,
				Size:      lm.getStatusBarHeight(args),
				Window:    "status",
			},
		},
	}
}

func (lm *LayoutManager) getMainLayoutChildren(args LayoutArgs) []*boxlayout.Box {
	if lm.screenManager.IsPortraitMode() {
		return lm.getPortraitLayout(args)
	}
	return lm.getLandscapeLayout(args)
}

func (lm *LayoutManager) getLandscapeLayout(args LayoutArgs) []*boxlayout.Box {
	sideWeight := lm.screenManager.GetSidePanelWeight()
	mainWeight := lm.screenManager.GetMainPanelWeight()
	
	children := []*boxlayout.Box{}
	
	if sideWeight > 0 && args.Config.ShowSidebar {
		children = append(children, &boxlayout.Box{
			Direction: boxlayout.ROW,
			Weight:    sideWeight,
			ConditionalChildren: func(width, height int) []*boxlayout.Box {
				return lm.getSidePanelChildren(args)
			},
		})
	}
	
	children = append(children, &boxlayout.Box{
		Direction: boxlayout.ROW,
		Weight:    mainWeight,
		ConditionalChildren: func(width, height int) []*boxlayout.Box {
			return lm.getMainPanelChildren(args)
		},
	})
	
	return children
}

func (lm *LayoutManager) getPortraitLayout(args LayoutArgs) []*boxlayout.Box {
	return []*boxlayout.Box{
		{
			Direction: boxlayout.ROW,
			Weight:    3,
			Window:    "messages",
		},
		{
			Direction: boxlayout.ROW,
			Size:      3,
			Window:    "input",
		},
		{
			Direction: boxlayout.ROW,
			Weight:    1,
			ConditionalChildren: func(width, height int) []*boxlayout.Box {
				if height > 30 {
					return []*boxlayout.Box{{Window: "debug"}}
				}
				return []*boxlayout.Box{}
			},
		},
	}
}

func (lm *LayoutManager) getSidePanelChildren(args LayoutArgs) []*boxlayout.Box {
	if lm.screenManager.IsCompactMode() {
		return []*boxlayout.Box{}
	}
	
	children := []*boxlayout.Box{}
	
	if args.Config.ShowSidebar {
		children = append(children, &boxlayout.Box{
			Window: "sidebar",
			Weight: 1,
		})
	}
	
	return children
}

func (lm *LayoutManager) getMainPanelChildren(args LayoutArgs) []*boxlayout.Box {
	inputHeight := lm.getInputHeight(args)
	
	children := []*boxlayout.Box{
		{
			Window:  "messages",
			Weight:  1,
		},
		{
			Window:  "input",
			Size:    inputHeight,
		},
	}
	
	if lm.shouldShowDebugPanel(args) {
		children = append(children, &boxlayout.Box{
			Window:  "debug",
			Weight:  1,
		})
	}
	
	return children
}

func (lm *LayoutManager) getInputHeight(args LayoutArgs) int {
	if args.Config.CompactMode || lm.screenManager.IsCompactMode() {
		return 3
	}
	return 4
}

func (lm *LayoutManager) getStatusBarHeight(args LayoutArgs) int {
	if lm.screenManager.IsCompactMode() {
		return 1
	}
	return 2
}

func (lm *LayoutManager) shouldShowDebugPanel(args LayoutArgs) bool {
	return args.Height > 25
}

func (lm *LayoutManager) createViews(args LayoutArgs) error {
	windowNames := []string{"messages", "input", "status"}
	
	if args.Config.ShowSidebar && !lm.screenManager.ShouldHideSidePanels() {
		windowNames = append(windowNames, "sidebar")
	}
	
	if lm.shouldShowDebugPanel(args) {
		windowNames = append(windowNames, "debug")
	}
	
	for _, windowName := range windowNames {
		if _, err := lm.windowManager.CreateOrUpdateView(windowName, windowName); err != nil {
			return err
		}
	}
	
	return nil
}

func (lm *LayoutManager) GetWindowManager() *WindowManager {
	return lm.windowManager
}

func (lm *LayoutManager) GetScreenManager() *ScreenManager {
	return lm.screenManager
}

func (lm *LayoutManager) SetConfig(config *LayoutConfig) {
	lm.config = config
}

func (lm *LayoutManager) GetConfig() *LayoutConfig {
	return lm.config
}

func (lm *LayoutManager) SetWindowComponent(windowName string, ctx types.Component) {
	lm.windowManager.SetWindowComponent(windowName, ctx)
}

func (lm *LayoutManager) GetWindowComponent(windowName string) types.Component {
	return lm.windowManager.GetWindowComponent(windowName)
}

func (lm *LayoutManager) SetFocus(windowName string) {
	lm.screenManager.SetFocusedWindow(windowName)
	if window := lm.windowManager.GetWindow(windowName); window != nil {
		if len(window.Views) > 0 && window.Views[0] != nil {
			view := window.Views[0]
			lm.gui.SetCurrentView(view.Name())
			// Set highlight if the component supports it
			if window.Component != nil {
				props := window.Component.GetWindowProperties()
				if props.Highlight {
					view.Highlight = true
				}
			}
		}
	}
}

func (lm *LayoutManager) ToggleScreenMode() {
	lm.screenManager.ToggleMode()
}