package ai

import (
	"strings"
	"testing"
)

func TestDecisionStep_AddOption(t *testing.T) {
	tests := []struct {
		name           string
		initialOptions map[string]*Chain
		addKey         string
		addChain       *Chain
		expectedLen    int
	}{
		{
			name:           "add to empty options",
			initialOptions: nil,
			addKey:         "TEST",
			addChain:       &Chain{Name: "TestChain"},
			expectedLen:    1,
		},
		{
			name: "add to existing options",
			initialOptions: map[string]*Chain{
				"EXISTING": &Chain{Name: "ExistingChain"},
			},
			addKey:      "NEW",
			addChain:    &Chain{Name: "NewChain"},
			expectedLen: 2,
		},
		{
			name: "overwrite existing option",
			initialOptions: map[string]*Chain{
				"TEST": &Chain{Name: "OldChain"},
			},
			addKey:      "TEST",
			addChain:    &Chain{Name: "NewChain"},
			expectedLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := &DecisionStep{
				Name:    "test_step",
				Options: tt.initialOptions,
			}

			ds.AddOption(tt.addKey, tt.addChain)

			if len(ds.Options) != tt.expectedLen {
				t.Errorf("expected %d options, got %d", tt.expectedLen, len(ds.Options))
			}

			if ds.Options[tt.addKey] != tt.addChain {
				t.Errorf("option %s was not set correctly", tt.addKey)
			}
		})
	}
}

func TestDecisionStep_BuildDecisionPrompt(t *testing.T) {
	tests := []struct {
		name           string
		decisionStep   *DecisionStep
		expectError    bool
		expectedKeys   []string
		shouldContain  []string
		shouldNotContain []string
	}{
		{
			name: "no options should error",
			decisionStep: &DecisionStep{
				Name:    "empty_step",
				Options: map[string]*Chain{},
			},
			expectError: true,
		},
		{
			name: "single option",
			decisionStep: &DecisionStep{
				Name: "single_step",
				Options: map[string]*Chain{
					"CHAT": &Chain{Name: "ChatResponse"},
				},
			},
			expectError:  false,
			expectedKeys: []string{"CHAT"},
			shouldContain: []string{
				"You are Genie",
				"Options:",
				"- CHAT: ChatResponse",
				"Please respond with only the option key",
			},
		},
		{
			name: "multiple options with context",
			decisionStep: &DecisionStep{
				Name:    "multi_step",
				Context: "Test context for decision making",
				Options: map[string]*Chain{
					"CHAT":    &Chain{Name: "ChatResponse"},
					"EXPLORE": &Chain{Name: "ExploreCode"},
					"ACTION":  &Chain{Name: "TakeAction"},
				},
			},
			expectError:  false,
			expectedKeys: []string{"ACTION", "CHAT", "EXPLORE"}, // Sorted alphabetically
			shouldContain: []string{
				"You are Genie",
				"Options:",
				"- ACTION: TakeAction",
				"- CHAT: ChatResponse", 
				"- EXPLORE: ExploreCode",
				"Context: Test context for decision making",
				"Please respond with only the option key",
			},
		},
		{
			name: "chain with empty name uses fallback description",
			decisionStep: &DecisionStep{
				Name: "fallback_step",
				Options: map[string]*Chain{
					"TEST": &Chain{Name: ""}, // Empty name
				},
			},
			expectError:  false,
			expectedKeys: []string{"TEST"},
			shouldContain: []string{
				"- TEST: Execute TEST chain",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptText, optionKeys, err := tt.decisionStep.BuildDecisionPrompt()

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check option keys
			if len(optionKeys) != len(tt.expectedKeys) {
				t.Errorf("expected %d option keys, got %d", len(tt.expectedKeys), len(optionKeys))
			}

			for i, expectedKey := range tt.expectedKeys {
				if i >= len(optionKeys) || optionKeys[i] != expectedKey {
					t.Errorf("expected option key %d to be %s, got %s", i, expectedKey, optionKeys[i])
				}
			}

			// Check prompt content
			for _, expectedContent := range tt.shouldContain {
				if !strings.Contains(promptText, expectedContent) {
					t.Errorf("prompt should contain %q but doesn't.\nPrompt: %s", expectedContent, promptText)
				}
			}

			for _, unexpectedContent := range tt.shouldNotContain {
				if strings.Contains(promptText, unexpectedContent) {
					t.Errorf("prompt should not contain %q but does.\nPrompt: %s", unexpectedContent, promptText)
				}
			}
		})
	}
}

func TestDecisionStep_BuildDecisionPrompt_Ordering(t *testing.T) {
	// Test that options are always sorted consistently
	ds := &DecisionStep{
		Name: "ordering_test",
		Options: map[string]*Chain{
			"ZEBRA":  &Chain{Name: "ZebraChain"},
			"ALPHA":  &Chain{Name: "AlphaChain"},
			"BRAVO":  &Chain{Name: "BravoChain"},
		},
	}

	promptText1, keys1, err1 := ds.BuildDecisionPrompt()
	if err1 != nil {
		t.Fatalf("unexpected error: %v", err1)
	}

	promptText2, keys2, err2 := ds.BuildDecisionPrompt()
	if err2 != nil {
		t.Fatalf("unexpected error: %v", err2)
	}

	// Results should be identical (deterministic ordering)
	if promptText1 != promptText2 {
		t.Error("prompt text should be identical between calls")
	}

	if len(keys1) != len(keys2) {
		t.Error("option keys length should be identical between calls")
	}

	for i := range keys1 {
		if keys1[i] != keys2[i] {
			t.Errorf("option key %d should be identical between calls: %s vs %s", i, keys1[i], keys2[i])
		}
	}

	// Keys should be alphabetically sorted
	expectedOrder := []string{"ALPHA", "BRAVO", "ZEBRA"}
	for i, expectedKey := range expectedOrder {
		if keys1[i] != expectedKey {
			t.Errorf("expected key %d to be %s, got %s", i, expectedKey, keys1[i])
		}
	}
}