package anthropic

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

// TestLive_HistoryWithToolReplayIsServedFromCache drives the real client
// through three growing turns whose history replays tool actions as
// tool_use and tool_result blocks, one of them for a tool the request
// does not declare, and checks the API accepts it and serves the prefix
// from cache. Costs a few cents; needs ANTHROPIC_API_KEY.
//
//	GENIE_LIVE_CACHE_TEST=1 ANTHROPIC_API_KEY=... go test ./pkg/llm/anthropic/ -run TestLive_ -v
func TestLive_HistoryWithToolReplayIsServedFromCache(t *testing.T) {
	if os.Getenv("GENIE_LIVE_CACHE_TEST") == "" {
		t.Skip("set GENIE_LIVE_CACHE_TEST=1 to run against the Anthropic API")
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

	nonce := uint32(time.Now().UnixNano())
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
	history := func(n int) []ai.HistoryTurn {
		turns := make([]ai.HistoryTurn, 0, n)
		for i := 0; i < n; i++ {
			turn := ai.HistoryTurn{User: filler(100+i, 40), Assistant: filler(1000+i, 300)}
			if i%4 == 0 {
				turn.Actions = []ai.HistoryAction{{Tool: "readFile", Args: "notes/" + fmt.Sprint(i) + ".md", Summary: "12 lines"}}
			}
			turns = append(turns, turn)
		}
		return turns
	}
	for k := 0; k < 3; k++ {
		prompt := ai.Prompt{
			ModelName:   "claude-haiku-4-5-20251001",
			Instruction: "You are a test agent. Reply with the single word OK.\n\n" + filler(5000, 2000),
			History:     history(20 + k),
			Context:     ai.TurnContext{Host: "[Current Working Memory]\nGoal: step " + fmt.Sprint(k)},
			Message:     filler(100+20+k, 40),
			Text:        filler(100+20+k, 40),
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
		t.Logf("turn %d: input=%d cached=%d written=%d", i, u.InputTokens, u.CacheReadInputTokens, u.CacheCreationInputTokens)
	}
	last := usage[2]
	total := last.InputTokens + last.CacheReadInputTokens + last.CacheCreationInputTokens
	require.Greater(t, float64(last.CacheReadInputTokens)/float64(total), 0.6, "history prefix must be served from cache by the third turn")
}
