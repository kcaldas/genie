package controllers

import (
	"testing"
	"time"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/layout"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	core_events "github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// simulatorGui implements types.Gui backed by a real gocui.Gui in simulator
// mode. The controllers call GetGui().Update(...) which needs a non-nil
// *gocui.Gui (Update sends to an internal channel); the main loop is never
// started, so queued update functions are never executed. PostUIUpdate runs
// callbacks immediately so the headless-safe parts execute inline.
type simulatorGui struct {
	gui *gocui.Gui
}

func (s *simulatorGui) GetGui() *gocui.Gui { return s.gui }
func (s *simulatorGui) PostUIUpdate(fn func()) {
	fn()
}

// confirmationTestEnv bundles the headless dependencies shared by the tool
// and user confirmation controller tests.
type confirmationTestEnv struct {
	gui             *simulatorGui
	stateAccessor   *state.StateAccessor
	layoutManager   *layout.LayoutManager
	inputComponent  *mockComponent
	configManager   *helpers.ConfigManager
	eventBus        core_events.EventBus
	commandEventBus *events.CommandEventBus
}

func newConfirmationTestEnv(t *testing.T) *confirmationTestEnv {
	t.Helper()

	// Isolate the config manager from the developer's real ~/.genie settings
	// so ToolConfigs contain only what each test sets explicitly.
	t.Setenv("HOME", t.TempDir())

	g, err := gocui.NewGui(gocui.OutputSimulator, true)
	require.NoError(t, err, "failed to create simulator gocui.Gui")
	t.Cleanup(g.Close)

	configManager, err := helpers.NewConfigManager()
	require.NoError(t, err)

	chatState := state.NewChatState(100)
	uiState := state.NewUIState()

	return &confirmationTestEnv{
		gui:             &simulatorGui{gui: g},
		stateAccessor:   state.NewStateAccessor(chatState, uiState),
		layoutManager:   layout.NewLayoutManager(g, &layout.LayoutConfig{}),
		inputComponent:  &mockComponent{key: "input", viewName: "input"},
		configManager:   configManager,
		eventBus:        core_events.NewEventBus(),
		commandEventBus: events.NewCommandEventBus(),
	}
}

// setAutoAccept enables auto-accept for the given tool name (in memory only).
func (e *confirmationTestEnv) setAutoAccept(t *testing.T, toolName string) {
	t.Helper()
	err := e.configManager.UpdateConfig(func(c *types.Config) {
		if c.ToolConfigs == nil {
			c.ToolConfigs = map[string]types.ToolConfig{}
		}
		c.ToolConfigs[toolName] = types.ToolConfig{AutoAccept: true}
	}, false)
	require.NoError(t, err)
}

// subscribeToolResponses captures published tool.confirmation.response events.
func (e *confirmationTestEnv) subscribeToolResponses() chan core_events.ToolConfirmationResponse {
	responses := make(chan core_events.ToolConfirmationResponse, 16)
	e.eventBus.Subscribe("tool.confirmation.response", func(ev interface{}) {
		if resp, ok := ev.(core_events.ToolConfirmationResponse); ok {
			responses <- resp
		}
	})
	return responses
}

// subscribeUserResponses captures published user.confirmation.response events.
func (e *confirmationTestEnv) subscribeUserResponses() chan core_events.UserConfirmationResponse {
	responses := make(chan core_events.UserConfirmationResponse, 16)
	e.eventBus.Subscribe("user.confirmation.response", func(ev interface{}) {
		if resp, ok := ev.(core_events.UserConfirmationResponse); ok {
			responses <- resp
		}
	})
	return responses
}

func waitForToolResponse(t *testing.T, ch chan core_events.ToolConfirmationResponse) core_events.ToolConfirmationResponse {
	t.Helper()
	select {
	case resp := <-ch:
		return resp
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for tool.confirmation.response")
		return core_events.ToolConfirmationResponse{}
	}
}

func waitForUserResponse(t *testing.T, ch chan core_events.UserConfirmationResponse) core_events.UserConfirmationResponse {
	t.Helper()
	select {
	case resp := <-ch:
		return resp
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for user.confirmation.response")
		return core_events.UserConfirmationResponse{}
	}
}

// drainBus flushes the async event bus so "no event was published" assertions
// are deterministic. After this call no further publishes are delivered.
func (e *confirmationTestEnv) drainBus(t *testing.T) {
	t.Helper()
	bus, ok := e.eventBus.(*core_events.InMemoryBus)
	require.True(t, ok, "expected *events.InMemoryBus")
	bus.Shutdown()
}

func newToolConfirmationController(t *testing.T) (*ToolConfirmationController, *confirmationTestEnv) {
	t.Helper()
	env := newConfirmationTestEnv(t)
	textViewer := component.NewTextViewerComponent(env.gui, "Text", env.configManager, env.commandEventBus)
	controller := NewToolConfirmationController(
		env.gui,
		env.stateAccessor,
		env.layoutManager,
		env.inputComponent,
		textViewer,
		env.configManager,
		env.eventBus,
		env.commandEventBus,
	)
	return controller, env
}

func toolRequest(executionID, toolName string) core_events.ToolConfirmationRequest {
	return core_events.ToolConfirmationRequest{
		ExecutionID: executionID,
		ToolName:    toolName,
		Command:     "rm -rf ./build",
		Message:     "Execute command: rm -rf ./build",
	}
}

func TestToolConfirmationController_RequestSetsWaitingState(t *testing.T) {
	controller, env := newToolConfirmationController(t)

	require.False(t, env.stateAccessor.IsWaitingConfirmation())

	err := controller.HandleToolConfirmationRequest(toolRequest("exec-1", "bash"))
	require.NoError(t, err)

	assert.True(t, env.stateAccessor.IsWaitingConfirmation(), "request should put controller in waiting state")
	require.NotNil(t, controller.ConfirmationComponent, "request should record a pending confirmation")
	assert.Equal(t, "exec-1", controller.ConfirmationComponent.ExecutionID, "pending confirmation must carry the request's ExecutionID")
}

func TestToolConfirmationController_RequestViaEventBus(t *testing.T) {
	controller, env := newToolConfirmationController(t)

	// PublishSync delivers on this goroutine, exercising the constructor's
	// subscription wiring deterministically.
	env.eventBus.PublishSync("tool.confirmation.request", toolRequest("exec-bus", "bash"))

	assert.True(t, env.stateAccessor.IsWaitingConfirmation())
	require.NotNil(t, controller.ConfirmationComponent)
	assert.Equal(t, "exec-bus", controller.ConfirmationComponent.ExecutionID)
}

func TestToolConfirmationController_ConfirmPublishesResponseWithSameExecutionID(t *testing.T) {
	controller, env := newToolConfirmationController(t)
	responses := env.subscribeToolResponses()

	require.NoError(t, controller.HandleToolConfirmationRequest(toolRequest("exec-42", "bash")))

	handled, err := controller.HandleKeyPress('1')
	require.NoError(t, err)
	assert.True(t, handled)

	resp := waitForToolResponse(t, responses)
	assert.Equal(t, "exec-42", resp.ExecutionID, "response must correlate with the request's ExecutionID")
	assert.True(t, resp.Confirmed)

	assert.False(t, env.stateAccessor.IsWaitingConfirmation(), "waiting state should clear after response")
	assert.Nil(t, controller.ConfirmationComponent, "pending confirmation should clear after response")
}

func TestToolConfirmationController_DenyPublishesNegativeResponse(t *testing.T) {
	denyKeys := []struct {
		name string
		key  interface{}
	}{
		{name: "key 2", key: '2'},
		{name: "key n", key: 'n'},
		{name: "key N", key: 'N'},
		{name: "escape", key: gocui.KeyEsc},
	}

	for _, tc := range denyKeys {
		t.Run(tc.name, func(t *testing.T) {
			controller, env := newToolConfirmationController(t)
			responses := env.subscribeToolResponses()

			require.NoError(t, controller.HandleToolConfirmationRequest(toolRequest("exec-deny", "writeFile")))

			handled, err := controller.HandleKeyPress(tc.key)
			require.NoError(t, err)
			assert.True(t, handled)

			resp := waitForToolResponse(t, responses)
			assert.Equal(t, "exec-deny", resp.ExecutionID)
			assert.False(t, resp.Confirmed, "deny keys must publish Confirmed=false")

			assert.False(t, env.stateAccessor.IsWaitingConfirmation())
			assert.Nil(t, controller.ConfirmationComponent)
		})
	}
}

func TestToolConfirmationController_KeyPressWithoutActiveConfirmation(t *testing.T) {
	controller, env := newToolConfirmationController(t)
	responses := env.subscribeToolResponses()

	handled, err := controller.HandleKeyPress('1')
	require.NoError(t, err)
	assert.False(t, handled, "keys should not be handled when no confirmation is pending")

	env.drainBus(t)
	assert.Empty(t, responses, "no response should be published without a pending confirmation")
}

func TestToolConfirmationController_UnrecognizedKeyKeepsConfirmationPending(t *testing.T) {
	controller, env := newToolConfirmationController(t)
	responses := env.subscribeToolResponses()

	require.NoError(t, controller.HandleToolConfirmationRequest(toolRequest("exec-key", "bash")))

	handled, err := controller.HandleKeyPress('x')
	require.NoError(t, err)
	assert.False(t, handled, "unrecognized keys must not resolve the confirmation")

	assert.True(t, env.stateAccessor.IsWaitingConfirmation())
	require.NotNil(t, controller.ConfirmationComponent)
	assert.Equal(t, "exec-key", controller.ConfirmationComponent.ExecutionID)

	env.drainBus(t)
	assert.Empty(t, responses)
}

func TestToolConfirmationController_AutoAcceptPublishesConfirmedResponse(t *testing.T) {
	controller, env := newToolConfirmationController(t)
	env.setAutoAccept(t, "trustedTool")
	responses := env.subscribeToolResponses()

	err := controller.HandleToolConfirmationRequest(toolRequest("exec-auto", "trustedTool"))
	require.NoError(t, err)

	resp := waitForToolResponse(t, responses)
	assert.Equal(t, "exec-auto", resp.ExecutionID)
	assert.True(t, resp.Confirmed, "auto-accept must confirm the request")

	assert.False(t, env.stateAccessor.IsWaitingConfirmation(), "auto-accept should not enter waiting state")
	assert.Nil(t, controller.ConfirmationComponent, "auto-accept should not show a dialog")
}

func TestToolConfirmationController_AutoAcceptOnlyAppliesToConfiguredTool(t *testing.T) {
	controller, env := newToolConfirmationController(t)
	env.setAutoAccept(t, "trustedTool")

	require.NoError(t, controller.HandleToolConfirmationRequest(toolRequest("exec-other", "otherTool")))

	assert.True(t, env.stateAccessor.IsWaitingConfirmation(), "tools without auto-accept must still prompt")
	require.NotNil(t, controller.ConfirmationComponent)
}

func TestToolConfirmationController_UserCancelClearsStateWithoutResponse(t *testing.T) {
	controller, env := newToolConfirmationController(t)
	responses := env.subscribeToolResponses()

	require.NoError(t, controller.HandleToolConfirmationRequest(toolRequest("exec-cancel", "bash")))
	require.True(t, env.stateAccessor.IsWaitingConfirmation())

	env.commandEventBus.Emit("user.input.cancel", nil)
	env.commandEventBus.WaitForPendingEvents()

	assert.False(t, env.stateAccessor.IsWaitingConfirmation(), "cancel should clear waiting state")
	assert.Nil(t, controller.ConfirmationComponent, "cancel should discard the pending confirmation")

	env.drainBus(t)
	assert.Empty(t, responses, "cancel does not publish a confirmation response")
}

func TestToolConfirmationController_UserCancelWithoutActiveConfirmationIsNoOp(t *testing.T) {
	controller, env := newToolConfirmationController(t)

	env.commandEventBus.Emit("user.input.cancel", nil)
	env.commandEventBus.WaitForPendingEvents()

	assert.False(t, env.stateAccessor.IsWaitingConfirmation())
	assert.Nil(t, controller.ConfirmationComponent)
}
