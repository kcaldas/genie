package ai

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
)

// CaptureMiddleware wraps any AI Gen implementation to capture interactions
type CaptureMiddleware struct {
	underlying   Gen
	capture      *InteractionCapture
	enabled      bool
	debugMode    bool
	providerName string
}

// CaptureConfig configures the capture middleware
type CaptureConfig struct {
	Enabled      bool
	DebugMode    bool
	OutputFile   string
	ProviderName string
}

// NewCaptureMiddleware creates a new capture middleware
func NewCaptureMiddleware(underlying Gen, config CaptureConfig) Gen {
	capture := NewInteractionCapture()

	if config.OutputFile != "" {
		capture.SetOutputFile(config.OutputFile)
	}

	middleware := &CaptureMiddleware{
		underlying:   underlying,
		capture:      capture,
		enabled:      config.Enabled,
		debugMode:    config.DebugMode,
		providerName: config.ProviderName,
	}

	if config.DebugMode {
		log.Printf("[CaptureMiddleware] Initialized with provider: %s, output: %s",
			config.ProviderName, config.OutputFile)
	}

	return middleware
}

// GenerateContent implements the Gen interface with capture
func (c *CaptureMiddleware) GenerateContent(ctx context.Context, prompt Prompt, debug bool, args ...string) (string, error) {
	// If capture is disabled, pass through directly
	if !c.enabled {
		return c.underlying.GenerateContent(ctx, prompt, debug, args...)
	}

	// Start capturing the interaction
	interaction := c.capture.StartInteraction(prompt, args)
	interaction.LLMProvider = c.providerName
	interaction.Debug = debug

	startTime := time.Now()

	if c.debugMode {
		log.Printf("[CaptureMiddleware] Starting interaction %s with %d tools",
			interaction.ID, len(interaction.Tools))
		log.Printf("[CaptureMiddleware] Prompt: %s", prompt.Text)
		log.Printf("[CaptureMiddleware] Args: %v", args)
	}

	// Call the underlying LLM
	response, err := c.underlying.GenerateContent(ctx, prompt, debug, args...)

	duration := time.Since(startTime)

	// Complete the capture
	c.capture.CompleteInteraction(interaction, response, err, duration)

	if c.debugMode {
		if err != nil {
			log.Printf("[CaptureMiddleware] Interaction %s completed with error in %v: %v",
				interaction.ID, duration, err)
		} else {
			log.Printf("[CaptureMiddleware] Interaction %s completed successfully in %v",
				interaction.ID, duration)
			log.Printf("[CaptureMiddleware] Response length: %d chars", len(response))

			// Log first part of response for debugging
			if len(response) > 100 {
				log.Printf("[CaptureMiddleware] Response preview: %q...", response[:100])
			} else {
				log.Printf("[CaptureMiddleware] Response: %q", response)
			}
		}
	}

	return response, err
}

// GenerateContentAttr implements the Gen interface with capture
func (c *CaptureMiddleware) GenerateContentAttr(ctx context.Context, prompt Prompt, debug bool, attrs []Attr) (string, error) {
	// If capture is disabled, pass through directly
	if !c.enabled {
		return c.underlying.GenerateContentAttr(ctx, prompt, debug, attrs)
	}

	// Start capturing the interaction
	interaction := c.capture.StartInteraction(prompt, nil) // Convert attrs to args below
	interaction.LLMProvider = c.providerName
	interaction.Debug = debug

	// Convert attrs to captured format
	for _, attr := range attrs {
		interaction.Attrs = append(interaction.Attrs, CapturedAttr{
			Key:   attr.Key,
			Value: attr.Value,
		})
	}

	startTime := time.Now()

	if c.debugMode {
		log.Printf("[CaptureMiddleware] Starting interaction %s with attrs: %v",
			interaction.ID, attrs)
	}

	// Call the underlying LLM
	response, err := c.underlying.GenerateContentAttr(ctx, prompt, debug, attrs)

	duration := time.Since(startTime)

	// Complete the capture
	c.capture.CompleteInteraction(interaction, response, err, duration)

	if c.debugMode {
		if err != nil {
			log.Printf("[CaptureMiddleware] Interaction %s completed with error in %v: %v",
				interaction.ID, duration, err)
		} else {
			log.Printf("[CaptureMiddleware] Interaction %s completed successfully in %v",
				interaction.ID, duration)
		}
	}

	return response, err
}

