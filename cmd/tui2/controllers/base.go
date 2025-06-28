package controllers

import (
	"github.com/kcaldas/genie/cmd/tui2/types"
)

type BaseController struct {
	context types.Component
	gui     types.IGuiCommon
}

func NewBaseController(ctx types.Component, gui types.IGuiCommon) *BaseController {
	return &BaseController{
		context: ctx,
		gui:     gui,
	}
}

func (c *BaseController) GetComponent() types.Component {
	return c.context
}

func (c *BaseController) HandleCommand(command string) error {
	return nil
}

func (c *BaseController) HandleInput(input string) error {
	return nil
}

func (c *BaseController) PostUIUpdate(fn func()) {
	if c.gui != nil {
		c.gui.PostUIUpdate(fn)
	}
}