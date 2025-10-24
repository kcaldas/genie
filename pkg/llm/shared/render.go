package shared

import (
	"fmt"
	"path/filepath"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/fileops"
)

func RenderPromptWithDebug(fileManager fileops.Manager, prompt ai.Prompt, debug bool, attrs []ai.Attr) (*ai.Prompt, error) {
	if debug {
		if err := SaveYAMLToTmp(fileManager, prompt, fmt.Sprintf("%s-initial-prompt.yaml", prompt.Name)); err != nil {
			return nil, err
		}
		if err := SaveYAMLToTmp(fileManager, attrs, fmt.Sprintf("%s-attrs.yaml", prompt.Name)); err != nil {
			return nil, err
		}
	}

	rendered, err := RenderPromptWithAttrs(prompt, attrs)
	if err != nil {
		return nil, err
	}

	if debug {
		if err := SaveYAMLToTmp(fileManager, rendered, fmt.Sprintf("%s-final-prompt.yaml", rendered.Name)); err != nil {
			return nil, err
		}
	}

	return rendered, nil
}

func RenderPromptWithAttrs(prompt ai.Prompt, attrs []ai.Attr) (*ai.Prompt, error) {
	rendered, err := ai.RenderPrompt(prompt, AttrSliceToMap(attrs))
	if err != nil {
		return nil, fmt.Errorf("error rendering prompt: %w", err)
	}
	return &rendered, nil
}

func AttrSliceToMap(attrs []ai.Attr) map[string]string {
	if len(attrs) == 0 {
		return nil
	}
	m := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		m[attr.Key] = attr.Value
	}
	return m
}

func SaveYAMLToTmp(fileManager fileops.Manager, object interface{}, filename string) error {
	filePath := filepath.Join("tmp", filename)
	return fileManager.WriteObjectAsYAML(filePath, object)
}
