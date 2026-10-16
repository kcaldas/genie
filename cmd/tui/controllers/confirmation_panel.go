package controllers

import (
	"fmt"
	"sync"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
	core_events "github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/logging"
)

// ConfirmationPanel turns the core's tool and user confirmation requests
// into dialogs and their answers back into responses. Tool requests show
// at once; user requests queue so one is answered at a time, as before.
type ConfirmationPanel struct {
	ui     types.Dispatcher
	state  types.IStateAccessor
	config *helpers.ConfigManager
	core   core_events.EventBus
	q      confirmQueue
}

// confirmQueue tracks the dialog on screen and the user confirmations
// waiting behind it. Every method is safe for concurrent use so the
// controller never holds a lock while dispatching or publishing.
type confirmQueue struct {
	mu      sync.Mutex
	active  string // execution ID of the dialog on screen, "" if none
	waiting []core_events.UserConfirmationRequest
	pending bool // a user confirmation is active or waiting
}

// show records id as the dialog on screen.
func (q *confirmQueue) show(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.active = id
}

// enqueue returns the request's position when another user confirmation
// is already pending, or claims the slot and returns 0.
func (q *confirmQueue) enqueue(req core_events.UserConfirmationRequest) (position int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.pending {
		q.waiting = append(q.waiting, req)
		return len(q.waiting)
	}
	q.pending = true
	return 0
}

// answered clears the active dialog if it is id.
func (q *confirmQueue) answered(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.active == id {
		q.active = ""
	}
}

// next pops the next waiting user confirmation, or releases the slot.
func (q *confirmQueue) next() (core_events.UserConfirmationRequest, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.waiting) == 0 {
		q.pending = false
		return core_events.UserConfirmationRequest{}, false
	}
	req := q.waiting[0]
	q.waiting = q.waiting[1:]
	return req, true
}

// clear drops everything and reports whether anything was open.
func (q *confirmQueue) clear() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	had := q.active != "" || q.pending
	q.active, q.waiting, q.pending = "", nil, false
	return had
}

func (q *confirmQueue) busy() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pending || q.active != ""
}

func (q *confirmQueue) status() (int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.waiting), q.pending
}

// NewConfirmationPanel subscribes to the confirmation topics on both buses.
func NewConfirmationPanel(ui types.Dispatcher, st types.IStateAccessor, cfg *helpers.ConfigManager, core core_events.EventBus, bus *events.CommandEventBus) *ConfirmationPanel {
	c := &ConfirmationPanel{ui: ui, state: st, config: cfg, core: core}
	core.Subscribe(uimsg.CoreToolConfirmationRequest, func(e interface{}) {
		if req, ok := e.(core_events.ToolConfirmationRequest); ok {
			c.handleTool(req)
		}
	})
	core.Subscribe(uimsg.CoreUserConfirmationRequest, func(e interface{}) {
		if req, ok := e.(core_events.UserConfirmationRequest); ok {
			c.handleUser(req)
		}
	})
	bus.Subscribe(uimsg.TopicConfirmationAnswered, func(e interface{}) {
		if a, ok := e.(uimsg.ConfirmationAnswer); ok {
			c.Respond(a)
		}
	})
	bus.Subscribe(uimsg.TopicInputCancel, func(interface{}) { c.cancel() })
	return c
}

func (c *ConfirmationPanel) logger() logging.Logger { return logging.GetGlobalLogger() }

func (c *ConfirmationPanel) autoAccept(name string) bool {
	tc, ok := c.config.GetConfig().ToolConfigs[name]
	return ok && tc.AutoAccept
}

func (c *ConfirmationPanel) handleTool(req core_events.ToolConfirmationRequest) {
	if c.autoAccept(req.ToolName) {
		c.logger().Debug("Auto-accepting confirmation for tool", "tool", req.ToolName)
		c.core.Publish(uimsg.CoreToolConfirmationResponse, core_events.ToolConfirmationResponse{ExecutionID: req.ExecutionID, Confirmed: true})
		return
	}
	c.state.SetWaitingConfirmation(true)
	c.show(uimsg.ConfirmationRequested{
		Kind:        uimsg.ConfirmTool,
		ExecutionID: req.ExecutionID,
		Title:       "Tool: " + req.ToolName,
		Prompt:      "1 - Yes | 2 - No",
		Content:     req.Message,
		ContentType: "text",
	})
}

