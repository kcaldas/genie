package openai

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/require"
)

// TestLive_MessageBoundaryHistoryIsServedFromCache drives the real client
// through three growing turns on a GPT-5.6-class model and checks that,
// by the third, most of the request is served from OpenAI's prompt cache.
// It costs a few cents and needs OPENAI_API_KEY, so it only runs when
// asked for:
//
//	GENIE_LIVE_CACHE_TEST=1 OPENAI_API_KEY=... go test ./pkg/llm/openai/ -run TestLive_ -v
func TestLive_MessageBoundaryHistoryIsServedFromCache(t *testing.T) {
	if os.Getenv("GENIE_LIVE_CACHE_TEST") == "" {
		t.Skip("set GENIE_LIVE_CACHE_TEST=1 to run against the OpenAI API")
	}
	bus := events.NewEventBus()
	var mu sync.Mutex
	var usage []events.TokenCountEvent
	bus.Subscribe(events.TokenCountEvent{}.Topic(), func(e interface{}) {
		if tc, ok := e.(events.TokenCountEvent); ok {
			mu.Lock()
			usage = append(usage, tc)
			mu.Unlock()
		}
	})
	client, err := NewClient(bus)
	require.NoError(t, err)

	nonce := uint32(time.Now().UnixNano()) // fresh content per run, fixed within it
	filler := func(seed, words int) string {
		vocab := strings.Fields("shipment freight carrier quote pallet dock terminal lane rate chapter draft manuscript editor scene character revision")
		var b strings.Builder
		x := uint32(seed) ^ nonce
		for i := 0; i < words; i++ {
			x = x*1664525 + 1013904223
			b.WriteString(vocab[int(x>>16)%len(vocab)])
			b.WriteByte(' ')
		}
		return b.String()
	}
	seed := 0
	turnText := func(i int) string { return filler(seed+i, 40) }
	history := func(n int) []ai.HistoryTurn {
		turns := make([]ai.HistoryTurn, 0, n)
		for i := 0; i < n; i++ {
			turns = append(turns, ai.HistoryTurn{User: turnText(i), Assistant: filler(seed+1000+i, 700)})
		}
		return turns
	}
	for k := 0; k < 3; k++ {
		n := 40 + k
		prompt := ai.Prompt{
			ModelName:   "gpt-5.6-luna",
			Instruction: "You are a test agent. Reply with the single word OK.\n\n" + filler(seed+5000, 3000),
			History:     history(n),
			Context:     ai.TurnContext{Host: "[Current Working Memory]\nGoal: step " + fmt.Sprint(k)},
			Message:     turnText(n),
			Text:        "# CURRENT MESSAGE\nUser: " + turnText(n),
			MaxTokens:   64,
		}
		_, err := client.GenerateContent(context.Background(), prompt, false)
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(usage) == 3
	}, 5*time.Second, 50*time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	for i, u := range usage {
		t.Logf("turn %d: input=%d cached=%d", i, u.InputTokens, u.CachedTokens)
	}
	last := usage[2]
	total := last.InputTokens + last.CachedTokens
	require.Greater(t, float64(last.CachedTokens)/float64(total), 0.6, "history prefix must be served from cache by the third turn")
}
