package shared

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptedTurn replays a fixed sequence of step outcomes and records
// every tool result fed back to it.
type scriptedTurn struct {
	steps     []func() (StepOutcome, error)
	stepIndex int
	fedBack   [][]ToolResult
}

func (s *scriptedTurn) Step(ctx context.Context, emit func(*ai.StreamChunk)) (StepOutcome, error) {
	if s.stepIndex >= len(s.steps) {
		return StepOutcome{}, fmt.Errorf("scriptedTurn: unexpected step %d", s.stepIndex)
	}
	step := s.steps[s.stepIndex]
	s.stepIndex++
	return step()
}

func (s *scriptedTurn) AddToolResults(ctx context.Context, results []ToolResult) error {
	s.fedBack = append(s.fedBack, results)
	return nil
}

func outcome(o StepOutcome) func() (StepOutcome, error) {
	return func() (StepOutcome, error) { return o, nil }
}

func stepErr(err error) func() (StepOutcome, error) {
	return func() (StepOutcome, error) { return StepOutcome{}, err }
}

func echoHandlers(t *testing.T) (map[string]ai.HandlerFunc, *[]string) {
	t.Helper()
	var invoked []string
	handlers := map[string]ai.HandlerFunc{
		"lookup": func(ctx context.Context, params map[string]any) (map[string]any, error) {
			invoked = append(invoked, fmt.Sprintf("lookup(%v)", params["q"]))
			return map[string]any{"answer": "42"}, nil
		},
		"failing": func(ctx context.Context, params map[string]any) (map[string]any, error) {
			invoked = append(invoked, "failing()")
			return nil, errors.New("tool exploded")
		},
	}
	return handlers, &invoked
}

func TestRunToolLoopAnswersWithoutTools(t *testing.T) {
	turn := &scriptedTurn{steps: []func() (StepOutcome, error){
		outcome(StepOutcome{Text: "direct answer"}),
	}}

	text, err := RunToolLoop(context.Background(), turn, nil, LoopConfig{}, nil)
	require.NoError(t, err)
	assert.Equal(t, "direct answer", text)
}

func TestRunToolLoopExecutesToolsAndFeedsResultsBack(t *testing.T) {
	turn := &scriptedTurn{steps: []func() (StepOutcome, error){
		outcome(StepOutcome{ToolCalls: []ToolCall{{ID: "1", Name: "lookup", Args: map[string]any{"q": "meaning"}}}}),
		outcome(StepOutcome{Text: "the answer is 42"}),
	}}
	handlers, invoked := echoHandlers(t)

	text, err := RunToolLoop(context.Background(), turn, handlers, LoopConfig{}, nil)
	require.NoError(t, err)
	assert.Equal(t, "the answer is 42", text)
	assert.Equal(t, []string{"lookup(meaning)"}, *invoked)

	require.Len(t, turn.fedBack, 1)
	require.Len(t, turn.fedBack[0], 1)
	assert.Equal(t, map[string]any{"answer": "42"}, turn.fedBack[0][0].Result)
	assert.NoError(t, turn.fedBack[0][0].Err)
}

// Tool failures are information for the model, not fatal errors.
func TestRunToolLoopFeedsToolErrorsBackToModel(t *testing.T) {
	turn := &scriptedTurn{steps: []func() (StepOutcome, error){
		outcome(StepOutcome{ToolCalls: []ToolCall{{Name: "failing"}}}),
		outcome(StepOutcome{Text: "I could not look that up"}),
	}}
	handlers, _ := echoHandlers(t)

	text, err := RunToolLoop(context.Background(), turn, handlers, LoopConfig{}, nil)
	require.NoError(t, err)
	assert.Equal(t, "I could not look that up", text)

	require.Len(t, turn.fedBack, 1)
	assert.ErrorContains(t, turn.fedBack[0][0].Err, "tool exploded")
}

func TestRunToolLoopReportsUnknownToolsToModel(t *testing.T) {
	turn := &scriptedTurn{steps: []func() (StepOutcome, error){
		outcome(StepOutcome{ToolCalls: []ToolCall{{Name: "hallucinatedTool"}}}),
		outcome(StepOutcome{Text: "sorry"}),
	}}
	handlers, _ := echoHandlers(t)

	_, err := RunToolLoop(context.Background(), turn, handlers, LoopConfig{}, nil)
	require.NoError(t, err)
	require.Len(t, turn.fedBack, 1)
	assert.ErrorContains(t, turn.fedBack[0][0].Err, "unknown tool")
}

func TestRunToolLoopDedupesIdenticalCallsWithinStep(t *testing.T) {
	call := ToolCall{Name: "lookup", Args: map[string]any{"q": "same"}}
	turn := &scriptedTurn{steps: []func() (StepOutcome, error){
		outcome(StepOutcome{ToolCalls: []ToolCall{call, call, call}}),
		outcome(StepOutcome{Text: "done"}),
	}}
	handlers, invoked := echoHandlers(t)

	_, err := RunToolLoop(context.Background(), turn, handlers, LoopConfig{}, nil)
	require.NoError(t, err)
	assert.Len(t, *invoked, 1, "identical duplicate calls in one step must execute once")
}

