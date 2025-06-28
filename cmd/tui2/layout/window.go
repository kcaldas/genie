package layout

import (
	"github.com/awesome-gocui/gocui"
	"github.com/jesseduffield/lazycore/pkg/boxlayout"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

type WindowManager struct {
	gui       *gocui.Gui
	windows   map[string]*Window
	viewCache map[string]*gocui.View
}

type Window struct {
	Name       string
	Title      string
	Dimensions boxlayout.Dimensions
	Views      []*gocui.View
	Context    types.Component
	
	Focusable   bool
	Editable    bool
	Wrap        bool
	Autoscroll  bool
	Highlight   bool
	Frame       bool
}

func NewWindowManager(gui *gocui.Gui) *WindowManager {
	return &WindowManager{
		gui:       gui,
		windows:   make(map[string]*Window),
		viewCache: make(map[string]*gocui.View),
	}
}

func (wm *WindowManager) CreateWindow(name string, dims boxlayout.Dimensions) *Window {
	window := &Window{
		Name:       name,
		Dimensions: dims,
		Views:      []*gocui.View{},
		Focusable:  true,
		Frame:      true,
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
	_, err := wm.gui.SetView(view.Name(), dims.X0, dims.Y0, dims.X1-1, dims.Y1-1, 0)
	return err
}

func (wm *WindowManager) CreateOrUpdateView(windowName, viewName string) (*gocui.View, error) {
	window := wm.windows[windowName]
	if window == nil {
		return nil, nil
	}
	
	dims := window.Dimensions
	view, err := wm.gui.SetView(viewName, dims.X0, dims.Y0, dims.X1-1, dims.Y1-1, 0)
	if err != nil && err != gocui.ErrUnknownView {
		return nil, err
	}
	
	if err == gocui.ErrUnknownView {
		wm.configureNewView(view, window)
		window.Views = append(window.Views, view)
	}
	
	wm.viewCache[viewName] = view
	return view, nil
}

func (wm *WindowManager) configureNewView(view *gocui.View, window *Window) {
	view.Title = window.Title
	view.Editable = window.Editable
	view.Wrap = window.Wrap
	view.Autoscroll = window.Autoscroll
	view.Highlight = window.Highlight
	view.Frame = window.Frame
}

func (wm *WindowManager) SetWindowComponent(windowName string, ctx types.Component) {
	window := wm.windows[windowName]
	if window != nil {
		window.Context = ctx
	}
}

func (wm *WindowManager) GetWindowComponent(windowName string) types.Component {
	window := wm.windows[windowName]
	if window != nil {
		return window.Context
	}
	return nil
}

func (wm *WindowManager) SetWindowTitle(windowName, title string) {
	window := wm.windows[windowName]
	if window != nil {
		window.Title = title
		for _, view := range window.Views {
			if view != nil {
				view.Title = title
			}
		}
	}
}

func (wm *WindowManager) SetWindowProperties(windowName string, props WindowProperties) {
	window := wm.windows[windowName]
	if window != nil {
		window.Focusable = props.Focusable
		window.Editable = props.Editable
		window.Wrap = props.Wrap
		window.Autoscroll = props.Autoscroll
		window.Highlight = props.Highlight
		window.Frame = props.Frame
		
		for _, view := range window.Views {
			if view != nil {
				view.Editable = props.Editable
				view.Wrap = props.Wrap
				view.Autoscroll = props.Autoscroll
				view.Highlight = props.Highlight
				view.Frame = props.Frame
			}
		}
	}
}

func (wm *WindowManager) GetAllWindows() map[string]*Window {
	return wm.windows
}

func (wm *WindowManager) DeleteWindow(name string) {
	if window := wm.windows[name]; window != nil {
		for _, view := range window.Views {
			if view != nil {
				wm.gui.DeleteView(view.Name())
			}
		}
		delete(wm.windows, name)
	}
}

type WindowProperties struct {
	Focusable  bool
	Editable   bool
	Wrap       bool
	Autoscroll bool
	Highlight  bool
	Frame      bool
}