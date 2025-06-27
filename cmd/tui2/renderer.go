package tui2

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// MarkdownRenderer interface for different markdown rendering implementations
type MarkdownRenderer interface {
	// Render converts markdown text to formatted output
	Render(content string) (string, error)
	
	// UpdateWidth adjusts the renderer for a new terminal width
	UpdateWidth(width int) error
	
	// IsEnabled returns true if the renderer is available
	IsEnabled() bool
}

// GlamourRenderer implements MarkdownRenderer using Glamour
type GlamourRenderer struct {
	renderer *glamour.TermRenderer
	width    int
	theme    string
	enabled  bool
}

// NewGlamourRenderer creates a new Glamour-based markdown renderer with auto theme
func NewGlamourRenderer(width int) MarkdownRenderer {
	return NewGlamourRendererWithTheme(width, "auto")
}

// NewGlamourRendererWithTheme creates a new Glamour-based markdown renderer with specific theme
func NewGlamourRendererWithTheme(width int, theme string) MarkdownRenderer {
	var renderer *glamour.TermRenderer
	var err error
	
	// Create renderer with specified theme
	if theme == "auto" {
		renderer, err = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(width),
		)
	} else {
		renderer, err = glamour.NewTermRenderer(
			glamour.WithStandardStyle(theme),
			glamour.WithWordWrap(width),
		)
	}
	
	if err != nil {
		// Fallback to disabled renderer if Glamour fails
		// Note: In a real app, this error should be logged properly
		return &GlamourRenderer{
			renderer: nil,
			width:    width,
			theme:    theme,
			enabled:  false,
		}
	}
	
	return &GlamourRenderer{
		renderer: renderer,
		width:    width,
		theme:    theme,
		enabled:  true,
	}
}

// Render converts markdown content to ANSI-formatted text
func (g *GlamourRenderer) Render(content string) (string, error) {
	if !g.enabled || g.renderer == nil {
		// Fallback to plain text if renderer is not available
		return content, nil
	}
	
	rendered, err := g.renderer.Render(content)
	if err != nil {
		// Fallback to plain text on render error
		return content, nil
	}
	
	// Clean up extra whitespace that Glamour sometimes adds
	return strings.TrimSpace(rendered), nil
}

// UpdateWidth adjusts the renderer for a new terminal width
func (g *GlamourRenderer) UpdateWidth(width int) error {
	if !g.enabled {
		g.width = width
		return nil
	}
	
	// Create new renderer with updated width and same theme
	var newRenderer *glamour.TermRenderer
	var err error
	
	if g.theme == "auto" {
		newRenderer, err = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(width),
		)
	} else {
		newRenderer, err = glamour.NewTermRenderer(
			glamour.WithStandardStyle(g.theme),
			glamour.WithWordWrap(width),
		)
	}
	
	if err != nil {
		// Keep existing renderer if update fails
		return err
	}
	
	g.renderer = newRenderer
	g.width = width
	return nil
}

// SetTheme changes the theme of the renderer
func (g *GlamourRenderer) SetTheme(theme string) error {
	if !g.enabled {
		g.theme = theme
		return nil
	}
	
	// Create new renderer with new theme and same width
	var newRenderer *glamour.TermRenderer
	var err error
	
	if theme == "auto" {
		newRenderer, err = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(g.width),
		)
	} else {
		newRenderer, err = glamour.NewTermRenderer(
			glamour.WithStandardStyle(theme),
			glamour.WithWordWrap(g.width),
		)
	}
	
	if err != nil {
		return err
	}
	
	g.renderer = newRenderer
	g.theme = theme
	return nil
}

// GetTheme returns the current theme
func (g *GlamourRenderer) GetTheme() string {
	return g.theme
}

// IsEnabled returns true if the Glamour renderer is available
func (g *GlamourRenderer) IsEnabled() bool {
	return g.enabled
}

// PlainTextRenderer implements MarkdownRenderer with no formatting (fallback)
type PlainTextRenderer struct {
	width int
}

// NewPlainTextRenderer creates a fallback renderer that outputs plain text
func NewPlainTextRenderer(width int) MarkdownRenderer {
	return &PlainTextRenderer{
		width: width,
	}
}

// Render returns the content as-is (no markdown processing)
func (p *PlainTextRenderer) Render(content string) (string, error) {
	return content, nil
}

// UpdateWidth updates the width setting (no-op for plain text)
func (p *PlainTextRenderer) UpdateWidth(width int) error {
	p.width = width
	return nil
}

// IsEnabled always returns true for plain text renderer
func (p *PlainTextRenderer) IsEnabled() bool {
	return true
}

// CustomRenderer placeholder for future goldmark implementation
type CustomRenderer struct {
	width   int
	enabled bool
}

// NewCustomRenderer creates a custom goldmark-based renderer (placeholder)
func NewCustomRenderer(width int) MarkdownRenderer {
	// TODO: Implement goldmark + custom ANSI renderer
	// For now, return disabled renderer
	return &CustomRenderer{
		width:   width,
		enabled: false, // Set to false until implemented
	}
}

// Render using custom goldmark renderer (placeholder)
func (c *CustomRenderer) Render(content string) (string, error) {
	if !c.enabled {
		return content, nil // Fallback to plain text
	}
	
	// TODO: Implement goldmark AST walking with ANSI output
	return content, nil
}

// UpdateWidth adjusts the custom renderer width
func (c *CustomRenderer) UpdateWidth(width int) error {
	c.width = width
	return nil
}

// IsEnabled returns the availability of the custom renderer
func (c *CustomRenderer) IsEnabled() bool {
	return c.enabled
}

// RendererType defines available renderer types
type RendererType string

const (
	GlamourRendererType    RendererType = "glamour"
	PlainTextRendererType  RendererType = "plaintext"
	CustomRendererType     RendererType = "custom"
)

// NewMarkdownRenderer creates a markdown renderer of the specified type
func NewMarkdownRenderer(rendererType RendererType, width int) MarkdownRenderer {
	switch rendererType {
	case GlamourRendererType:
		return NewGlamourRenderer(width)
	case CustomRendererType:
		return NewCustomRenderer(width)
	case PlainTextRendererType:
		fallthrough
	default:
		return NewPlainTextRenderer(width)
	}
}

// NewMarkdownRendererWithFallback creates a renderer with automatic fallback
func NewMarkdownRendererWithFallback(preferredType RendererType, width int) MarkdownRenderer {
	// Try preferred renderer first
	renderer := NewMarkdownRenderer(preferredType, width)
	
	// If preferred renderer is not enabled, fallback to Glamour
	if !renderer.IsEnabled() && preferredType != GlamourRendererType {
		renderer = NewMarkdownRenderer(GlamourRendererType, width)
	}
	
	// If Glamour also fails, fallback to plain text
	if !renderer.IsEnabled() {
		renderer = NewMarkdownRenderer(PlainTextRendererType, width)
	}
	
	return renderer
}