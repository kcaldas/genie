package genie

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/skills"
	"github.com/kcaldas/genie/pkg/toolctx"
	"github.com/kcaldas/genie/pkg/tools"
	"github.com/stretchr/testify/require"
)

type observedSkillProvider struct {
	hostSkillProvider
	ids []string
	err error
}

func (p *observedSkillProvider) ListSkills(ctx context.Context) ([]skills.SkillMetadata, error) {
	id, _ := toolctx.SessionID(ctx)
	p.ids = append(p.ids, id)
	if p.err != nil {
		return nil, p.err
	}
	return p.hostSkillProvider.ListSkills(ctx)
}

type skillTurnRunner struct {
	run    func(context.Context) error
	calls  int
	prompt *ai.Prompt
	data   map[string]string
}

func (r *skillTurnRunner) RunPrompt(ctx context.Context, prompt *ai.Prompt, data map[string]string, _ events.EventBus) (string, error) {
	r.calls++
	r.prompt, r.data = prompt, data
	if r.run != nil {
		if err := r.run(ctx); err != nil {
			return "", err
		}
	}
	return "completed answer", nil
}
func (r *skillTurnRunner) RunPromptStream(ctx context.Context, prompt *ai.Prompt, data map[string]string, bus events.EventBus) (string, error) {
	return r.RunPrompt(ctx, prompt, data, bus)
}
func (*skillTurnRunner) CountTokens(context.Context, *ai.Prompt, map[string]string, events.EventBus) (*ai.TokenCount, error) {
	return &ai.TokenCount{}, nil
}
func (*skillTurnRunner) GetStatus() *ai.Status { return nil }

func skillChat(t *testing.T, g Genie, ctx context.Context, message string) events.ChatResponseEvent {
	t.Helper()
	responses := make(chan events.ChatResponseEvent, 1)
	unsubscribe := g.GetEventBus().Subscribe(events.ChatResponseEvent{}.Topic(), func(event interface{}) {
		responses <- event.(events.ChatResponseEvent)
	})
	defer unsubscribe()
	require.NoError(t, g.Chat(ctx, message))
	select {
	case response := <-responses:
		return response
	case <-time.After(5 * time.Second):
		t.Fatal("chat response timed out")
		return events.ChatResponseEvent{}
	}
}

func startSkillGenie(t *testing.T, provider skills.Provider) (*core, Session, *skillTurnRunner) {
	t.Helper()
	g, err := NewGenie(WithSkillProvider(provider))
	require.NoError(t, err)
	c := g.(*core)
	runner := &skillTurnRunner{}
	c.promptRunner = runner
	sess, err := c.Start(nil, nil, WithPersonaYAML([]byte("name: test\ninstruction: Test prompt\ntext: '{{.message}}'\n")))
	require.NoError(t, err)
	return c, sess, runner
}

func TestSkillProviderReceivesActualParentAndChildSessions(t *testing.T) {
	t.Chdir(t.TempDir())
	provider := &observedSkillProvider{hostSkillProvider: hostSkillProvider{name: "host-only"}}
	parent, parentSession, _ := startSkillGenie(t, provider)
	require.Contains(t, provider.ids, parentSession.GetID(), "startup context uses the actual session")
	provider.ids = nil
	response := skillChat(t, parent, toolctx.WithSessionID(context.Background(), "caller-supplied"), "parent turn")
	require.NoError(t, response.Error)
	require.NotEmpty(t, provider.ids)
	for _, id := range provider.ids {
		require.Equal(t, parentSession.GetID(), id)
	}
	child, _, err := (&nativeTaskExecutor{parent: parent}).newChildGenie()
	require.NoError(t, err)
	childSession, err := child.Start(nil, nil, WithPersonaYAML([]byte("name: child\ninstruction: Child prompt\n")))
	require.NoError(t, err)
	require.NotEqual(t, parentSession.GetID(), childSession.GetID())
	provider.ids = nil
	response = skillChat(t, child, toolctx.WithSessionID(context.Background(), parentSession.GetID()), "child turn")
	require.NoError(t, response.Error)
	require.NotEmpty(t, provider.ids)
	for _, id := range provider.ids {
		require.Equal(t, childSession.GetID(), id)
	}
}

func TestActiveSkillCatalogFailureStopsTurnAndRetainsContextForRetry(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	require.NoError(t, os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("Project instructions survive"), 0600))
	provider := &observedSkillProvider{hostSkillProvider: hostSkillProvider{name: "host-only"}}
	c, _, runner := startSkillGenie(t, provider)
	skillTool, ok := c.toolRegistry.Get("Skill")
	require.True(t, ok)
	runner.run = func(ctx context.Context) error {
		_, err := skillTool.(*tools.SkillTool).Run(ctx, tools.SkillParams{Skill: "host-only", File: "guide.md"})
		return err
	}
	require.NoError(t, skillChat(t, c, context.Background(), "Earlier question").Error)
	runner.run = nil
	provider.err = errors.New("temporary catalog outage")
	response := skillChat(t, c, context.Background(), "Failed attempt")
	require.ErrorIs(t, response.Error, provider.err)
	require.Equal(t, 1, runner.calls, "failed context must not reach the model")
	_, err := c.GetContext(context.Background())
	require.ErrorIs(t, err, provider.err)
	provider.err = nil
	require.NoError(t, skillChat(t, c, context.Background(), "Retry question").Error)
	require.Equal(t, 2, runner.calls)
	require.Contains(t, runner.data["chat"], "Earlier question")
	require.Contains(t, runner.data["chat"], "completed answer")
	require.NotContains(t, runner.data["chat"], "Failed attempt")
	require.Contains(t, runner.prompt.SystemPromptUserContext, "Project instructions survive")
	require.Contains(t, runner.prompt.SystemPromptUserContext, "host instructions")
	require.Contains(t, runner.prompt.SystemPromptUserContext, "host reference")
}
