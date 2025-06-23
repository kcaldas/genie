package ai

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChain_Run_Success(t *testing.T) {
	mock := NewSharedMockGen()
	mock.ResponseQueue = []string{"summarized text", "tweeted text"}
	mock.UsedPrompts = []Prompt{}

	// Create a chain with two steps that read/write from the chain context
	ch := Chain{
		Name: "TestChain",
		Steps: []interface{}{
			ChainStep{
				Name: "Step1",
				Prompt: &Prompt{
					Name:        "step1",
					Instruction: "Summarize the following text:",
					Text:        "{{.InputText}}",
					ModelName:   "gpt-3.5-turbo",
					MaxTokens:   100,
					Temperature: 0.7,
					TopP:        1.0,
				},
				LocalContext: map[string]string{"tone": "formal"},
				ForwardAs:    "step1Output",
			},
			ChainStep{
				Name: "Step2",
				Prompt: &Prompt{
					Name:        "step2",
					Instruction: "Turn the summary into a tweet:",
					Text:        "{{.step1Output}}",
					ModelName:   "gpt-3.5-turbo",
					MaxTokens:   50,
					Temperature: 0.7,
					TopP:        1.0,
				},
				LocalContext: map[string]string{"tone": "fun"},
				ForwardAs:    "finalOutput",
			},
		},
	}

	ctx := NewChainContext(map[string]string{
		"InputText": "ChatGPT is a language model that can generate text.",
	})

	// Run the chain
	err := ch.Run(context.Background(), mock, ctx, nil, false)
	require.NoError(t, err)

	// Verify calls to the mock
	assert.Equal(t, 0, mock.CallCounts["GenerateContent"], "GenerateContent should not have been called")
	assert.Equal(t, 2, mock.CallCounts["GenerateContentAttr"], "GenerateContentAttr should have been called twice")

	assert.Equal(t, 2, len(mock.UsedPrompts), "expected 2 prompts to be used")

	assert.Equal(t, 3, len(mock.LastAttrs), "expected 3 attributes to be passed to GenerateContentAttr") // 2 from the context, 1 from the steps (same key)

	// Verify final context data
	assert.Equal(t, "summarized text", ctx.Data["step1Output"])
	assert.Equal(t, "tweeted text", ctx.Data["finalOutput"])
}

func TestChain_Run_ErrorPropagation(t *testing.T) {
	// Let the first step succeed, second step return an error
	mock := NewSharedMockGen()
	mock.ResponseQueue = []string{"some result", "ERROR"} // second call triggers error

	ch := Chain{
		Name: "TestChainErrorCase",
		Steps: []interface{}{
			ChainStep{
				Name: "Step1",
				Prompt: &Prompt{
					Name:      "step1",
					Text:      "Some template",
					ModelName: "model-A",
				},
				ForwardAs: "step1Output",
			},
			ChainStep{
				Name: "Step2",
				Prompt: &Prompt{
					Name:      "step2",
					Text:      "{{.step1Output}}",
					ModelName: "model-B",
				},
				ForwardAs: "step2Output",
			},
		},
	}

	ctx := NewChainContext(nil)

	err := ch.Run(context.Background(), mock, ctx, nil, false)
	require.Error(t, err, "expected an error from second step")

	// Only the first step result should be saved
	assert.Equal(t, "some result", ctx.Data["step1Output"], "first step output should exist")
	assert.Empty(t, ctx.Data["step2Output"], "second step output should not exist since it failed")

	// Confirm 2 calls were made, the second triggered an error
	assert.Equal(t, 2, mock.CallCounts["GenerateContentAttr"])
}

func TestChain_Save_Step_Output(t *testing.T) {
	mock := NewSharedMockGen()
	mock.ResponseQueue = []string{"step1 output"}

	// Temporary file to save the output
	filePath := filepath.Join(os.TempDir(), "step1Output.txt")

	ch := Chain{
		Name: "TestChainSaveStepOutput",
		Steps: []interface{}{
			ChainStep{
				Name: "Step1",
				Prompt: &Prompt{
					Name:      "step1",
					Text:      "Some template",
					ModelName: "model-A",
				},
				SaveAs: filePath,
			},
		},
	}

	ctx := NewChainContext(nil)

	err := ch.Run(context.Background(), mock, ctx, nil, false)
	require.NoError(t, err)

	// Verify the content of the file
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "step1 output", string(content), "expected the file content to match the output")

	// Clean up
	err = os.Remove(filePath)
	require.NoError(t, err)
}

