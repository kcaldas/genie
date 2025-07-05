package presentation

import (
	"fmt"
	"strconv"
	"strings"
	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/types"
)

// ConvertColorToGocuiColor converts various color formats to gocui.Attribute
// Supports: ANSI codes, 256-color codes, and RGB hex codes
func ConvertColorToGocuiColor(colorStr string) gocui.Attribute {
	// Handle RGB hex codes (true color mode)
	if strings.HasPrefix(colorStr, "#") {
		return parseHexColor(colorStr)
	}
	
	// Handle ANSI escape sequences
	if strings.HasPrefix(colorStr, "\033[") {
		return parseAnsiColor(colorStr)
	}
	
	// Default fallback
	return gocui.ColorDefault
}

// parseHexColor converts RGB hex codes to gocui color (for true color mode)
func parseHexColor(hex string) gocui.Attribute {
	if len(hex) != 7 || hex[0] != '#' {
		return gocui.ColorDefault
	}
	
	// Extract RGB components
	r, err1 := strconv.ParseInt(hex[1:3], 16, 32)
	g, err2 := strconv.ParseInt(hex[3:5], 16, 32)
	b, err3 := strconv.ParseInt(hex[5:7], 16, 32)
	
	if err1 != nil || err2 != nil || err3 != nil {
		return gocui.ColorDefault
	}
	
	// For gocui, we need to map RGB to closest basic color
	// This is a simplified mapping for border colors
	return mapRGBToGocuiColor(int(r), int(g), int(b))
}

// mapRGBToGocuiColor maps RGB values to the closest gocui basic color
func mapRGBToGocuiColor(r, g, b int) gocui.Attribute {
	// Simple heuristic to map RGB to basic colors
	if r > 200 && g > 200 && b > 200 {
		return gocui.ColorWhite
	}
	if r < 100 && g < 100 && b < 100 {
		return gocui.ColorBlack
	}
	
	// Find dominant color
	if r > g && r > b {
		if r > 128 {
			return gocui.ColorRed | gocui.AttrBold
		}
		return gocui.ColorRed
	}
	if g > r && g > b {
		if g > 128 {
			return gocui.ColorGreen | gocui.AttrBold
		}
		return gocui.ColorGreen
	}
	if b > r && b > g {
		if b > 128 {
			return gocui.ColorBlue | gocui.AttrBold
		}
		return gocui.ColorBlue
	}
	
	// Mixed colors
	if r > 128 && g > 128 && b < 100 {
		return gocui.ColorYellow | gocui.AttrBold
	}
	if r > 128 && g < 100 && b > 128 {
		return gocui.ColorMagenta | gocui.AttrBold
	}
	if r < 100 && g > 128 && b > 128 {
		return gocui.ColorCyan | gocui.AttrBold
	}
	
	return gocui.ColorDefault
}