func (c *ConfirmationPanel) handleUser(req core_events.UserConfirmationRequest) {
	if c.autoAccept(req.Title) {
		c.logger().Debug("Auto-accepting confirmation", "title", req.Title)
		c.core.Publish(uimsg.CoreUserConfirmationResponse, core_events.UserConfirmationResponse{ExecutionID: req.ExecutionID, Confirmed: true})
		return
	}
	c.state.SetWaitingConfirmation(true)
	if pos := c.q.enqueue(req); pos > 0 {
		c.state.AddMessage(types.Message{
			Role:    "system",
			Content: fmt.Sprintf("Confirmation request queued (position %d): %s", pos, req.Message),
		})
		c.ui.Dispatch(uimsg.ChatChanged{})
		return
	}
	c.show(dialogRequestFor(req))
}

// dialogRequestFor builds the dialog request for a user confirmation, with
// the defaults the old UI used.
func dialogRequestFor(req core_events.UserConfirmationRequest) uimsg.ConfirmationRequested {
	title := req.Title
	if title == "" {
		title = "Confirm Action"
	}
	confirm, cancel := req.ConfirmText, req.CancelText
	if confirm == "" {
		confirm = "Confirm"
	}
	if cancel == "" {
		cancel = "Cancel"
	}
	ctype := "text"
	switch {
	case req.ContentType == "diff" && req.Content != "":
		ctype = "diff"
		if req.FilePath != "" {
			title = "Diff: " + req.FilePath
		}
	case req.ContentType == "markdown" && req.Content != "":
		ctype = "markdown"
		if req.FilePath != "" {
			title = "Markdown: " + req.FilePath
		}
	}
	content := req.Content
	if content == "" {
		content = req.Message
	}
	return uimsg.ConfirmationRequested{
		Kind:        uimsg.ConfirmUser,
		ExecutionID: req.ExecutionID,
		Title:       title,
		Prompt:      fmt.Sprintf("1 - %s | 2 - %s", confirm, cancel),
		Content:     content,
		ContentType: ctype,
	}
}

func (c *ConfirmationPanel) show(req uimsg.ConfirmationRequested) {
	c.q.show(req.ExecutionID)
	c.ui.Dispatch(req)
}

// Respond publishes the answer to the core and moves on to the next
// queued user confirmation, if any.
func (c *ConfirmationPanel) Respond(a uimsg.ConfirmationAnswer) {
	c.q.answered(a.ExecutionID)
	c.logger().Debug("Confirmation answered", "id", a.ExecutionID, "confirmed", a.Confirmed)
	switch a.Kind {
	case uimsg.ConfirmTool:
		c.core.Publish(uimsg.CoreToolConfirmationResponse, core_events.ToolConfirmationResponse{ExecutionID: a.ExecutionID, Confirmed: a.Confirmed})
		c.state.SetWaitingConfirmation(c.q.busy())
	case uimsg.ConfirmUser:
		c.core.Publish(uimsg.CoreUserConfirmationResponse, core_events.UserConfirmationResponse{ExecutionID: a.ExecutionID, Confirmed: a.Confirmed})
		c.next()
	}
}

// next shows the next queued user confirmation or clears the waiting flag.
func (c *ConfirmationPanel) next() {
	req, ok := c.q.next()
	if !ok {
		c.state.SetWaitingConfirmation(false)
		return
	}
	c.show(dialogRequestFor(req))
}

// cancel drops the open dialog and the queue without answering, as Esc on
// the old prompt did.
func (c *ConfirmationPanel) cancel() {
	if !c.q.clear() {
		return
	}
	c.state.SetWaitingConfirmation(false)
	c.ui.Dispatch(uimsg.ConfirmationClosed{})
}

// QueueStatus reports the queued user confirmations and whether one is
// being answered, for tests.
func (c *ConfirmationPanel) QueueStatus() (queued int, pending bool) {
	return c.q.status()
}
