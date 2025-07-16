package mcp

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// NewMCPClientFromConfig creates an MCP client by discovering and loading configuration files
func NewMCPClientFromConfig() (*Client, error) {
	// Get current working directory for project-scoped config
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	// Try to find MCP configuration file
	configPath, err := FindConfigFile(cwd)
	if err != nil {
		// No MCP config found, return a client with empty config
		// This allows the system to work without MCP servers
		return NewClient(&Config{McpServers: make(map[string]ServerConfig)}), nil
	}

	// Load the configuration
	config, err := LoadConfig(configPath)
	if err != nil {
		// If we can't load the config, return an empty client rather than failing
		// This ensures the system remains functional even with invalid MCP config
		return NewClient(&Config{McpServers: make(map[string]ServerConfig)}), nil
	}

	// Create client with the loaded configuration
	client := NewClient(config)

	// Connect to servers synchronously with a short timeout
	// This ensures tools are available immediately but doesn't block startup too long
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	
	if err := client.ConnectToServers(ctx); err != nil {
		// Log error but don't fail startup - just return client with no tools
		// This allows the system to work even if MCP servers are slow/unavailable
	}

	return client, nil
}

// GetConfigPath returns the path to the MCP configuration file if it exists
func GetConfigPath(projectRoot string) (string, bool) {
	configPath, err := FindConfigFile(projectRoot)
	if err != nil {
		return "", false
	}
	return configPath, true
}

// LoadProjectConfig loads MCP configuration from a specific project directory
func LoadProjectConfig(projectRoot string) (*Config, error) {
	projectConfig := filepath.Join(projectRoot, ".mcp.json")
	if _, err := os.Stat(projectConfig); err == nil {
		return LoadConfig(projectConfig)
	}
	return nil, os.ErrNotExist
}

// LoadUserConfig loads MCP configuration from user-scoped locations
func LoadUserConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Try multiple user config locations
	userConfigPaths := []string{
		filepath.Join(homeDir, ".config", "claude", "mcp.json"),
		filepath.Join(homeDir, ".mcp.json"),
	}

	for _, configPath := range userConfigPaths {
		if _, err := os.Stat(configPath); err == nil {
			return LoadConfig(configPath)
		}
	}

	return nil, os.ErrNotExist
}