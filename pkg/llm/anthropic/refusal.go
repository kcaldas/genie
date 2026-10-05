package anthropic

import (
	"errors"
	"net/http"

	anthropic_sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/kcaldas/genie/pkg/ai"
)

// A response that refuses retry (an LLM proxy's budget refusal) is final.
func init() {
	ai.RegisterResponseError(func(err error) *http.Response {
		var e *anthropic_sdk.Error
		if errors.As(err, &e) {
			return e.Response
		}
		return nil
	})
}
