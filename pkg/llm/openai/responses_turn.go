package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/kcaldas/genie/pkg/llm/shared/toolpayload"
)

type responsesTurnState struct {
	client    *Client
	params    responses.ResponseNewParams
	input     responses.ResponseInputParam
	modelName string
	toolUsed  bool
}

func (c *Client) newResponsesTurn(prompt ai.Prompt, modelName string) (*responsesTurnState, error) {
	instructions, err := c.buildInstructions(prompt)
	if err != nil {
		return nil, err
	}
	input := c.buildResponseInitialInput(prompt)

	params := responses.ResponseNewParams{
		Model:   shared.ResponsesModel(modelName),
		Store:   openai.Bool(false),
		Include: []responses.ResponseIncludable{responses.ResponseIncludableReasoningEncryptedContent},
	}
	if instructions != "" {
		params.Instructions = openai.String(instructions)
	}
	params.Input = responses.ResponseNewParamsInputUnion{OfInputItemList: input}
	c.applyResponsesGenerationConfig(&params, prompt)

	return &responsesTurnState{
		client:    c,
		params:    params,
		input:     input,
		modelName: modelName,
	}, nil
}

func (t *responsesTurnState) Step(ctx context.Context, emit func(*ai.StreamChunk)) (llmshared.StepOutcome, error) {
	params := t.params
	params.Input = responses.ResponseNewParamsInputUnion{OfInputItemList: t.input}

	if emit != nil {
		return t.stepStreaming(ctx, params, emit)
	}
	return t.stepBlocking(ctx, params)
}

func (t *responsesTurnState) stepBlocking(ctx context.Context, params responses.ResponseNewParams) (llmshared.StepOutcome, error) {
	c := t.client
	if c.responses == nil {
		return llmshared.StepOutcome{}, errors.New("openai responses client not configured")
	}

	resp, err := c.responses.New(ctx, params)
	if err != nil {
		return llmshared.StepOutcome{}, fmt.Errorf("openai response: %w", err)
	}

	c.publishResponsesUsage(t.modelName, resp.Usage)
	return t.recordResponse(resp, false, nil)
}

