package shared

import (
	"encoding/json"
	"fmt"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/llm/shared/toolpayload"
)

// BuildToolResultMessages converts executed tool results into the
// provider's message type M. Handler failures are logged on the bus and
// fed back to the model as an error payload; viewImage/viewDocument
// results get their media payload extracted and appended as an extra
// message right after the tool response.
func BuildToolResultMessages[M any](
	bus events.EventBus,
	results []ToolResult,
	newToolMessage func(callID, payload string) M,
	newImageMessage func(*toolpayload.Payload) M,
	newDocumentMessage func(*toolpayload.Payload) M,
) ([]M, error) {
	messages := make([]M, 0, len(results))

	for _, result := range results {
		payloadMap := result.Result
		if result.Err != nil {
			if bus != nil {
				bus.Publish(events.NotificationEvent{}.Topic(), events.NotificationEvent{
					Message: fmt.Sprintf("tool %s returned error: %v", result.Call.Name, result.Err),
				})
			}
			payloadMap = map[string]any{
				"error": fmt.Sprintf("function %q returned an error: %v", result.Call.Name, result.Err),
			}
		}

		var extra []M
		switch result.Call.Name {
		case "viewImage":
			img, sanitized, err := toolpayload.Extract(payloadMap)
			if err != nil {
				return nil, fmt.Errorf("invalid viewImage response: %w", err)
			}
			payloadMap = sanitized
			if img != nil {
				extra = append(extra, newImageMessage(img))
			}
		case "viewDocument":
			doc, sanitized, err := toolpayload.Extract(payloadMap)
			if err != nil {
				return nil, fmt.Errorf("invalid viewDocument response: %w", err)
			}
			payloadMap = sanitized
			if doc != nil {
				extra = append(extra, newDocumentMessage(doc))
			}
		}

		payload, err := json.Marshal(payloadMap)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal response for function %q: %w", result.Call.Name, err)
		}

		messages = append(messages, newToolMessage(result.Call.ID, string(payload)))
		messages = append(messages, extra...)
	}

	return messages, nil
}
