package ai

import (
	"fmt"
	"sort"
	"strings"
)

// DecisionStep represents a decision point in a chain where the LLM chooses a path.
// - Name: identifier for the decision step
// - Context: optional context to help the LLM make the decision
// - Options: map of option_key -> Chain to execute
// - SaveAs: optionally save the decision result
type DecisionStep struct {
	Name    string
	Context string
	Options map[string]*Chain
	SaveAs  string
}

// AddOption adds a new option to the decision step
func (d *DecisionStep) AddOption(key string, chain *Chain) {
	if d.Options == nil {
		d.Options = make(map[string]*Chain)
	}
	d.Options[key] = chain
}

// BuildDecisionPrompt creates the prompt text for the LLM to make a decision
func (d *DecisionStep) BuildDecisionPrompt() (string, []string, error) {
	if len(d.Options) == 0 {
		return "", nil, fmt.Errorf("no options available for decision step %s (options map is empty or nil)", d.Name)
	}

	// Get sorted option keys for consistent ordering
	optionKeys := make([]string, 0, len(d.Options))
	for key := range d.Options {
		optionKeys = append(optionKeys, key)
	}
	sort.Strings(optionKeys)

	// Build the decision prompt
	promptText := "You are Genie, a friendly AI software engineer. Based on the current context, you need to choose one of the following options:\n\n"
	promptText += "Options:\n"

	// List all available options
	for _, key := range optionKeys {
		chain := d.Options[key]
		description := chain.Description
		if description == "" {
			description = chain.Name
		}
		if description == "" {
			description = "Execute " + key + " chain"
		}
		promptText += fmt.Sprintf("- %s: %s\n", key, description)
	}

	// Add context if provided
	if d.Context != "" {
		promptText += fmt.Sprintf("\nContext: %s\n", d.Context)
	}

	promptText += "\nPlease respond with only the option key. Valid options are: " + strings.Join(optionKeys, ", ") + "."

	return promptText, optionKeys, nil
}