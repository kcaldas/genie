package genie

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewGenie_DefaultHasNoRecorder pins the zero-behavior-change contract:
// NewGenie() without recording options must not construct a recorder, so no
// recording code path can ever write anything.
func TestNewGenie_DefaultHasNoRecorder(t *testing.T) {
	g, err := NewGenie()
	require.NoError(t, err)

	c, ok := g.(*core)
	require.True(t, ok)
	assert.Nil(t, c.recorder, "default NewGenie must have a nil recorder")
}

func TestProvideSessionRecorder_DefaultOptionsNil(t *testing.T) {
	assert.Nil(t, provideSessionRecorder(applyOptions()))
}
