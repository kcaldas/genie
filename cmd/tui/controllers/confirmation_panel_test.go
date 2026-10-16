package controllers

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
	core_events "github.com/kcaldas/genie/pkg/events"
)

type recordingUI struct {
	mu   sync.Mutex
	msgs chan any
}

func (r *recordingUI) Dispatch(m any) { r.msgs <- m }

type confirmEnv struct {
	panel *ConfirmationPanel
	ui    *recordingUI
	core  core_events.EventBus
	bus   *events.CommandEventBus
	state *state.StateAccessor
	chat  *state.ChatState
}

func newConfirmEnv(t *testing.T) *confirmEnv {
	t.Helper()
	chat := state.NewChatState(100)
	env := &confirmEnv{
		ui:   &recordingUI{msgs: make(chan any, 32)},
		core: core_events.NewEventBus(),
		bus:  events.NewCommandEventBus(),
		chat: chat,
	}
	env.state = state.NewStateAccessor(chat, state.NewUIState())
	env.panel = NewConfirmationPanel(env.ui, env.state, createTestConfigManager(), env.core, env.bus)
	return env
}

func (e *confirmEnv) nextRequest(t *testing.T) uimsg.ConfirmationRequested {
	t.Helper()
	for {
		select {
		case m := <-e.ui.msgs:
			if req, ok := m.(uimsg.ConfirmationRequested); ok {
				return req
			}
		case <-time.After(2 * time.Second):
			t.Fatal("no ConfirmationRequested dispatched")
		}
	}
}

func (e *confirmEnv) expectNoRequest(t *testing.T) {
	t.Helper()
	select {
	case m := <-e.ui.msgs:
		if _, ok := m.(uimsg.ConfirmationRequested); ok {
			t.Fatalf("unexpected dialog request %+v", m)
		}
	case <-time.After(100 * time.Millisecond):
	}
}

func toolResponses(e *confirmEnv) chan core_events.ToolConfirmationResponse {
	ch := make(chan core_events.ToolConfirmationResponse, 8)
	e.core.Subscribe("tool.confirmation.response", func(ev interface{}) {
		if r, ok := ev.(core_events.ToolConfirmationResponse); ok {
			ch <- r
		}
	})
	return ch
}

func userResponses(e *confirmEnv) chan core_events.UserConfirmationResponse {
	ch := make(chan core_events.UserConfirmationResponse, 8)
	e.core.Subscribe("user.confirmation.response", func(ev interface{}) {
		if r, ok := ev.(core_events.UserConfirmationResponse); ok {
			ch <- r
		}
	})
	return ch
}

func recv[T any](t *testing.T, ch chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a response")
	}
	var zero T
	return zero
}

func TestToolRequestOpensDialogAndAnswerPublishesResponse(t *testing.T) {
	env := newConfirmEnv(t)
	responses := toolResponses(env)
	env.core.Publish("tool.confirmation.request", core_events.ToolConfirmationRequest{ExecutionID: "x1", ToolName: "bash", Message: "run ls?"})
	req := env.nextRequest(t)
	if req.Kind != uimsg.ConfirmTool || req.Title != "Tool: bash" || req.Content != "run ls?" || !strings.HasPrefix(req.Prompt, "1 - Yes | 2 - No") {
		t.Fatalf("request = %+v", req)
	}
	if !env.state.IsWaitingConfirmation() {
		t.Fatal("waiting flag should be set")
	}
	env.bus.Emit(uimsg.TopicConfirmationAnswered, uimsg.ConfirmationAnswer{Kind: uimsg.ConfirmTool, ExecutionID: "x1", Confirmed: false})
	resp := recv(t, responses)
	if resp.ExecutionID != "x1" || resp.Confirmed {
		t.Fatalf("response = %+v", resp)
	}
	env.bus.WaitForPendingEvents()
	if env.state.IsWaitingConfirmation() {
		t.Fatal("waiting flag should clear after the answer")
	}
}

func TestAutoAcceptSkipsTheDialog(t *testing.T) {
	env := newConfirmEnv(t)
	cfg := env.panel.config.GetConfig()
	if cfg.ToolConfigs == nil {
		cfg.ToolConfigs = map[string]types.ToolConfig{}
	}
	cfg.ToolConfigs["bash"] = types.ToolConfig{AutoAccept: true}
	responses := toolResponses(env)
	env.core.Publish("tool.confirmation.request", core_events.ToolConfirmationRequest{ExecutionID: "auto", ToolName: "bash"})
	if resp := recv(t, responses); !resp.Confirmed || resp.ExecutionID != "auto" {
		t.Fatalf("auto-accept response = %+v", resp)
	}
	env.expectNoRequest(t)
	env.core.Publish("tool.confirmation.request", core_events.ToolConfirmationRequest{ExecutionID: "other", ToolName: "writeFile"})
	env.nextRequest(t) // not auto-accepted
}