func (t *responsesTurnState) stepStreaming(ctx context.Context, params responses.ResponseNewParams, emit func(*ai.StreamChunk)) (llmshared.StepOutcome, error) {
	c := t.client
	if c.responses == nil {
		return llmshared.StepOutcome{}, errors.New("openai responses client not configured")
	}

	stream := c.responses.NewStreaming(ctx, params)
	defer stream.Close()

	var completed *responses.Response
	var eventToolCalls []responses.ResponseFunctionToolCall

	for stream.Next() {
		event := stream.Current()
		switch event.Type {
		case "response.output_text.delta":
			if event.Delta.OfString != "" {
				emit(&ai.StreamChunk{Text: event.Delta.OfString})
			}
		case "response.output_item.done":
			if event.Item.Type == "function_call" {
				eventToolCalls = append(eventToolCalls, event.Item.AsFunctionCall())
			}
		case "response.completed":
			resp := event.Response
			completed = &resp
		case "response.failed", "response.incomplete":
			if event.Response.Error.Message != "" {
				return llmshared.StepOutcome{}, fmt.Errorf("openai response %s: %s", event.Response.Status, event.Response.Error.Message)
			}
			return llmshared.StepOutcome{}, fmt.Errorf("openai response %s", event.Response.Status)
		case "error":
			return llmshared.StepOutcome{}, fmt.Errorf("openai response stream error: %s", event.Message)
		}
	}

	if err := stream.Err(); err != nil {
		return llmshared.StepOutcome{}, fmt.Errorf("openai response stream: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return llmshared.StepOutcome{}, err
	}
	if completed == nil {
		return llmshared.StepOutcome{}, errors.New("openai response stream returned no completed response")
	}

	if tc := c.publishResponsesUsage(t.modelName, completed.Usage); tc != nil {
		emit(&ai.StreamChunk{TokenCount: tc})
	}
	outcome, err := t.recordResponse(completed, true, eventToolCalls)
	if err != nil {
		return llmshared.StepOutcome{}, err
	}
	if len(outcome.ToolCalls) > 0 {
		emit(responseToolCallChunk(outcome.ToolCalls))
	}
	return outcome, nil
}

func (t *responsesTurnState) recordResponse(resp *responses.Response, streamed bool, streamedCalls []responses.ResponseFunctionToolCall) (llmshared.StepOutcome, error) {
	if resp == nil {
		return llmshared.StepOutcome{}, errors.New("openai response was nil")
	}
	if len(resp.Output) == 0 {
		if t.toolUsed {
			return llmshared.StepOutcome{}, nil
		}
		return llmshared.StepOutcome{}, errors.New("openai returned an empty response")
	}

	for _, item := range resp.Output {
		if param, ok := responseOutputItemToInput(item); ok {
			t.input = append(t.input, param)
		}
	}

	text := responseOutputText(resp.Output)
	toolCalls, err := responseToolCalls(resp.Output)
	if err != nil {
		return llmshared.StepOutcome{}, err
	}
	if len(toolCalls) == 0 && len(streamedCalls) > 0 {
		toolCalls, err = responseToolCallsFromDoneEvents(streamedCalls)
		if err != nil {
			return llmshared.StepOutcome{}, err
		}
	}

	if len(toolCalls) == 0 {
		if strings.TrimSpace(text) == "" {
			if t.toolUsed {
				return llmshared.StepOutcome{}, nil
			}
			return llmshared.StepOutcome{}, errors.New("openai returned an empty response")
		}
		return llmshared.StepOutcome{Text: text}, nil
	}

	if text != "" && !streamed {
		notification := events.NotificationEvent{Message: strings.TrimSpace(text)}
		t.client.eventBus.Publish(notification.Topic(), notification)
	}

	t.toolUsed = true
	return llmshared.StepOutcome{ToolCalls: toolCalls}, nil
}

func (t *responsesTurnState) AddToolResults(ctx context.Context, results []llmshared.ToolResult) error {
	for _, result := range results {
		handlerResp := result.Result
		if result.Err != nil {
			t.client.eventBus.Publish(events.NotificationEvent{}.Topic(), events.NotificationEvent{
				Message: fmt.Sprintf("tool %s returned error: %v", result.Call.Name, result.Err),
			})
			handlerResp = map[string]any{
				"error": fmt.Sprintf("function %q returned an error: %v", result.Call.Name, result.Err),
			}
		}

		var media *toolpayload.Payload
		if extracted, sanitized, ok := toolpayload.Native(handlerResp); ok {
			handlerResp = sanitized
			media = extracted
		} else if sanitized != nil {
			handlerResp = sanitized
		}

		payload, err := json.Marshal(handlerResp)
		if err != nil {
			return fmt.Errorf("unable to marshal response for function %q: %w", result.Call.Name, err)
		}
		t.input = append(t.input, responses.ResponseInputItemUnionParam{
			OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
				CallID: result.Call.ID,
				Output: string(payload),
			},
		})

		if media != nil {
			switch result.Call.Name {
			case "viewImage":
				t.input = append(t.input, buildResponseImageUserMessage(media))
			case "viewDocument":
				t.input = append(t.input, buildResponseDocumentUserMessage(media))
			}
		}
	}

	return ctx.Err()
}

func (c *Client) buildResponseInitialInput(prompt ai.Prompt) responses.ResponseInputParam {
	return responses.ResponseInputParam{c.buildResponseUserMessage(prompt.Text, prompt.Images)}
}

func (c *Client) buildResponseUserMessage(text string, images []*ai.Image) responses.ResponseInputItemUnionParam {
	trimmed := strings.TrimSpace(text)
	var parts []responses.ResponseInputContentUnionParam
	if trimmed != "" {
		parts = append(parts, responses.ResponseInputContentParamOfInputText(trimmed))
	}
	for _, img := range images {
		if img == nil || len(img.Data) == 0 {
			continue
		}
		mimeType := img.Type
		if strings.TrimSpace(mimeType) == "" {
			mimeType = "image/png"
		}
		dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(img.Data))
		image := responses.ResponseInputImageParam{
			Detail:   responses.ResponseInputImageDetailAuto,
			ImageURL: openai.String(dataURL),
		}
		parts = append(parts, responses.ResponseInputContentUnionParam{OfInputImage: &image})
	}

	return responses.ResponseInputItemUnionParam{
		OfInputMessage: &responses.ResponseInputItemMessageParam{
			Role:    "user",
			Content: parts,
		},
	}
}

