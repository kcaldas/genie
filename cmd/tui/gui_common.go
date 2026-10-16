package tui

import (
	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
)

type Gui struct {
	gui *gocui.Gui
}

func (g *Gui) GetGui() *gocui.Gui {
	return g.gui
}

func (g *Gui) PostUIUpdate(fn func()) {
	g.gui.Update(func(*gocui.Gui) error {
		fn()
		return nil
	})
}

// chatRenderDispatcher is the gocui-side implementation of types.Dispatcher:
// a ChatChanged message becomes a re-render of the messages view on the
// gocui update loop.
type chatRenderDispatcher struct {
	gui      types.Gui
	messages types.Component
}

// ProvideChatDispatcher builds the dispatcher the chat controller uses to
// reach the gocui messages component.
func ProvideChatDispatcher(gui types.Gui, messages types.Component) types.Dispatcher {
	return &chatRenderDispatcher{gui: gui, messages: messages}
}

func (d *chatRenderDispatcher) Dispatch(msg any) {
	switch msg.(type) {
	case uimsg.ChatChanged, uimsg.ChatStreaming:
		d.gui.PostUIUpdate(func() {
			_ = d.messages.Render()
		})
	}
}

// gocuiOutputMode converts the OutputMode config string to a gocui.OutputMode.
//
//   - "true": 24-bit color (default)
//   - "256": 256-color mode
//   - "normal": 8-color mode
//   - "simulator": headless mode for tests
func gocuiOutputMode(outputMode string) gocui.OutputMode {
	switch outputMode {
	case "normal":
		return gocui.OutputNormal
	case "256":
		return gocui.Output256
	case "simulator":
		return gocui.OutputSimulator
	default:
		return gocui.OutputTrue
	}
}
