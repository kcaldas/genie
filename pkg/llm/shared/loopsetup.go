package shared

import (
	"context"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
)

// NewLoopConfig maps a prompt's tool-iteration limit and the
// environment retry settings onto the shared agent-loop configuration.
// Step-level retry replaces per-provider whole-turn retries, so
// transient API failures never re-execute tool side effects.
func NewLoopConfig(configManager config.Manager, maxToolIterations int32, defaultMaxIterations int) LoopConfig {
	maxIterations := int(maxToolIterations)
	if maxIterations <= 0 {
		maxIterations = defaultMaxIterations
	}
	cfg := LoopConfig{MaxIterations: maxIterations}

	retry := ai.GetRetryConfigFromEnv(configManager)
	if retry.Enabled {
		cfg.StepRetries = retry.MaxRetries
		cfg.StepBackoff = retry.InitialBackoff
	}
	return cfg
}

// RunToolLoopStream runs the shared agent loop in a producer goroutine
// and exposes emitted chunks as an ai.Stream. Loop errors surface on
// the stream unless the consumer already cancelled it.
func RunToolLoopStream(
	ctx context.Context,
	turn TurnState,
	handlers map[string]ai.HandlerFunc,
	cfg LoopConfig,
) ai.Stream {
	streamCtx, cancel := context.WithCancel(ctx)
	ch := make(chan StreamResult, 1)

	go func() {
		defer close(ch)
		defer RecoverToStream(ch)

		emit := func(chunk *ai.StreamChunk) {
			select {
			case ch <- StreamResult{Chunk: chunk}:
			case <-streamCtx.Done():
			}
		}

		if _, err := RunToolLoop(streamCtx, turn, handlers, cfg, emit); err != nil {
			if streamCtx.Err() != nil {
				return
			}
			select {
			case ch <- StreamResult{Err: err}:
			case <-streamCtx.Done():
			}
		}
	}()

	return NewChunkStream(streamCtx, cancel, ch)
}
