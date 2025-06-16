package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
)

// Mock LLM client for testing
type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) GenerateContent(prompt ai.Prompt, debug bool, args ...string) (string, error) {
	return m.response, m.err
}

func (m *mockLLMClient) GenerateContentAttr(prompt ai.Prompt, debug bool, attrs []ai.Attr) (string, error) {
	return m.response, m.err
}

func TestAskCommand(t *testing.T) {
	t.Run("should exist and be named ask", func(t *testing.T) {
		cmd := NewAskCommand()
		
		if cmd.Use != "ask" {
			t.Errorf("Expected command name to be 'ask', got %s", cmd.Use)
		}
	})
	
	t.Run("should accept a prompt argument", func(t *testing.T) {
		// Use mock client for testing to avoid environment dependencies
		mockClient := &mockLLMClient{
			response: "mock response",
			err:      nil,
		}
		cmd := NewAskCommandWithLLM(mockClient)
		
		// Set up command with a simple prompt
		cmd.SetArgs([]string{"What is 2+2?"})
		
		err := cmd.Execute()
		if err != nil {
			t.Errorf("Expected command to execute without error, got %v", err)
		}
	})
	
	t.Run("should call LLM with the provided prompt", func(t *testing.T) {
		mockClient := &mockLLMClient{
			response: "2+2 equals 4",
			err:      nil,
		}
		
		var output bytes.Buffer
		cmd := NewAskCommandWithLLM(mockClient)
		cmd.SetOut(&output)
		cmd.SetArgs([]string{"What is 2+2?"})
		
		err := cmd.Execute()
		if err != nil {
			t.Errorf("Expected command to execute without error, got %v", err)
		}
		
		outputStr := output.String()
		if !strings.Contains(outputStr, "2+2 equals 4") {
			t.Errorf("Expected output to contain LLM response '2+2 equals 4', got %s", outputStr)
		}
	})
	
	t.Run("should return error when no prompt provided", func(t *testing.T) {
		mockClient := &mockLLMClient{
			response: "should not be called",
			err:      nil,
		}
		cmd := NewAskCommandWithLLM(mockClient)
		
		// Set no arguments
		cmd.SetArgs([]string{})
		
		err := cmd.Execute()
		if err == nil {
			t.Error("Expected command to return error when no arguments provided")
		}
	})
	
	t.Run("should return helpful error when GOOGLE_CLOUD_PROJECT not set", func(t *testing.T) {
		// Clear the environment variable for this test
		originalValue := os.Getenv("GOOGLE_CLOUD_PROJECT")
		os.Unsetenv("GOOGLE_CLOUD_PROJECT")
		defer func() {
			if originalValue != "" {
				os.Setenv("GOOGLE_CLOUD_PROJECT", originalValue)
			}
		}()
		
		cmd := NewAskCommand()
		cmd.SetArgs([]string{"test prompt"})
		
		err := cmd.Execute()
		if err == nil {
			t.Error("Expected command to return error when GOOGLE_CLOUD_PROJECT not set")
		}
		
		errorMsg := err.Error()
		if !strings.Contains(errorMsg, "GOOGLE_CLOUD_PROJECT") {
			t.Errorf("Expected error message to mention GOOGLE_CLOUD_PROJECT, got: %s", errorMsg)
		}
		
		if !strings.Contains(errorMsg, "export GOOGLE_CLOUD_PROJECT") {
			t.Errorf("Expected error message to include setup instructions, got: %s", errorMsg)
		}
	})
}