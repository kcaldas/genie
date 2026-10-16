// Package render turns message bodies into terminal text using the
// configured themes: markdown through glamour, diffs through the diff
// formatter. The side panel and the confirmation dialog share it.
package render

import (
	"strings"

	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
)

// largeMarkdown is the size above which markdown falls back to the plain
// ascii style, since full styling of huge documents produces unwieldy
// amounts of ANSI.
const largeMarkdown = 50000

// Config is what rendering reads from the TUI configuration.
type Config interface {
	GetConfig() *types.Config
	GetTheme() *types.Theme
}

// Markdown renders content through glamour at width. On any error the raw
// content is returned so nothing is ever lost.
func Markdown(cfg Config, content string, width int) string {
	c := cfg.GetConfig()
	glamourTheme := c.GlamourTheme
	if len(content) > largeMarkdown {
		glamourTheme = "ascii"
	}
	r, err := presentation.CreateMarkdownRendererWithWidth(cfg.GetTheme(), c.Theme, glamourTheme, width)
	if err != nil {
		return content
	}
	out, err := r.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(out)
}

// Diff colours a unified diff with the configured diff theme, resolving
// "auto" from the main theme.
func Diff(cfg Config, content string) string {
	c := cfg.GetConfig()
	name := c.DiffTheme
	if name == "" || name == "auto" {
		name = presentation.GetDiffThemeForMainTheme(c.Theme)
	}
	return presentation.NewDiffFormatter(presentation.GetDiffTheme(name)).Format(content)
}

// Body renders content by type: "diff", "markdown", or anything else as
// plain text. The result is split into lines.
func Body(cfg Config, content, ctype string, width int) []string {
	switch ctype {
	case "diff":
		content = Diff(cfg, content)
	case "markdown":
		if content != "" {
			content = Markdown(cfg, content, width)
		}
	}
	return strings.Split(content, "\n")
}
