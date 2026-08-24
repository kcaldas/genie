package genie

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kcaldas/genie/pkg/events"
)

// Bounds applied when converting tool events into activities. These are
// safety caps, not policy: they keep every digest small no matter what a
// tool produced. What matters (Sticky) is declared by tools, never here.
const (
	maxActivityFieldBytes   = 160
	maxActivitiesPerRequest = 50
)

// toActivity converts a tool event into its bounded activity record. This
// is the only door: truncation happens here and nowhere else.
func toActivity(event events.ToolExecutedEvent) events.ToolActivity {
	return events.ToolActivity{
		Tool:    event.ToolName,
		Args:    truncateActivityField(formatActivityArgs(event.Parameters)),
		Success: event.Success,
		Summary: truncateActivityField(event.Message),
		// Sticky iff the tool said so — nil (no opinion) resolves to
		// false, and core applies no heuristics of its own.
		Sticky: event.Sticky != nil && *event.Sticky,
	}
}

// formatActivityArgs renders parameters as a deterministic one-liner:
// `key="value"` pairs in sorted key order.
func formatActivityArgs(params map[string]any) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		switch value := params[key].(type) {
		case string:
			pairs = append(pairs, fmt.Sprintf("%s=%q", key, value))
		default:
			pairs = append(pairs, fmt.Sprintf("%s=%v", key, value))
		}
	}
	return strings.Join(pairs, " ")
}

// truncateActivityField caps a field at maxActivityFieldBytes with an
// explicit marker, cutting at a rune boundary.
func truncateActivityField(text string) string {
	if len(text) <= maxActivityFieldBytes {
		return text
	}
	marker := "…"
	cut := maxActivityFieldBytes - len(marker)
	for cut > 0 && !isRuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + marker
}

func isRuneStart(b byte) bool {
	return b&0xC0 != 0x80
}

// subscribeActivityCollector accumulates each chat request's tool digest
// from the synchronously published tool events, so it is complete by the
// time processChat returns. Events without a request ID (tools run
// outside a chat turn) are not collected.
func (g *core) subscribeActivityCollector() {
	g.eventBus.Subscribe(events.ToolExecutedEvent{}.Topic(), func(event any) {
		e, ok := event.(events.ToolExecutedEvent)
		if !ok || e.RequestID == "" {
			return
		}
		g.activityMu.Lock()
		defer g.activityMu.Unlock()
		log, found := g.activities[e.RequestID]
		if !found {
			log = newActivityLog()
			g.activities[e.RequestID] = log
		}
		log.add(toActivity(e))
	})
}

// takeActivities removes and returns a request's accumulated digest.
// Called exactly once per request at turn end (including the panic
// path), so buckets never leak.
func (g *core) takeActivities(requestID string) []events.ToolActivity {
	g.activityMu.Lock()
	defer g.activityMu.Unlock()
	log, found := g.activities[requestID]
	if !found {
		return nil
	}
	delete(g.activities, requestID)
	return log.drain()
}

// activityLog accumulates one request's activities. Entries are bounded
// individually at creation; the collection is capped at drain time,
// where the full turn is known and sticky entries can be honored.
type activityLog struct {
	entries []events.ToolActivity
}

func newActivityLog() *activityLog {
	return &activityLog{}
}

func (l *activityLog) add(activity events.ToolActivity) {
	l.entries = append(l.entries, activity)
}

// drain returns the turn's activities in execution order, capped at
// maxActivitiesPerRequest with omission markers in place of dropped
// runs. Selection under the cap:
//
//   - Sticky entries survive wherever they fall — the tool said keep
//     this, and the cap must not overrule it (newest preferred if
//     sticky alone exceeds the cap).
//   - The remaining budget drops the middle, biased toward the tail
//     (1/3 head, 2/3 tail): the head shows what the turn set out to
//     do; the final calls are where the model, after exploring,
//     actually acts.
func (l *activityLog) drain() []events.ToolActivity {
	if len(l.entries) <= maxActivitiesPerRequest {
		return l.entries
	}

	keep := make([]bool, len(l.entries))
	budget := maxActivitiesPerRequest

	for i := len(l.entries) - 1; i >= 0 && budget > 0; i-- {
		if l.entries[i].Sticky {
			keep[i] = true
			budget--
		}
	}

	headShare := budget / 3
	tailShare := budget - headShare
	for i := len(l.entries) - 1; i >= 0 && tailShare > 0; i-- {
		if !keep[i] {
			keep[i] = true
			tailShare--
		}
	}
	for i := 0; i < len(l.entries) && headShare > 0; i++ {
		if !keep[i] {
			keep[i] = true
			headShare--
		}
	}

	activities := make([]events.ToolActivity, 0, maxActivitiesPerRequest+2)
	dropped := 0
	for i, entry := range l.entries {
		if !keep[i] {
			dropped++
			continue
		}
		if dropped > 0 {
			activities = append(activities, omissionMarker(dropped))
			dropped = 0
		}
		activities = append(activities, entry)
	}
	if dropped > 0 {
		activities = append(activities, omissionMarker(dropped))
	}
	return activities
}

func omissionMarker(count int) events.ToolActivity {
	return events.ToolActivity{
		Tool:    "…",
		Success: true,
		Summary: fmt.Sprintf("%d more actions omitted", count),
	}
}
