package genie

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToActivityCopiesOutcome(t *testing.T) {
	event := events.ToolExecutedEvent{
		ToolName:   "bash",
		Parameters: map[string]any{"command": "go test ./..."},
		Success:    false,
		Message:    "Failed: TestResponsesUsage",
	}

	activity := toActivity(event)

	assert.Equal(t, "bash", activity.Tool)
	assert.Equal(t, `command="go test ./..."`, activity.Args)
	assert.False(t, activity.Success)
	assert.Equal(t, "Failed: TestResponsesUsage", activity.Summary)
	assert.False(t, activity.Sticky)
}

// Sticky is resolved from the tool's explicit hint alone: set → believed,
// nil → false. No heuristics in core.
func TestToActivityResolvesStickyFromToolHint(t *testing.T) {
	sticky := true
	notSticky := false
	tests := []struct {
		name string
		hint *bool
		want bool
	}{
		{name: "tool says keep", hint: &sticky, want: true},
		{name: "tool says don't bother", hint: &notSticky, want: false},
		{name: "no opinion defaults to false", hint: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activity := toActivity(events.ToolExecutedEvent{ToolName: "edit", Sticky: tt.hint})
			assert.Equal(t, tt.want, activity.Sticky)
		})
	}
}

func TestToActivityFormatsArgsInSortedKeyOrder(t *testing.T) {
	event := events.ToolExecutedEvent{
		ToolName:   "searchInFiles",
		Parameters: map[string]any{"scope": "pkg/llm", "query": "publishUsage", "max": 8},
		Success:    true,
		Message:    "Executed",
	}

	activity := toActivity(event)

	assert.Equal(t, `max=8 query="publishUsage" scope="pkg/llm"`, activity.Args)
}

func TestToActivityTruncatesLongFields(t *testing.T) {
	longValue := strings.Repeat("x", 4000)
	event := events.ToolExecutedEvent{
		ToolName:   "writeFile",
		Parameters: map[string]any{"content": longValue},
		Success:    true,
		Message:    strings.Repeat("y", 4000),
	}

	activity := toActivity(event)

	assert.LessOrEqual(t, len(activity.Args), maxActivityFieldBytes)
	assert.LessOrEqual(t, len(activity.Summary), maxActivityFieldBytes)
	assert.True(t, strings.HasSuffix(activity.Args, "…"), "truncation must be explicit")
	assert.True(t, strings.HasSuffix(activity.Summary, "…"), "truncation must be explicit")
}

func TestActivityLogPreservesOrderBelowCap(t *testing.T) {
	log := newActivityLog()
	for i := range 3 {
		log.add(events.ToolActivity{Tool: fmt.Sprintf("tool-%d", i)})
	}

	activities := log.drain()

	require.Len(t, activities, 3)
	for i, activity := range activities {
		assert.Equal(t, fmt.Sprintf("tool-%d", i), activity.Tool)
	}
}

// Capping drops the middle, biased toward the tail: the head only needs
// to show what the turn set out to do, while the final calls are where
// the model, after exploring, actually acts.
func TestActivityLogDropsTheMiddleAtCap(t *testing.T) {
	log := newActivityLog()
	total := maxActivitiesPerRequest + 10
	for i := range total {
		log.add(events.ToolActivity{Tool: fmt.Sprintf("tool-%d", i)})
	}

	activities := log.drain()

	require.Len(t, activities, maxActivitiesPerRequest+1, "capped entries plus one omission marker")

	head := maxActivitiesPerRequest / 3
	assert.Equal(t, "tool-0", activities[0].Tool)
	assert.Equal(t, fmt.Sprintf("tool-%d", head-1), activities[head-1].Tool)

	marker := activities[head]
	assert.Contains(t, marker.Summary, "10 more actions")

	firstTail := total - (maxActivitiesPerRequest - head)
	assert.Equal(t, fmt.Sprintf("tool-%d", firstTail), activities[head+1].Tool,
		"the entry after the marker is the first kept tail entry")
	assert.Equal(t, fmt.Sprintf("tool-%d", total-1), activities[len(activities)-1].Tool)
}

// Sticky entries survive capping wherever they fall: the tool said keep
// this, and the cap must not overrule it.
func TestActivityLogKeepsStickyWhenCapping(t *testing.T) {
	log := newActivityLog()
	total := maxActivitiesPerRequest + 10
	stickyIndex := maxActivitiesPerRequest / 2 // deep in the dropped middle
	for i := range total {
		log.add(events.ToolActivity{
			Tool:   fmt.Sprintf("tool-%d", i),
			Sticky: i == stickyIndex,
		})
	}

	activities := log.drain()

	kept := 0
	stickySurvived := false
	for _, activity := range activities {
		if activity.Tool == "…" {
			continue // omission markers don't count against the cap
		}
		kept++
		if activity.Tool == fmt.Sprintf("tool-%d", stickyIndex) {
			stickySurvived = true
		}
	}
	assert.True(t, stickySurvived, "sticky entry must survive the cap")
	assert.Equal(t, maxActivitiesPerRequest, kept)

	for i := 1; i < len(activities); i++ {
		if activities[i-1].Tool == "…" || activities[i].Tool == "…" {
			continue
		}
		var prev, cur int
		fmt.Sscanf(activities[i-1].Tool, "tool-%d", &prev)
		fmt.Sscanf(activities[i].Tool, "tool-%d", &cur)
		assert.Less(t, prev, cur, "kept entries stay in execution order")
	}
}
