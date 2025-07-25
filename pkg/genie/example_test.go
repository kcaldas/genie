package genie_test

import (
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/genie"
)

func TestSimpleMessage(t *testing.T) {
	fixture := genie.NewTestFixture(t)
	fixture.ExpectSimpleMessage("Hello", "Hi there!")

	fixture.StartAndGetSession()
	err := fixture.StartChat("Hello")
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	response := fixture.WaitForResponse(2 * time.Second)
	if response == nil {
		t.Fatal("No response received")
	}
	if response.Error != nil {
		t.Fatalf("Unexpected error: %v", response.Error)
	}
	if response.Response != "Hi there!" {
		t.Fatalf("Expected 'Hi there!', got %q", response.Response)
	}
}

func TestMultipleMessages(t *testing.T) {
	testCases := []struct {
		input, expected string
	}{
		{"Hello", "Hi there!"},
		{"How are you?", "I'm great!"},
		{"Goodbye", "See you later!"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			fixture := genie.NewTestFixture(t)
			fixture.ExpectSimpleMessage(tc.input, tc.expected)

			fixture.StartAndGetSession()
			err := fixture.StartChat(tc.input)
			if err != nil {
				t.Fatalf("Chat failed: %v", err)
			}

			response := fixture.WaitForResponse(2 * time.Second)
			if response == nil {
				t.Fatal("No response received")
			}
			if response.Response != tc.expected {
				t.Fatalf("Expected %q, got %q", tc.expected, response.Response)
			}
		})
	}
}

func TestMockToolCalls(t *testing.T) {
	fixture := genie.NewTestFixture(t)

	fixture.ExpectMessage("list files").
		MockTool("listFiles").Returns(map[string]any{
		"files": []string{"main.go", "test.txt"},
	}).
		RespondWith("Found 2 files")

	fixture.StartAndGetSession()
	err := fixture.StartChat("list files")
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	response := fixture.WaitForResponse(2 * time.Second)
	if response == nil {
		t.Fatal("No response received")
	}
	if response.Response != "Found 2 files" {
		t.Fatalf("Expected 'Found 2 files', got %q", response.Response)
	}
}
