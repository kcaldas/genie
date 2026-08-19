package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestIDContextRoundTrip(t *testing.T) {
	ctx := ContextWithRequestID(context.Background(), "req-1")
	assert.Equal(t, "req-1", RequestIDFromContext(ctx))
}

func TestRequestIDFromContextDefaults(t *testing.T) {
	assert.Empty(t, RequestIDFromContext(context.Background()), "unset context yields empty ID")
	assert.Empty(t, RequestIDFromContext(nil), "nil context yields empty ID") //nolint:staticcheck // nil-safety is part of the contract
}

func TestContextWithBlankRequestIDLeavesContextUnchanged(t *testing.T) {
	base := context.Background()
	assert.Equal(t, base, ContextWithRequestID(base, ""))
}
