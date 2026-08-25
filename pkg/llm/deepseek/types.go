package deepseek

import (
	"github.com/kcaldas/genie/pkg/llm/openaicompat"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

// DeepSeek speaks the OpenAI-compatible chat-completions protocol; the
// wire types live in openaicompat and are aliased here for brevity.
type (
	chatRequest      = openaicompat.ChatRequest
	chatMessage      = openaicompat.ChatMessage
	contentPart      = openaicompat.ContentPart
	imageURL         = openaicompat.ImageURL
	responseFormat   = openaicompat.ResponseFormat
	chatResponse     = openaicompat.ChatResponse
	chatChoice       = openaicompat.ChatChoice
	responseMessage  = openaicompat.ResponseMessage
	responseContent  = openaicompat.ResponseContent
	usage            = openaicompat.Usage
	toolCall         = llmshared.ChatToolCall
	toolCallFunction = llmshared.ChatToolCallFunction
)

var (
	newMessageContent         = openaicompat.NewMessageContent
	newMessageContentFromText = openaicompat.NewMessageContentFromText
)
