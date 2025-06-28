package types

import (
	"github.com/awesome-gocui/gocui"
)

type UserInput struct {
	Message string
	IsSlashCommand bool
}

type Message struct {
	Role    string
	Content string
}

type FocusablePanel int

const (
	PanelMessages FocusablePanel = iota
	PanelInput
	PanelDebug
	PanelStatus
)

type KeyBinding struct {
	View    string
	Key     interface{}
	Mod     gocui.Modifier
	Handler func(*gocui.Gui, *gocui.View) error
}

type Theme struct {
	Primary   string
	Secondary string
	Tertiary  string
	Error     string
	Warning   string
	Success   string
	Muted     string
}

type Config struct {
	ShowCursor          bool
	MarkdownRendering   bool
	Theme               string
	WrapMessages        bool
	ShowTimestamps      bool
	
	Layout LayoutConfig
}

type LayoutConfig struct {
	ChatPanelWidth    float64
	ShowSidebar       bool
	CompactMode       bool
	ResponsePanelMode string
	MinPanelWidth     int
	MinPanelHeight    int
	BorderStyle       string
	PortraitMode      string
	SidePanelWidth    float64
	ExpandedSidePanel bool
}