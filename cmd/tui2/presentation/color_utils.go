package presentation

import (
	"strconv"
	"strings"
	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

// ConvertAnsiToGocuiColor converts ANSI color codes to gocui.Attribute
// This is a simplified conversion for basic colors
func ConvertAnsiToGocuiColor(ansiCode string) gocui.Attribute {
	// Remove ANSI escape sequences and extract color code
	code := strings.TrimPrefix(ansiCode, "\033[")
	code = strings.TrimSuffix(code, "m")
	
	if colorNum, err := strconv.Atoi(code); err == nil {
		switch colorNum {
		case 30: return gocui.ColorBlack
		case 31: return gocui.ColorRed
		case 32: return gocui.ColorGreen
		case 33: return gocui.ColorYellow
		case 34: return gocui.ColorBlue
		case 35: return gocui.ColorMagenta
		case 36: return gocui.ColorCyan
		case 37: return gocui.ColorWhite
		case 90: return gocui.ColorBlack | gocui.AttrBold
		case 91: return gocui.ColorRed | gocui.AttrBold
		case 92: return gocui.ColorGreen | gocui.AttrBold
		case 93: return gocui.ColorYellow | gocui.AttrBold
		case 94: return gocui.ColorBlue | gocui.AttrBold
		case 95: return gocui.ColorMagenta | gocui.AttrBold
		case 96: return gocui.ColorCyan | gocui.AttrBold
		case 97: return gocui.ColorWhite | gocui.AttrBold
		case 40: return gocui.ColorBlack
		case 41: return gocui.ColorRed
		case 42: return gocui.ColorGreen
		case 43: return gocui.ColorYellow
		case 44: return gocui.ColorBlue
		case 45: return gocui.ColorMagenta
		case 46: return gocui.ColorCyan
		case 47: return gocui.ColorWhite
		case 100: return gocui.ColorBlack | gocui.AttrBold
		case 101: return gocui.ColorRed | gocui.AttrBold
		case 102: return gocui.ColorGreen | gocui.AttrBold
		case 103: return gocui.ColorYellow | gocui.AttrBold
		case 104: return gocui.ColorBlue | gocui.AttrBold
		case 105: return gocui.ColorMagenta | gocui.AttrBold
		case 106: return gocui.ColorCyan | gocui.AttrBold
		case 107: return gocui.ColorWhite | gocui.AttrBold
		}
	}
	
	// Default fallback
	return gocui.ColorDefault
}

// GetThemeFocusColors returns the appropriate gocui colors for focus states
func GetThemeFocusColors(theme *types.Theme) (bg gocui.Attribute, fg gocui.Attribute) {
	bg = ConvertAnsiToGocuiColor(theme.FocusBackground)
	fg = ConvertAnsiToGocuiColor(theme.FocusForeground)
	return
}

// GetThemeBorderColor returns the appropriate border color for the given state
func GetThemeBorderColor(theme *types.Theme, focused bool, muted bool) string {
	if muted {
		return theme.BorderMuted
	}
	if focused {
		return theme.BorderFocused
	}
	return theme.BorderDefault
}