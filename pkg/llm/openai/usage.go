package openai

import (
	"strconv"

	"github.com/openai/openai-go/packages/respjson"
)

// cacheWriteTokens reads cache_write_tokens from a usage details object.
// GPT-5.6-class models report it; the pinned SDK predates the field, so it
// only appears among the raw extra fields, which the SDK does not mark as
// valid. Zero when absent or not a number.
func cacheWriteTokens(extra map[string]respjson.Field) int32 {
	field, ok := extra["cache_write_tokens"]
	if !ok || field.Raw() == "" {
		return 0
	}
	n, err := strconv.ParseInt(field.Raw(), 10, 32)
	if err != nil {
		return 0
	}
	return int32(n)
}