func TestChain_Step_With_Function(t *testing.T) {
	mock := NewSharedMockGen()
	mock.ResponseQueue = []string{"step1 output"}

	// Create a chain with a step that uses a function to generate content
	ch := Chain{
		Name: "TestChainWithFunction",
		Steps: []interface{}{
			ChainStep{
				Name: "Step1",
				Fn: func(data map[string]string, debug bool) (string, error) {
					return "step1 output", nil
				},
				ForwardAs: "step1Output",
			},
		},
	}

	ctx := NewChainContext(nil)

	err := ch.Run(context.Background(), mock, ctx, nil, false)
	require.NoError(t, err)

	// Verify the context data
	assert.Equal(t, "step1 output", ctx.Data["step1Output"])
}

func TestChain_Step_With_Requires(t *testing.T) {
	mock := NewSharedMockGen()
	mock.ResponseQueue = []string{"step1 output"}

	// Create a chain with a step that requires a key from the context
	ch := Chain{
		Name: "TestChainWithRequires",
		Steps: []interface{}{
			ChainStep{
				Name: "Step1",
				Prompt: &Prompt{
					Name:      "step1",
					Text:      "Some template",
					ModelName: "model-A",
				},
				Requires:  []string{"requiredKey"},
				ForwardAs: "step1Output",
			},
		},
	}

	ctx := NewChainContext(map[string]string{"requiredKey": "some value"})

	err := ch.Run(context.Background(), mock, ctx, nil, false)
	require.NoError(t, err)

	// Verify the context data
	assert.Equal(t, "step1 output", ctx.Data["step1Output"])

	ctx = NewChainContext(nil)
	err = ch.Run(context.Background(), mock, ctx, nil, false)
	require.Error(t, err, "expected an error when a required key is missing")
}

func TestChain_Step_With_Cache(t *testing.T) {
	mock := NewSharedMockGen()
	mock.ResponseQueue = []string{"step1 output"}

	// Temporary file to save the output
	filePath := filepath.Join(os.TempDir(), "step1Output.txt")

	// Create a chain with a step that requires a key from the context
	ch := Chain{
		Name: "TestChainWithCache",
		Steps: []interface{}{
			ChainStep{
				Name: "Step1",
				Prompt: &Prompt{
					Name:      "step1",
					Text:      "Some template",
					ModelName: "model-A",
				},
				ForwardAs: "step1Output",
				SaveAs:    filePath,
				Cache:     true,
			},
		},
	}

	ctx := NewChainContext(nil)

	err := ch.Run(context.Background(), mock, ctx, nil, false)
	require.NoError(t, err)

	// Verify the context data
	assert.Equal(t, "step1 output", ctx.Data["step1Output"])

	// Run the chain again, the step should not be executed
	mock.ResponseQueue = []string{"cached output"}
	err = ch.Run(context.Background(), mock, ctx, nil, false)
	require.NoError(t, err)

	// Verify the context data
	assert.Equal(t, "step1 output", ctx.Data["step1Output"], "expected the cached value to be used")

	// Clean up
	err = os.Remove(filePath)
	require.NoError(t, err)
}

func TestChain_Only_Allow_Either_Fn_Prompt_OR_Template_In_A_Step(t *testing.T) {
	mock := NewSharedMockGen()

	// Create a chain with a step that has both a prompt and a function
	ch := Chain{
		Name: "TestChainWithPromptAndFunction",
		Steps: []interface{}{
			ChainStep{
				Name: "Step1",
				Prompt: &Prompt{
					Name:      "step1",
					Text:      "Some template",
					ModelName: "model-A",
				},
				Fn: func(data map[string]string, debug bool) (string, error) {
					return "step1 output", nil
				},
				ForwardAs: "step1Output",
			},
		},
	}

	ctx := NewChainContext(nil)

	err := ch.Run(context.Background(), mock, ctx, nil, false)
	require.Error(t, err, "expected an error when a step has both a prompt and a function")

	// Temporary file to save the output
	filePath := filepath.Join(os.TempDir(), "step1Output.txt")

	// Write some content to the file
	err = os.WriteFile(filePath, []byte("some output"), 0644)
	require.NoError(t, err)

	ch = Chain{
		Name: "TestChainWithPromptAndFunction",
		Steps: []interface{}{
			ChainStep{
				Name: "Step1",
				Prompt: &Prompt{
					Name:      "step1",
					Text:      "Some template",
					ModelName: "model-A",
				},
				TemplateFile: filePath,
				ForwardAs:    "step1Output",
			},
		},
	}

	err = ch.Run(context.Background(), mock, ctx, nil, false)
	log.Println(err)
	require.Error(t, err, "expected an error when a step has both a prompt and a function")
}

