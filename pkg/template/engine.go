package template

import (
	"bytes"
	"path/filepath"
	"strings"
	"text/template"
)

// Engine provides template rendering functionality
type Engine interface {
	RenderString(templateContent string, data map[string]string) (string, error)
	RenderFile(filePath string, data map[string]string) (string, error)
}

// DefaultEngine implements the Engine interface
type DefaultEngine struct {
}

// NewEngine creates a new default template engine
func NewEngine() Engine {
	return &DefaultEngine{}
}

// RenderString renders a template string with the provided data
func (e *DefaultEngine) RenderString(templateContent string, data map[string]string) (string, error) {
	tmpl, err := template.New("template").Funcs(template.FuncMap{
		"indent": indent,
	}).Parse(templateContent)

	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	return buf.String(), err
}

// RenderFile renders a template file with the provided data
func (e *DefaultEngine) RenderFile(filePath string, data map[string]string) (string, error) {
	templ := template.New(filepath.Base(filePath)).Funcs(template.FuncMap{
		"indent": indent,
	})
	templ, err := templ.ParseFiles(filePath)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := templ.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// indent adds the specified number of spaces to the beginning of each line
func indent(spaces int, text string) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
