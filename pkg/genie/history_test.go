package genie

import (
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/ctx"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
)

// A recorded activity's outcome travels into the prompt history as
// Failed, so providers can replay a failed call as an error result.
func TestHistoryTurns_CarriesOutcome(t *testing.T) {
	turns := historyTurns([]ctx.Message{{
		User: "fix it",
		Activities: []events.ToolActivity{
			{Tool: "bash", Args: `command="go test"`, Success: false, Summary: "Failed: TestX"},
			{Tool: "writeFile", Args: `path="a.go"`, Success: true, Summary: "Wrote a.go"},
		},
		Assistant: "fixed",
	}})

	assert.Equal(t, []ai.HistoryTurn{{
		User: "fix it",
		Actions: []ai.HistoryAction{
			{Tool: "bash", Args: `command="go test"`, Summary: "Failed: TestX", Failed: true},
			{Tool: "writeFile", Args: `path="a.go"`, Summary: "Wrote a.go"},
		},
		Assistant: "fixed",
	}}, turns)
}

func TestHistoryTurns_EmptyIsNil(t *testing.T) {
	assert.Nil(t, historyTurns(nil))
}

// The cap's omission marker must carry the name providers fold on.
func TestOmissionMarkerUsesTheSharedName(t *testing.T) {
	marker := omissionMarker(4)
	assert.Equal(t, ai.OmittedActions, marker.Tool)
	assert.Equal(t, "4 more actions omitted", marker.Summary)
	assert.True(t, marker.Success)
}
