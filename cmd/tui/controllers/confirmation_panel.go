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

	mu      sync.Mutex
	active  *uimsg.ConfirmationRequested // the dialog on screen, if any
	queue   []core_events.UserConfirmationRequest
	pending bool // a user confirmation is active or queued
}

// NewConfirmationPanel subscribes to the confirmation topics on both buses.
func NewConfirmationPanel(ui types.Dispatcher, st types.IStateAccessor, cfg *helpers.ConfigManager, core core_events.EventBus, bus *events.CommandEventBus) *ConfirmationPanel {
	c := &ConfirmationPanel{ui: ui, state: st, config: cfg, core: core}
	core.Subscribe("tool.confirmation.request", func(e interface{}) {
		if req, ok := e.(core_events.ToolConfirmationRequest); ok {
			c.handleTool(req)
		}
	})
	core.Subscribe("user.confirmation.request", func(e interface{}) {
		if req, ok := e.(core_events.UserConfirmationRequest); ok {
			c.handleUser(req)
		}
	})
	bus.Subscribe(uimsg.TopicConfirmationAnswered, func(e interface{}) {
		if a, ok := e.(uimsg.ConfirmationAnswer); ok {
			c.Respond(a)
		}
	})
	bus.Subscribe("user.input.cancel", func(interface{}) { c.cancel() })
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
		c.core.Publish("tool.confirmation.response", core_events.ToolConfirmationResponse{ExecutionID: req.ExecutionID, Confirmed: true})
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
		c.core.Publish("user.confirmation.response", core_events.UserConfirmationResponse{ExecutionID: req.ExecutionID, Confirmed: true})
		return
	}
	c.state.SetWaitingConfirmation(true)
	c.mu.Lock()
	if c.pending {
		c.queue = append(c.queue, req)
		pos := len(c.queue)
		c.mu.Unlock()
		c.state.AddMessage(types.Message{
			Role:    "system",
			Content: fmt.Sprintf("Confirmation request queued (position %d): %s", pos, req.Message),
		})
		c.ui.Dispatch(uimsg.ChatChanged{})
		return
	}
	c.pending = true
	c.mu.Unlock()
	c.show(dialogRequestFor(req))
}

// userRequest builds the dialog request for a user confirmation, with the
// defaults the old UI used.
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
	c.mu.Lock()
	c.active = &req
	c.mu.Unlock()
	c.ui.Dispatch(req)
}

// Respond publishes the answer to the core and moves on to the next
// queued user confirmation, if any.
func (c *ConfirmationPanel) Respond(a uimsg.ConfirmationAnswer) {
	c.mu.Lock()
	if c.active != nil && c.active.ExecutionID == a.ExecutionID {
		c.active = nil
	}
	c.mu.Unlock()
	c.logger().Debug("Confirmation answered", "id", a.ExecutionID, "confirmed", a.Confirmed)
	switch a.Kind {
	case uimsg.ConfirmTool:
		c.core.Publish("tool.confirmation.response", core_events.ToolConfirmationResponse{ExecutionID: a.ExecutionID, Confirmed: a.Confirmed})
		c.state.SetWaitingConfirmation(c.hasPending())
	case uimsg.ConfirmUser:
		c.core.Publish("user.confirmation.response", core_events.UserConfirmationResponse{ExecutionID: a.ExecutionID, Confirmed: a.Confirmed})
		c.next()
	}
}

// next shows the next queued user confirmation or clears the pending flag.
func (c *ConfirmationPanel) next() {
	c.mu.Lock()
	if len(c.queue) == 0 {
		c.pending = false
		c.mu.Unlock()
		c.state.SetWaitingConfirmation(false)
		return
	}
	req := c.queue[0]
	c.queue = c.queue[1:]
	c.mu.Unlock()
	c.show(dialogRequestFor(req))
}

func (c *ConfirmationPanel) hasPending() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pending || c.active != nil
}

// cancel drops the open dialog and the queue without answering, as Esc on
// the old prompt did.
func (c *ConfirmationPanel) cancel() {
	c.mu.Lock()
	had := c.active != nil || c.pending
	c.active, c.queue, c.pending = nil, nil, false
	c.mu.Unlock()
	if !had {
		return
	}
	c.state.SetWaitingConfirmation(false)
	c.ui.Dispatch(uimsg.ConfirmationClosed{})
}

// QueueStatus reports the queued user confirmations and whether one is
// being answered, for tests.
func (c *ConfirmationPanel) QueueStatus() (queued int, pending bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.queue), c.pending
}
