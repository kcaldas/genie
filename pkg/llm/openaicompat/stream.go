package openaicompat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

// StreamChat streams a single tool-free generation as an ai.Stream.
func (c *Core) StreamChat(ctx context.Context, req ChatRequest) ai.Stream {
	streamCtx, cancel := context.WithCancel(ctx)
	ch := make(chan llmshared.StreamResult, 1)

	go c.runStreamingChat(streamCtx, ch, req)

	return llmshared.NewChunkStream(streamCtx, cancel, ch)
}

// BlockingLoopStream executes the shared tool loop without streaming
// and emits the final text as one chunk. Used when tools are involved:
// the streaming API is not used for tool execution.
func (c *Core) BlockingLoopStream(ctx context.Context, turn *Turn, cfg llmshared.LoopConfig) ai.Stream {
	streamCtx, cancel := context.WithCancel(ctx)
	ch := make(chan llmshared.StreamResult, 1)

	go func() {
		defer close(ch)
		defer llmshared.RecoverToStream(ch)

		resp, err := llmshared.RunToolLoop(streamCtx, turn, turn.Handlers(), cfg, nil)
		if err != nil {
			select {
			case ch <- llmshared.StreamResult{Err: err}:
			case <-streamCtx.Done():
			}
			return
		}
		select {
		case ch <- llmshared.StreamResult{Chunk: &ai.StreamChunk{Text: resp}}:
		case <-streamCtx.Done():
		}
	}()

	return llmshared.NewChunkStream(streamCtx, cancel, ch)
}

func (c *Core) emitStreamChunk(ctx context.Context, ch chan<- llmshared.StreamResult, chunk *ai.StreamChunk) error {
	if chunk == nil {
		return nil
	}
	select {
	case ch <- llmshared.StreamResult{Chunk: chunk}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type toolCallAccumulator struct {
	ID        string
	Name      string
	Type      string
	Arguments strings.Builder
}

// flushToolCallChunks converts accumulated tool-call deltas into
// consumer-facing chunks. Unparseable argument payloads are surfaced
// raw rather than dropped.
func flushToolCallChunks(acc map[string]*toolCallAccumulator) []*ai.ToolCallChunk {
	if len(acc) == 0 {
		return nil
	}
	chunks := make([]*ai.ToolCallChunk, 0, len(acc))
	for _, call := range acc {
		params := map[string]any{}
		args := strings.TrimSpace(call.Arguments.String())
		if args != "" {
			if err := json.Unmarshal([]byte(args), &params); err != nil {
				params = map[string]any{
					"raw": args,
				}
			}
		}
		chunks = append(chunks, &ai.ToolCallChunk{
			ID:         call.ID,
			Name:       call.Name,
			Parameters: params,
		})
	}
	return chunks
}

// runStreamingChat streams a single tool-free generation. Tool-call
// deltas are accumulated and reported to the consumer as chunks, but
// never executed (tool execution uses the blocking loop). Streamed
// reasoning is aggregated into one thinking event per response.
func (c *Core) runStreamingChat(ctx context.Context, ch chan<- llmshared.StreamResult, req ChatRequest) {
	defer close(ch)
	defer llmshared.RecoverToStream(ch)

	acc := make(map[string]*toolCallAccumulator)
	var reasoning strings.Builder

	err := c.SendChatStream(ctx, req, func(resp *ChatStreamResponse) error {
		if resp.Error != nil && resp.Error.Message != "" {
			return fmt.Errorf("%s error: %s", c.Provider, resp.Error.Message)
		}

		for _, choice := range resp.Choices {
			delta := choice.Delta
			reasoning.WriteString(delta.ReasoningText())

			if text := delta.Text(); text != "" {
				if err := c.emitStreamChunk(ctx, ch, &ai.StreamChunk{Text: text}); err != nil {
					return err
				}
			}

			if len(delta.ToolCalls) > 0 {
				for _, call := range delta.ToolCalls {
					entry := acc[call.ID]
					if entry == nil {
						entry = &toolCallAccumulator{ID: call.ID, Type: call.Type}
						acc[call.ID] = entry
					}
					if call.Function.Name != "" {
						entry.Name = call.Function.Name
					}
					if call.Function.Arguments != "" {
						entry.Arguments.WriteString(call.Function.Arguments)
					}
				}
			}

			// Flush only on an explicit finish: argument fragments
			// arrive across several deltas, and flushing early would
			// emit partial, unparseable calls. Anything still pending
			// at end of stream is flushed below.
			if len(acc) > 0 && (choice.FinishReason == "tool_calls" || choice.FinishReason == "stop") {
				if err := c.emitStreamChunk(ctx, ch, &ai.StreamChunk{ToolCalls: flushToolCallChunks(acc)}); err != nil {
					return err
				}
				acc = make(map[string]*toolCallAccumulator)
			}
		}

		if resp.Usage != nil {
			tokenCount := c.PublishUsage(ctx, req.Model, resp.Usage)
			if tokenCount != nil {
				if err := c.emitStreamChunk(ctx, ch, &ai.StreamChunk{TokenCount: tokenCount}); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err == nil && len(acc) > 0 {
		_ = c.emitStreamChunk(ctx, ch, &ai.StreamChunk{ToolCalls: flushToolCallChunks(acc)})
	}

	// One aggregated data event per response (session recording et al.).
	if text := strings.TrimSpace(reasoning.String()); err == nil && text != "" {
		thinkingEvent := events.ThinkingEvent{Text: text}
		c.EventBus.PublishSync(thinkingEvent.Topic(), thinkingEvent)
	}

	if err != nil && ctx.Err() == nil {
		select {
		case ch <- llmshared.StreamResult{Err: err}:
		case <-ctx.Done():
		}
	}
}
