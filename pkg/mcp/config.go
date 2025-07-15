package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Config represents the structure of .mcp.json configuration files
type Config struct {
	McpServers map[string]ServerConfig `json:"mcpServers"`
}

// ServerConfig defines the configuration for an MCP server
type ServerConfig struct {
	// For stdio servers
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	
	// For SSE/HTTP servers
	Type    string            `json:"type,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// TransportType represents the type of transport for an MCP server
type TransportType string

const (
	TransportStdio TransportType = "stdio"
	TransportSSE   TransportType = "sse"
	TransportHTTP  TransportType = "http"
)

// GetTransportType returns the transport type for this server config
func (sc ServerConfig) GetTransportType() TransportType {
	switch strings.ToLower(sc.Type) {
	case "sse":
		return TransportSSE
	case "http":
		return TransportHTTP
	default:
		// Default to stdio if not specified or if command is present
		return TransportStdio
	}
}

// LoadConfig loads MCP configuration from a .mcp.json file
func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables in the JSON
	expandedData := expandEnvVars(string(data))

	var config Config
	if err := json.Unmarshal([]byte(expandedData), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	return &config, nil
}

// FindConfigFile looks for .mcp.json files in project and user scope
func FindConfigFile(projectRoot string) (string, error) {
	// First try project-scoped config
	projectConfig := filepath.Join(projectRoot, ".mcp.json")
	if _, err := os.Stat(projectConfig); err == nil {
		return projectConfig, nil
	}

	// Then try user-scoped config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	userConfig := filepath.Join(homeDir, ".config", "claude", "mcp.json")
	if _, err := os.Stat(userConfig); err == nil {
		return userConfig, nil
	}

	// Alternative user config location
	altUserConfig := filepath.Join(homeDir, ".mcp.json")
	if _, err := os.Stat(altUserConfig); err == nil {
		return altUserConfig, nil
	}

	return "", fmt.Errorf("no MCP configuration file found")
}

// expandEnvVars expands environment variables in the format ${VAR} or ${VAR:-default}
func expandEnvVars(input string) string {
	// Regex to match ${VAR} or ${VAR:-default}
	re := regexp.MustCompile(`\$\{([^}:]+)(?::-(.*?))?\}`)
	
	return re.ReplaceAllStringFunc(input, func(match string) string {
		// Extract variable name and default value
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}
		
		varName := matches[1]
		defaultValue := ""
		if len(matches) > 2 {
			defaultValue = matches[2]
		}
		
		// Get environment variable value
		if value := os.Getenv(varName); value != "" {
			return value
		}
		
		return defaultValue
	})
}

// Validate checks if the server configuration is valid
func (sc ServerConfig) Validate() error {
	transport := sc.GetTransportType()
	
	switch transport {
	case TransportStdio:
		if sc.Command == "" {
			return fmt.Errorf("command is required for stdio transport")
		}
	case TransportSSE, TransportHTTP:
		if sc.URL == "" {
			return fmt.Errorf("url is required for %s transport", transport)
		}
	}
	
	return nil
}