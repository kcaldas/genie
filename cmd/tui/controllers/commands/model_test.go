package commands

import (
	"errors"
	"sync"
	"testing"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collected gathers bus events, which arrive on their own goroutines.
type collected struct {
	mu     sync.Mutex
	events []interface{}
}

func collect(bus *events.CommandEventBus, topic string) *collected {
	c := &collected{}
	bus.Subscribe(topic, func(e interface{}) {
		c.mu.Lock()
		c.events = append(c.events, e)
		c.mu.Unlock()
	})
	return c
}

func (c *collected) all() []interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]interface{}(nil), c.events...)
}

type fakeDefaults struct {
	saved  map[string]any
	global string
}

func (f *fakeDefaults) GetConfig() *types.Config { return &types.Config{Model: f.global} }

func (f *fakeDefaults) SetLocalValue(key string, value any) error {
	if f.saved == nil {
		f.saved = map[string]any{}
	}
	f.saved[key] = value
	return nil
}

func newModelCommand(t *testing.T) (*ModelCommand, *MockGenieService, *types.MockNotification, *events.CommandEventBus, *fakeDefaults) {
	t.Helper()
	g := &MockGenieService{providers: []string{"genai", "openai"}, models: map[string][]string{"openai": {"gpt-5.5", "gpt-5.5-mini"}}}
	n := &types.MockNotification{}
	bus := events.NewCommandEventBus()
	d := &fakeDefaults{}
	return NewModelCommand(n, g, bus, d), g, n, bus, d
}

func TestModelCommandSetsAndAnnounces(t *testing.T) {
	cmd, g, n, bus, _ := newModelCommand(t)
	announced := collect(bus, uimsg.TopicModelChanged)

	require.NoError(t, cmd.Execute([]string{"openai/gpt-5.5"}))
	bus.WaitForPendingEvents()
	assert.Equal(t, genie.ModelSelection{Provider: "openai", Model: "gpt-5.5"}, g.model)
	require.Len(t, announced.all(), 1)
	assert.Equal(t, map[string]interface{}{"provider": "openai", "model": "gpt-5.5"}, announced.all()[0])
	assert.Contains(t, n.SystemMessages[0], "Using openai/gpt-5.5")

	require.NoError(t, cmd.Execute([]string{"reset"}))
	bus.WaitForPendingEvents()
	assert.True(t, g.model.IsZero())
	assert.Contains(t, n.SystemMessages[1], "cleared")
	assert.Len(t, announced.all(), 2)
}

func TestModelCommandReportsBackendErrors(t *testing.T) {
	cmd, g, _, _, _ := newModelCommand(t)
	g.setModelErr = errors.New("unknown LLM provider \"nope\"")
	err := cmd.Execute([]string{"nope/x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope")
}

func TestModelCommandNoArgsOpensThePicker(t *testing.T) {
	cmd, _, _, bus, _ := newModelCommand(t)
	kinds := collect(bus, uimsg.TopicPaletteOpen)
	require.NoError(t, cmd.Execute(nil))
	bus.WaitForPendingEvents()
	assert.Equal(t, []interface{}{uimsg.PaletteModels}, kinds.all())
}

func TestModelCommandListsProvidersAndModels(t *testing.T) {
	cmd, _, n, _, _ := newModelCommand(t)
	require.NoError(t, cmd.Execute([]string{"list"}))
	require.Len(t, n.SystemMessages, 1)
	msg := n.SystemMessages[0]
	assert.Contains(t, msg, "Current: genai/gemini-3.7-flash")
	assert.Contains(t, msg, "genai: cannot list models")
	assert.Contains(t, msg, "• openai/gpt-5.5-mini")
}

func TestModelCommandDefaultPersists(t *testing.T) {
	cmd, g, n, _, d := newModelCommand(t)
	require.NoError(t, cmd.Execute([]string{"default", "openai/gpt-5.5"}))
	assert.Equal(t, "openai/gpt-5.5", d.saved["Model"])
	assert.Equal(t, "gpt-5.5", g.model.Model)
	assert.Contains(t, n.SystemMessages[1], "every start")

	require.NoError(t, cmd.Execute([]string{"default", "clear"}))
	assert.Nil(t, d.saved["Model"])
	assert.True(t, g.model.IsZero())

	cmd.defaults = nil
	assert.Error(t, cmd.Execute([]string{"default", "x"}))
}

func TestModelCommandRejectsBadSpecs(t *testing.T) {
	cmd, g, _, _, _ := newModelCommand(t)
	assert.Error(t, cmd.Execute([]string{"/"}), "an empty spec must not act as reset")
	assert.Error(t, cmd.Execute([]string{"gpt", "5"}), "several words are not one model")
	assert.True(t, g.model.IsZero())
}

func TestModelCommandDefaultClearWarnsAboutGlobal(t *testing.T) {
	cmd, _, n, _, d := newModelCommand(t)
	d.global = "openai/gpt-5.5"
	require.NoError(t, cmd.Execute([]string{"default", "clear"}))
	assert.Contains(t, n.SystemMessages[len(n.SystemMessages)-1], "~/.genie/settings.tui.json still sets")
}
