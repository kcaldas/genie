package genie

import (
	"context"
	"fmt"

	"github.com/kcaldas/genie/pkg/events"
)

// Test-only bridges exposing white-box internals to package genie_test.
// This file is compiled only into the test binary and never ships.

// ApplySessionContextForTest exposes applySessionContext.
func ApplySessionContextForTest(ctx context.Context, session Session) context.Context {
	return applySessionContext(ctx, session)
}

// NativeTaskPromptForTest exposes nativeTaskPrompt.
func NativeTaskPromptForTest(prompt string) string {
	return nativeTaskPrompt(prompt)
}

// NewChildGenieForTest builds the child Genie the native task executor
// would create for g, which must be a *core produced by this package.
func NewChildGenieForTest(g Genie) (Genie, events.EventBus, error) {
	parent, ok := g.(*core)
	if !ok {
		return nil, nil, fmt.Errorf("NewChildGenieForTest: %T is not a core genie", g)
	}
	executor := newNativeTaskExecutor(parent).(*nativeTaskExecutor)
	return executor.newChildGenie()
}