func TestUserRequestsQueueOneAtATime(t *testing.T) {
	env := newConfirmEnv(t)
	responses := userResponses(env)
	env.core.Publish("user.confirmation.request", core_events.UserConfirmationRequest{ExecutionID: "u1", Title: "Apply", Content: "--- a\n+++ b", ContentType: "diff", FilePath: "a.go"})
	first := env.nextRequest(t)
	if first.Kind != uimsg.ConfirmUser || first.Title != "Diff: a.go" || first.ContentType != "diff" || !strings.HasPrefix(first.Prompt, "1 - Confirm | 2 - Cancel") {
		t.Fatalf("first = %+v", first)
	}
	env.core.Publish("user.confirmation.request", core_events.UserConfirmationRequest{ExecutionID: "u2", Message: "second?", ConfirmText: "Go", CancelText: "Stop"})
	env.expectNoRequest(t)
	queued, pending := env.panel.QueueStatus()
	if queued != 1 || !pending {
		t.Fatalf("queue = %d pending = %v", queued, pending)
	}
	if last := env.chat.GetLastMessage(); last == nil || last.Content != "Confirmation request queued (position 1): second?" {
		t.Fatalf("queued notice missing: %+v", last)
	}
	env.bus.Emit(uimsg.TopicConfirmationAnswered, uimsg.ConfirmationAnswer{Kind: uimsg.ConfirmUser, ExecutionID: "u1", Confirmed: true})
	if resp := recv(t, responses); resp.ExecutionID != "u1" || !resp.Confirmed {
		t.Fatalf("first response = %+v", resp)
	}
	second := env.nextRequest(t)
	if second.ExecutionID != "u2" || second.Title != "Confirm Action" || !strings.HasPrefix(second.Prompt, "1 - Go | 2 - Stop") || second.Content != "second?" {
		t.Fatalf("second = %+v", second)
	}
	env.bus.Emit(uimsg.TopicConfirmationAnswered, uimsg.ConfirmationAnswer{Kind: uimsg.ConfirmUser, ExecutionID: "u2", Confirmed: false})
	if resp := recv(t, responses); resp.ExecutionID != "u2" || resp.Confirmed {
		t.Fatalf("second response = %+v", resp)
	}
	env.bus.WaitForPendingEvents()
	if _, pending := env.panel.QueueStatus(); pending || env.state.IsWaitingConfirmation() {
		t.Fatal("nothing should remain pending")
	}
}

func TestCancelClosesWithoutAnswering(t *testing.T) {
	env := newConfirmEnv(t)
	responses := toolResponses(env)
	env.core.Publish("tool.confirmation.request", core_events.ToolConfirmationRequest{ExecutionID: "c1", ToolName: "bash"})
	env.nextRequest(t)
	env.bus.Emit("user.input.cancel", nil)
	env.bus.WaitForPendingEvents()
	select {
	case m := <-env.ui.msgs:
		if _, ok := m.(uimsg.ConfirmationClosed); !ok {
			t.Fatalf("expected ConfirmationClosed, got %#v", m)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel must close the dialog")
	}
	select {
	case r := <-responses:
		t.Fatalf("cancel must not answer, got %+v", r)
	case <-time.After(100 * time.Millisecond):
	}
	if env.state.IsWaitingConfirmation() {
		t.Fatal("waiting flag should clear on cancel")
	}
	env.bus.Emit("user.input.cancel", nil) // nothing open: no-op
	env.bus.WaitForPendingEvents()
	select {
	case m := <-env.ui.msgs:
		t.Fatalf("no-op cancel dispatched %#v", m)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestAlwaysScopeAutoAcceptsLaterRequests(t *testing.T) {
	env := newConfirmEnv(t)
	responses := toolResponses(env)
	env.core.Publish(uimsg.CoreToolConfirmationRequest, core_events.ToolConfirmationRequest{ExecutionID: "s1", ToolName: "bash", Message: "ls"})
	req := env.nextRequest(t)
	if req.ToolName != "bash" || !strings.Contains(req.Prompt, "a - Always") {
		t.Fatalf("request = %+v", req)
	}
	env.bus.Emit(uimsg.TopicConfirmationAnswered, uimsg.ConfirmationAnswer{Kind: uimsg.ConfirmTool, ExecutionID: "s1", ToolName: "bash", Confirmed: true, Scope: uimsg.ScopeSession})
	if r := recv(t, responses); !r.Confirmed {
		t.Fatalf("first response = %+v", r)
	}
	env.bus.WaitForPendingEvents()
	if tc := env.panel.config.GetConfig().ToolConfigs["bash"]; !tc.AutoAccept {
		t.Fatal("session scope must turn on auto-accept for the tool")
	}
	env.core.Publish(uimsg.CoreToolConfirmationRequest, core_events.ToolConfirmationRequest{ExecutionID: "s2", ToolName: "bash", Message: "ls again"})
	if r := recv(t, responses); !r.Confirmed || r.ExecutionID != "s2" {
		t.Fatalf("second request should auto-accept: %+v", r)
	}
	env.expectNoRequest(t)
	env.core.Publish(uimsg.CoreToolConfirmationRequest, core_events.ToolConfirmationRequest{ExecutionID: "s3", ToolName: "writeFile"})
	env.nextRequest(t) // other tools still ask
}
