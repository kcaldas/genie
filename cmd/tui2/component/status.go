package component

import (
	"fmt"
	"runtime"

	"github.com/kcaldas/genie/cmd/tui2/types"
)

type StatusComponent struct {
	*BaseComponent
	stateAccessor types.IStateAccessor
}

func NewStatusComponent(gui types.IGuiCommon, state types.IStateAccessor) *StatusComponent {
	ctx := &StatusComponent{
		BaseComponent: NewBaseComponent("status", "status", gui),
		stateAccessor: state,
	}

	// Configure StatusComponent specific properties
	ctx.SetTitle(" Status ")
	ctx.SetWindowProperties(types.WindowProperties{
		Focusable:  false,
		Editable:   false,
		Wrap:       false,
		Autoscroll: false,
		Highlight:  false,
		Frame:      true,
	})

	ctx.SetWindowName("status")
	ctx.SetControlledBounds(true)

	return ctx
}

func (c *StatusComponent) Render() error {
	v := c.GetView()
	if v == nil {
		return nil
	}

	v.Clear()

	// Memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memMB := m.Alloc / 1024 / 1024

	// Status line
	status := "Ready"
	if c.stateAccessor.IsLoading() {
		status = "Processing..."
	}

	// Message count
	msgCount := len(c.stateAccessor.GetMessages())

	// Build status line with debug info
	statusLine := fmt.Sprintf(" Status: %s | Messages: %d | Memory: %dMB",
		status, msgCount, memMB)

	// Write the status line
	fmt.Fprint(v, statusLine)

	return nil
}
