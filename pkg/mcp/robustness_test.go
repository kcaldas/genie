package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Receive abandoned by context cancellation must not steal the next
// line from the server: the old implementation left a goroutine blocked
// in Scanner.Scan that consumed the following response, permanently
// desynchronizing the JSON-RPC pairing for that server.
func TestStdioTransportStaysInSyncAfterCancelledReceive(t *testing.T) {
	transport := NewStdioTransport("cat", nil, nil)
	require.NoError(t, transport.Connect(context.Background()))
	defer transport.Close()

	// Nothing to read yet: a cancelled Receive must return promptly.
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := transport.Receive(cancelled)
	require.Error(t, err)

	// Whatever arrives next must be delivered to the NEXT Receive call,
	// in order, not swallowed by the abandoned one.
	require.NoError(t, transport.Send(context.Background(), map[string]any{"id": 1}))
	require.NoError(t, transport.Send(context.Background(), map[string]any{"id": 2}))

	ctx, cancelAll := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelAll()

	first, err := transport.Receive(ctx)
	require.NoError(t, err)
	assert.Contains(t, string(first), `"id":1`)

	second, err := transport.Receive(ctx)
	require.NoError(t, err)
	assert.Contains(t, string(second), `"id":2`)
}

// A server that floods stderr must not wedge: if nobody drains the
// stderr pipe, the child blocks on write(2) once the pipe buffer fills
// and never gets around to answering on stdout.
func TestStdioTransportSurvivesStderrFlood(t *testing.T) {
	script := `head -c 1048576 /dev/zero | tr '\0' 'x' >&2; echo '{"id":42}'`
	transport := NewStdioTransport("sh", []string{"-c", script}, nil)
	require.NoError(t, transport.Connect(context.Background()))
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	line, err := transport.Receive(ctx)
	require.NoError(t, err, "a stderr flood must not block the server's stdout response")
	assert.Contains(t, string(line), `"id":42`)
}

// queuedTransport replays canned messages, simulating a chatty server.
type queuedTransport struct {
	queue [][]byte
	sent  []any
}

func (q *queuedTransport) Send(ctx context.Context, message interface{}) error {
	q.sent = append(q.sent, message)
	return nil
}

func (q *queuedTransport) Receive(ctx context.Context) ([]byte, error) {
	if len(q.queue) == 0 {
		return nil, fmt.Errorf("no more messages")
	}
	msg := q.queue[0]
	q.queue = q.queue[1:]
	return msg, nil
}

func (q *queuedTransport) Close() error      { return nil }
func (q *queuedTransport) IsConnected() bool { return true }

// A server may emit any number of log notifications before answering;
// the client must keep reading until the matching response arrives
// (the old implementation gave up after five messages).
func TestReceiveResponseSkipsManyNotifications(t *testing.T) {
	var queue [][]byte
	for i := 0; i < 20; i++ {
		notif, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"method":  "notifications/message",
			"params":  map[string]any{"level": "info", "data": fmt.Sprintf("log line %d", i)},
		})
		queue = append(queue, notif)
	}
	response, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      7,
		"result":  map[string]any{"ok": true},
	})
	queue = append(queue, response)

	client := NewClient(&Config{})
	transport := &queuedTransport{queue: queue}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.receiveResponseForRequest(ctx, transport, 7)
	require.NoError(t, err, "notifications before the response must be skipped, however many there are")
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
}
