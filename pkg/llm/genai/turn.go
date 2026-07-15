package genai

import (
	"context"
	"fmt"
	"iter"
	"log"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/kcaldas/genie/pkg/llm/shared/toolpayload"
	"google.golang.org/genai"
)

// turnState drives one chat turn against the Gemini API for the shared
// agent loop. It owns the provider-native conversation ([]*genai.Content)
// and appends assistant messages and tool results as the loop advances.
type turnState struct {
	client    *Client
	modelName string
	contents  []*genai.Content
	config    *genai.GenerateContentConfig
	stepCount int
	toolUsed  bool
}

func (g *Client) newTurn(p ai.Prompt) *turnState {
	return &turnState{
		client:    g,
		modelName: p.ModelName,
		contents:  g.buildInitialContents(p),
		config:    g.buildGenerateConfig(p),
	}
}

// Step runs one model request. With emit set it streams; otherwise it
// performs a single blocking generation call.
func (t *turnState) Step(ctx context.Context, emit func(*ai.StreamChunk)) (llmshared.StepOutcome, error) {
	t.stepCount++
	if emit != nil {
		return t.stepStreaming(ctx, emit)
	}
	return t.stepBlocking(ctx, emit)
}

func (t *turnState) stepBlocking(ctx context.Context, emit func(*ai.StreamChunk)) (llmshared.StepOutcome, error) {
	g := t.client

	var (
		result *genai.GenerateContentResponse
		err    error
	)
	if g.callGenerateContentFn != nil {
		result, err = g.callGenerateContentFn(ctx, t.modelName, t.contents, t.config, nil)
	} else {
		result, err = g.Client.Models.GenerateContent(ctx, t.modelName, t.contents, t.config)
	}
	if err != nil {
		return llmshared.StepOutcome{}, fmt.Errorf("error generating content: %w", err)
	}
	g.publishUsageMetadata(t.modelName, result.UsageMetadata)

	// Malformed function calls: feed the failure back to the model and
	// ask the loop to re-run the step.
	if len(result.Candidates) > 0 && result.Candidates[0].FinishReason == genai.FinishReasonMalformedFunctionCall {
		t.appendMalformedRecovery(result.Candidates[0].Content, result.Candidates[0].FinishMessage)
		return llmshared.StepOutcome{RetryStep: true}, nil
	}

	if len(result.Candidates) == 0 {
		return llmshared.StepOutcome{}, fmt.Errorf("no candidates generated")
	}

	fnCalls := result.FunctionCalls()
	if len(fnCalls) == 0 {
		return t.finalOutcome(result.Candidates[0].Content)
	}

	return t.recordAssistantStep(result.Candidates[0].Content, true), nil
}

func (t *turnState) stepStreaming(ctx context.Context, emit func(*ai.StreamChunk)) (llmshared.StepOutcome, error) {
	g := t.client

	var stream iter.Seq2[*genai.GenerateContentResponse, error]
	if g.generateContentStreamFn != nil {
		stream = g.generateContentStreamFn(ctx, t.modelName, t.contents, t.config)
	} else {
		stream = g.Client.Models.GenerateContentStream(ctx, t.modelName, t.contents, t.config)
	}

	debug := g.Config.GetBoolWithDefault("GENIE_DEBUG", false)
	chunkCount := 0
	var allParts []*genai.Part
	var lastResp *genai.GenerateContentResponse
	var lastUsageMetadata *genai.GenerateContentResponseUsageMetadata
	var lastFinishReason genai.FinishReason
	var lastFinishMessage string

	for resp, err := range stream {
		if err != nil {
			return llmshared.StepOutcome{}, fmt.Errorf("error generating streamed content: %w", err)
		}
		lastResp = resp
		chunkCount++

		if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
			for _, part := range resp.Candidates[0].Content.Parts {
				if !isEmptyPart(part) {
					if part.FunctionCall != nil {
						log.Printf("DEBUG streaming: accumulating FunctionCall name=%s args=%v (chunk %d, total parts so far: %d)",
							part.FunctionCall.Name, part.FunctionCall.Args, chunkCount, len(allParts)+1)
					}
					allParts = append(allParts, part)
				}
			}
		}
		if resp.UsageMetadata != nil {
			lastUsageMetadata = resp.UsageMetadata
		}
		if len(resp.Candidates) > 0 && resp.Candidates[0].FinishReason != "" {
			lastFinishReason = resp.Candidates[0].FinishReason
			lastFinishMessage = resp.Candidates[0].FinishMessage
		}

		if chunk := g.responseToStreamChunk(resp); chunk != nil {
			emit(chunk)
		}
	}
	if debug {
		notification := events.NotificationEvent{
			Message:     fmt.Sprintf("Stream: %d chunks, %d parts", chunkCount, len(allParts)),
			ContentType: "debug",
		}
		g.EventBus.Publish(notification.Topic(), notification)
	}
	if err := ctx.Err(); err != nil {
		return llmshared.StepOutcome{}, err
	}
	if lastResp == nil {
		return llmshared.StepOutcome{}, fmt.Errorf("no response received from model")
	}
	if err := g.checkFinishReason(lastFinishReason, lastFinishMessage); err != nil {
		return llmshared.StepOutcome{}, err
	}

	if lastFinishReason == genai.FinishReasonMalformedFunctionCall {
		var content *genai.Content
		if len(allParts) > 0 {
			content = &genai.Content{Parts: allParts, Role: "model"}
		}
		t.appendMalformedRecovery(content, lastFinishMessage)
		return llmshared.StepOutcome{RetryStep: true}, nil
	}

	if tc := g.publishUsageMetadata(t.modelName, lastUsageMetadata); tc != nil {
		emit(&ai.StreamChunk{TokenCount: tc})
	}

	accumulated := &genai.Content{Parts: allParts, Role: "model"}
	if !contentHasFunctionCalls(accumulated) {
		// The text already reached the consumer via emit; an empty final
		// step simply ends the stream.
		return llmshared.StepOutcome{Text: t.client.joinContentParts(accumulated)}, nil
	}

	// Interim text notifications are a blocking-mode concern; in
	// streaming mode the text already went out through emit.
	return t.recordAssistantStep(accumulated, false), nil
}