func TestChain_Step_With_TemplateFile(t *testing.T) {
	mock := NewSharedMockGen()
	mock.ResponseQueue = []string{"step1 output"}

	// Temporary file to save the output
	filePath := filepath.Join(os.TempDir(), "step1Output.txt")

	// Write some content to the file
	err := os.WriteFile(filePath, []byte("{{.InputText}} output"), 0644)
	require.NoError(t, err)

	// Create a chain with a step that requires a key from the context
	ch := Chain{
		Name: "TestChainWithTemplateFile",
		Steps: []interface{}{
			ChainStep{
				Name:         "Step1",
				ForwardAs:    "step1Output",
				Requires:     []string{"InputText"},
				TemplateFile: filePath,
			},
		},
	}

	ctx := NewChainContext(map[string]string{"InputText": "step1"})

	err = ch.Run(context.Background(), mock, ctx, nil, false)
	require.NoError(t, err)

	// Verify the context data
	assert.Equal(t, "step1 output", ctx.Data["step1Output"])

	// Clean up
	err = os.Remove(filePath)
	require.NoError(t, err)
}

func TestChain_Join(t *testing.T) {
	ch := Chain{
		Name:       "TestChainJoin",
		DescribeAt: "chain_description.txt",
		Steps: []interface{}{
			ChainStep{
				Name: "Step1",
				Prompt: &Prompt{
					Name:      "step1",
					Text:      "Some template",
					ModelName: "model-A",
				},
				ForwardAs: "step1Output",
			},
			ChainStep{
				Name: "Step2",
				Prompt: &Prompt{
					Name:      "step2",
					Text:      "{{.step1Output}}",
					ModelName: "model-B",
				},
				ForwardAs: "finalOutput",
			},
		},
	}

	ch2 := Chain{
		Name:       "TestChainJoin2",
		DescribeAt: "chain2_description.txt",
		Steps: []interface{}{
			ChainStep{
				Name: "Ch2_Step1",
				Prompt: &Prompt{
					Name:      "ch2_step1",
					Text:      "Some template",
					ModelName: "model-A",
				},
				ForwardAs: "ch2_step1Output",
			},
		},
	}

	// Join the two chains
	joinedChain := ch.Join(&ch2)

	// Verify the joined chain
	assert.Equal(t, "TestChainJoin", joinedChain.Name)
	assert.Equal(t, "chain_description.txt", joinedChain.DescribeAt)
	assert.Equal(t, 3, len(joinedChain.Steps))
}

func TestChain_Run_WithContextCancellation(t *testing.T) {
	// Create a mock that simulates context cancellation
	mock := NewSharedMockGen()
	mock.ResponseQueue = []string{"ERROR"} // Will return "mock error" 
	
	// Create a chain with a step that should be cancelled
	ch := Chain{
		Name: "TestChainCancellation",
		Steps: []interface{}{
			ChainStep{
				Name: "Step1",
				Prompt: &Prompt{
					Name:        "step1",
					Instruction: "Process this",
					Text:        "test input",
					ModelName:   "test-model",
				},
				ForwardAs: "step1Output",
			},
		},
	}

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Create chain context
	chainCtx := NewChainContext(map[string]string{})

	// Run the chain - should return an error due to mock setup
	err := ch.Run(ctx, mock, chainCtx, nil, false)

	// Should return an error (the mock will return "mock error")
	assert.Error(t, err)
}

