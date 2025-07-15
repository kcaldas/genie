package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".mcp.json")

	configContent := `{
  "mcpServers": {
    "test-server": {
      "command": "/path/to/server",
      "args": ["--arg1", "value1"],
      "env": {
        "TEST_VAR": "test_value"
      }
    },
    "sse-server": {
      "type": "sse",
      "url": "https://example.com/mcp",
      "headers": {
        "Authorization": "Bearer token123"
      }
    }
  }
}`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load the config
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify the config was loaded correctly
	if len(config.McpServers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(config.McpServers))
	}

	// Test stdio server
	testServer, exists := config.McpServers["test-server"]
	if !exists {
		t.Error("test-server not found in config")
	}

	if testServer.Command != "/path/to/server" {
		t.Errorf("Expected command '/path/to/server', got '%s'", testServer.Command)
	}

	if len(testServer.Args) != 2 || testServer.Args[0] != "--arg1" || testServer.Args[1] != "value1" {
		t.Errorf("Expected args ['--arg1', 'value1'], got %v", testServer.Args)
	}

	if testServer.Env["TEST_VAR"] != "test_value" {
		t.Errorf("Expected env TEST_VAR='test_value', got '%s'", testServer.Env["TEST_VAR"])
	}

	// Test SSE server
	sseServer, exists := config.McpServers["sse-server"]
	if !exists {
		t.Error("sse-server not found in config")
	}

	if sseServer.Type != "sse" {
		t.Errorf("Expected type 'sse', got '%s'", sseServer.Type)
	}

	if sseServer.URL != "https://example.com/mcp" {
		t.Errorf("Expected URL 'https://example.com/mcp', got '%s'", sseServer.URL)
	}

	if sseServer.Headers["Authorization"] != "Bearer token123" {
		t.Errorf("Expected Authorization header 'Bearer token123', got '%s'", sseServer.Headers["Authorization"])
	}
}

func TestExpandEnvVars(t *testing.T) {
	// Set test environment variables
	os.Setenv("TEST_VAR", "test_value")
	os.Setenv("API_KEY", "secret123")
	defer func() {
		os.Unsetenv("TEST_VAR")
		os.Unsetenv("API_KEY")
	}()

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `{"url": "${TEST_VAR}"}`,
			expected: `{"url": "test_value"}`,
		},
		{
			input:    `{"token": "${API_KEY}"}`,
			expected: `{"token": "secret123"}`,
		},
		{
			input:    `{"url": "${NONEXISTENT:-default_value}"}`,
			expected: `{"url": "default_value"}`,
		},
		{
			input:    `{"url": "${TEST_VAR:-fallback}"}`,
			expected: `{"url": "test_value"}`,
		},
		{
			input:    `{"url": "${MISSING_VAR:-}"}`,
			expected: `{"url": ""}`,
		},
	}

	for _, test := range tests {
		result := expandEnvVars(test.input)
		if result != test.expected {
			t.Errorf("expandEnvVars(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestServerConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  ServerConfig
		wantErr bool
	}{
		{
			name: "valid stdio server",
			config: ServerConfig{
				Command: "/path/to/server",
				Args:    []string{"--arg"},
			},
			wantErr: false,
		},
		{
			name: "invalid stdio server - no command",
			config: ServerConfig{
				Args: []string{"--arg"},
			},
			wantErr: true,
		},
		{
			name: "valid SSE server",
			config: ServerConfig{
				Type: "sse",
				URL:  "https://example.com/mcp",
			},
			wantErr: false,
		},
		{
			name: "invalid SSE server - no URL",
			config: ServerConfig{
				Type: "sse",
			},
			wantErr: true,
		},
		{
			name: "valid HTTP server",
			config: ServerConfig{
				Type: "http",
				URL:  "https://example.com/mcp",
			},
			wantErr: false,
		},
		{
			name: "invalid HTTP server - no URL",
			config: ServerConfig{
				Type: "http",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ServerConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetTransportType(t *testing.T) {
	tests := []struct {
		config   ServerConfig
		expected TransportType
	}{
		{
			config:   ServerConfig{Command: "/path/to/server"},
			expected: TransportStdio,
		},
		{
			config:   ServerConfig{Type: "stdio"},
			expected: TransportStdio,
		},
		{
			config:   ServerConfig{Type: "sse", URL: "https://example.com"},
			expected: TransportSSE,
		},
		{
			config:   ServerConfig{Type: "http", URL: "https://example.com"},
			expected: TransportHTTP,
		},
		{
			config:   ServerConfig{Type: "unknown"},
			expected: TransportStdio, // defaults to stdio
		},
	}

	for _, test := range tests {
		result := test.config.GetTransportType()
		if result != test.expected {
			t.Errorf("GetTransportType() = %v, expected %v for config %+v", result, test.expected, test.config)
		}
	}
}