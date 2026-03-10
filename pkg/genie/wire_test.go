package genie

import (
	"testing"

	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/require"
)

func TestProvideAIGen_DoesNotWarmUpDefaultProvider(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("GENIE_LLM_PROVIDER", "")

	eb := events.NewEventBus()
	manager := config.NewConfigManager()

	gen, err := provideAIGen(eb, manager)
	require.NoError(t, err)
	require.NotNil(t, gen)

	status := gen.GetStatus()
	require.NotNil(t, status)
	require.False(t, status.Connected)
	require.Equal(t, "genai", status.Backend)
	require.Contains(t, status.Message, "no valid AI backend configured")
}
