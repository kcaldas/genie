package controllers

import (
	"context"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
	"github.com/kcaldas/genie/pkg/genie"
)

// ModelStatus keeps the UI told which provider and model the next turn
// will use: once at start and again whenever the persona changes, since a
// persona may pin its own model.
type ModelStatus struct {
	genie genie.Genie
	bus   *events.CommandEventBus
}

// NewModelStatus subscribes to persona changes.
func NewModelStatus(g genie.Genie, bus *events.CommandEventBus) *ModelStatus {
	s := &ModelStatus{genie: g, bus: bus}
	bus.Subscribe(uimsg.TopicPersonaChanged, func(interface{}) { s.Announce() })
	return s
}

// Announce emits the current selection on the command bus.
func (s *ModelStatus) Announce() {
	sel := s.genie.Model(context.Background())
	s.bus.Emit(uimsg.TopicModelChanged, map[string]interface{}{"provider": sel.Provider, "model": sel.Model})
}
