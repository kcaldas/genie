package process

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeadTailBuffer_SmallOutput(t *testing.T) {
	buf := NewHeadTailBuffer(1024, 1024)
	n, err := buf.Write([]byte("hello world"))
	require.NoError(t, err)
	assert.Equal(t, 11, n)
	assert.Equal(t, "hello world", buf.Snapshot())
	assert.Equal(t, int64(11), buf.TotalBytes())
}

func TestHeadTailBuffer_FitsInHead(t *testing.T) {
	buf := NewHeadTailBuffer(100, 100)
	buf.Write([]byte("short"))
	snap := buf.Snapshot()
	assert.Equal(t, "short", snap)
	assert.NotContains(t, snap, "truncated")
}

func TestHeadTailBuffer_Truncation(t *testing.T) {
	// Head=10, Tail=10, write 30 bytes
	buf := NewHeadTailBuffer(10, 10)

	// Write "0123456789" (fills head)
	buf.Write([]byte("0123456789"))
	// Write "AAAAAAAAAA" (fills tail)
	buf.Write([]byte("AAAAAAAAAA"))
	// Write "BBBBBBBBBB" (overwrites tail)
	buf.Write([]byte("BBBBBBBBBB"))

	snap := buf.Snapshot()
	assert.Contains(t, snap, "0123456789")   // head preserved
	assert.Contains(t, snap, "BBBBBBBBBB")   // latest tail
	assert.Contains(t, snap, "truncated")     // middle dropped
	assert.Equal(t, int64(30), buf.TotalBytes())
}

func TestHeadTailBuffer_ExactHeadFit(t *testing.T) {
	buf := NewHeadTailBuffer(5, 5)
	buf.Write([]byte("12345"))
	assert.Equal(t, "12345", buf.Snapshot())
}

func TestHeadTailBuffer_HeadPlusTailNoTruncation(t *testing.T) {
	// Head=5, Tail=5, write 10 bytes → head full, tail full, no truncation
	buf := NewHeadTailBuffer(5, 5)
	buf.Write([]byte("1234567890"))
	snap := buf.Snapshot()
	assert.Contains(t, snap, "12345")
	assert.Contains(t, snap, "67890")
	// Total = 10, head = 5, tail = 5 → truncated = 0
	assert.NotContains(t, snap, "truncated")
}

func TestHeadTailBuffer_LargeWrite(t *testing.T) {
	buf := NewHeadTailBuffer(512, 512)

	// Write 2048 bytes
	data := strings.Repeat("X", 2048)
	buf.Write([]byte(data))

	snap := buf.Snapshot()
	assert.Contains(t, snap, "truncated")
	assert.Equal(t, int64(2048), buf.TotalBytes())

	// Head should be 512 X's
	assert.True(t, strings.HasPrefix(snap, strings.Repeat("X", 512)))
}

func TestHeadTailBuffer_MultipleWrites(t *testing.T) {
	buf := NewHeadTailBuffer(10, 10)
	buf.Write([]byte("hello"))
	buf.Write([]byte(" world"))
	assert.Equal(t, "hello world", buf.Snapshot())
}

func TestHeadTailBuffer_Drain_FirstCall(t *testing.T) {
	buf := NewHeadTailBuffer(1024, 1024)
	buf.Write([]byte("hello"))
	d := buf.Drain()
	assert.Equal(t, "hello", d)
}

func TestHeadTailBuffer_Drain_NoNewData(t *testing.T) {
	buf := NewHeadTailBuffer(1024, 1024)
	buf.Write([]byte("hello"))
	buf.Drain()
	d := buf.Drain()
	assert.Equal(t, "", d)
}

func TestHeadTailBuffer_Drain_IncrementalReads(t *testing.T) {
	buf := NewHeadTailBuffer(1024, 1024)
	buf.Write([]byte("hello"))
	d1 := buf.Drain()
	assert.Equal(t, "hello", d1)

	buf.Write([]byte(" world"))
	d2 := buf.Drain()
	assert.NotEmpty(t, d2)
	assert.Contains(t, d2, "world")
}

func TestHeadTailBuffer_ConcurrentWrites(t *testing.T) {
	buf := NewHeadTailBuffer(1024, 1024)

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				buf.Write([]byte("x"))
			}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, int64(1000), buf.TotalBytes())
}

func TestHeadTailBuffer_Empty(t *testing.T) {
	buf := NewHeadTailBuffer(1024, 1024)
	assert.Equal(t, "", buf.Snapshot())
	assert.Equal(t, "", buf.Drain())
	assert.Equal(t, int64(0), buf.TotalBytes())
}

func TestHeadTailBuffer_TailRingWrap(t *testing.T) {
	// Tail capacity 4, write enough to wrap multiple times
	buf := NewHeadTailBuffer(2, 4)
	buf.Write([]byte("AB"))           // fills head
	buf.Write([]byte("CDEF"))         // fills tail: [C,D,E,F]
	buf.Write([]byte("GH"))           // overwrites: [G,H,E,F] pos=2

	snap := buf.Snapshot()
	assert.Contains(t, snap, "AB")    // head
	assert.Contains(t, snap, "EFGH")  // tail in order
}
