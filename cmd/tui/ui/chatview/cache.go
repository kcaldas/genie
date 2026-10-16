package chatview

import "github.com/kcaldas/genie/cmd/tui/types"

// widthsPerMessage bounds how many renderings (one per width) are kept for
// a single message, so toggling the right panel hits a warm cache without
// the cache growing with every resize.
const widthsPerMessage = 2

type cacheKey struct {
	id         int64
	width      int
	gen        uint64
	contentLen int
	role       string
	ctype      string
	live       bool
	expanded   bool // cards only: rendered open
}

// renderCache stores rendered lines per message. Entries for a message are
// kept newest-first and capped at widthsPerMessage.
type renderCache struct {
	byID map[int64][]cacheEntry
}

type cacheEntry struct {
	key   cacheKey
	lines []string
}

func newRenderCache() *renderCache {
	return &renderCache{byID: make(map[int64][]cacheEntry)}
}

func (c *renderCache) get(key cacheKey) ([]string, bool) {
	for _, e := range c.byID[key.id] {
		if e.key == key {
			return e.lines, true
		}
	}
	return nil, false
}

// put stores lines for key. A streaming message keeps only its newest
// rendering, since every chunk invalidates the previous one.
func (c *renderCache) put(key cacheKey, lines []string) {
	entries := c.byID[key.id]
	kept := entries[:0]
	for _, e := range entries {
		if e.key.width != key.width && !key.live && !e.key.live {
			kept = append(kept, e)
		}
	}
	kept = append([]cacheEntry{{key: key, lines: lines}}, kept...)
	if len(kept) > widthsPerMessage {
		kept = kept[:widthsPerMessage]
	}
	c.byID[key.id] = kept
}

// pruneExcept drops every message that is no longer in state.
func (c *renderCache) pruneExcept(msgs []types.Message) {
	if len(c.byID) <= len(msgs) {
		return
	}
	live := make(map[int64]struct{}, len(msgs))
	for _, m := range msgs {
		live[m.ID] = struct{}{}
	}
	for id := range c.byID {
		if _, ok := live[id]; !ok {
			delete(c.byID, id)
		}
	}
}

// size returns the number of cached renderings, for tests.
func (c *renderCache) size() int {
	n := 0
	for _, e := range c.byID {
		n += len(e)
	}
	return n
}
