package shared

import (
	"context"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
)

// ModelInputAdmissionLimit returns the usable input-token ceiling for one
// request. Some providers report a true input-only limit; others report one
// context window shared by input and generated output. Shared windows reserve
// the requested output, bounded by the discovered model output ceiling.
func ModelInputAdmissionLimit(prompt ai.Prompt) int {
	if prompt.ModelCapabilities == nil || prompt.ModelCapabilities.InputTokenLimit <= 0 {
		return 0
	}
	caps := prompt.ModelCapabilities
	limit := caps.InputTokenLimit
	if !caps.SharedContextWindow && caps.Source != ai.CapabilitySourceFallback {
		return limit
	}

	reserve := int(prompt.MaxTokens)
	if caps.OutputTokenLimit > 0 && reserve > caps.OutputTokenLimit {
		reserve = caps.OutputTokenLimit
	}
	if reserve <= 0 {
		return limit
	}
	if reserve >= limit {
		return 1
	}
	return limit - reserve
}

// NewLoopConfig maps a prompt's tool-iteration limit and the
// environment retry settings onto the shared agent-loop configuration.
// Step-level retry replaces per-provider whole-turn retries, so
// transient API failures never re-execute tool side effects.
func NewLoopConfig(configManager config.Manager, bus events.EventBus, prompt ai.Prompt, defaultMaxIterations int) LoopConfig {
	maxToolIterations := prompt.MaxToolIterations
	maxIterations := int(maxToolIterations)
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
