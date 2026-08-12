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
	"google.golang.org/genai"
)

// turnState drives one chat turn against the Gemini API for the shared
// agent loop. It owns the provider-native conversation ([]*genai.Content)
// and appends assistant messages and tool results as the loop advances.
type turnState struct {
	client     *Client
	modelName  string
	contents   []*genai.Content
	config     *genai.GenerateContentConfig
	stepCount  int
	toolUsed   bool
	inputLimit int
}

func (g *Client) newTurn(p ai.Prompt) *turnState {
	return &turnState{
		client:     g,
		modelName:  p.ModelName,
		contents:   g.buildInitialContents(p),
		config:     g.buildGenerateConfig(p),
		inputLimit: llmshared.ModelInputAdmissionLimit(p),
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
	usage := g.publishUsageMetadata(t.modelName, result.UsageMetadata)

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
		outcome, err := t.finalOutcome(result.Candidates[0].Content)
		outcome.Usage = usage
		return outcome, err
	}

	outcome := t.recordAssistantStep(result.Candidates[0].Content, true)
	outcome.Usage = usage
	return outcome, nil
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

	usage := g.publishUsageMetadata(t.modelName, lastUsageMetadata)
	if usage != nil {
		emit(&ai.StreamChunk{TokenCount: usage})
	}

	accumulated := &genai.Content{Parts: allParts, Role: "model"}
	if !contentHasFunctionCalls(accumulated) {
		// The text already reached the consumer via emit; an empty final
		// step simply ends the stream.
		return llmshared.StepOutcome{Text: t.client.joinContentParts(accumulated), Usage: usage}, nil
	}

	// Interim text notifications are a blocking-mode concern; in
	// streaming mode the text already went out through emit.
	outcome := t.recordAssistantStep(accumulated, false)
	outcome.Usage = usage
	return outcome, nil
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
func (t *turnState) AddToolResults(ctx context.Context, results []llmshared.PreparedToolResult) error {
	responseContent, mediaContents, _ := t.buildToolResultContents(results, true, false)
	if t.inputLimit <= 0 {
		t.appendToolResultContents(responseContent, mediaContents)
		return nil
	}

	if fits, err := t.toolResultContentsFit(ctx, responseContent, mediaContents); err != nil {
		return err
	} else if fits {
		t.appendToolResultContents(responseContent, mediaContents)
		return nil
	}

	responseContent, _, hadMedia := t.buildToolResultContents(results, false, false)
	if hadMedia {
		if fits, err := t.toolResultContentsFit(ctx, responseContent, nil); err != nil {
			return err
		} else if fits {
			t.appendToolResultContents(responseContent, nil)
			return nil
		}
	}

	responseContent, _, _ = t.buildToolResultContents(results, false, true)
	if fits, err := t.toolResultContentsFit(ctx, responseContent, nil); err != nil {
		return err
	} else if !fits {
		return ai.NonRetryable(fmt.Errorf(
			"gemini tool results cannot fit the model input envelope even after omission (limit %d tokens)",
			t.inputLimit,
		))
	}
	t.appendToolResultContents(responseContent, nil)
	return nil
}

func (t *turnState) buildToolResultContents(results []llmshared.PreparedToolResult, includeMedia, omit bool) (*genai.Content, []*genai.Content, bool) {
	responseParts := make([]*genai.Part, 0, len(results))
	var mediaContents []*genai.Content
	hadMedia := false

	for _, result := range results {
		if omit {
			result = llmshared.OmittedToolResult(result, "model input budget exhausted")
		}
		encoded := llmshared.EncodeToolResult(result, supportsGeminiBlob)

		for _, blob := range encoded.Blobs {
			hadMedia = true
			if includeMedia {
				mediaContents = append(mediaContents, buildGeminiBlobContent(blob))
			} else {
				encoded.Text += "\n[" + llmshared.DescribeBlob(blob) + " omitted: model input budget exhausted]"
			}
		}

		part := genai.NewPartFromFunctionResponse(result.Call.Name, map[string]any{
			"output":   encoded.Text,
			"is_error": encoded.IsError,
		})
		// Echo back the FunctionCall ID so the model can match responses to calls
		if result.Call.ID != "" {
			part.FunctionResponse.ID = result.Call.ID
		}
		responseParts = append(responseParts, part)
	}

	var responseContent *genai.Content
	if len(responseParts) > 0 {
		responseContent = &genai.Content{
			Parts: responseParts,
			Role:  string(roleFunctionResponse),
		}
	}
	return responseContent, mediaContents, hadMedia
}

func (t *turnState) appendToolResultContents(responseContent *genai.Content, mediaContents []*genai.Content) {
	if responseContent != nil {
		t.contents = append(t.contents, responseContent)
	}
	t.contents = append(t.contents, mediaContents...)
}

func (t *turnState) toolResultContentsFit(ctx context.Context, responseContent *genai.Content, mediaContents []*genai.Content) (bool, error) {
	candidate := append([]*genai.Content(nil), t.contents...)
	if responseContent != nil {
		candidate = append(candidate, responseContent)
	}
	candidate = append(candidate, mediaContents...)
	config := &genai.CountTokensConfig{}
	if t.config != nil {
		config.SystemInstruction = t.config.SystemInstruction
		config.Tools = t.config.Tools
	}
	var (
		count *genai.CountTokensResponse
		err   error
	)
	if t.client.countTokensFn != nil {
		count, err = t.client.countTokensFn(ctx, t.modelName, candidate, config)
	} else {
		count, err = t.client.Client.Models.CountTokens(ctx, t.modelName, candidate, config)
	}
	if err != nil {
		return false, fmt.Errorf("gemini preflight tool-result token count: %w", err)
	}
	return int(count.TotalTokens) <= t.inputLimit, nil
}

var geminiBlobMIMETypes = map[string]struct{}{
	"application/pdf": {},
	"audio/aac":       {},
	"audio/aiff":      {},
	"audio/flac":      {},
	"audio/mp3":       {},
	"audio/ogg":       {},
	"audio/wav":       {},
	"image/heic":      {},
	"image/heif":      {},
	"image/jpeg":      {},
	"image/png":       {},
	"image/webp":      {},
	"text/html":       {},
	"text/markdown":   {},
	"text/plain":      {},
	"text/xml":        {},
	"video/3gpp":      {},
	"video/avi":       {},
	"video/mov":       {},
	"video/mp4":       {},
	"video/mpeg":      {},
	"video/mpg":       {},
	"video/quicktime": {},
	"video/webm":      {},
	"video/wmv":       {},
	"video/x-flv":     {},
}

func supportsGeminiBlob(blob ai.BlobContent) bool {
	mimeType := strings.ToLower(strings.TrimSpace(blob.MIMEType))
	if idx := strings.IndexByte(mimeType, ';'); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	_, ok := geminiBlobMIMETypes[mimeType]
	return ok
}
