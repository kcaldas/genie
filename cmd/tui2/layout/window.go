package layout

import (
	"github.com/awesome-gocui/gocui"
	"github.com/jesseduffield/lazycore/pkg/boxlayout"
)

type WindowManager struct {
	gui     *gocui.Gui
	windows map[string]*Window
}

type Window struct {
	Name       string
	Dimensions boxlayout.Dimensions
	Views      []*gocui.View
}

func NewWindowManager(gui *gocui.Gui) *WindowManager {
	return &WindowManager{
		gui:     gui,
		windows: make(map[string]*Window),
	}
}

func (wm *WindowManager) CreateWindow(name string, dims boxlayout.Dimensions) *Window {
	window := &Window{
		Name:       name,
		Dimensions: dims,
		Views:      []*gocui.View{},
	}
	
	wm.windows[name] = window
	return window
}

func (wm *WindowManager) GetWindow(name string) *Window {
	return wm.windows[name]
}

func (wm *WindowManager) UpdateWindowDimensions(name string, dims boxlayout.Dimensions) error {
	window := wm.windows[name]
	if window == nil {
		return nil
	}
	
	window.Dimensions = dims
	
	if len(window.Views) > 0 {
		view := window.Views[0]
		if view != nil {
			return wm.updateViewDimensions(view, dims)
		}
	}
	
	return nil
}

func (wm *WindowManager) updateViewDimensions(view *gocui.View, dims boxlayout.Dimensions) error {
	_, err := wm.gui.SetView(view.Name(), dims.X0, dims.Y0, dims.X1-1, dims.Y1, 0)
	return err
}

func (wm *WindowManager) CreateOrUpdateView(windowName, viewName string) (*gocui.View, error) {
	window := wm.windows[windowName]
	if window == nil {
		return nil, nil
	}
	
	dims := window.Dimensions
	view, err := wm.gui.SetView(viewName, dims.X0, dims.Y0, dims.X1-1, dims.Y1, 0)
	if err != nil && err != gocui.ErrUnknownView {
		return nil, err
	}
	
	if err == gocui.ErrUnknownView {
		window.Views = append(window.Views, view)
	}
	
	return view, nil
}



