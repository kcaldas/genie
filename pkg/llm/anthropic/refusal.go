package anthropic

import (
	"errors"

	anthropic_sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/kcaldas/genie/pkg/ai"
)

// A response that refuses retry (an LLM proxy's budget refusal) is final.
func init() {
	ai.RegisterFinalError(func(err error) bool {
		var e *anthropic_sdk.Error
		return errors.As(err, &e) && e.Response != nil && ai.RefusesRetry(e.Response.Header)
	})
}
