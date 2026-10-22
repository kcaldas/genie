package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kcaldas/genie/pkg/tools"
)

// Config represents the structure of .mcp.json configuration files
type Config struct {
	McpServers map[string]ServerConfig `json:"mcpServers"`
}

// ServerConfig defines the configuration for an MCP server
type ServerConfig struct {
	// For stdio servers
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`

	// ToolPrefix disambiguates model-visible names; wire names remain unchanged.
	ToolPrefix string `json:"toolPrefix,omitempty"`
	// For SSE/HTTP servers
	Type    string            `json:"type,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`

	// Display metadata (Genie extension; other .mcp.json readers ignore
	// these fields). DisplayName is the human-readable server name used
	// to group its tools. SignalText is the progress text hosts may show
	// while any of this server's tools runs. Tools carries per-tool
	// overrides keyed by tool name.
	DisplayName string                       `json:"displayName,omitempty"`
	SignalText  tools.LocalizedText          `json:"signalText,omitempty"`
	Tools       map[string]ToolDisplayConfig `json:"tools,omitempty"`
}

// ToolDisplayConfig is the per-tool display override inside a server's
// config entry.
type ToolDisplayConfig struct {
	DisplayName string              `json:"displayName,omitempty"`
	SignalText  tools.LocalizedText `json:"signalText,omitempty"`
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
	case "http", "streamable-http":
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
	switch strings.ToLower(sc.Type) {
	case "", "stdio", "http", "streamable-http", "sse":
	default:
		return fmt.Errorf("unknown MCP transport %q", sc.Type)
	}
	if sc.ToolPrefix != "" && !regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`).MatchString(sc.ToolPrefix) {
		return fmt.Errorf("toolPrefix contains invalid tool-name characters")
	}

	switch transport {
	case TransportStdio:
		if sc.Command == "" {
			return fmt.Errorf("command is required for stdio transport")
		}
	case TransportSSE, TransportHTTP:
		u, err := url.Parse(sc.URL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
			return fmt.Errorf("MCP URL must be an absolute HTTP(S) URL without credentials or fragment")
		}
		for name, value := range sc.Headers {
			lower := strings.ToLower(name)
			if strings.ContainsAny(name+value, "\r\n") {
				return fmt.Errorf("invalid MCP header")
			}
			if lower == "host" || lower == "content-length" || lower == "content-type" || lower == "accept" || strings.HasPrefix(lower, "mcp-") {
				return fmt.Errorf("MCP protocol header %q cannot be overridden", name)
			}
		}
	}

	return nil
}