// parseAnsiColor handles ANSI escape sequences (including 256-color)
func parseAnsiColor(ansiCode string) gocui.Attribute {
	// Remove ANSI escape sequences
	code := strings.TrimPrefix(ansiCode, "\033[")
	code = strings.TrimSuffix(code, "m")
	
	// Handle 256-color codes (38;5;n or 48;5;n)
	if strings.Contains(code, ";5;") {
		return parse256Color(code)
	}
	
	// Handle basic ANSI codes
	if colorNum, err := strconv.Atoi(code); err == nil {
		switch colorNum {
		// Standard foreground colors (30-37)
		case 30: return gocui.ColorBlack
		case 31: return gocui.ColorRed
		case 32: return gocui.ColorGreen
		case 33: return gocui.ColorYellow
		case 34: return gocui.ColorBlue
		case 35: return gocui.ColorMagenta
		case 36: return gocui.ColorCyan
		case 37: return gocui.ColorWhite
		
		// Bright foreground colors (90-97)
		case 90: return gocui.ColorBlack | gocui.AttrBold
		case 91: return gocui.ColorRed | gocui.AttrBold
		case 92: return gocui.ColorGreen | gocui.AttrBold
		case 93: return gocui.ColorYellow | gocui.AttrBold
		case 94: return gocui.ColorBlue | gocui.AttrBold
		case 95: return gocui.ColorMagenta | gocui.AttrBold
		case 96: return gocui.ColorCyan | gocui.AttrBold
		case 97: return gocui.ColorWhite | gocui.AttrBold
		
		// Background colors (40-47) - treat as foreground for border colors
		case 40: return gocui.ColorBlack
		case 41: return gocui.ColorRed
		case 42: return gocui.ColorGreen
		case 43: return gocui.ColorYellow
		case 44: return gocui.ColorBlue
		case 45: return gocui.ColorMagenta
		case 46: return gocui.ColorCyan
		case 47: return gocui.ColorWhite
		
		// Bright background colors (100-107)
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
	
	return gocui.ColorDefault
}

// parse256Color handles 256-color ANSI codes
func parse256Color(code string) gocui.Attribute {
	// Extract color number from "38;5;n" or "48;5;n"
	parts := strings.Split(code, ";")
	if len(parts) >= 3 && parts[1] == "5" {
		if colorNum, err := strconv.Atoi(parts[2]); err == nil {
			return map256ColorToGocui(colorNum)
		}
	}
	return gocui.ColorDefault
}

// map256ColorToGocui maps 256-color numbers to basic gocui colors
func map256ColorToGocui(colorNum int) gocui.Attribute {
	// Standard colors (0-15) map to basic colors
	switch colorNum {
	case 0: return gocui.ColorBlack
	case 1: return gocui.ColorRed
	case 2: return gocui.ColorGreen
	case 3: return gocui.ColorYellow
	case 4: return gocui.ColorBlue
	case 5: return gocui.ColorMagenta
	case 6: return gocui.ColorCyan
	case 7: return gocui.ColorWhite
	case 8: return gocui.ColorBlack | gocui.AttrBold
	case 9: return gocui.ColorRed | gocui.AttrBold
	case 10: return gocui.ColorGreen | gocui.AttrBold
	case 11: return gocui.ColorYellow | gocui.AttrBold
	case 12: return gocui.ColorBlue | gocui.AttrBold
	case 13: return gocui.ColorMagenta | gocui.AttrBold
	case 14: return gocui.ColorCyan | gocui.AttrBold
	case 15: return gocui.ColorWhite | gocui.AttrBold
	}
	
	// For colors 16-255, map to closest basic color
	// This is a simplified mapping
	if colorNum >= 16 && colorNum <= 21 {
		return gocui.ColorBlue | gocui.AttrBold
	}
	if colorNum >= 22 && colorNum <= 27 {
		return gocui.ColorGreen | gocui.AttrBold
	}
	if colorNum >= 28 && colorNum <= 33 {
		return gocui.ColorCyan | gocui.AttrBold
	}
	if colorNum >= 34 && colorNum <= 39 {
		return gocui.ColorRed | gocui.AttrBold
	}
	if colorNum >= 40 && colorNum <= 45 {
		return gocui.ColorMagenta | gocui.AttrBold
	}
	if colorNum >= 46 && colorNum <= 51 {
		return gocui.ColorCyan | gocui.AttrBold
	}
	
	// Grayscale colors (232-255)
	if colorNum >= 232 && colorNum <= 243 {
		return gocui.ColorBlack | gocui.AttrBold
	}
	if colorNum >= 244 && colorNum <= 255 {
		return gocui.ColorWhite
	}
	
	return gocui.ColorDefault
}

// ConvertAnsiToGocuiColor is deprecated - use ConvertColorToGocuiColor instead
// Kept for backwards compatibility
func ConvertAnsiToGocuiColor(ansiCode string) gocui.Attribute {
	return ConvertColorToGocuiColor(ansiCode)
}

// ConvertColorToAnsi converts various color formats to ANSI escape sequences
// This is used for text content (not borders)
func ConvertColorToAnsi(colorStr string) string {
	// Handle RGB hex codes - convert to true color ANSI
	if strings.HasPrefix(colorStr, "#") {
		return hexToTrueColorAnsi(colorStr)
	}
	
	// Already an ANSI code
	if strings.HasPrefix(colorStr, "\033[") {
		return colorStr
	}
	
	// Default - return as-is
	return colorStr
}

// hexToTrueColorAnsi converts hex color to 24-bit ANSI escape sequence
func hexToTrueColorAnsi(hex string) string {
	if len(hex) != 7 || hex[0] != '#' {
		return "\033[37m" // Default to white
	}
	
	// Extract RGB components
	r, err1 := strconv.ParseInt(hex[1:3], 16, 32)
	g, err2 := strconv.ParseInt(hex[3:5], 16, 32)  
	b, err3 := strconv.ParseInt(hex[5:7], 16, 32)
	
	if err1 != nil || err2 != nil || err3 != nil {
		return "\033[37m" // Default to white
	}
	
	// Create 24-bit color ANSI sequence: ESC[38;2;R;G;Bm
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
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