func (c *Client) applyResponsesGenerationConfig(params *responses.ResponseNewParams, prompt ai.Prompt) {
	modelCfg := c.config.GetModelConfig()
	targetModel := string(params.Model)
	allowSampling := allowsSamplingParams(targetModel)

	maxTokens := prompt.MaxTokens
	if maxTokens <= 0 {
		maxTokens = modelCfg.MaxTokens
	}
	if maxTokens > 0 {
		params.MaxOutputTokens = openai.Int(int64(maxTokens))
	}

	if allowSampling {
		temperature := prompt.Temperature
		if temperature <= 0 {
			temperature = modelCfg.Temperature
		}
		if temperature > 0 {
			params.Temperature = openai.Float(float64(temperature))
			topP := prompt.TopP
			if supportsTopP(targetModel) && topP > 0 && math.Abs(float64(topP)-1.0) > 1e-6 {
				params.TopP = openai.Float(float64(topP))
			} else if topP > 0 && math.Abs(float64(topP)-1.0) > 1e-6 {
				c.logger.Debug("top_p not supported for model; using default", "model", targetModel)
			}
		}
	} else {
		if prompt.Temperature > 0 && prompt.Temperature != 1.0 {
			c.logger.Debug("temperature not supported for model; using default", "model", targetModel)
		}
		if prompt.TopP > 0 && prompt.TopP != 1.0 {
			c.logger.Debug("top_p not supported for model; using default", "model", targetModel)
		}
	}

	if len(prompt.Functions) > 0 {
		params.Tools = mapResponseFunctions(prompt.Functions)
		params.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{
			OfToolChoiceMode: openai.Opt(responses.ToolChoiceOptionsAuto),
		}
	}

	if prompt.ResponseSchema != nil {
		params.Text = responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigUnionParam{
				OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
					Name:   chooseSchemaName(prompt),
					Schema: schemaToMap(prompt.ResponseSchema),
					Strict: openai.Bool(true),
				},
			},
		}
	}
}

func responseOutputItemToInput(item responses.ResponseOutputItemUnion) (responses.ResponseInputItemUnionParam, bool) {
	switch item.Type {
	case "message":
		msg := item.AsMessage().ToParam()
		return responses.ResponseInputItemUnionParam{OfOutputMessage: &msg}, true
	case "function_call":
		call := item.AsFunctionCall()
		param := responses.ResponseFunctionToolCallParam{
			Arguments: call.Arguments,
			CallID:    call.CallID,
			Name:      call.Name,
			Status:    call.Status,
		}
		if call.ID != "" {
			param.ID = openai.String(call.ID)
		}
		return responses.ResponseInputItemUnionParam{OfFunctionCall: &param}, true
	case "reasoning":
		reasoning := item.AsReasoning()
		if strings.TrimSpace(reasoning.EncryptedContent) == "" {
			return responses.ResponseInputItemUnionParam{}, false
		}
		summary := make([]responses.ResponseReasoningItemSummaryParam, 0, len(reasoning.Summary))
		for _, part := range reasoning.Summary {
			summary = append(summary, responses.ResponseReasoningItemSummaryParam{Text: part.Text})
		}
		param := responses.ResponseReasoningItemParam{
			ID:               reasoning.ID,
			Summary:          summary,
			EncryptedContent: openai.String(reasoning.EncryptedContent),
			Status:           reasoning.Status,
		}
		return responses.ResponseInputItemUnionParam{OfReasoning: &param}, true
	default:
		return responses.ResponseInputItemUnionParam{}, false
	}
}

func responseOutputText(output []responses.ResponseOutputItemUnion) string {
	var b strings.Builder
	for _, item := range output {
		if item.Type != "message" {
			continue
		}
		msg := item.AsMessage()
		for _, part := range msg.Content {
			if part.Type == "output_text" {
				b.WriteString(part.Text)
			}
		}
	}
	return b.String()
}

