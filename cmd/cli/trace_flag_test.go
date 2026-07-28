package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigureTraceEnv(t *testing.T) {
	t.Setenv("GENIE_SESSION_RECORDING", "")

	// No flag: env untouched.
	require.NoError(t, configureTraceEnv(""))
	assert.Empty(t, os.Getenv("GENIE_SESSION_RECORDING"))

	// Valid level lands in the env.
	require.NoError(t, configureTraceEnv("full"))
	assert.Equal(t, "full", os.Getenv("GENIE_SESSION_RECORDING"))

	// Flag wins over an inherited env value.
	t.Setenv("GENIE_SESSION_RECORDING", "standard")
	require.NoError(t, configureTraceEnv("full"))
	assert.Equal(t, "full", os.Getenv("GENIE_SESSION_RECORDING"))

	// Garbage fails fast instead of silently recording nothing.
	assert.Error(t, configureTraceEnv("verbose"))
}
