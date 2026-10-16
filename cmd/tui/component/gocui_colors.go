package component

import (
	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/types"
)

// ThemeColor converts a theme hex color to a gocui attribute.
func ThemeColor(hexColor string) gocui.Attribute {
	return gocui.GetColor(hexColor)
}

// AnsiToGocuiColor converts an ANSI or hex color string to a gocui attribute.
func AnsiToGocuiColor(color string) gocui.Attribute {
	return gocui.GetColor(color)
}

// ThemeFocusColors returns the background and foreground attributes used
// for focused views. Themes do not define them, so both are the default.
func ThemeFocusColors(_ *types.Theme) (gocui.Attribute, gocui.Attribute) {
	return gocui.ColorDefault, gocui.ColorDefault
}