func responseToolCalls(output []responses.ResponseOutputItemUnion) ([]llmshared.ToolCall, error) {
	calls := make([]llmshared.ToolCall, 0)
	for _, item := range output {
		if item.Type != "function_call" {
			continue
		}
		call := item.AsFunctionCall()
		sharedCall, err := responseFunctionToolCallToShared(call)
		if err != nil {
			return nil, err
		}
		calls = append(calls, sharedCall)
	}
	return calls, nil
}

func responseToolCallsFromDoneEvents(calls []responses.ResponseFunctionToolCall) ([]llmshared.ToolCall, error) {
	out := make([]llmshared.ToolCall, 0, len(calls))
	for _, call := range calls {
		sharedCall, err := responseFunctionToolCallToShared(call)
		if err != nil {
			return nil, err
		}
		out = append(out, sharedCall)
	}
	return out, nil
}

func responseFunctionToolCallToShared(call responses.ResponseFunctionToolCall) (llmshared.ToolCall, error) {
	args := map[string]any{}
	if strings.TrimSpace(call.Arguments) != "" {
		if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
			return llmshared.ToolCall{}, fmt.Errorf("invalid arguments for function %q: %w", call.Name, err)
		}
	}
	return llmshared.ToolCall{ID: call.CallID, Name: call.Name, Args: args}, nil
}

func buildResponseImageUserMessage(img *toolpayload.Payload) responses.ResponseInputItemUnionParam {
	text := toolpayload.SanitizePath(img.Path)
	return (&Client{}).buildResponseUserMessage(fmt.Sprintf("Image retrieved from %s", text), []*ai.Image{
		{Type: img.MIMEType, Data: img.Data},
	})
}

func buildResponseDocumentUserMessage(doc *toolpayload.Payload) responses.ResponseInputItemUnionParam {
	text := toolpayload.SanitizePath(doc.Path)
	content := fmt.Sprintf("Document retrieved from %s (MIME: %s, %d bytes).", text, doc.MIMEType, doc.SizeBytes)
	notice := "This provider does not support inline PDFs; refer to the tool response for access."
	return (&Client{}).buildResponseUserMessage(content+"\n\n"+notice, nil)
}

func responseToolCallChunk(calls []llmshared.ToolCall) *ai.StreamChunk {
	toolChunks := make([]*ai.ToolCallChunk, 0, len(calls))
	for _, call := range calls {
		toolChunks = append(toolChunks, &ai.ToolCallChunk{
			ID:         call.ID,
			Name:       call.Name,
			Parameters: call.Args,
		})
	}
	return &ai.StreamChunk{ToolCalls: toolChunks}
}

func (c *Client) publishResponsesUsage(modelName string, usage responses.ResponseUsage) *ai.TokenCount {
	if usage.TotalTokens == 0 && usage.InputTokens == 0 && usage.OutputTokens == 0 {
		return nil
	}

	cached := int32(usage.InputTokensDetails.CachedTokens)
	if strings.TrimSpace(modelName) == "" {
		modelName = c.resolveModelName("")
	}
	event := events.TokenCountEvent{
		Provider:             "openai",
		Model:                modelName,
		InputTokens:          int32(usage.InputTokens) - cached,
		OutputTokens:         int32(usage.OutputTokens),
		CachedTokens:         cached,
		CacheReadInputTokens: cached,
		TotalTokens:          int32(usage.TotalTokens),
	}
	c.eventBus.Publish(event.Topic(), event)

	if c.config.GetBoolWithDefault("GENIE_TOKEN_DEBUG", false) {
		raw, _ := json.MarshalIndent(usage, "", "  ")
		notification := events.NotificationEvent{Message: string(raw)}
		c.eventBus.Publish(notification.Topic(), notification)
	}

	return &ai.TokenCount{
		TotalTokens:  int32(usage.TotalTokens),
		InputTokens:  int32(usage.InputTokens),
		OutputTokens: int32(usage.OutputTokens),
	}
}
