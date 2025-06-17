package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSessionInteractionEvent_Topic(t *testing.T) {
	event := SessionInteractionEvent{}
	assert.Equal(t, "session.interaction", event.Topic())
}
