package layout

import (
	"math"
)

type ResponsiveDesign struct {
	breakpoints map[string]BreakpointConfig
}

type BreakpointConfig struct {
	MinWidth     int
	MinHeight    int
	LayoutMode   string
	PanelRatios  map[string]float64
	HiddenPanels []string
	Features     []string
}

func NewResponsiveDesign() *ResponsiveDesign {
	return &ResponsiveDesign{
		breakpoints: map[string]BreakpointConfig{
			"xs": {
				MinWidth:    0,
				MinHeight:   0,
				LayoutMode:  "minimal",
				PanelRatios: map[string]float64{"messages": 0.8, "input": 0.2},
				HiddenPanels: []string{"debug", "sidebar"},
				Features:    []string{"compact-input", "no-borders"},
			},
			"sm": {
				MinWidth:    60,
				MinHeight:   20,
				LayoutMode:  "compact",
				PanelRatios: map[string]float64{"messages": 0.75, "input": 0.25},
				HiddenPanels: []string{"sidebar"},
				Features:    []string{"compact-input"},
			},
			"md": {
				MinWidth:    80,
				MinHeight:   30,
				LayoutMode:  "standard",
				PanelRatios: map[string]float64{"messages": 0.7, "input": 0.2, "debug": 0.1},
				HiddenPanels: []string{},
				Features:    []string{},
			},
			"lg": {
				MinWidth:    120,
				MinHeight:   40,
				LayoutMode:  "expanded",
				PanelRatios: map[string]float64{"messages": 0.6, "input": 0.15, "debug": 0.15, "sidebar": 0.1},
				HiddenPanels: []string{},
				Features:    []string{"enhanced-sidebar", "multi-column"},
			},
			"xl": {
				MinWidth:    160,
				MinHeight:   50,
				LayoutMode:  "full",
				PanelRatios: map[string]float64{"messages": 0.5, "input": 0.15, "debug": 0.2, "sidebar": 0.15},
				HiddenPanels: []string{},
				Features:    []string{"enhanced-sidebar", "multi-column", "split-view"},
			},
		},
	}
}

func (rd *ResponsiveDesign) GetBreakpoint(width, height int) string {
	breakpoints := []string{"xl", "lg", "md", "sm", "xs"}
	
	for _, bp := range breakpoints {
		config := rd.breakpoints[bp]
		if width >= config.MinWidth && height >= config.MinHeight {
			return bp
		}
	}
	
	return "xs"
}

func (rd *ResponsiveDesign) GetConfig(breakpoint string) BreakpointConfig {
	if config, exists := rd.breakpoints[breakpoint]; exists {
		return config
	}
	return rd.breakpoints["xs"]
}

func (rd *ResponsiveDesign) ShouldHidePanel(panelName, breakpoint string) bool {
	config := rd.GetConfig(breakpoint)
	for _, hidden := range config.HiddenPanels {
		if hidden == panelName {
			return true
		}
	}
	return false
}

func (rd *ResponsiveDesign) HasFeature(feature, breakpoint string) bool {
	config := rd.GetConfig(breakpoint)
	for _, f := range config.Features {
		if f == feature {
			return true
		}
	}
	return false
}

func (rd *ResponsiveDesign) GetPanelRatio(panelName, breakpoint string) float64 {
	config := rd.GetConfig(breakpoint)
	if ratio, exists := config.PanelRatios[panelName]; exists {
		return ratio
	}
	return 0.1
}

func (rd *ResponsiveDesign) CalculateAdaptiveLayout(width, height int, config *LayoutConfig) *LayoutConfig {
	breakpoint := rd.GetBreakpoint(width, height)
	bpConfig := rd.GetConfig(breakpoint)
	
	adaptedConfig := *config
	
	switch bpConfig.LayoutMode {
	case "minimal":
		adaptedConfig.ShowSidebar = false
		adaptedConfig.CompactMode = true
		adaptedConfig.ResponsePanelMode = "overlay"
		adaptedConfig.MinPanelWidth = 15
		adaptedConfig.MinPanelHeight = 2
		
	case "compact":
		adaptedConfig.ShowSidebar = false
		adaptedConfig.CompactMode = true
		adaptedConfig.ResponsePanelMode = "split"
		adaptedConfig.MinPanelWidth = 20
		adaptedConfig.MinPanelHeight = 3
		
	case "standard":
		adaptedConfig.ShowSidebar = width > 100
		adaptedConfig.CompactMode = false
		adaptedConfig.ResponsePanelMode = "split"
		adaptedConfig.ChatPanelWidth = rd.GetPanelRatio("messages", breakpoint)
		
	case "expanded":
		adaptedConfig.ShowSidebar = true
		adaptedConfig.CompactMode = false
		adaptedConfig.ResponsePanelMode = "split"
		adaptedConfig.ChatPanelWidth = rd.GetPanelRatio("messages", breakpoint)
		
	case "full":
		adaptedConfig.ShowSidebar = true
		adaptedConfig.CompactMode = false
		adaptedConfig.ResponsePanelMode = "multi"
		adaptedConfig.ChatPanelWidth = rd.GetPanelRatio("messages", breakpoint)
	}
	
	return &adaptedConfig
}

func (rd *ResponsiveDesign) GetAdaptiveWeights(panelName string, width, height int) int {
	breakpoint := rd.GetBreakpoint(width, height)
	ratio := rd.GetPanelRatio(panelName, breakpoint)
	
	baseWeight := int(math.Round(ratio * 100))
	if baseWeight < 1 {
		baseWeight = 1
	}
	
	return baseWeight
}

func (rd *ResponsiveDesign) ShouldUsePortraitLayout(width, height int) bool {
	aspectRatio := float64(width) / float64(height)
	return aspectRatio < 1.8 && width <= 84
}

func (rd *ResponsiveDesign) GetOptimalPanelHeight(panelName string, totalHeight int, breakpoint string) int {
	ratio := rd.GetPanelRatio(panelName, breakpoint)
	height := int(math.Round(float64(totalHeight) * ratio))
	
	switch panelName {
	case "input":
		if rd.HasFeature("compact-input", breakpoint) {
			return min(height, 3)
		}
		return min(height, 5)
		
	case "debug":
		return min(height, totalHeight/3)
		
	case "messages":
		minHeight := totalHeight / 4
		return max(height, minHeight)
		
	default:
		return height
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}