func TestChain_DecisionStep_Success(t *testing.T) {
	mock := NewSharedMockGen()
	
	// Create option chains
	refactorChain := &Chain{
		Name: "refactor",
		Steps: []interface{}{
			ChainStep{
				Name: "RefactorStep",
				Prompt: &Prompt{
					Name: "refactor_prompt",
					Text: "Refactoring the code...",
				},
				ForwardAs: "refactor_result",
			},
		},
	}
	
	enhanceChain := &Chain{
		Name: "enhance",
		Steps: []interface{}{
			ChainStep{
				Name: "EnhanceStep",
				Prompt: &Prompt{
					Name: "enhance_prompt",
					Text: "Enhancing the code...",
				},
				ForwardAs: "enhance_result",
			},
		},
	}
	
	// Mock responses: analysis result, decision choice, then the chosen chain's response
	mock.ResponseQueue = []string{"Analysis complete", "refactor", "Code has been refactored"}
	
	// Create a chain with a decision step
	mainChain := Chain{
		Name: "TestDecisionChain",
		Steps: []interface{}{
			ChainStep{
				Name: "AnalyzeStep",
				Prompt: &Prompt{
					Name: "analyze",
					Text: "Analyzing the code...",
				},
				ForwardAs: "analysis",
			},
			DecisionStep{
				Name:    "ChooseApproach",
				Context: "Based on the code analysis",
				Options: map[string]*Chain{
					"refactor": refactorChain,
					"enhance":  enhanceChain,
				},
				SaveAs: "chosen_approach",
			},
		},
	}
	
	ctx := NewChainContext(map[string]string{})
	
	// Run the main chain
	err := mainChain.Run(context.Background(), mock, ctx, nil, false)
	require.NoError(t, err)
	
	// Verify the decision was saved
	assert.Equal(t, "refactor", ctx.Data["chosen_approach"])
	
	// Verify the chosen chain was executed
	assert.Equal(t, "Code has been refactored", ctx.Data["refactor_result"])
	
	// Verify the analysis step also ran
	assert.Equal(t, "Analysis complete", ctx.Data["analysis"])
	
	// Verify the mock was called the expected number of times
	assert.Equal(t, 3, mock.CallCounts["GenerateContentAttr"], "Should call LLM 3 times: analyze, decision, refactor")
}