func TestRunToolLoopAbortsOnIdenticalConsecutiveCallSets(t *testing.T) {
	same := StepOutcome{ToolCalls: []ToolCall{{Name: "lookup", Args: map[string]any{"q": "loop"}}}}
	var steps []func() (StepOutcome, error)
	for i := 0; i < 10; i++ {
		steps = append(steps, outcome(same))
	}
	turn := &scriptedTurn{steps: steps}
	handlers, invoked := echoHandlers(t)

	_, err := RunToolLoop(context.Background(), turn, handlers, LoopConfig{MaxConsecutiveRepeats: 3}, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "stuck in loop")
	assert.LessOrEqual(t, len(*invoked), 3, "the loop must abort at the repeat limit, not run to MaxIterations")
}

// A model alternating between two identical call-sets (A/B/A/B...) must
// also trip the guard — a simple consecutive-duplicate counter resets
// on every alternation and never fires.
func TestRunToolLoopAbortsOnAlternatingCallSets(t *testing.T) {
	a := StepOutcome{ToolCalls: []ToolCall{{Name: "lookup", Args: map[string]any{"q": "a"}}}}
	b := StepOutcome{ToolCalls: []ToolCall{{Name: "lookup", Args: map[string]any{"q": "b"}}}}
	var steps []func() (StepOutcome, error)
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			steps = append(steps, outcome(a))
		} else {
			steps = append(steps, outcome(b))
		}
	}
	turn := &scriptedTurn{steps: steps}
	handlers, _ := echoHandlers(t)

	_, err := RunToolLoop(context.Background(), turn, handlers, LoopConfig{MaxConsecutiveRepeats: 3}, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "stuck in loop")
}

func TestRunToolLoopHonorsMaxIterations(t *testing.T) {
	var steps []func() (StepOutcome, error)
	for i := 0; i < 50; i++ {
		i := i
		steps = append(steps, outcome(StepOutcome{
			ToolCalls: []ToolCall{{Name: "lookup", Args: map[string]any{"q": fmt.Sprintf("q-%d", i)}}},
		}))
	}
	turn := &scriptedTurn{steps: steps}
	handlers, invoked := echoHandlers(t)

	_, err := RunToolLoop(context.Background(), turn, handlers, LoopConfig{MaxIterations: 5}, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "exceeded 5 tool iterations")
	assert.Len(t, *invoked, 5)
}

func TestRunToolLoopBoundsProviderRetrySteps(t *testing.T) {
	var steps []func() (StepOutcome, error)
	for i := 0; i < 10; i++ {
		steps = append(steps, outcome(StepOutcome{RetryStep: true}))
	}
	turn := &scriptedTurn{steps: steps}

	_, err := RunToolLoop(context.Background(), turn, nil, LoopConfig{MaxRetrySteps: 3}, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "malformed")
	assert.Equal(t, 4, turn.stepIndex, "retry steps must be bounded, not run to MaxIterations")
}

// A transient model failure retries the REQUEST only: previously
// executed tool side effects must not be replayed.
func TestRunToolLoopRetriesFailedStepWithoutReplayingTools(t *testing.T) {
	turn := &scriptedTurn{steps: []func() (StepOutcome, error){
		outcome(StepOutcome{ToolCalls: []ToolCall{{Name: "lookup", Args: map[string]any{"q": "x"}}}}),
		stepErr(errors.New("http 503: overloaded")),
		outcome(StepOutcome{Text: "recovered"}),
	}}
	handlers, invoked := echoHandlers(t)

	text, err := RunToolLoop(context.Background(), turn, handlers, LoopConfig{
		StepRetries: 2,
		StepBackoff: time.Millisecond,
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, "recovered", text)
	assert.Len(t, *invoked, 1, "step retry must not re-execute tools")
}

func TestRunToolLoopDoesNotRetryCancelledStep(t *testing.T) {
	turn := &scriptedTurn{steps: []func() (StepOutcome, error){
		stepErr(context.Canceled),
		outcome(StepOutcome{Text: "should never be reached"}),
	}}

	_, err := RunToolLoop(context.Background(), turn, nil, LoopConfig{
		StepRetries: 3,
		StepBackoff: time.Millisecond,
	}, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, turn.stepIndex)
}

func TestRunToolLoopStopsWhenContextCancelledBetweenSteps(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	turn := &scriptedTurn{steps: []func() (StepOutcome, error){
		func() (StepOutcome, error) {
			cancel() // user cancels while tools would run next
			return StepOutcome{ToolCalls: []ToolCall{{Name: "lookup", Args: map[string]any{"q": "x"}}}}, nil
		},
		outcome(StepOutcome{Text: "should never be reached"}),
	}}
	handlers, _ := echoHandlers(t)

	_, err := RunToolLoop(ctx, turn, handlers, LoopConfig{}, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// Distinct call-sets must never trip the repetition guard.
func TestRepetitionGuardAllowsProgress(t *testing.T) {
	guard := repetitionGuard{limit: 3}
	for i := 0; i < 20; i++ {
		calls := []ToolCall{{Name: "lookup", Args: map[string]any{"q": fmt.Sprintf("q-%d", i)}}}
		require.False(t, guard.observe(calls), "distinct call %d must not trip the guard", i)
	}
}
