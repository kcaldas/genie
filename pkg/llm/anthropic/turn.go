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
)

// turnState drives one chat turn against the Anthropic Messages API for
// the shared agent loop. It owns the provider-native conversation
// ([]anthropic_sdk.MessageParam) and appends assistant messages and
// tool_result messages as the loop advances.
type turnState struct {
	client *Client
	// params carries the per-turn request template (model, max tokens,
	// system blocks with cache markers, tools); Messages is set per step.
	params       anthropic_sdk.MessageNewParams
	countParams  anthropic_sdk.MessageCountTokensParams
	messages     []anthropic_sdk.MessageParam
	hasHandlers  bool
	toolUsed     bool
	inputLimit   int
	supportsBlob func(ai.BlobContent) bool
}

func (c *Client) newTurn(prompt ai.Prompt) (*turnState, error) {
	params, err := c.buildParams(prompt)
	if err != nil {
		return nil, err
	}
	countParams, err := c.buildCountTokensParams(prompt)
	if err != nil {
		return nil, err
	}
	return &turnState{
		client:      c,
		params:      params,
		countParams: countParams,
		messages:    append([]anthropic_sdk.MessageParam(nil), params.Messages...),
		hasHandlers: len(prompt.Handlers) > 0,
		inputLimit:  anthropicInputAdmissionLimit(prompt, params.MaxTokens),
		supportsBlob: llmshared.SupportsBlobForModel(prompt.ModelCapabilities, func(blob ai.BlobContent) bool {
			return llmshared.SupportsImagesOnly(blob) || blob.MIMEType == "application/pdf"
		}),
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
	usage := physicalUsageTokenCount(resp.Usage)

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
		return llmshared.StepOutcome{Text: responseText, Usage: usage}, nil
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

	outcome, err := t.recordAssistantStep(resp.ToParam(), toolCalls)
	outcome.Usage = usage
	return outcome, err
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
	usage := physicalUsageTokenCount(acc.Usage)
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
		return llmshared.StepOutcome{Text: responseText, Usage: usage}, nil
	}

	t.toolUsed = true

	if !t.hasHandlers {
		return llmshared.StepOutcome{}, fmt.Errorf("model requested %d tool calls but no handlers were provided", len(toolCalls))
	}

	emit(&ai.StreamChunk{ToolCalls: toolCallChunks(toolCalls)})

	outcome, err := t.recordAssistantStep(acc.ToParam(), toolCalls)
	outcome.Usage = usage
	return outcome, err
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
// message correlated by tool_use ID (plus any image or document
// payloads, which follow as separate user messages).
func (t *turnState) AddToolResults(ctx context.Context, results []llmshared.PreparedToolResult, latestUsage *ai.TokenCount) error {
	resultMessage, mediaMessages, _ := buildAnthropicToolResultMessages(results, true, false, t.supportsBlob)
	if t.inputLimit <= 0 || !llmshared.NeedsExactToolResultAdmission(t.inputLimit, latestUsage, results) {
		t.appendToolResultMessages(resultMessage, mediaMessages)
		return nil
	}

	if fits, err := t.toolResultMessagesFit(ctx, resultMessage, mediaMessages); err != nil {
		log.Printf("WARNING: Anthropic tool-result admission count failed; proceeding with bounded results: %v", err)
		t.appendToolResultMessages(resultMessage, mediaMessages)
		return nil
	} else if fits {
		t.appendToolResultMessages(resultMessage, mediaMessages)
		return nil
	}

	resultMessage, _, hadMedia := buildAnthropicToolResultMessages(results, false, false, t.supportsBlob)
	if hadMedia {
		if fits, err := t.toolResultMessagesFit(ctx, resultMessage, nil); err != nil {
			log.Printf("WARNING: Anthropic media-free tool-result admission count failed; proceeding without media: %v", err)
			t.appendToolResultMessages(resultMessage, nil)
			return nil
		} else if fits {
			t.appendToolResultMessages(resultMessage, nil)
			return nil
		}
	}

	resultMessage, _, _ = buildAnthropicToolResultMessages(results, false, true, t.supportsBlob)
	if fits, err := t.toolResultMessagesFit(ctx, resultMessage, nil); err != nil {
		log.Printf("WARNING: Anthropic minimal tool-result admission count failed; proceeding with correlated omissions: %v", err)
		t.appendToolResultMessages(resultMessage, nil)
		return nil
	} else if !fits {
		return ai.NonRetryable(fmt.Errorf(
			"anthropic tool results cannot fit the model input envelope even after omission (limit %d tokens)",
			t.inputLimit,
		))
	}
	t.appendToolResultMessages(resultMessage, nil)
	return nil
}

func buildAnthropicToolResultMessages(results []llmshared.PreparedToolResult, includeMedia, omit bool, supportsBlob func(ai.BlobContent) bool) (anthropic_sdk.MessageParam, []anthropic_sdk.MessageParam, bool) {
	toolResultBlocks := make([]anthropic_sdk.ContentBlockParamUnion, 0, len(results))
	var mediaMessages []anthropic_sdk.MessageParam
	hadMedia := false

	for _, res := range results {
		if omit {
			res = llmshared.OmittedToolResult(res, "model input budget exhausted")
		}
		encoded := llmshared.EncodeToolResult(res, supportsBlob)

		for _, blob := range encoded.Blobs {
			hadMedia = true
			if !includeMedia {
				encoded.Text += "\n[" + llmshared.DescribeBlob(blob) + " omitted: model input budget exhausted]"
				continue
			}
			blocks := []anthropic_sdk.ContentBlockParamUnion{anthropic_sdk.NewTextBlock(llmshared.DescribeBlob(blob))}
			if llmshared.SupportsImagesOnly(blob) {
				blocks = append(blocks, anthropic_sdk.NewImageBlockBase64(blob.MIMEType, llmshared.BlobBase64(blob)))
			} else {
				blocks = append(blocks, anthropic_sdk.NewDocumentBlock(anthropic_sdk.Base64PDFSourceParam{Data: llmshared.BlobBase64(blob)}))
			}
			mediaMessages = append(mediaMessages, anthropic_sdk.NewUserMessage(blocks...))
		}
		toolResultBlocks = append(toolResultBlocks, anthropic_sdk.NewToolResultBlock(res.Call.ID, encoded.Text, encoded.IsError))
	}

	var resultMessage anthropic_sdk.MessageParam
	if len(toolResultBlocks) > 0 {
		resultMessage = anthropic_sdk.NewUserMessage(toolResultBlocks...)
	}
	return resultMessage, mediaMessages, hadMedia
}

func (t *turnState) appendToolResultMessages(resultMessage anthropic_sdk.MessageParam, mediaMessages []anthropic_sdk.MessageParam) {
	if len(resultMessage.Content) > 0 {
		t.messages = append(t.messages, resultMessage)
	}
	t.messages = append(t.messages, mediaMessages...)
}

func (t *turnState) toolResultMessagesFit(ctx context.Context, resultMessage anthropic_sdk.MessageParam, mediaMessages []anthropic_sdk.MessageParam) (bool, error) {
	messages := append([]anthropic_sdk.MessageParam(nil), t.messages...)
	if len(resultMessage.Content) > 0 {
		messages = append(messages, resultMessage)
	}
	messages = append(messages, mediaMessages...)

	params := t.countParams
	params.Messages = messages
	count, err := t.client.messages.CountTokens(ctx, params)
	if err != nil {
		return false, fmt.Errorf("anthropic preflight tool-result token count: %w", err)
	}
	return int(count.InputTokens) <= t.inputLimit, nil
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

// physicalUsageTokenCount includes cached reads and writes because those
// tokens still occupy the model's request envelope even though Anthropic
// reports them separately for billing and cache telemetry.
func physicalUsageTokenCount(usage anthropic_sdk.Usage) *ai.TokenCount {
	input := usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
	if input == 0 && usage.OutputTokens == 0 {
		return nil
	}
	return &ai.TokenCount{
		InputTokens:  int32(input),
		OutputTokens: int32(usage.OutputTokens),
		TotalTokens:  int32(input + usage.OutputTokens),
	}
}
