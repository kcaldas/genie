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

// activityLog accumulates one request's activities under a cap that drops
// the middle: the first and last calls of a long turn carry the most
// signal (what it set out to do, how it ended).
type activityLog struct {
	head    []events.ToolActivity
	tail    []events.ToolActivity // ring once full
	tailPos int
	dropped int
}

func newActivityLog() *activityLog {
	return &activityLog{}
}

func (l *activityLog) add(activity events.ToolActivity) {
	headCap := maxActivitiesPerRequest / 2
	tailCap := maxActivitiesPerRequest - headCap

	if len(l.head) < headCap {
		l.head = append(l.head, activity)
		return
	}
	if len(l.tail) < tailCap {
		l.tail = append(l.tail, activity)
		return
	}
	l.tail[l.tailPos] = activity
	l.tailPos = (l.tailPos + 1) % tailCap
	l.dropped++
}

// drain returns the accumulated activities in order, with an omission
// marker in place of any dropped middle section.
func (l *activityLog) drain() []events.ToolActivity {
	activities := make([]events.ToolActivity, 0, len(l.head)+len(l.tail)+1)
	activities = append(activities, l.head...)
	if l.dropped > 0 {
		activities = append(activities, events.ToolActivity{
			Tool:    "…",
			Success: true,
			Summary: fmt.Sprintf("%d more actions omitted", l.dropped),
		})
	}
	activities = append(activities, l.tail[l.tailPos:]...)
	activities = append(activities, l.tail[:l.tailPos]...)
	return activities
}
