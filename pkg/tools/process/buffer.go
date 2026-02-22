package process

import (
	"fmt"
	"sync"
)

// HeadTailBuffer captures the first headCap bytes and the last tailCap bytes
// of a stream. The middle is dropped. This preserves early output (errors, startup)
// and recent output (latest state) while bounding memory.
type HeadTailBuffer struct {
	mu       sync.Mutex
	head     []byte
	tail     []byte
	headCap  int
	tailCap  int
	tailPos  int   // write position in tail ring buffer
	total    int64 // total bytes ever written
	headFull bool
	tailWrap bool // true once tail has wrapped around

	// drain tracking
	lastDrain int64 // total bytes at last Drain() call
}

// NewHeadTailBuffer creates a buffer that keeps the first headCap bytes
// and the last tailCap bytes.
func NewHeadTailBuffer(headCap, tailCap int) *HeadTailBuffer {
	return &HeadTailBuffer{
		head:    make([]byte, 0, headCap),
		tail:    make([]byte, tailCap),
		headCap: headCap,
		tailCap: tailCap,
	}
}

// Write implements io.Writer. Thread-safe.
func (b *HeadTailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	n := len(p)
	remaining := p

	// Fill head first
	if !b.headFull {
		space := b.headCap - len(b.head)
		if space > 0 {
			take := space
			if take > len(remaining) {
				take = len(remaining)
			}
			b.head = append(b.head, remaining[:take]...)
			remaining = remaining[take:]
			if len(b.head) >= b.headCap {
				b.headFull = true
			}
		}
	}

	// Write remainder to tail ring buffer
	for len(remaining) > 0 {
		if b.tailCap == 0 {
			break
		}
		take := b.tailCap - b.tailPos
		if take > len(remaining) {
			take = len(remaining)
		}
		copy(b.tail[b.tailPos:b.tailPos+take], remaining[:take])
		remaining = remaining[take:]
		b.tailPos += take
		if b.tailPos >= b.tailCap {
			b.tailPos = 0
			b.tailWrap = true
		}
	}

	b.total += int64(n)
	return n, nil
}

// Snapshot returns the captured output: head + "[truncated N bytes]" + tail.
// If output fits entirely in head, returns just head with no truncation marker.
func (b *HeadTailBuffer) Snapshot() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.snapshotLocked()
}

func (b *HeadTailBuffer) snapshotLocked() string {
	headBytes := b.headLen()
	tailBytes := b.tailLen()

	// If everything fits in head (no tail data)
	if tailBytes == 0 {
		return string(b.head[:headBytes])
	}

	truncated := b.total - int64(headBytes) - int64(tailBytes)

	tail := b.orderedTail()

	if truncated > 0 {
		return string(b.head[:headBytes]) +
			fmt.Sprintf("\n[...truncated %d bytes...]\n", truncated) +
			string(tail)
	}

	// head is full but no truncation — just concatenate
	return string(b.head[:headBytes]) + string(tail)
}

// Drain returns new output since the last Drain() call. On first call,
// returns all output. Thread-safe.
func (b *HeadTailBuffer) Drain() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	// First drain or no new data
	if b.total == b.lastDrain {
		return ""
	}

	// For simplicity, drain returns the full snapshot — callers compare
	// against previous output. This works because LLMs need context.
	result := b.snapshotLocked()
	b.lastDrain = b.total
	return result
}

// TotalBytes returns total bytes written to the buffer.
func (b *HeadTailBuffer) TotalBytes() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.total
}

func (b *HeadTailBuffer) headLen() int {
	return len(b.head)
}

func (b *HeadTailBuffer) tailLen() int {
	if !b.tailWrap {
		return b.tailPos
	}
	return b.tailCap
}

// orderedTail returns tail bytes in chronological order.
func (b *HeadTailBuffer) orderedTail() []byte {
	if !b.tailWrap {
		return b.tail[:b.tailPos]
	}
	// Ring wrapped: tailPos..end + 0..tailPos
	ordered := make([]byte, b.tailCap)
	n := copy(ordered, b.tail[b.tailPos:])
	copy(ordered[n:], b.tail[:b.tailPos])
	return ordered
}
