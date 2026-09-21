package genie

import (
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/ctx"
)

// historyTurns converts the chat provider's kept messages into the
// prompt's provider-neutral history.
func historyTurns(messages []ctx.Message) []ai.HistoryTurn {
	if len(messages) == 0 {
		return nil
	}
	turns := make([]ai.HistoryTurn, 0, len(messages))
	for _, msg := range messages {
		turn := ai.HistoryTurn{User: msg.User, Assistant: msg.Assistant}
		for _, activity := range msg.Activities {
			turn.Actions = append(turn.Actions, ai.HistoryAction{Tool: activity.Tool, Args: activity.Args, Summary: activity.Summary})
		}
		turns = append(turns, turn)
	}
	return turns
}
