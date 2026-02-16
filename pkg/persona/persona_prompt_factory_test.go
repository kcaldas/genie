package persona

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/prompts"
	"github.com/kcaldas/genie/pkg/tools"
)

func TestPersonaPromptFactory_ProjectPersonaMissingToolsFailsLoudly(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	personaDir := filepath.Join(tmp, ".genie", "personas", "genie")
	require.NoError(t, os.MkdirAll(personaDir, 0o755))

	promptPath := filepath.Join(personaDir, "prompt.yaml")
	promptYAML := `name: "genie"
required_tools:
  - "send_message"
llm_provider: ollama
model_name: gpt-oss:20b
`
	require.NoError(t, os.WriteFile(promptPath, []byte(promptYAML), 0o644))

	eventBus := &events.NoOpEventBus{}
	registry := tools.NewDefaultRegistry(eventBus, tools.NewTodoManager(), nil, nil)
	loader := prompts.NewPromptLoader(eventBus, registry)

	factory := &PersonaPromptFactory{
		promptLoader: loader,
		userHome:     "",
	}

	ctx := context.WithValue(context.Background(), "cwd", tmp)

	prompt, err := factory.GetPrompt(ctx, "genie")
	require.Error(t, err)
	assert.Nil(t, prompt)
	assert.Contains(t, err.Error(), "missing required tools: [send_message]")
	assert.Contains(t, err.Error(), "either register the missing tools or remove them")
}
