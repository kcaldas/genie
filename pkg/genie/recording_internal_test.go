package genie

import (
	"path/filepath"
	"testing"

	"github.com/kcaldas/genie/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewGenie_DefaultHasNoRecorder pins the zero-behavior-change contract:
// NewGenie() without recording options must not construct a recorder, so no
// recording code path can ever write anything.
func TestNewGenie_DefaultHasNoRecorder(t *testing.T) {
	t.Setenv("GENIE_SESSION_RECORDING", "")
	g, err := NewGenie()
	require.NoError(t, err)

	c, ok := g.(*core)
	require.True(t, ok)
	assert.Nil(t, c.recorder, "default NewGenie must have a nil recorder")
}

func TestProvideSessionRecorder_DefaultOptionsNil(t *testing.T) {
	t.Setenv("GENIE_SESSION_RECORDING", "")
	assert.Nil(t, provideSessionRecorder(applyOptions()))
}

func TestProvideSessionRecorder_EnvActivation(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	t.Setenv("GENIE_SESSION_RECORDING", "standard")
	rec := provideSessionRecorder(applyOptions())
	require.NotNil(t, rec, "env activation must build a recorder")

	matches, err := filepath.Glob(filepath.Join(dir, ".genie", "sessions", "*.session.jsonl"))
	require.NoError(t, err)
	assert.Len(t, matches, 1, "session file created under <genie home>/.genie/sessions")

	// Host wiring wins over env.
	hostRec := session.NewRecorder(session.NewMemoryStorage(), session.LevelStandard)
	assert.Same(t, hostRec, provideSessionRecorder(applyOptions(WithSessionRecorder(hostRec))))
}

func TestProvideSessionRecorder_EnvInvalidValueDisables(t *testing.T) {
	t.Setenv("GENIE_SESSION_RECORDING", "verbose")
	assert.Nil(t, provideSessionRecorder(applyOptions()))

	t.Setenv("GENIE_SESSION_RECORDING", "off")
	assert.Nil(t, provideSessionRecorder(applyOptions()))
}
