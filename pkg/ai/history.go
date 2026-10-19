package ai

// HistoryTurn is one completed exchange of the conversation: what the user
// said, what the assistant did with tools on the way, and what it answered.
// Providers lay these out as native chat messages — the user's text, the
// tool calls and their results, the reply — so the history is an
// append-only prefix their prompt caches can match.
type HistoryTurn struct {
	User      string
	Actions   []HistoryAction
	Assistant string
}

// HistoryAction is a tool call the assistant made during a turn, reduced
// to a one-line record: the tool, its bounded arguments, and its outcome.
// Providers replay it as a native tool call and result, never as prose on
// either speaker's side.
type HistoryAction struct {
	Tool    string
	Args    string // `key="value"` pairs, bounded at creation
	Summary string
	Failed  bool
}

// OmittedActions is the Tool name of a digest entry that stands in for
// calls the cap dropped. Its Summary says how many.
const OmittedActions = "…"

// TurnContext is the context assembled for one model call beyond the
// persona's rendered instruction and the conversation history. Project is
// stable across a workspace and rides with the system prompt. The rest
// changes from turn to turn and is laid out AFTER the history, in the
// final user message, so a change never invalidates the cached prefix in
// front of it.
type TurnContext struct {
	// Project is the workspace's own context: AGENTS.md, GENIE.md, CLAUDE.md
	// and the shared context files above it.
	Project string
	// Files is the accumulator of files the assistant read with tools,
	// most recent first.
	Files string
	// Skill is the instruction body of the active skill, if any.
	Skill string
	// Host is what the embedding host injects per user or conversation:
	// working memory, MEMORY.md, and similar.
	Host string
	// Tasks is the current task list, if the session keeps one.
	Tasks string
}
