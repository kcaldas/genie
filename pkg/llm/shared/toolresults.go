package shared

import (
	"encoding/json"
	"fmt"
)

// BuildToolResultMessages converts prepared tool results into the
// provider's message type M: one tool message per result, each followed
// by the attachments this provider can encode.
//
// Results arrive normalized — errors are already body text and
// attachments are already lifted out and bounded — so this only
// serializes and encodes. `supports` declares what the provider can
// render; anything else is reported in the body by SplitAttachments
// rather than dropped.
func BuildToolResultMessages[M any](
	results []PreparedToolResult,
	supports func(Attachment) bool,
	newToolMessage func(callID, payload string) M,
	newAttachmentMessage func(Attachment) M,
) ([]M, error) {
	messages := make([]M, 0, len(results))

	for _, result := range results {
		body, attachments := SplitAttachments(result, supports)

		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal response for function %q: %w", result.Call.Name, err)
		}

		messages = append(messages, newToolMessage(result.Call.ID, string(payload)))
		for _, attachment := range attachments {
			messages = append(messages, newAttachmentMessage(attachment))
		}
	}

	return messages, nil
}
