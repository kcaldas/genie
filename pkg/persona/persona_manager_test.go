package persona

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPersonaManager is a mock implementation of PersonaManager
type MockPersonaManager struct {
	mock.Mock
}

func (m *MockPersonaManager) GetPrompt(ctx context.Context) (*ai.Prompt, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ai.Prompt), args.Error(1)
}

func (m *MockPersonaManager) ListPersonas(ctx context.Context) ([]Persona, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Persona), args.Error(1)
}

// TestPersonaManagerInterface ensures the interface is properly defined
func TestPersonaManagerInterface(t *testing.T) {
	// This test verifies that MockPersonaManager implements PersonaManager
	var _ PersonaManager = (*MockPersonaManager)(nil)
}

// TestMockPersonaManager_GetPrompt tests the mock implementation
func TestMockPersonaManager_GetPrompt(t *testing.T) {
	mockManager := new(MockPersonaManager)
	ctx := context.Background()

	// Create a mock prompt
	mockPrompt := &ai.Prompt{}

	// Set up expectations
	mockManager.On("GetPrompt", ctx).Return(mockPrompt, nil)

	// Call the method
	prompt, err := mockManager.GetPrompt(ctx)

	// Assert results
	assert.NoError(t, err)
	assert.NotNil(t, prompt)
	assert.Equal(t, mockPrompt, prompt)

	// Verify expectations were met
	mockManager.AssertExpectations(t)
}

// TestMockPersonaManager_GetPrompt_Error tests error handling
func TestMockPersonaManager_GetPrompt_Error(t *testing.T) {
	mockManager := new(MockPersonaManager)
	ctx := context.Background()

	// Set up expectations for error case
	mockManager.On("GetPrompt", ctx).Return(nil, assert.AnError)

	// Call the method
	prompt, err := mockManager.GetPrompt(ctx)

	// Assert error results
	assert.Error(t, err)
	assert.Nil(t, prompt)

	// Verify expectations were met
	mockManager.AssertExpectations(t)
}

// MockPersonaAwarePromptFactory is a mock implementation of persona.PersonaAwarePromptFactory
type MockPersonaAwarePromptFactory struct {
	mock.Mock
}

func (m *MockPersonaAwarePromptFactory) GetPrompt(ctx context.Context, personaName string) (*ai.Prompt, error) {
	args := m.Called(ctx, personaName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ai.Prompt), args.Error(1)
}

// MockConfigManager is a mock implementation of config.Manager
type MockConfigManager struct {
	mock.Mock
}

func (m *MockConfigManager) GetString(key string) (string, error) {
	args := m.Called(key)
	return args.String(0), args.Error(1)
}

func (m *MockConfigManager) GetStringWithDefault(key, defaultValue string) string {
	args := m.Called(key, defaultValue)
	return args.String(0)
}

func (m *MockConfigManager) RequireString(key string) string {
	args := m.Called(key)
	return args.String(0)
}

func (m *MockConfigManager) GetInt(key string) (int, error) {
	args := m.Called(key)
	return args.Int(0), args.Error(1)
}

func (m *MockConfigManager) GetIntWithDefault(key string, defaultValue int) int {
	args := m.Called(key, defaultValue)
	return args.Int(0)
}

func (m *MockConfigManager) GetBoolWithDefault(key string, defaultValue bool) bool {
	args := m.Called(key, defaultValue)
	return args.Bool(0)
}

func (m *MockConfigManager) GetModelConfig() config.ModelConfig {
	args := m.Called()
	return args.Get(0).(config.ModelConfig)
}

func (m *MockConfigManager) GetDurationWithDefault(key string, defaultValue time.Duration) time.Duration {
	args := m.Called(key, defaultValue)
	return args.Get(0).(time.Duration)
}

// TestDefaultPersonaManager_GetPrompt tests the default implementation
func TestDefaultPersonaManager_GetPrompt(t *testing.T) {
	mockFactory := new(MockPersonaAwarePromptFactory)
	mockConfig := new(MockConfigManager)
	
	// Set up config expectations - should return "genie" as default
	mockConfig.On("GetStringWithDefault", "GENIE_PERSONA", "genie").Return("genie")
	
	manager := NewDefaultPersonaManager(mockFactory, mockConfig)

	ctx := context.Background()

	// Create a mock prompt
	mockPrompt := &ai.Prompt{}

	// Set up expectations - default persona is "genie"
	mockFactory.On("GetPrompt", ctx, "genie").Return(mockPrompt, nil)

	// Call the method
	prompt, err := manager.GetPrompt(ctx)

	// Assert results
	assert.NoError(t, err)
	assert.NotNil(t, prompt)
	assert.Equal(t, mockPrompt, prompt)

	// Verify expectations were met
	mockFactory.AssertExpectations(t)
	mockConfig.AssertExpectations(t)
}

