package shared

import (
	"context"
	"log"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
)

// ModelInputAdmissionLimit returns the registry context window after reserving
// room for the next generation. Registry windows are shared by request input
// and generated output, so tool results must not spend the whole window.
func ModelInputAdmissionLimit(prompt ai.Prompt) int {
	caps := prompt.ModelCapabilities
	if caps == nil || caps.InputTokenLimit <= 0 {
		return 0
	}
	limit := caps.InputTokenLimit
	if !caps.SharedContextWindow {
		return limit
	}

	reserve := int(prompt.MaxTokens)
	if reserve <= 0 {
		reserve = caps.OutputTokenLimit
	}
	if caps.OutputTokenLimit > 0 && reserve > caps.OutputTokenLimit {
		reserve = caps.OutputTokenLimit
	}
	if reserve <= 0 {
		return limit
	}
	maxReserve := limit / 2
	if reserve > maxReserve {
		log.Printf(
			"configured max output tokens %d exceeds half the %d-token window for %q; capping the generation reserve at %d",
			reserve, limit, caps.Model, maxReserve,
		)
		reserve = maxReserve
	}
	return limit - reserve
}

// NewLoopConfig maps a prompt's tool-iteration limit and the
// environment retry settings onto the shared agent-loop configuration.
// Step-level retry replaces per-provider whole-turn retries, so
// transient API failures never re-execute tool side effects.
func NewLoopConfig(configManager config.Manager, bus events.EventBus, prompt ai.Prompt, defaultMaxIterations int) LoopConfig {
	maxIterations := int(prompt.MaxToolIterations)
	if maxIterations <= 0 {
		maxIterations = defaultMaxIterations
	}
	admissionPrompt := prompt
	if admissionPrompt.MaxTokens <= 0 && configManager != nil {
		admissionPrompt.MaxTokens = configManager.GetModelConfig().MaxTokens
	}
	cfg := LoopConfig{
		MaxIterations:   maxIterations,
		InputTokenLimit: ModelInputAdmissionLimit(admissionPrompt),
		Limits:          ToolResultLimitsFromEnv(configManager),
		Bus:             bus,
	}

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
