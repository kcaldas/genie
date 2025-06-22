package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Interaction represents a complete LLM interaction for capture and replay
type Interaction struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	Prompt       CapturedPrompt         `json:"prompt"`
	Args         []string               `json:"args"`
	Attrs        []CapturedAttr         `json:"attrs,omitempty"`
	Response     string                 `json:"response"`
	Error        *CapturedError         `json:"error,omitempty"`
	Duration     time.Duration          `json:"duration"`
	LLMProvider  string                 `json:"llm_provider"`
	Tools        []string               `json:"tools"`
	Context      map[string]interface{} `json:"context,omitempty"`
	Debug        bool                   `json:"debug"`
}

// CapturedPrompt represents a prompt that can be serialized
type CapturedPrompt struct {
	Name        string                     `json:"name"`
	Text        string                     `json:"text"`
	Instruction string                     `json:"instruction"`
	Functions   []CapturedFunction         `json:"functions,omitempty"`
	Context     map[string]interface{}     `json:"context,omitempty"`
}

// CapturedFunction represents a function declaration for serialization
type CapturedFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// CapturedAttr represents an attribute for serialization
type CapturedAttr struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CapturedError represents an error that can be serialized
type CapturedError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// InteractionCapture manages the recording and storage of LLM interactions
type InteractionCapture struct {
	interactions []Interaction
	outputFile   string
	mutex        sync.RWMutex
	maxSize      int // Maximum interactions to keep in memory
}

// NewInteractionCapture creates a new interaction capture instance
func NewInteractionCapture() *InteractionCapture {
	return &InteractionCapture{
		interactions: make([]Interaction, 0),
		maxSize:      1000, // Default: keep last 1000 interactions
	}
}

// SetOutputFile configures where interactions should be saved
func (ic *InteractionCapture) SetOutputFile(filename string) {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()
	ic.outputFile = filename
}

// StartInteraction begins recording a new interaction
func (ic *InteractionCapture) StartInteraction(prompt Prompt, args []string) *Interaction {
	interaction := &Interaction{
		ID:        generateInteractionID(),
		Timestamp: time.Now(),
		Prompt:    convertPromptForCapture(prompt),
		Args:      append([]string{}, args...), // Copy args
		Context:   make(map[string]interface{}),
	}
	
	// Extract tools from prompt
	if prompt.Functions != nil {
		for _, fn := range prompt.Functions {
			interaction.Tools = append(interaction.Tools, fn.Name)
		}
	}
	
	return interaction
}

// CompleteInteraction finishes recording an interaction
func (ic *InteractionCapture) CompleteInteraction(interaction *Interaction, response string, err error, duration time.Duration) {
	interaction.Response = response
	interaction.Duration = duration
	
	if err != nil {
		interaction.Error = &CapturedError{
			Message: err.Error(),
			Type:    fmt.Sprintf("%T", err),
		}
	}
	
	ic.mutex.Lock()
	defer ic.mutex.Unlock()
	
	// Add to interactions list
	ic.interactions = append(ic.interactions, *interaction)
	
	// Trim if we exceed max size
	if len(ic.interactions) > ic.maxSize {
		ic.interactions = ic.interactions[len(ic.interactions)-ic.maxSize:]
	}
	
	// Auto-save if output file is configured
	if ic.outputFile != "" {
		ic.saveToFileUnsafe() // Already holding lock
	}
}

// GetInteractions returns all captured interactions
func (ic *InteractionCapture) GetInteractions() []Interaction {
	ic.mutex.RLock()
	defer ic.mutex.RUnlock()
	
	result := make([]Interaction, len(ic.interactions))
	copy(result, ic.interactions)
	return result
}

// GetLastInteraction returns the most recent interaction
func (ic *InteractionCapture) GetLastInteraction() *Interaction {
	ic.mutex.RLock()
	defer ic.mutex.RUnlock()
	
	if len(ic.interactions) == 0 {
		return nil
	}
	
	interaction := ic.interactions[len(ic.interactions)-1]
	return &interaction
}

