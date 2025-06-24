package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
	
	"github.com/kcaldas/genie/pkg/genie"
)


func TestAskCommand(t *testing.T) {
	t.Run("should exist and be named ask", func(t *testing.T) {
		cmd := NewAskCommand()

		if cmd.Use != "ask" {
			t.Errorf("Expected command name to be 'ask', got %s", cmd.Use)
		}
	})

	t.Run("should accept a prompt argument", func(t *testing.T) {
		// Use test fixture for testing to avoid environment dependencies
		fixture := genie.NewTestFixture(t)
		fixture.ExpectSimpleMessage("What is 2+2?", "4")
		
		// Use the fixture's Genie and EventBus
		cmd := NewAskCommandWithGenie(func() (genie.Genie, *genie.Session) {
			session, _ := fixture.Genie.Start(nil)
			return fixture.Genie, session
		})

		// Set up command with a simple prompt
		cmd.SetArgs([]string{"What is 2+2?"})

		err := cmd.Execute()
		if err != nil {
			t.Errorf("Expected command to execute without error, got %v", err)
		}
	})

	t.Run("should call Genie with the provided prompt", func(t *testing.T) {
		fixture := genie.NewTestFixture(t)
		fixture.ExpectSimpleMessage("What is 2+2?", "2+2 equals 4")

		var output bytes.Buffer
		cmd := NewAskCommandWithGenie(func() (genie.Genie, *genie.Session) {
			session, _ := fixture.Genie.Start(nil)
			return fixture.Genie, session
		})
		cmd.SetOut(&output)
		cmd.SetArgs([]string{"What is 2+2?"})

		err := cmd.Execute()
		if err != nil {
			t.Errorf("Expected command to execute without error, got %v", err)
		}

		// Note: CLI tests with async Genie would need event waiting
		// For now, just verify command execution succeeded
	})

	t.Run("should return error when no prompt provided", func(t *testing.T) {
		fixture := genie.NewTestFixture(t)
		cmd := NewAskCommandWithGenie(func() (genie.Genie, *genie.Session) {
			session, _ := fixture.Genie.Start(nil)
			return fixture.Genie, session
		})

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