// TestDefaultPersonaManager_GetPrompt_FactoryError tests error handling from factory
func TestDefaultPersonaManager_GetPrompt_FactoryError(t *testing.T) {
	mockFactory := new(MockPersonaAwarePromptFactory)
	mockConfig := new(MockConfigManager)
	
	// Set up config expectations - should return "genie" as default
	mockConfig.On("GetStringWithDefault", "GENIE_PERSONA", "genie").Return("genie")
	
	manager := NewDefaultPersonaManager(mockFactory, mockConfig)

	ctx := context.Background()

	// Set up expectations for error case - default persona is "genie"
	mockFactory.On("GetPrompt", ctx, "genie").Return(nil, assert.AnError)

	// Call the method
	prompt, err := manager.GetPrompt(ctx)

	// Assert error results
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "assert.AnError general error for testing")
	assert.Nil(t, prompt)

	// Verify expectations were met
	mockFactory.AssertExpectations(t)
	mockConfig.AssertExpectations(t)
}

// TestDefaultPersonaManager_ListPersonas_AlwaysReturnsInternalPersonas tests that ListPersonas always returns at least internal personas
func TestDefaultPersonaManager_ListPersonas_AlwaysReturnsInternalPersonas(t *testing.T) {
	mockFactory := new(MockPersonaAwarePromptFactory)
	mockConfig := new(MockConfigManager)
	
	// Set up config expectations
	mockConfig.On("GetStringWithDefault", "GENIE_PERSONA", "genie").Return("genie")
	
	manager := NewDefaultPersonaManager(mockFactory, mockConfig)
	
	ctx := context.Background()
	
	// Call the method
	personas, err := manager.ListPersonas(ctx)
	
	// Assert results - should always succeed and return at least internal personas
	assert.NoError(t, err)
	assert.NotNil(t, personas)
	assert.Greater(t, len(personas), 0, "Should have at least internal personas")
	
	// Verify all returned personas are internal
	for _, persona := range personas {
		assert.Equal(t, PersonaSourceInternal, persona.Source)
	}
}

// TestDefaultPersonaManager_ListPersonas_InternalPersonas tests listing internal personas
func TestDefaultPersonaManager_ListPersonas_InternalPersonas(t *testing.T) {
	mockFactory := new(MockPersonaAwarePromptFactory)
	mockConfig := new(MockConfigManager)
	
	// Set up config expectations
	mockConfig.On("GetStringWithDefault", "GENIE_PERSONA", "genie").Return("genie")
	
	manager := NewDefaultPersonaManager(mockFactory, mockConfig)
	
	ctx := context.Background()
	
	// Call the method
	personas, err := manager.ListPersonas(ctx)
	
	// This test expects to find the internal personas
	assert.NoError(t, err)
	assert.NotNil(t, personas)
	
	// We know there are at least these internal personas based on the file listing
	expectedPersonas := map[string]bool{
		"engineer":        true,
		"genie":           true,
		"minimal":         true,
		"persona_creator": true,
		"product_owner":   true,
	}
	
	// Check that we have at least the expected internal personas
	assert.GreaterOrEqual(t, len(personas), len(expectedPersonas))
	
	// Check each persona has the right properties
	for _, persona := range personas {
		if expectedPersonas[persona.ID] {
			assert.Equal(t, PersonaSourceInternal, persona.Source)
			assert.NotEmpty(t, persona.Name)
		}
	}
}

// TestDefaultPersonaManager_ListPersonas_UserPersonas tests listing user personas
func TestDefaultPersonaManager_ListPersonas_UserPersonas(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "genie-test-personas")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)
	
	// Create a test persona directory structure
	userPersonasDir := filepath.Join(tempDir, ".genie", "personas")
	testPersonaDir := filepath.Join(userPersonasDir, "test-persona")
	err = os.MkdirAll(testPersonaDir, 0755)
	assert.NoError(t, err)
	
	// Create a test prompt.yaml file
	promptContent := `name: "Test Persona"
instruction: |
  You are a test persona.
`
	err = os.WriteFile(filepath.Join(testPersonaDir, "prompt.yaml"), []byte(promptContent), 0644)
	assert.NoError(t, err)
	
	// Create manager with mocked home directory
	mockFactory := new(MockPersonaAwarePromptFactory)
	mockConfig := new(MockConfigManager)
	mockConfig.On("GetStringWithDefault", "GENIE_PERSONA", "genie").Return("genie")
	
	manager := &DefaultPersonaManager{
		promptFactory:  mockFactory,
		configManager:  mockConfig,
		defaultPersona: "genie",
		userHome:       tempDir,
	}
	
	ctx := context.Background()
	
	// Call the method
	personas, err := manager.ListPersonas(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, personas)
	
	// Find the test persona
	var foundTestPersona bool
	for _, persona := range personas {
		if persona.ID == "test-persona" {
			foundTestPersona = true
			assert.Equal(t, "Test Persona", persona.Name)
			assert.Equal(t, PersonaSourceUser, persona.Source)
		}
	}
	
	assert.True(t, foundTestPersona, "Should find the test persona")
}

