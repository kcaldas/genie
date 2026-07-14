package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	anthropic_sdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/kcaldas/genie/pkg/llm/shared/toolpayload"
)

// turnState drives one chat turn against the Anthropic Messages API for
// the shared agent loop. It owns the provider-native conversation
// ([]anthropic_sdk.MessageParam) and appends assistant messages and
// tool_result messages as the loop advances.
type turnState struct {
	client *Client
	// params carries the per-turn request template (model, max tokens,
	// system blocks with cache markers, tools); Messages is set per step.
	params      anthropic_sdk.MessageNewParams
	messages    []anthropic_sdk.MessageParam
	hasHandlers bool
	toolUsed    bool
}

func (c *Client) newTurn(prompt ai.Prompt) (*turnState, error) {
	params, err := c.buildParams(prompt)
	if err != nil {
		return nil, err
	}
	return &turnState{
		client:      c,
		params:      params,
		messages:    append([]anthropic_sdk.MessageParam(nil), params.Messages...),
		hasHandlers: len(prompt.Handlers) > 0,
	}, nil
}

// Step runs one model request. With emit set it streams; otherwise it
// performs a single blocking generation call.
func (t *turnState) Step(ctx context.Context, emit func(*ai.StreamChunk)) (llmshared.StepOutcome, error) {
	params := t.params
	params.Messages = t.messages
	if emit != nil {
		return t.stepStreaming(ctx, params, emit)
	}
	return t.stepBlocking(ctx, params)
}

func (t *turnState) stepBlocking(ctx context.Context, params anthropic_sdk.MessageNewParams) (llmshared.StepOutcome, error) {
	c := t.client

	resp, err := c.messages.New(ctx, params)
	if err != nil {
		return llmshared.StepOutcome{}, fmt.Errorf("anthropic messages: %w", err)
	}

	c.publishUsage(string(params.Model), resp.Usage)

	showThinking := c.config.GetBoolWithDefault("ANTHROPIC_SHOW_THINKING", false)
	responseText, toolCalls := c.parseResponse(resp, showThinking)
	responseText = strings.TrimSpace(responseText)

	if len(toolCalls) == 0 {
		t.messages = append(t.messages, resp.ToParam())
		if responseText == "" {
			// An empty answer is fine after tool work; without any it is
			// a hard failure.
			if t.toolUsed {
				return llmshared.StepOutcome{}, nil
			}
			return llmshared.StepOutcome{}, errEmptyResponse
		}
		return llmshared.StepOutcome{Text: responseText}, nil
	}

	t.toolUsed = true

	// Interim assistant text before tool calls surfaces as a notification
	// in the blocking path (streaming delivers it through the stream).
	if responseText != "" {
		c.publishText(responseText)
	}

	if !t.hasHandlers {
		return llmshared.StepOutcome{}, fmt.Errorf("model requested %d tool calls but no handlers were provided", len(toolCalls))
	}

	return t.recordAssistantStep(resp.ToParam(), toolCalls)
}

