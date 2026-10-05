package openai

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/kcaldas/genie/pkg/ai"
	openai "github.com/openai/openai-go/v3"
)

// A response that refuses retry (an LLM proxy's budget refusal) is final.
func init() {
	ai.RegisterResponseError(func(err error) *http.Response {
		var e *openai.Error
		if errors.As(err, &e) {
			return e.Response
		}
		return nil
	})
}

// apiError adds the provider's own description to an SDK error, whose text is
// only the HTTP status. The request URL stays out: it can carry secrets.
func apiError(err error) error {
	var e *openai.Error
	if !errors.As(err, &e) || e.Message == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, e.Message)
}