// GetInteractionByID finds an interaction by its ID
func (ic *InteractionCapture) GetInteractionByID(id string) *Interaction {
	ic.mutex.RLock()
	defer ic.mutex.RUnlock()
	
	for _, interaction := range ic.interactions {
		if interaction.ID == id {
			return &interaction
		}
	}
	return nil
}

// SaveToFile saves all captured interactions to a JSON file
func (ic *InteractionCapture) SaveToFile(filename string) error {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()
	
	ic.outputFile = filename
	return ic.saveToFileUnsafe()
}

// saveToFileUnsafe saves to file without acquiring lock (internal use)
func (ic *InteractionCapture) saveToFileUnsafe() error {
	if ic.outputFile == "" {
		return fmt.Errorf("no output file configured")
	}
	
	data, err := json.MarshalIndent(ic.interactions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal interactions: %w", err)
	}
	
	return os.WriteFile(ic.outputFile, data, 0644)
}

// LoadFromFile loads interactions from a JSON file
func (ic *InteractionCapture) LoadFromFile(filename string) error {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()
	
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filename, err)
	}
	
	var interactions []Interaction
	if err := json.Unmarshal(data, &interactions); err != nil {
		return fmt.Errorf("failed to unmarshal interactions: %w", err)
	}
	
	ic.interactions = interactions
	return nil
}

// Clear removes all captured interactions
func (ic *InteractionCapture) Clear() {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()
	ic.interactions = nil
}

// GetSummary returns a human-readable summary of captured interactions
func (ic *InteractionCapture) GetSummary() string {
	ic.mutex.RLock()
	defer ic.mutex.RUnlock()
	
	if len(ic.interactions) == 0 {
		return "No interactions captured"
	}
	
	summary := fmt.Sprintf("Captured %d interactions:\n", len(ic.interactions))
	
	for i, interaction := range ic.interactions {
		status := "✅ Success"
		if interaction.Error != nil {
			status = fmt.Sprintf("❌ Error: %s", interaction.Error.Message)
		}
		
		summary += fmt.Sprintf("  %d. %s - %v (%v) - %s\n",
			i+1,
			interaction.ID,
			interaction.Timestamp.Format("15:04:05"),
			interaction.Duration,
			status)
	}
	
	return summary
}

// Utility functions

func generateInteractionID() string {
	return fmt.Sprintf("interaction_%d", time.Now().UnixNano())
}

func convertPromptForCapture(prompt Prompt) CapturedPrompt {
	captured := CapturedPrompt{
		Name:        prompt.Name,
		Text:        prompt.Text,
		Instruction: prompt.Instruction,
		Context:     make(map[string]interface{}),
	}
	
	// Convert functions for serialization
	if prompt.Functions != nil {
		captured.Functions = make([]CapturedFunction, len(prompt.Functions))
		for i, fn := range prompt.Functions {
			captured.Functions[i] = CapturedFunction{
				Name:        fn.Name,
				Description: fn.Description,
				Parameters:  convertSchemaToMap(fn.Parameters),
			}
		}
	}
	
	return captured
}

func convertSchemaToMap(schema *Schema) map[string]interface{} {
	if schema == nil {
		return nil
	}
	
	result := map[string]interface{}{
		"type": schema.Type,
	}
	
	if schema.Description != "" {
		result["description"] = schema.Description
	}
	
	if schema.Properties != nil {
		props := make(map[string]interface{})
		for name, prop := range schema.Properties {
			props[name] = convertSchemaToMap(prop)
		}
		result["properties"] = props
	}
	
	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}
	
	return result
}

// LoadInteractionsFromFile is a convenience function to load interactions from a file
func LoadInteractionsFromFile(filename string) ([]Interaction, error) {
	capture := NewInteractionCapture()
	if err := capture.LoadFromFile(filename); err != nil {
		return nil, err
	}
	return capture.GetInteractions(), nil
}

// SaveInteractionsToFile is a convenience function to save interactions to a file
func SaveInteractionsToFile(interactions []Interaction, filename string) error {
	data, err := json.MarshalIndent(interactions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal interactions: %w", err)
	}
	
	return os.WriteFile(filename, data, 0644)
}