// CountTokens delegates to the underlying LLM client
func (c *CaptureMiddleware) CountTokens(ctx context.Context, p Prompt, debug bool, args ...string) (*TokenCount, error) {
	return c.underlying.CountTokens(ctx, p, debug, args...)
}

// CountTokens delegates to the underlying LLM client
func (c *CaptureMiddleware) CountTokensAttr(ctx context.Context, p Prompt, debug bool, attrs []Attr) (*TokenCount, error) {
	return c.underlying.CountTokensAttr(ctx, p, debug, attrs)
}

// GetStatus delegates to the underlying LLM client
func (c *CaptureMiddleware) GetStatus() *Status {
	return c.underlying.GetStatus()
}

// Capture-specific methods

// GetCapture returns the underlying capture for inspection
func (c *CaptureMiddleware) GetCapture() *InteractionCapture {
	return c.capture
}

// SaveCapture saves captured interactions to a file
func (c *CaptureMiddleware) SaveCapture(filename string) error {
	return c.capture.SaveToFile(filename)
}

// LoadCapture loads interactions from a file (for replay scenarios)
func (c *CaptureMiddleware) LoadCapture(filename string) error {
	return c.capture.LoadFromFile(filename)
}

// GetCapturedInteractions returns all captured interactions
func (c *CaptureMiddleware) GetCapturedInteractions() []Interaction {
	return c.capture.GetInteractions()
}

// GetLastInteraction returns the most recent interaction
func (c *CaptureMiddleware) GetLastInteraction() *Interaction {
	return c.capture.GetLastInteraction()
}

// PrintCaptureSummary prints a summary of captured interactions
func (c *CaptureMiddleware) PrintCaptureSummary() {
	fmt.Println(c.capture.GetSummary())
}

// Configuration helpers

// GetCaptureConfigFromEnv creates capture config from environment variables
func GetCaptureConfigFromEnv(providerName string) CaptureConfig {
	config := CaptureConfig{
		ProviderName: providerName,
		Enabled:      false,
		DebugMode:    false,
	}

	// Check for capture enablement
	if os.Getenv("GENIE_CAPTURE_LLM") == "true" {
		config.Enabled = true
	}

	// Check for debug mode
	if os.Getenv("GENIE_DEBUG") == "true" {
		config.Enabled = true
		config.DebugMode = true
	}

	// Set output file if specified
	if outputFile := os.Getenv("GENIE_CAPTURE_FILE"); outputFile != "" {
		config.OutputFile = outputFile
	} else if config.Enabled {
		// Default output file based on provider and timestamp
		timestamp := time.Now().Format("20060102-150405")
		config.OutputFile = fmt.Sprintf("genie-capture-%s-%s.json", providerName, timestamp)
	}

	return config
}

// Convenience functions for testing

// EnableCaptureForTesting enables capture with in-memory storage for testing
func EnableCaptureForTesting(underlying Gen) *CaptureMiddleware {
	config := CaptureConfig{
		Enabled:      true,
		DebugMode:    false, // Usually don't want debug noise in tests
		ProviderName: "test",
	}

	return NewCaptureMiddleware(underlying, config).(*CaptureMiddleware)
}

// EnableDebugCaptureForTesting enables capture with debug output for testing
func EnableDebugCaptureForTesting(underlying Gen) *CaptureMiddleware {
	config := CaptureConfig{
		Enabled:      true,
		DebugMode:    true,
		ProviderName: "test-debug",
	}

	return NewCaptureMiddleware(underlying, config).(*CaptureMiddleware)
}