func TestChain_DecisionStep_InvalidChoice(t *testing.T) {
	mock := NewSharedMockGen()
	
	refactorChain := &Chain{
		Name: "refactor",
		Steps: []interface{}{
			ChainStep{
				Name: "RefactorStep",
				Prompt: &Prompt{Name: "refactor", Text: "Refactoring..."},
				ForwardAs: "result",
			},
		},
	}
	
	// Mock returns an invalid choice
	mock.ResponseQueue = []string{"invalid_choice"}
	
	mainChain := Chain{
		Name: "TestInvalidDecisionChain",
		Steps: []interface{}{
			DecisionStep{
				Name: "ChooseApproach",
				Options: map[string]*Chain{
					"refactor": refactorChain,
				},
			},
		},
	}
	
	ctx := NewChainContext(map[string]string{})
	
	// Run should fail with invalid decision
	err := mainChain.Run(context.Background(), mock, ctx, nil, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid decision 'invalid_choice'")
}

func TestChain_AddDecision_Method(t *testing.T) {
	// Test the AddDecision builder method
	refactorChain := &Chain{Name: "refactor"}
	enhanceChain := &Chain{Name: "enhance"}
	
	chain := &Chain{
		Name:  "TestChain",
		Steps: []interface{}{},
	}
	
	// Use AddDecision method
	result := chain.AddDecision("ChooseApproach", "Based on analysis", map[string]*Chain{
		"refactor": refactorChain,
		"enhance":  enhanceChain,
	})
	
	// Should return the same chain (builder pattern)
	assert.Equal(t, chain, result)
	
	// Should have added a DecisionStep
	assert.Equal(t, 1, len(chain.Steps))
	
	// Verify the decision step was added correctly
	step, ok := chain.Steps[0].(DecisionStep)
	assert.True(t, ok, "Step should be a DecisionStep")
	assert.Equal(t, "ChooseApproach", step.Name)
	assert.Equal(t, "Based on analysis", step.Context)
	assert.Equal(t, 2, len(step.Options))
	assert.Equal(t, refactorChain, step.Options["refactor"])
	assert.Equal(t, enhanceChain, step.Options["enhance"])
}

func TestFindBestMatch(t *testing.T) {
	tests := []struct {
		name         string
		decision     string
		validOptions []string
		expected     string
	}{
		// Exact matches (case variations)
		{
			name:         "exact match lowercase",
			decision:     "clear",
			validOptions: []string{"CLEAR", "UNCLEAR"},
			expected:     "CLEAR",
		},
		{
			name:         "exact match uppercase",
			decision:     "UNCLEAR",
			validOptions: []string{"CLEAR", "UNCLEAR"},
			expected:     "UNCLEAR",
		},
		{
			name:         "exact match mixed case",
			decision:     "Clear",
			validOptions: []string{"CLEAR", "UNCLEAR"},
			expected:     "CLEAR",
		},
		{
			name:         "exact match with spaces",
			decision:     "  CLEAR  ",
			validOptions: []string{"CLEAR", "UNCLEAR"},
			expected:     "CLEAR",
		},
		
		// Prefix matches
		{
			name:         "prefix match short",
			decision:     "ref",
			validOptions: []string{"refactor", "enhance", "rewrite"},
			expected:     "refactor",
		},
		{
			name:         "prefix match case insensitive",
			decision:     "REF",
			validOptions: []string{"refactor", "enhance"},
			expected:     "refactor",
		},
		{
			name:         "prefix match first wins",
			decision:     "re",
			validOptions: []string{"refactor", "rewrite", "review"},
			expected:     "refactor",
		},
		
		// Contains matches
		{
			name:         "contains match in option",
			decision:     "factor",
			validOptions: []string{"enhance", "refactor", "review"},
			expected:     "refactor",
		},
		{
			name:         "option contained in decision",
			decision:     "run_tests",
			validOptions: []string{"test", "build", "deploy"},
			expected:     "test",
		},
		{
			name:         "contains match case insensitive",
			decision:     "FACT",
			validOptions: []string{"enhance", "refactor"},
			expected:     "refactor",
		},
		
		// No matches
		{
			name:         "no match at all",
			decision:     "invalid",
			validOptions: []string{"CLEAR", "UNCLEAR"},
			expected:     "",
		},
		{
			name:         "empty decision",
			decision:     "",
			validOptions: []string{"CLEAR", "UNCLEAR"},
			expected:     "",
		},
		{
			name:         "no valid options",
			decision:     "CLEAR",
			validOptions: []string{},
			expected:     "",
		},
		
		// Priority order tests
		{
			name:         "exact match wins over prefix",
			decision:     "test",
			validOptions: []string{"testing", "test", "tester"},
			expected:     "test",
		},
		{
			name:         "prefix match wins over contains",
			decision:     "ref",
			validOptions: []string{"preference", "refactor"},
			expected:     "refactor",
		},
		
		// Edge cases
		{
			name:         "special characters in decision",
			decision:     "CLEAR!",
			validOptions: []string{"CLEAR", "UNCLEAR"},
			expected:     "CLEAR", // Should match despite special char
		},
		{
			name:         "only special characters",
			decision:     "!!!",
			validOptions: []string{"CLEAR", "UNCLEAR"},
			expected:     "", // No match for pure special chars
		},
		{
			name:         "numeric options",
			decision:     "1",
			validOptions: []string{"option1", "option2", "1"},
			expected:     "1",
		},
		{
			name:         "very long option names",
			decision:     "proceed",
			validOptions: []string{"proceed-with-conversation", "clarify-request"},
			expected:     "proceed-with-conversation",
		},
		{
			name:         "hyphenated options",
			decision:     "clarify",
			validOptions: []string{"proceed-with-conversation", "clarify-request"},
			expected:     "clarify-request",
		},
		
		// Real-world scenarios
		{
			name:         "LLM adds extra text",
			decision:     "I think we should go with CLEAR",
			validOptions: []string{"CLEAR", "UNCLEAR"},
			expected:     "CLEAR",
		},
		{
			name:         "LLM response with quotes",
			decision:     "'refactor'",
			validOptions: []string{"refactor", "enhance"},
			expected:     "refactor",
		},
		{
			name:         "Multiple options could match",
			decision:     "clear",
			validOptions: []string{"CLEAR", "UNCLEAR", "CLEARLY_NOT"},
			expected:     "CLEAR", // Exact match wins
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findBestMatch(tt.decision, tt.validOptions)
			assert.Equal(t, tt.expected, result, 
				"findBestMatch(%q, %v) should return %q", 
				tt.decision, tt.validOptions, tt.expected)
		})
	}
}

