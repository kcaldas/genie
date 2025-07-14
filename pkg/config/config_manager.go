package config

import (
	"fmt"
	"os"
	"strconv"
)

// ModelConfig represents the default model configuration
type ModelConfig struct {
	ModelName   string
	MaxTokens   int32
	Temperature float32
	TopP        float32
}

// Manager provides configuration management functionality
type Manager interface {
	GetString(key string) (string, error)
	GetStringWithDefault(key, defaultValue string) string
	RequireString(key string) string
	GetInt(key string) (int, error)
	GetIntWithDefault(key string, defaultValue int) int
	GetBoolWithDefault(key string, defaultValue bool) bool
	GetModelConfig() ModelConfig
}

// DefaultManager implements the Manager interface
type DefaultManager struct {
}

// NewConfigManager creates a new default config manager
func NewConfigManager() Manager {
	return &DefaultManager{}
}

// GetString gets a configuration value by key, returns error if not found
func (m *DefaultManager) GetString(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("configuration key %s not found", key)
	}
	return value, nil
}

// GetStringWithDefault gets a configuration value by key, returns default if not found
func (m *DefaultManager) GetStringWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// RequireString gets a configuration value by key, panics if not found
func (m *DefaultManager) RequireString(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("required configuration key %s not found", key))
	}
	return value
}

// GetInt gets an integer configuration value by key, returns error if not found or invalid
func (m *DefaultManager) GetInt(key string) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return 0, fmt.Errorf("configuration key %s not found", key)
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("configuration key %s has invalid integer value: %s", key, value)
	}
	return intValue, nil
}

// GetIntWithDefault gets an integer configuration value by key, returns default if not found or invalid
func (m *DefaultManager) GetIntWithDefault(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intValue
}

// GetBoolWithDefault gets a boolean configuration value by key, returns default if not found or invalid
func (m *DefaultManager) GetBoolWithDefault(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return boolValue
}

// GetModelConfig returns the default model configuration from environment variables or defaults
func (m *DefaultManager) GetModelConfig() ModelConfig {
	modelName := m.GetStringWithDefault("GENIE_MODEL_NAME", "gemini-1.5-pro-latest")

	maxTokensStr := m.GetStringWithDefault("GENIE_MAX_TOKENS", "8192")
	maxTokens, err := strconv.ParseInt(maxTokensStr, 10, 32)
	if err != nil {
		maxTokens = 8192 // fallback to default
	}

	tempStr := m.GetStringWithDefault("GENIE_MODEL_TEMPERATURE", "0.7")
	temperature, err := strconv.ParseFloat(tempStr, 32)
	if err != nil {
		temperature = 0.7 // fallback to default
	}

	topPStr := m.GetStringWithDefault("GENIE_TOP_P", "0.9")
	topP, err := strconv.ParseFloat(topPStr, 32)
	if err != nil {
		topP = 0.9 // fallback to default
	}

	return ModelConfig{
		ModelName:   modelName,
		MaxTokens:   int32(maxTokens),
		Temperature: float32(temperature),
		TopP:        float32(topP),
	}
}