func contentHasFunctionCalls(content *genai.Content) bool {
	if content == nil {
		return false
	}
	for _, part := range content.Parts {
		if part != nil && part.FunctionCall != nil {
			return true
		}
	}
	return false
}

// finalOutcome converts the final (tool-free) assistant content into
// the turn's answer, preserving the historic empty-answer semantics:
// an empty answer is an error only when no tool was used this turn.
func (t *turnState) finalOutcome(content *genai.Content) (llmshared.StepOutcome, error) {
	if content == nil {
		if t.toolUsed {
			return llmshared.StepOutcome{}, nil
		}
		return llmshared.StepOutcome{}, fmt.Errorf("no content in response candidate")
	}
	text := t.client.joinContentParts(content)
	if strings.TrimSpace(text) == "" {
		if t.toolUsed {
			return llmshared.StepOutcome{}, nil
		}
		return llmshared.StepOutcome{}, fmt.Errorf("no usable content in response candidates")
	}
	return llmshared.StepOutcome{Text: text}, nil
}

// recordAssistantStep dedupes the model's echoed call parts, compacts
// prior heavy content, appends the assistant message to the
// conversation, and converts the calls for the shared loop.
func (t *turnState) recordAssistantStep(content *genai.Content, notifyInterimText bool) llmshared.StepOutcome {
	g := t.client
	t.toolUsed = true

	if dropped := dedupeFunctionCallParts(content); dropped > 0 {
		log.Printf("WARNING: model repeated identical function calls; dropped %d duplicate call part(s) from one response", dropped)
	}

	// Compact prior iteration contents to prevent unbounded context
	// growth before appending the new assistant message.
	compactPriorContents(t.contents)

	if notifyInterimText {
		if contentStr := strings.TrimSpace(g.joinContentParts(content)); contentStr != "" {
			notification := events.NotificationEvent{Message: contentStr}
			g.EventBus.Publish(notification.Topic(), notification)
		}
	}

	t.contents = append(t.contents, content)

	resp := &genai.GenerateContentResponse{Candidates: []*genai.Candidate{{Content: content}}}
	fnCalls := resp.FunctionCalls()
	calls := make([]llmshared.ToolCall, 0, len(fnCalls))
	for _, fc := range fnCalls {
		args := make(map[string]any, len(fc.Args))
		for k, v := range fc.Args {
			args[k] = v
		}
		calls = append(calls, llmshared.ToolCall{ID: fc.ID, Name: fc.Name, Args: args})
	}
	return llmshared.StepOutcome{ToolCalls: calls}
}

// appendMalformedRecovery echoes the malformed model content (if any)
// and a corrective user message so the next step can recover.
func (t *turnState) appendMalformedRecovery(content *genai.Content, finishMessage string) {
	if content != nil {
		t.contents = append(t.contents, content)
	}
	t.contents = append(t.contents, &genai.Content{
		Parts: []*genai.Part{genai.NewPartFromText(
			"Your previous function call was malformed and could not be processed. Details: " +
				finishMessage + ". Please try again with a valid function call.",
		)},
		Role: "user",
	})
}

// AddToolResults converts executed tool results into function-response
// parts (plus any media payloads, which must follow the function
// response to satisfy the Gemini function-calling protocol).
func (t *turnState) AddToolResults(ctx context.Context, results []llmshared.ToolResult) error {
	g := t.client

	responseParts := make([]*genai.Part, 0, len(results))
	var mediaContents []*genai.Content

	for _, result := range results {
		handlerResp := result.Result
		if result.Err != nil {
			if g.EventBus != nil {
				g.EventBus.Publish(events.NotificationEvent{}.Topic(), events.NotificationEvent{
					Message: fmt.Sprintf("tool %s returned error: %v", result.Call.Name, result.Err),
				})
			}
			handlerResp = map[string]any{
				"error": fmt.Sprintf("function %q returned an error: %v", result.Call.Name, result.Err),
			}
		}

		switch result.Call.Name {
		case "viewImage":
			img, sanitized, err := toolpayload.Extract(handlerResp)
			if err != nil {
				return fmt.Errorf("invalid viewImage response: %w", err)
			}
			handlerResp = sanitized
			if img != nil {
				mediaContents = append(mediaContents, buildGeminiImageContent(img))
			}
		case "viewDocument":
			doc, sanitized, err := toolpayload.Extract(handlerResp)
			if err != nil {
				return fmt.Errorf("invalid viewDocument response: %w", err)
			}
			handlerResp = sanitized
			if doc != nil {
				mediaContents = append(mediaContents, buildGeminiDocumentContent(doc))
			}
		}

		part := genai.NewPartFromFunctionResponse(result.Call.Name, handlerResp)
		// Echo back the FunctionCall ID so the model can match responses to calls
		if result.Call.ID != "" {
			part.FunctionResponse.ID = result.Call.ID
		}
		responseParts = append(responseParts, part)
	}

	if len(responseParts) > 0 {
		t.contents = append(t.contents, &genai.Content{
			Parts: responseParts,
			Role:  string(roleFunctionResponse),
		})
	}
	t.contents = append(t.contents, mediaContents...)
	return nil
}
