package shared

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
)

// ToolCall is a provider-neutral tool invocation requested by the model.
type ToolCall struct {
	ID   string // provider correlation id; empty when the provider has none
	Name string
	Args map[string]any
}

// ToolResult is the outcome of executing one ToolCall. Err is reported
// back to the model as the tool's failure — it does not abort the turn.
type ToolResult struct {
	Call   ToolCall
	Result map[string]any
	Err    error
}

// StepOutcome is what one model request produced.
type StepOutcome struct {
	// Text is the assistant text of this step. The driver returns the
	// final step's text (the step that requested no tools) as the
	// turn's response.
	Text string
	// ToolCalls the model asked to run before it can continue.
	ToolCalls []ToolCall
	// RetryStep signals the provider needs the step re-run after it
	// adjusted its own conversation state (e.g. malformed-tool-call
	// recovery). No tools are executed for such a step.
	RetryStep bool
}

// TurnState is the minimal per-provider surface the shared agent loop
// drives: run ONE model request against the accumulated conversation,
// and append tool results for the next request. Implementations own
// their provider-native message history.
type TurnState interface {
	Step(ctx context.Context, emit func(*ai.StreamChunk)) (StepOutcome, error)
	AddToolResults(ctx context.Context, results []ToolResult) error
}

// LoopConfig bounds and hardens the tool-calling loop.
type LoopConfig struct {
	// MaxIterations caps model steps per turn (default 20).
	MaxIterations int
	// MaxConsecutiveRepeats is how many times in a row the model may
	// request an identical tool-call set (including period-2 A/B/A/B
	// alternation) before the loop aborts (default 3).
	MaxConsecutiveRepeats int
	// MaxRetrySteps caps consecutive provider-requested step retries,
	// e.g. malformed tool-call recovery (default 3).
	MaxRetrySteps int
	// StepRetries and StepBackoff retry an individual failed model
	// request. Because the retry wraps a single request — not the whole
	// turn — tool side effects are never re-executed. Zero disables.
	StepRetries int
	StepBackoff time.Duration
}

func (c LoopConfig) withDefaults() LoopConfig {
	if c.MaxIterations <= 0 {
		c.MaxIterations = 20
	}
	if c.MaxConsecutiveRepeats <= 0 {
		c.MaxConsecutiveRepeats = 3
	}
	if c.MaxRetrySteps <= 0 {
		c.MaxRetrySteps = 3
	}
	if c.StepBackoff <= 0 {
		c.StepBackoff = time.Second
	}
	return c
}

// RunToolLoop drives the provider-neutral agent loop: step the model,
// execute requested tools, feed results back, repeat until the model
// answers without tool calls or a guard trips. It returns the final
// step's text.
//
// Guards (previously implemented only in the genai client) apply to
// every provider: duplicate calls within a step are dropped, identical
// consecutive call-sets (including period-2 alternation) abort the
// loop, provider step-retries are bounded, and each failed model
// request is retried with backoff without re-executing tool side
// effects.
func RunToolLoop(
	ctx context.Context,
	turn TurnState,
	handlers map[string]ai.HandlerFunc,
	cfg LoopConfig,
	emit func(*ai.StreamChunk),
) (string, error) {
	cfg = cfg.withDefaults()

	guard := repetitionGuard{limit: cfg.MaxConsecutiveRepeats}
	retrySteps := 0

	for iteration := 0; iteration < cfg.MaxIterations; iteration++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		outcome, err := stepWithRetry(ctx, turn, cfg, emit)
		if err != nil {
			return "", err
		}

		if outcome.RetryStep {
			retrySteps++
			if retrySteps > cfg.MaxRetrySteps {
				return "", fmt.Errorf("model produced %d consecutive malformed steps; aborting turn", retrySteps)
			}
			continue
		}
		retrySteps = 0

		if len(outcome.ToolCalls) == 0 {
			return outcome.Text, nil
		}

		calls := dedupeToolCalls(outcome.ToolCalls)
		if guard.observe(calls) {
			return "", fmt.Errorf("model stuck in loop: repeated the same tool calls %d times in a row", cfg.MaxConsecutiveRepeats)
		}

		results := executeToolCalls(ctx, calls, handlers)
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if err := turn.AddToolResults(ctx, results); err != nil {
			return "", fmt.Errorf("failed to record tool results: %w", err)
		}
	}

	return "", fmt.Errorf("turn exceeded %d tool iterations without a final answer", cfg.MaxIterations)
}

