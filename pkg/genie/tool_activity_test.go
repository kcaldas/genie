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

func TestActivityLogDropsTheMiddleAtCap(t *testing.T) {
	log := newActivityLog()
	total := maxActivitiesPerRequest + 10
	for i := range total {
		log.add(events.ToolActivity{Tool: fmt.Sprintf("tool-%d", i)})
	}

	activities := log.drain()

	require.Len(t, activities, maxActivitiesPerRequest+1, "capped entries plus one omission marker")

	head := maxActivitiesPerRequest / 2
	assert.Equal(t, "tool-0", activities[0].Tool)
	assert.Equal(t, fmt.Sprintf("tool-%d", head-1), activities[head-1].Tool)

	marker := activities[head]
	assert.Contains(t, marker.Summary, "10 more actions")

	assert.Equal(t, fmt.Sprintf("tool-%d", total-1), activities[len(activities)-1].Tool)
}
