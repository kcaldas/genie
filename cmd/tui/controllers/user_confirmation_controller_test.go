package controllers

import (
	"fmt"
	"testing"

	"github.com/kcaldas/genie/cmd/tui/component"
	core_events "github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newUserConfirmationController(t *testing.T) (*UserConfirmationController, *confirmationTestEnv) {
	t.Helper()
	env := newConfirmationTestEnv(t)
	diffViewer := component.NewDiffViewerComponent(env.gui, "Diff", env.configManager, env.commandEventBus)
	textViewer := component.NewTextViewerComponent(env.gui, "Text", env.configManager, env.commandEventBus)
	controller := NewUserConfirmationController(
		env.gui,
		env.stateAccessor,
		env.layoutManager,
		env.inputComponent,
		diffViewer,
		textViewer,
		env.configManager,
		env.eventBus,
		env.commandEventBus,
	)
	return controller, env
}

func userRequest(executionID string) core_events.UserConfirmationRequest {
	return core_events.UserConfirmationRequest{
		ExecutionID: executionID,
		Title:       "writeFile",
		Content:     "--- a/main.go\n+++ b/main.go\n",
		ContentType: "diff",
		FilePath:    "main.go",
		Message:     "Write changes to main.go?",
	}
}

func TestUserConfirmationController_RequestSetsWaitingAndProcessing(t *testing.T) {
	controller, env := newUserConfirmationController(t)

	require.False(t, env.stateAccessor.IsWaitingConfirmation())

	err := controller.HandleUserConfirmationRequest(userRequest("exec-1"))
	require.NoError(t, err)

	assert.True(t, env.stateAccessor.IsWaitingConfirmation(), "request should put controller in waiting state")
	require.NotNil(t, controller.ConfirmationComponent, "request should record a pending confirmation")
	assert.Equal(t, "exec-1", controller.ConfirmationComponent.ExecutionID)

	queued, processing := controller.GetConfirmationQueueStatus()
	assert.Equal(t, 0, queued)
	assert.True(t, processing)
}

func TestUserConfirmationController_RequestViaEventBus(t *testing.T) {
	controller, env := newUserConfirmationController(t)

	env.eventBus.PublishSync("user.confirmation.request", userRequest("exec-bus"))

	assert.True(t, env.stateAccessor.IsWaitingConfirmation())
	require.NotNil(t, controller.ConfirmationComponent)
	assert.Equal(t, "exec-bus", controller.ConfirmationComponent.ExecutionID)
}

func TestUserConfirmationController_DefaultConfirmAndCancelText(t *testing.T) {
	controller, _ := newUserConfirmationController(t)

	// No ConfirmText/CancelText provided: defaults should be used.
	require.NoError(t, controller.HandleUserConfirmationRequest(core_events.UserConfirmationRequest{
		ExecutionID: "exec-defaults",
		Message:     "Proceed?",
	}))

	require.NotNil(t, controller.ConfirmationComponent)
	title := controller.ConfirmationComponent.GetTitle()
	assert.Contains(t, title, "1 - Confirm")
	assert.Contains(t, title, "2 - Cancel")
}

func TestUserConfirmationController_CustomConfirmAndCancelText(t *testing.T) {
	controller, _ := newUserConfirmationController(t)

	require.NoError(t, controller.HandleUserConfirmationRequest(core_events.UserConfirmationRequest{
		ExecutionID: "exec-custom",
		Message:     "Apply plan?",
		ConfirmText: "Apply",
		CancelText:  "Reject",
	}))

	require.NotNil(t, controller.ConfirmationComponent)
	title := controller.ConfirmationComponent.GetTitle()
	assert.Contains(t, title, "1 - Apply")
	assert.Contains(t, title, "2 - Reject")
}

func TestUserConfirmationController_ConfirmPublishesResponseWithSameExecutionID(t *testing.T) {
	controller, env := newUserConfirmationController(t)
	responses := env.subscribeUserResponses()

	require.NoError(t, controller.HandleUserConfirmationRequest(userRequest("exec-42")))

	handled, err := controller.HandleKeyPress('1')
	require.NoError(t, err)
	assert.True(t, handled)

	resp := waitForUserResponse(t, responses)
	assert.Equal(t, "exec-42", resp.ExecutionID, "response must correlate with the request's ExecutionID")
	assert.True(t, resp.Confirmed)

	assert.False(t, env.stateAccessor.IsWaitingConfirmation())
	assert.Nil(t, controller.ConfirmationComponent)

	queued, processing := controller.GetConfirmationQueueStatus()
	assert.Equal(t, 0, queued)
	assert.False(t, processing, "processing flag should clear once the queue is empty")
}

func TestUserConfirmationController_DenyPublishesNegativeResponse(t *testing.T) {
	controller, env := newUserConfirmationController(t)
	responses := env.subscribeUserResponses()

	require.NoError(t, controller.HandleUserConfirmationRequest(userRequest("exec-deny")))

	handled, err := controller.HandleKeyPress('2')
	require.NoError(t, err)
	assert.True(t, handled)

	resp := waitForUserResponse(t, responses)
	assert.Equal(t, "exec-deny", resp.ExecutionID)
	assert.False(t, resp.Confirmed, "denial must publish Confirmed=false")

	assert.False(t, env.stateAccessor.IsWaitingConfirmation())
	assert.Nil(t, controller.ConfirmationComponent)
}