func (t *turnState) stepStreaming(ctx context.Context, params anthropic_sdk.MessageNewParams, emit func(*ai.StreamChunk)) (llmshared.StepOutcome, error) {
	c := t.client

	stream := c.messages.NewStreaming(ctx, params)
	defer stream.Close()

	acc := &anthropic_sdk.Message{}
	showThinking := c.config.GetBoolWithDefault("ANTHROPIC_SHOW_THINKING", false)

	for stream.Next() {
		event := stream.Current()
		if err := acc.Accumulate(event); err != nil {
			return llmshared.StepOutcome{}, err
		}
		if deltaEvent, ok := event.AsAny().(anthropic_sdk.ContentBlockDeltaEvent); ok {
			switch delta := deltaEvent.Delta.AsAny().(type) {
			case anthropic_sdk.TextDelta:
				emit(&ai.StreamChunk{Text: delta.Text})
			case anthropic_sdk.ThinkingDelta:
				thinking := strings.TrimSpace(delta.Thinking)
				if thinking != "" {
					if showThinking {
						notification := events.NotificationEvent{
							Message:     thinking,
							ContentType: "thought",
						}
						c.eventBus.Publish(notification.Topic(), notification)
					}
					emit(&ai.StreamChunk{Thinking: thinking})
				}
			}
		}
	}

	if err := stream.Err(); err != nil {
		return llmshared.StepOutcome{}, fmt.Errorf("anthropic streaming: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return llmshared.StepOutcome{}, err
	}

	c.publishUsage(string(params.Model), acc.Usage)
	if tc := usageTokenCount(acc.Usage); tc != nil {
		emit(&ai.StreamChunk{TokenCount: tc})
	}

	responseText, toolCalls := c.parseResponse(acc, false)
	responseText = strings.TrimSpace(responseText)

	if len(toolCalls) == 0 {
		t.messages = append(t.messages, acc.ToParam())
		if responseText == "" {
			return llmshared.StepOutcome{}, errEmptyResponse
		}
		// The text already reached the consumer through emit.
		return llmshared.StepOutcome{Text: responseText}, nil
	}

	t.toolUsed = true

	if !t.hasHandlers {
		return llmshared.StepOutcome{}, fmt.Errorf("model requested %d tool calls but no handlers were provided", len(toolCalls))
	}

	emit(&ai.StreamChunk{ToolCalls: toolCallChunks(toolCalls)})

	return t.recordAssistantStep(acc.ToParam(), toolCalls)
}

// recordAssistantStep converts the step's tool_use blocks for the shared
// loop and appends the assistant message to the conversation. Duplicate
// calls (same name and args) are dropped from both the outgoing calls and
// the recorded message — mirroring the driver's within-step dedupe — so
// every tool_use kept in history receives a matching tool_result.
func (t *turnState) recordAssistantStep(message anthropic_sdk.MessageParam, toolCalls []toolCall) (llmshared.StepOutcome, error) {
	calls := make([]llmshared.ToolCall, 0, len(toolCalls))
	seen := make(map[string]bool, len(toolCalls))
	var droppedIDs map[string]bool

	for _, call := range toolCalls {
		args := map[string]any{}
		if len(call.Input) > 0 && string(call.Input) != "null" {
			if err := json.Unmarshal(call.Input, &args); err != nil {
				return llmshared.StepOutcome{}, fmt.Errorf("invalid arguments for tool %q: %w", call.Name, err)
			}
		}
		fp := fingerprintToolCall(call.Name, args)
		if seen[fp] {
			if droppedIDs == nil {
				droppedIDs = make(map[string]bool)
			}
			droppedIDs[call.ID] = true
			continue
		}
		seen[fp] = true
		calls = append(calls, llmshared.ToolCall{ID: call.ID, Name: call.Name, Args: args})
	}

	if len(droppedIDs) > 0 {
		log.Printf("WARNING: model repeated identical tool calls; dropped %d duplicate tool_use block(s) from one response", len(droppedIDs))
		message.Content = dropToolUseBlocks(message.Content, droppedIDs)
	}

	t.messages = append(t.messages, message)
	return llmshared.StepOutcome{ToolCalls: calls}, nil
}

// AddToolResults converts executed tool results into a tool_result user
// message correlated by tool_use ID (plus any image payloads, which
// follow as separate user messages).
func (t *turnState) AddToolResults(ctx context.Context, results []llmshared.ToolResult) error {
	c := t.client

	toolResultBlocks := make([]anthropic_sdk.ContentBlockParamUnion, 0, len(results))
	var imageMessages []anthropic_sdk.MessageParam

	for _, res := range results {
		result := res.Result
		if res.Err != nil {
			// Return the error to the model as a tool result so it can
			// recover (apologise, retry with different args, escalate)
			// instead of aborting the conversation. We still log the
			// failure for ops visibility.
			c.eventBus.Publish(events.NotificationEvent{}.Topic(), events.NotificationEvent{
				Message: fmt.Sprintf("tool %s returned error: %v", res.Call.Name, res.Err),
			})
			result = map[string]any{
				"error": fmt.Sprintf("tool %q returned an error: %v", res.Call.Name, res.Err),
			}
		}

		if res.Call.Name == "viewImage" {
			img, sanitized, err := toolpayload.Extract(result)
			if err != nil {
				return fmt.Errorf("invalid viewImage response: %w", err)
			}
			result = sanitized
			if img != nil {
				blocks := []anthropic_sdk.ContentBlockParamUnion{}
				if text := toolpayload.SanitizePath(img.Path); text != "" {
					blocks = append(blocks, anthropic_sdk.NewTextBlock(fmt.Sprintf("Image retrieved from %s", text)))
				}
				blocks = append(blocks, anthropic_sdk.NewImageBlockBase64(img.MIMEType, img.Base64Data))
				imageMessages = append(imageMessages, anthropic_sdk.NewUserMessage(blocks...))
			}
		}

		payload, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("unable to marshal response for tool %q: %w", res.Call.Name, err)
		}

		toolResultBlocks = append(toolResultBlocks, anthropic_sdk.NewToolResultBlock(res.Call.ID, string(payload), false))
	}

	if len(toolResultBlocks) > 0 {
		t.messages = append(t.messages, anthropic_sdk.NewUserMessage(toolResultBlocks...))
	}
	t.messages = append(t.messages, imageMessages...)
	return nil
}

// dropToolUseBlocks filters out the tool_use blocks whose IDs were
// deduped away, keeping every other block untouched.
func dropToolUseBlocks(blocks []anthropic_sdk.ContentBlockParamUnion, ids map[string]bool) []anthropic_sdk.ContentBlockParamUnion {
	kept := make([]anthropic_sdk.ContentBlockParamUnion, 0, len(blocks))
	for _, block := range blocks {
		if block.OfToolUse != nil && ids[block.OfToolUse.ID] {
			continue
		}
		kept = append(kept, block)
	}
	return kept
}

// fingerprintToolCall mirrors the shared driver's call fingerprint so the
// duplicates dropped here are exactly the calls the driver would drop.
func fingerprintToolCall(name string, args map[string]any) string {
	var sb strings.Builder
	sb.WriteString(name)
	sb.WriteByte('(')
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%s=%v", k, args[k])
	}
	sb.WriteByte(')')
	return sb.String()
}

// toolCallChunks converts tool_use calls into stream chunks, falling back
// to the raw input when the arguments are not valid JSON.
func toolCallChunks(toolCalls []toolCall) []*ai.ToolCallChunk {
	chunks := make([]*ai.ToolCallChunk, 0, len(toolCalls))
	for _, call := range toolCalls {
		var params map[string]any
		if len(call.Input) > 0 && string(call.Input) != "null" {
			if err := json.Unmarshal(call.Input, &params); err != nil {
				params = map[string]any{
					"raw": string(call.Input),
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

// usageTokenCount converts Anthropic usage into a stream TokenCount, or
// nil when the step reported no tokens.
func usageTokenCount(usage anthropic_sdk.Usage) *ai.TokenCount {
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		return nil
	}
	return &ai.TokenCount{
		InputTokens:  int32(usage.InputTokens),
		OutputTokens: int32(usage.OutputTokens),
		TotalTokens:  int32(usage.InputTokens + usage.OutputTokens),
	}
}
