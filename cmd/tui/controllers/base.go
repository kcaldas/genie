package controllers

import (
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type BaseController struct {
	context       types.Component
	gui           types.IGuiCommon
	configManager *helpers.ConfigManager
}

func NewBaseController(ctx types.Component, gui types.IGuiCommon, configManager *helpers.ConfigManager) *BaseController {
	return &BaseController{
		context:       ctx,
		gui:           gui,
		configManager: configManager,
	}
}

func (c *BaseController) GetComponent() types.Component {
	return c.context
}

func (c *BaseController) PostUIUpdate(fn func()) {
	if c.gui != nil {
		c.gui.PostUIUpdate(fn)
	}
}

// GetConfig returns the current config from ConfigManager
func (c *BaseController) GetConfig() *types.Config {
	return c.configManager.GetConfig()
}

// GetTheme returns the current theme from ConfigManager
func (c *BaseController) GetTheme() *types.Theme {
	return c.configManager.GetTheme()
}

// GetConfigManager returns the ConfigManager for direct access
func (c *BaseController) GetConfigManager() *helpers.ConfigManager {
	return c.configManager
}

