package cloudchat

import (
	"github.com/kcaldas/genie/pkg/llm/openaicompat"
)

// Every provider here speaks the OpenAI-compatible chat-completions
// protocol; the wire types live in openaicompat and are aliased here for
// brevity.
type (
	chatRequest        = openaicompat.ChatRequest
	chatMessage        = openaicompat.ChatMessage
	contentPart        = openaicompat.ContentPart
	responseFormat     = openaicompat.ResponseFormat
	chatResponse       = openaicompat.ChatResponse
	chatChoice         = openaicompat.ChatChoice
	responseMessage    = openaicompat.ResponseMessage
	responseContent    = openaicompat.ResponseContent
	usage              = openaicompat.Usage
	promptTokensDetail = openaicompat.PromptTokensDetail
)

var newMessageContentFromText = openaicompat.NewMessageContentFromText
