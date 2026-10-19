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
			turn.Actions = append(turn.Actions, ai.HistoryAction{Tool: activity.Tool, Args: activity.Args, Summary: activity.Summary, Failed: !activity.Success})
		}
		turns = append(turns, turn)
	}
	return turns
}

// historyText renders the kept turns for the recorder and the context
// viewer. Named so callers that shadow the ctx package with a context
// value can still reach the renderer.
func historyText(messages []ctx.Message) string {
	return ctx.FormatHistory(messages)
}