func TestUserConfirmationController_QueuesConcurrentRequests(t *testing.T) {
	controller, env := newUserConfirmationController(t)

	require.NoError(t, controller.HandleUserConfirmationRequest(userRequest("exec-a")))
	require.NoError(t, controller.HandleUserConfirmationRequest(userRequest("exec-b")))
	require.NoError(t, controller.HandleUserConfirmationRequest(userRequest("exec-c")))

	// First request is active; the other two are queued.
	require.NotNil(t, controller.ConfirmationComponent)
	assert.Equal(t, "exec-a", controller.ConfirmationComponent.ExecutionID)

	queued, processing := controller.GetConfirmationQueueStatus()
	assert.Equal(t, 2, queued)
	assert.True(t, processing)

	// Queued requests are surfaced to the user as system messages.
	messages := env.stateAccessor.GetMessages()
	require.Len(t, messages, 2)
	for i, msg := range messages {
		assert.Equal(t, "system", msg.Role)
		assert.Contains(t, msg.Content, fmt.Sprintf("Confirmation request queued (position %d)", i+1))
	}
}

func TestUserConfirmationController_AnswersQueuedRequestsInOrder(t *testing.T) {
	controller, env := newUserConfirmationController(t)
	responses := env.subscribeUserResponses()

	require.NoError(t, controller.HandleUserConfirmationRequest(userRequest("exec-a")))
	require.NoError(t, controller.HandleUserConfirmationRequest(userRequest("exec-b")))
	require.NoError(t, controller.HandleUserConfirmationRequest(userRequest("exec-c")))

	answers := []struct {
		key       rune
		id        string
		confirmed bool
	}{
		{key: '1', id: "exec-a", confirmed: true},
		{key: '2', id: "exec-b", confirmed: false},
		{key: '1', id: "exec-c", confirmed: true},
	}

	for _, answer := range answers {
		require.NotNil(t, controller.ConfirmationComponent, "next queued confirmation should become active")
		assert.Equal(t, answer.id, controller.ConfirmationComponent.ExecutionID, "queue must be processed in FIFO order")

		handled, err := controller.HandleKeyPress(answer.key)
		require.NoError(t, err)
		require.True(t, handled)

		resp := waitForUserResponse(t, responses)
		assert.Equal(t, answer.id, resp.ExecutionID)
		assert.Equal(t, answer.confirmed, resp.Confirmed)
	}

	// Every request was answered; nothing dropped, nothing left over.
	assert.Nil(t, controller.ConfirmationComponent)
	queued, processing := controller.GetConfirmationQueueStatus()
	assert.Equal(t, 0, queued)
	assert.False(t, processing)

	env.drainBus(t)
	assert.Empty(t, responses, "no extra responses should be published")
}

func TestUserConfirmationController_AutoAcceptByTitle(t *testing.T) {
	controller, env := newUserConfirmationController(t)
	env.setAutoAccept(t, "writeFile") // user confirmations key auto-accept off event.Title
	responses := env.subscribeUserResponses()

	err := controller.HandleUserConfirmationRequest(userRequest("exec-auto"))
	require.NoError(t, err)

	resp := waitForUserResponse(t, responses)
	assert.Equal(t, "exec-auto", resp.ExecutionID)
	assert.True(t, resp.Confirmed, "auto-accept must confirm the request")

	assert.False(t, env.stateAccessor.IsWaitingConfirmation(), "auto-accept should not enter waiting state")
	assert.Nil(t, controller.ConfirmationComponent, "auto-accept should not show a dialog")

	queued, processing := controller.GetConfirmationQueueStatus()
	assert.Equal(t, 0, queued)
	assert.False(t, processing)
}

func TestUserConfirmationController_KeyPressWithoutActiveConfirmation(t *testing.T) {
	controller, env := newUserConfirmationController(t)
	responses := env.subscribeUserResponses()

	handled, err := controller.HandleKeyPress('1')
	require.NoError(t, err)
	assert.False(t, handled)

	env.drainBus(t)
	assert.Empty(t, responses)
}

func TestUserConfirmationController_UserCancelClearsStateWithoutResponse(t *testing.T) {
	controller, env := newUserConfirmationController(t)
	responses := env.subscribeUserResponses()

	require.NoError(t, controller.HandleUserConfirmationRequest(userRequest("exec-cancel")))
	require.True(t, env.stateAccessor.IsWaitingConfirmation())

	env.commandEventBus.Emit("user.input.cancel", nil)
	env.commandEventBus.WaitForPendingEvents()

	assert.False(t, env.stateAccessor.IsWaitingConfirmation(), "cancel should clear waiting state")
	assert.Nil(t, controller.ConfirmationComponent, "cancel should discard the pending confirmation")

	queued, processing := controller.GetConfirmationQueueStatus()
	assert.Equal(t, 0, queued)
	assert.False(t, processing, "cancel should stop processing")

	env.drainBus(t)
	assert.Empty(t, responses, "cancel does not publish a confirmation response")
}

func TestUserConfirmationController_UserCancelWithoutActiveConfirmationIsNoOp(t *testing.T) {
	controller, env := newUserConfirmationController(t)

	env.commandEventBus.Emit("user.input.cancel", nil)
	env.commandEventBus.WaitForPendingEvents()

	assert.False(t, env.stateAccessor.IsWaitingConfirmation())
	assert.Nil(t, controller.ConfirmationComponent)

	_, processing := controller.GetConfirmationQueueStatus()
	assert.False(t, processing)
}