// stepWithRetry retries an individual model request on transient
// failures. Tool execution happens outside this function, so retried
// requests never replay side effects.
func stepWithRetry(ctx context.Context, turn TurnState, cfg LoopConfig, emit func(*ai.StreamChunk)) (StepOutcome, error) {
	var (
		outcome StepOutcome
		err     error
	)

	backoff := cfg.StepBackoff
	for attempt := 0; ; attempt++ {
		outcome, err = turn.Step(ctx, emit)
		if err == nil {
			return outcome, nil
		}
		if attempt >= cfg.StepRetries || !ai.IsRetryable(err) {
			return StepOutcome{}, err
		}

		log.Printf("Model step failed (attempt %d): %v. Retrying in %v...", attempt+1, err, backoff)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return StepOutcome{}, fmt.Errorf("aborted while waiting to retry model step: %w", ctx.Err())
		}
		backoff *= 2
	}
}

// executeToolCalls runs the requested tools sequentially. Handler
// errors and unknown tools become ToolResult.Err so the model can see
// and correct them; a context cancellation stops execution.
func executeToolCalls(ctx context.Context, calls []ToolCall, handlers map[string]ai.HandlerFunc) []ToolResult {
	results := make([]ToolResult, 0, len(calls))
	for _, call := range calls {
		if ctx.Err() != nil {
			results = append(results, ToolResult{Call: call, Err: ctx.Err()})
			continue
		}

		handler, ok := handlers[call.Name]
		if !ok {
			results = append(results, ToolResult{
				Call: call,
				Err:  fmt.Errorf("unknown tool %q — only registered tools may be called", call.Name),
			})
			continue
		}

		result, err := handler(ctx, call.Args)
		results = append(results, ToolResult{Call: call, Result: result, Err: err})
	}
	return results
}

// dedupeToolCalls drops exact duplicates (same name and args) within a
// single step, keeping the first occurrence.
func dedupeToolCalls(calls []ToolCall) []ToolCall {
	if len(calls) < 2 {
		return calls
	}
	seen := make(map[string]bool, len(calls))
	out := calls[:0:0]
	for _, call := range calls {
		fp := fingerprintCall(call)
		if seen[fp] {
			continue
		}
		seen[fp] = true
		out = append(out, call)
	}
	return out
}

// repetitionGuard aborts loops where the model keeps requesting the
// same tool-call set: identical consecutive sets (period 1) or an
// A/B/A/B alternation (period 2).
type repetitionGuard struct {
	limit   int
	history []string
}

// observe records a step's call-set fingerprint and reports whether
// the repetition limit has been reached.
func (g *repetitionGuard) observe(calls []ToolCall) bool {
	fingerprints := make([]string, len(calls))
	for i, call := range calls {
		fingerprints[i] = fingerprintCall(call)
	}
	sort.Strings(fingerprints)
	fp := strings.Join(fingerprints, "|")

	g.history = append(g.history, fp)
	if len(g.history) > 2*g.limit {
		g.history = g.history[len(g.history)-2*g.limit:]
	}

	return g.repeats(1) >= g.limit || g.repeats(2) >= g.limit
}

// repeats counts how many times the trailing pattern of the given
// period has repeated consecutively at the end of history.
func (g *repetitionGuard) repeats(period int) int {
	if len(g.history) < period {
		return 0
	}
	count := 0
	for i := len(g.history) - period; i >= 0; i -= period {
		match := true
		for j := 0; j < period; j++ {
			if g.history[i+j] != g.history[len(g.history)-period+j] {
				match = false
				break
			}
		}
		if !match {
			break
		}
		count++
	}
	if period == 2 {
		// An A/A/A/A run also matches period 2; require genuine
		// alternation to avoid double counting period-1 runs.
		if len(g.history) >= 2 && g.history[len(g.history)-1] == g.history[len(g.history)-2] {
			return 0
		}
	}
	return count
}

func fingerprintCall(call ToolCall) string {
	var sb strings.Builder
	sb.WriteString(call.Name)
	sb.WriteByte('(')
	keys := make([]string, 0, len(call.Args))
	for k := range call.Args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%s=%v", k, call.Args[k])
	}
	sb.WriteByte(')')
	return sb.String()
}
