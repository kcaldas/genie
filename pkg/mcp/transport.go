package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
)

// Transport defines the interface for MCP communication transports
type Transport interface {
	// Send sends a message to the server
	Send(ctx context.Context, message interface{}) error
	
	// Receive receives a message from the server
	Receive(ctx context.Context) ([]byte, error)
	
	// Close closes the transport connection
	Close() error
	
	// IsConnected returns true if the transport is connected
	IsConnected() bool
}

// Connectable is an optional interface for transports that need explicit connection
type Connectable interface {
	Connect(ctx context.Context) error
}

// StdioTransport implements MCP communication over stdio
type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	reader *bufio.Scanner
	mu     sync.RWMutex
	closed bool
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport(command string, args []string, env []string) *StdioTransport {
	cmd := exec.Command(command, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	return &StdioTransport{
		cmd: cmd,
	}
}

// Connect establishes the stdio connection
func (t *StdioTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.closed {
		return fmt.Errorf("transport is closed")
	}
	
	var err error
	
	// Set up pipes
	t.stdin, err = t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	
	t.stdout, err = t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	
	t.stderr, err = t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	// Start the process
	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}
	
	// Set up reader for stdout
	t.reader = bufio.NewScanner(t.stdout)
	
	return nil
}

// Send sends a JSON message over stdin
func (t *StdioTransport) Send(ctx context.Context, message interface{}) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	if t.closed || t.stdin == nil {
		return fmt.Errorf("transport is not connected")
	}
	
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	// Write JSON message followed by newline
	if _, err := t.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	
	return nil
}

// Receive receives a JSON message from stdout
func (t *StdioTransport) Receive(ctx context.Context) ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	if t.closed || t.reader == nil {
		return nil, fmt.Errorf("transport is not connected")
	}
	
	// Read line with context support
	done := make(chan bool)
	var data []byte
	var err error
	
	go func() {
		defer close(done)
		if t.reader.Scan() {
			data = []byte(t.reader.Text())
		} else {
			err = t.reader.Err()
			if err == nil {
				err = io.EOF
			}
		}
	}()
	
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
		if err != nil {
			return nil, err
		}
		return data, nil
	}
}

// Close closes the stdio transport
func (t *StdioTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.closed {
		return nil
	}
	
	t.closed = true
	
	// Close pipes
	if t.stdin != nil {
		t.stdin.Close()
	}
	if t.stdout != nil {
		t.stdout.Close()
	}
	if t.stderr != nil {
		t.stderr.Close()
	}
	
	// Terminate process
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
		t.cmd.Wait()
	}
	
	return nil
}

// IsConnected returns true if the stdio transport is connected
func (t *StdioTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return !t.closed && t.cmd != nil && t.cmd.Process != nil
}

// HTTPTransport implements MCP communication over HTTP
type HTTPTransport struct {
	baseURL string
	headers map[string]string
	client  *http.Client
	mu      sync.RWMutex
	closed  bool
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(baseURL string, headers map[string]string) *HTTPTransport {
	return &HTTPTransport{
		baseURL: baseURL,
		headers: headers,
		client:  &http.Client{},
	}
}

// Connect establishes the HTTP connection (no-op for HTTP)
func (t *HTTPTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.closed {
		return fmt.Errorf("transport is closed")
	}
	
	// HTTP doesn't need explicit connection
	return nil
}

// Send sends a JSON message over HTTP POST
func (t *HTTPTransport) Send(ctx context.Context, message interface{}) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	if t.closed {
		return fmt.Errorf("transport is closed")
	}
	
	// For HTTP transport, we would typically use a request-response pattern
	// This is a simplified implementation
	return fmt.Errorf("HTTP transport not fully implemented yet")
}

// Receive receives a JSON message from HTTP (not applicable for request-response)
func (t *HTTPTransport) Receive(ctx context.Context) ([]byte, error) {
	return nil, fmt.Errorf("HTTP transport uses request-response pattern, use Send instead")
}

// Close closes the HTTP transport
func (t *HTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.closed = true
	return nil
}

// IsConnected returns true if the HTTP transport is connected
func (t *HTTPTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return !t.closed
}

// SSETransport implements MCP communication over Server-Sent Events
type SSETransport struct {
	url     string
	headers map[string]string
	client  *http.Client
	mu      sync.RWMutex
	closed  bool
}

// NewSSETransport creates a new SSE transport
func NewSSETransport(url string, headers map[string]string) *SSETransport {
	return &SSETransport{
		url:     url,
		headers: headers,
		client:  &http.Client{},
	}
}

// Connect establishes the SSE connection
func (t *SSETransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.closed {
		return fmt.Errorf("transport is closed")
	}
	
	// SSE connection would be established here
	// This is a simplified implementation
	return fmt.Errorf("SSE transport not fully implemented yet")
}

// Send sends a message over SSE (typically not used for SSE)
func (t *SSETransport) Send(ctx context.Context, message interface{}) error {
	return fmt.Errorf("SSE transport is typically read-only")
}

// Receive receives a message from SSE stream
func (t *SSETransport) Receive(ctx context.Context) ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	if t.closed {
		return nil, fmt.Errorf("transport is closed")
	}
	
	// SSE message receiving would be implemented here
	return nil, fmt.Errorf("SSE transport not fully implemented yet")
}

// Close closes the SSE transport
func (t *SSETransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.closed = true
	return nil
}

// IsConnected returns true if the SSE transport is connected
func (t *SSETransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return !t.closed
}

// TransportFactory creates transports based on server configuration
type TransportFactory struct{}

// NewTransportFactory creates a new transport factory
func NewTransportFactory() *TransportFactory {
	return &TransportFactory{}
}

// CreateTransport creates a transport based on the server configuration
func (f *TransportFactory) CreateTransport(config ServerConfig) (Transport, error) {
	switch config.GetTransportType() {
	case TransportStdio:
		if config.Command == "" {
			return nil, fmt.Errorf("command is required for stdio transport")
		}
		
		// Convert env map to slice
		var envSlice []string
		for k, v := range config.Env {
			envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
		}
		
		return NewStdioTransport(config.Command, config.Args, envSlice), nil
		
	case TransportHTTP:
		if config.URL == "" {
			return nil, fmt.Errorf("url is required for HTTP transport")
		}
		return NewHTTPTransport(config.URL, config.Headers), nil
		
	case TransportSSE:
		if config.URL == "" {
			return nil, fmt.Errorf("url is required for SSE transport")
		}
		return NewSSETransport(config.URL, config.Headers), nil
		
	default:
		return nil, fmt.Errorf("unsupported transport type: %s", config.GetTransportType())
	}
}