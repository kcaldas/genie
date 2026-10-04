package openai

import (
	"errors"

	"github.com/kcaldas/genie/pkg/ai"
	openai "github.com/openai/openai-go/v3"
)

// A response that refuses retry (an LLM proxy's budget refusal) is final.
func init() {
	ai.RegisterFinalError(func(err error) bool {
		var e *openai.Error
		return errors.As(err, &e) && e.Response != nil && ai.RefusesRetry(e.Response.Header)
	})
}