// TestDefaultPersonaManager_ListPersonas_ProjectPersonas tests listing project personas
func TestDefaultPersonaManager_ListPersonas_ProjectPersonas(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "genie-test-project")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)
	
	// Create a test persona directory structure for project
	projectPersonasDir := filepath.Join(tempDir, ".genie", "personas")
	projectPersonaDir := filepath.Join(projectPersonasDir, "project-persona")
	err = os.MkdirAll(projectPersonaDir, 0755)
	assert.NoError(t, err)
	
	// Create a test prompt.yaml file
	promptContent := `name: "Project Specific Persona"
instruction: |
  You are a project-specific persona.
`
	err = os.WriteFile(filepath.Join(projectPersonaDir, "prompt.yaml"), []byte(promptContent), 0644)
	assert.NoError(t, err)
	
	// Create manager
	mockFactory := new(MockPersonaAwarePromptFactory)
	mockConfig := new(MockConfigManager)
	mockConfig.On("GetStringWithDefault", "GENIE_PERSONA", "genie").Return("genie")
	
	manager := NewDefaultPersonaManager(mockFactory, mockConfig)
	
	// Set context with cwd
	ctx := context.WithValue(context.Background(), "cwd", tempDir)
	
	// Call the method
	personas, err := manager.ListPersonas(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, personas)
	
	// Find the project persona
	var foundProjectPersona bool
	for _, persona := range personas {
		if persona.ID == "project-persona" {
			foundProjectPersona = true
			assert.Equal(t, "Project Specific Persona", persona.Name)
			assert.Equal(t, PersonaSourceProject, persona.Source)
		}
	}
	
	assert.True(t, foundProjectPersona, "Should find the project persona")
}

// TestDefaultPersonaManager_ListPersonas_Priority tests priority handling when personas have same ID
func TestDefaultPersonaManager_ListPersonas_Priority(t *testing.T) {
	// Create temporary directories for testing
	userDir, err := os.MkdirTemp("", "genie-test-user")
	assert.NoError(t, err)
	defer os.RemoveAll(userDir)
	
	projectDir, err := os.MkdirTemp("", "genie-test-project")
	assert.NoError(t, err)
	defer os.RemoveAll(projectDir)
	
	// Create a persona with same ID in user and project directories
	personaID := "genie" // Using an ID that exists in internal personas
	
	// Create user persona
	userPersonasDir := filepath.Join(userDir, ".genie", "personas")
	userPersonaDir := filepath.Join(userPersonasDir, personaID)
	err = os.MkdirAll(userPersonaDir, 0755)
	assert.NoError(t, err)
	
	userPromptContent := `name: "User Genie"
instruction: |
  User version of genie.
`
	err = os.WriteFile(filepath.Join(userPersonaDir, "prompt.yaml"), []byte(userPromptContent), 0644)
	assert.NoError(t, err)
	
	// Create project persona
	projectPersonasDir := filepath.Join(projectDir, ".genie", "personas")
	projectPersonaDir := filepath.Join(projectPersonasDir, personaID)
	err = os.MkdirAll(projectPersonaDir, 0755)
	assert.NoError(t, err)
	
	projectPromptContent := `name: "Project Genie"
instruction: |
  Project version of genie.
`
	err = os.WriteFile(filepath.Join(projectPersonaDir, "prompt.yaml"), []byte(projectPromptContent), 0644)
	assert.NoError(t, err)
	
	// Create manager with mocked home directory
	mockFactory := new(MockPersonaAwarePromptFactory)
	mockConfig := new(MockConfigManager)
	mockConfig.On("GetStringWithDefault", "GENIE_PERSONA", "genie").Return("genie")
	
	manager := &DefaultPersonaManager{
		promptFactory:  mockFactory,
		configManager:  mockConfig,
		defaultPersona: "genie",
		userHome:       userDir,
	}
	
	// Set context with project cwd
	ctx := context.WithValue(context.Background(), "cwd", projectDir)
	
	// Call the method
	personas, err := manager.ListPersonas(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, personas)
	
	// Find the genie persona - it should be the project version
	var foundGenie bool
	for _, persona := range personas {
		if persona.ID == personaID {
			foundGenie = true
			assert.Equal(t, "Project Genie", persona.Name, "Should use project version due to priority")
			assert.Equal(t, PersonaSourceProject, persona.Source)
		}
	}
	
	assert.True(t, foundGenie, "Should find the genie persona")
	
	// Test without project context - should find user version
	ctxNoProject := context.Background()
	personas, err = manager.ListPersonas(ctxNoProject)
	assert.NoError(t, err)
	
	for _, persona := range personas {
		if persona.ID == personaID {
			assert.Equal(t, "User Genie", persona.Name, "Should use user version when no project")
			assert.Equal(t, PersonaSourceUser, persona.Source)
		}
	}
}