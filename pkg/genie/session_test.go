package genie

import (
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionGeneratesUniqueIDs(t *testing.T) {
	bus := events.NewEventBus()

	s1 := NewSession("/home", "/work", nil, nil, bus, nil)
	s2 := NewSession("/home", "/work", nil, nil, bus, nil)

	assert.NotEmpty(t, s1.GetID())
	assert.NotEmpty(t, s2.GetID())
	assert.NotEqual(t, s1.GetID(), s2.GetID(), "each session must get a unique ID")
}

func TestNewSessionRecordsCreationTimestamp(t *testing.T) {
	bus := events.NewEventBus()

	before := time.Now().Add(-time.Second)
	s := NewSession("/home", "/work", nil, nil, bus, nil)
	after := time.Now().Add(time.Second)

	createdAt, err := time.Parse(time.RFC3339, s.GetCreatedAt())
	require.NoError(t, err, "createdAt must be an RFC3339 timestamp, got %q", s.GetCreatedAt())
	assert.True(t, createdAt.After(before) && createdAt.Before(after),
		"createdAt %v must fall within the creation window", createdAt)
}
