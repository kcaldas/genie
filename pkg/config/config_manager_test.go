package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_GetString(t *testing.T) {
	manager := NewConfigManager()

	// Set a test environment variable
	os.Setenv("TEST_KEY", "test_value")
	defer os.Unsetenv("TEST_KEY")

	value, err := manager.GetString("TEST_KEY")
	require.NoError(t, err)
	assert.Equal(t, "test_value", value)
}

func TestManager_GetString_Missing(t *testing.T) {
	manager := NewConfigManager()

	_, err := manager.GetString("NON_EXISTENT_KEY")
	assert.Error(t, err)
}

func TestManager_GetStringWithDefault(t *testing.T) {
	manager := NewConfigManager()

	// Test with existing key
	os.Setenv("TEST_KEY", "test_value")
	defer os.Unsetenv("TEST_KEY")

	value := manager.GetStringWithDefault("TEST_KEY", "default_value")
	assert.Equal(t, "test_value", value)

	// Test with missing key
	value = manager.GetStringWithDefault("NON_EXISTENT_KEY", "default_value")
	assert.Equal(t, "default_value", value)
}

func TestManager_RequireString(t *testing.T) {
	manager := NewConfigManager()

	// Test with existing key
	os.Setenv("TEST_KEY", "test_value")
	defer os.Unsetenv("TEST_KEY")

	value := manager.RequireString("TEST_KEY")
	assert.Equal(t, "test_value", value)
}

func TestManager_RequireString_Panics(t *testing.T) {
	manager := NewConfigManager()

	// Test with missing key should panic
	assert.Panics(t, func() {
		manager.RequireString("NON_EXISTENT_KEY")
	})
}

func TestManager_GetBoolWithDefault(t *testing.T) {
	manager := NewConfigManager()

	// Test with existing key (true)
	os.Setenv("TEST_BOOL_TRUE", "true")
	defer os.Unsetenv("TEST_BOOL_TRUE")
	value := manager.GetBoolWithDefault("TEST_BOOL_TRUE", false)
	assert.True(t, value)

	// Test with existing key (false)
	os.Setenv("TEST_BOOL_FALSE", "false")
	defer os.Unsetenv("TEST_BOOL_FALSE")
	value = manager.GetBoolWithDefault("TEST_BOOL_FALSE", true)
	assert.False(t, value)

	// Test with missing key
	value = manager.GetBoolWithDefault("NON_EXISTENT_BOOL_KEY", true)
	assert.True(t, value)

	// Test with invalid value
	os.Setenv("TEST_BOOL_INVALID", "not-a-bool")
	defer os.Unsetenv("TEST_BOOL_INVALID")
	value = manager.GetBoolWithDefault("TEST_BOOL_INVALID", true)
	assert.True(t, value)
}