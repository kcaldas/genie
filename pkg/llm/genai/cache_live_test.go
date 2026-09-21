package genai

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

// TestLive_HistoryPrefixIsServedFromCache drives the real Gemini client
// through three growing turns and checks that, by the third, most of the
// request is served from Gemini's implicit cache. It costs a few cents
// and needs Vertex credentials, so it only runs when asked for:
//
//	GENIE_LIVE_CACHE_TEST=1 GENAI_BACKEND=vertex GOOGLE_CLOUD_PROJECT=<project> \
//	  GOOGLE_CLOUD_LOCATION=global go test ./pkg/llm/genai/ -run TestLive_ -v
func TestLive_HistoryPrefixIsServedFromCache(t *testing.T) {
	if os.Getenv("GENIE_LIVE_CACHE_TEST") == "" {
		t.Skip("set GENIE_LIVE_CACHE_TEST=1 to run against Vertex")
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

	filler := func(seed, words int) string {
		vocab := strings.Fields("shipment freight carrier quote pallet dock terminal lane rate chapter draft manuscript editor scene character revision")
		var b strings.Builder
		x := uint32(seed)
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
			turns = append(turns, ai.HistoryTurn{User: filler(100+i, 40), Assistant: filler(200+i, 700)})
		}
		return turns
	}
	for k := 0; k < 3; k++ {
		prompt := ai.Prompt{
			Instruction: "You are a test agent. Reply with the single word OK.\n\n" + filler(1, 3000),
			History:     history(40 + k),
			Context:     ai.TurnContext{Host: "[Current Working Memory]\nGoal: step " + fmt.Sprint(k)},
			Text:        fmt.Sprintf("User: message %d", k),
			ModelName:   "gemini-3.7-flash",
			MaxTokens:   256,
		}
		_, err := client.GenerateContent(context.Background(), prompt, false)
		require.NoError(t, err)
	}

	// The bus delivers off the calling goroutine; give the last event a moment.
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
