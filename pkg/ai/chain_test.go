package ai

import (
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
		Steps: []ChainStep{
			{
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
			{
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
	err := ch.Run(mock, ctx, false)
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
		Steps: []ChainStep{
			{
				Name: "Step1",
				Prompt: &Prompt{
					Name:      "step1",
					Text:      "Some template",
					ModelName: "model-A",
				},
				ForwardAs: "step1Output",
			},
			{
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

	err := ch.Run(mock, ctx, false)
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
		Steps: []ChainStep{
			{
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

	err := ch.Run(mock, ctx, false)
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
		Steps: []ChainStep{
			{
				Name: "Step1",
				Fn: func(data map[string]string, debug bool) (string, error) {
					return "step1 output", nil
				},
				ForwardAs: "step1Output",
			},
		},
	}

	ctx := NewChainContext(nil)

	err := ch.Run(mock, ctx, false)
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
		Steps: []ChainStep{
			{
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

	err := ch.Run(mock, ctx, false)
	require.NoError(t, err)

	// Verify the context data
	assert.Equal(t, "step1 output", ctx.Data["step1Output"])

	ctx = NewChainContext(nil)
	err = ch.Run(mock, ctx, false)
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
		Steps: []ChainStep{
			{
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

	err := ch.Run(mock, ctx, false)
	require.NoError(t, err)

	// Verify the context data
	assert.Equal(t, "step1 output", ctx.Data["step1Output"])

	// Run the chain again, the step should not be executed
	mock.ResponseQueue = []string{"cached output"}
	err = ch.Run(mock, ctx, false)
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
		Steps: []ChainStep{
			{
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

	err := ch.Run(mock, ctx, false)
	require.Error(t, err, "expected an error when a step has both a prompt and a function")

	// Temporary file to save the output
	filePath := filepath.Join(os.TempDir(), "step1Output.txt")

	// Write some content to the file
	err = os.WriteFile(filePath, []byte("some output"), 0644)
	require.NoError(t, err)

	ch = Chain{
		Name: "TestChainWithPromptAndFunction",
		Steps: []ChainStep{
			{
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

	err = ch.Run(mock, ctx, false)
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
		Steps: []ChainStep{
			{
				Name:         "Step1",
				ForwardAs:    "step1Output",
				Requires:     []string{"InputText"},
				TemplateFile: filePath,
			},
		},
	}

	ctx := NewChainContext(map[string]string{"InputText": "step1"})

	err = ch.Run(mock, ctx, false)
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
		Steps: []ChainStep{
			{
				Name: "Step1",
				Prompt: &Prompt{
					Name:      "step1",
					Text:      "Some template",
					ModelName: "model-A",
				},
				ForwardAs: "step1Output",
			},
			{
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
		Steps: []ChainStep{
			{
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
