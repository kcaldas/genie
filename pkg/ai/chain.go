package ai

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/fileops"
	"github.com/kcaldas/genie/pkg/logging"
	tplengine "github.com/kcaldas/genie/pkg/template"
)

// Chain represents a sequence of chain steps.
// - Name: a name/identifier for this chain.
// - Steps: the ordered list of steps to be executed.
type Chain struct {
	Name       string
	Steps      []interface{} // Can be ChainStep, DecisionStep, or UserConfirmationStep
	DescribeAt string
}

// ChainStep holds information about a single step in a chain.
// - Name: a human-friendly name or identifier for the step.
// - Type: the type of step (GenerateStep or FunctionStep).
// - Prompt: the base prompt for the step (which may contain Go template placeholders).
// - Data: a map of additional data that can be used to customize the prompt.
// - SaveAs: the key under which the output of this step is stored in the chain context.
// - Fn: a function that can be used to generate the content for this step.
type ChainStep struct {
	Name            string
	LocalContext    map[string]string
	ForwardAs       string
	Cache           bool
	SaveAs          string
	Requires        []string
	Prompt          *Prompt
	Fn              func(data map[string]string, debug bool) (string, error)
	TemplateFile    string
	ResponseHandler string // Process response through this handler
}

// DecisionStep represents a decision point in a chain where the LLM chooses a path.
// - Name: identifier for the decision step
// - Context: optional context to help the LLM make the decision
// - Options: map of option_key -> Chain to execute
// - SaveAs: optionally save the decision result
type DecisionStep struct {
	Name     string
	Context  string
	Options  map[string]*Chain
	SaveAs   string
}

// UserConfirmationStep represents a user confirmation point in a chain.
// - Name: identifier for the confirmation step
// - Message: message to show the user for confirmation
// - ConfirmChain: chain to execute if user confirms
// - CancelChain: chain to execute if user cancels
// - SaveAs: optionally save the confirmation result
type UserConfirmationStep struct {
	Name         string
	Message      string
	ConfirmChain *Chain
	CancelChain  *Chain
	SaveAs       string
}

// ChainContext holds information that gets passed through the steps in the chain.
//   - Data: a map used to store any data that steps may produce or consume.
//     For instance, a step might store the entire output text under a particular key.
//   - DecisionStepCounts: tracks execution count per decision step name to prevent infinite loops
type ChainContext struct {
	Data               map[string]string
	DecisionStepCounts map[string]int
}

// NewChainContext returns a new context with optional initial data.
func NewChainContext(initial map[string]string) *ChainContext {
	if initial == nil {
		initial = make(map[string]string)
	}
	return &ChainContext{
		Data:               initial,
		DecisionStepCounts: make(map[string]int),
	}
}

func (c *Chain) validateStepAction(step *ChainStep) error {
	nonNilCount := 0
	if step.Prompt != nil {
		nonNilCount++
	}
	if step.Fn != nil {
		nonNilCount++
	}
	if step.TemplateFile != "" {
		nonNilCount++
	}
	if nonNilCount != 1 {
		return fmt.Errorf("step %s must have exactly one of prompt, fn or template file", step.Name)
	}
	return nil
}

func (c *Chain) Run(ctx context.Context, gen Gen, chainCtx *ChainContext, eventBus events.EventBus, debug bool) error {
	logger := logging.NewChainLogger(c.Name)
	logger.Info("chain execution started", "steps", len(c.Steps))
	totalSteps := len(c.Steps)
	stepCount := 0
	for _, stepInterface := range c.Steps {
		stepCount++
		
		// Type switch to handle different step types
		switch step := stepInterface.(type) {
		case ChainStep:
			// Handle regular chain step
			if err := c.executeChainStep(ctx, gen, chainCtx, step, stepCount, totalSteps, logger, debug); err != nil {
				return err
			}
		case DecisionStep:
			// Handle decision step
			if err := c.executeDecisionStep(ctx, gen, chainCtx, step, stepCount, totalSteps, logger, eventBus, debug); err != nil {
				return err
			}
		case UserConfirmationStep:
			// Handle user confirmation step
			if err := c.executeUserConfirmationStep(ctx, gen, chainCtx, step, stepCount, totalSteps, logger, eventBus, debug); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown step type: %T", step)
		}
	}
	logger.Info("chain execution completed")

	if c.DescribeAt != "" {
		err := os.WriteFile(c.DescribeAt, []byte(c.Describe()), 0644)
		if err != nil {
			logger.Error("failed to write chain description", "file", c.DescribeAt, "error", err)
		}
	}

	return nil
}

// executeChainStep handles the execution of a regular ChainStep
func (c *Chain) executeChainStep(ctx context.Context, gen Gen, chainCtx *ChainContext, step ChainStep, stepCount, totalSteps int, logger logging.Logger, debug bool) error {
	var (
		output string
	)

	cacheExists := false
	// Cached on file. It exists if the file exists and is not a directory
	fileInfo, saveAsStatErr := os.Stat(step.SaveAs)
	if saveAsStatErr != nil {
		if !os.IsNotExist(saveAsStatErr) {
			return saveAsStatErr
		} else {
			cacheExists = false
		}
	} else {
		if fileInfo.IsDir() {
			return fmt.Errorf("the saveAs path %s is a directory", step.SaveAs)
		}
		cacheExists = true
	}

	// Check if the cache exists and step has a cache flag set to true
	if cacheExists && step.Cache {
		logger.Info("step cached", "step", stepCount, "total", totalSteps, "name", step.Name, "file", step.SaveAs)
		// Load the saved data on fileData
		fileData, err := os.ReadFile(step.SaveAs)
		if err != nil {
			return err
		}
		output = string(fileData)
	} else {
		logger.Info("step executing", "step", stepCount, "total", totalSteps, "name", step.Name)
		allData := make(map[string]string)

		// Add all the data from the chain context to the data for this step
		for k, v := range chainCtx.Data {
			allData[k] = v
		}

		// Add all the data from the step local context to the data for this Step
		for k, v := range step.LocalContext {
			allData[k] = v
		}

		// Check if all the required keys are present in the context
		for _, requiredKey := range step.Requires {
			if _, ok := allData[requiredKey]; !ok {
				return fmt.Errorf("step %s requires key %s which is not present in the context", step.Name, requiredKey)
			}
		}

		// Only allow either a prompt, a function or a template file
		err := c.validateStepAction(&step)
		if err != nil {
			return err
		}

		if step.Prompt != nil {
			// Generate the content for the step
			logger.Info("calling LLM", "step", step.Name, "prompt", step.Prompt.Name)
			output, err = gen.GenerateContentAttr(ctx, *step.Prompt, debug, MapToAttr(allData))
			if err != nil {
				logger.Error("LLM error", "step", step.Name, "error", err)
				return err
			}
			logger.Info("LLM response", "step", step.Name, "output_length", len(output), "empty", output == "")
			if output == "" {
				logger.Warn("empty LLM response", "step", step.Name, "prompt", step.Prompt.Name)
			}
		}

		if step.Fn != nil {
			// Get the content for the step from the function
			output, err = step.Fn(allData, debug)
			if err != nil {
				return err
			}
		}

		if step.TemplateFile != "" {
			// Render the content using the template file
			output, err = c.renderFile(step.TemplateFile, allData)
			if err != nil {
				return err
			}
		}

		// Save raw response first if SaveAs is specified (before handler processing)
		if step.SaveAs != "" {
			logger.Info("saving raw step output", "step", step.Name, "file", step.SaveAs)
			// Save the raw output to the saveAs file using file manager
			fileManager := fileops.NewFileOpsManager()
			err := fileManager.WriteFile(step.SaveAs, []byte(output))
			if err != nil {
				logger.Warn("failed to save raw output", "step", step.Name, "file", step.SaveAs, "error", err)
			}
		}

		// Process response through handler if specified
		if step.ResponseHandler != "" {
			// Get handler registry from context
			if handlerRegistry, ok := ctx.Value("handlerRegistry").(HandlerRegistry); ok {
				logger.Info("processing response through handler", "step", step.Name, "handler", step.ResponseHandler)
				processedOutput, err := handlerRegistry.ProcessResponse(ctx, step.ResponseHandler, output)
				if err != nil {
					logger.Error("response handler error", "step", step.Name, "handler", step.ResponseHandler, "error", err)
					return fmt.Errorf("response handler '%s' failed: %w", step.ResponseHandler, err)
				}
				logger.Info("response processed successfully", "step", step.Name, "handler", step.ResponseHandler, "result_length", len(processedOutput))
				output = processedOutput
			} else {
				logger.Warn("no handlerRegistry in context, skipping response handler", "step", step.Name, "handler", step.ResponseHandler)
			}
		}
	}

	if step.ForwardAs != "" {
		// Save the output to the chain context
		chainCtx.Data[step.ForwardAs] = output
	}

	return nil
}

// executeDecisionStep handles the execution of a DecisionStep
func (c *Chain) executeDecisionStep(ctx context.Context, gen Gen, chainCtx *ChainContext, step DecisionStep, stepCount, totalSteps int, logger logging.Logger, eventBus events.EventBus, debug bool) error {
	// Get current execution count for this specific decision step
	currentCount := chainCtx.DecisionStepCounts[step.Name]
	logger.Info("decision step executing", "step", stepCount, "total", totalSteps, "name", step.Name, "execution_count", currentCount)
	
	// Prevent infinite loops for this specific decision step
	const maxExecutionsPerDecision = 3
	if currentCount >= maxExecutionsPerDecision {
		logger.Info("decision step execution limit reached, forcing fallback", "step", step.Name, "count", currentCount)
		// For clarity decisions, fallback to CLEAR (proceed with conversation)
		// This prevents getting stuck in clarification loops
		if fallbackChain, exists := step.Options["CLEAR"]; exists {
			if step.SaveAs != "" {
				chainCtx.Data[step.SaveAs] = "CLEAR"
			}
			return fallbackChain.Run(ctx, gen, chainCtx, eventBus, debug)
		}
		// If no CLEAR option, take the first available option
		for key, chain := range step.Options {
			if step.SaveAs != "" {
				chainCtx.Data[step.SaveAs] = key
			}
			return chain.Run(ctx, gen, chainCtx, eventBus, debug)
		}
	}
	
	// Increment execution count for this specific decision step
	chainCtx.DecisionStepCounts[step.Name]++
	
	// Build the decision prompt
	var promptText string
	promptText = "Based on the current context, you need to choose one of the following options:\n\n"
	promptText += "Options:\n"
	
	// List all available options
	optionKeys := make([]string, 0, len(step.Options))
	for key, chain := range step.Options {
		optionKeys = append(optionKeys, key)
		description := chain.Name
		if description == "" {
			description = "Execute " + key + " chain"
		}
		promptText += fmt.Sprintf("- %s: %s\n", key, description)
	}
	
	// Add context if provided
	if step.Context != "" {
		promptText += fmt.Sprintf("\nContext: %s\n", step.Context)
	}
	
	promptText += "\nPlease respond with only the option key (e.g., \"" + optionKeys[0] + "\")."
	
	// Get default model configuration and use it for the decision prompt
	configManager := config.NewConfigManager()
	modelConfig := configManager.GetModelConfig()
	
	// Create a prompt for the decision with model configuration from config
	decisionPrompt := Prompt{
		Name:        step.Name + "_decision",
		Text:        promptText,
		ModelName:   modelConfig.ModelName,
		MaxTokens:   1000, // Increased for gemini-2.5-pro compatibility
		Temperature: 0.1, // Low temperature for consistent decision making  
		TopP:        modelConfig.TopP,
	}
	
	// Get the decision from the LLM
	rawDecision, err := gen.GenerateContentAttr(ctx, decisionPrompt, debug, MapToAttr(chainCtx.Data))
	if err != nil {
		return fmt.Errorf("failed to get decision for step %s: %w", step.Name, err)
	}
	
	logger.Info("raw decision response", "step", step.Name, "raw_response", rawDecision)
	
	// Clean up the decision (remove quotes, trim whitespace)
	decision := string(bytes.TrimSpace([]byte(rawDecision)))
	decision = string(bytes.Trim([]byte(decision), "\"'`"))
	
	// Check if the decision is valid
	chosenChain, ok := step.Options[decision]
	if !ok {
		// Try to find a partial match in case the LLM response is slightly off
		decision = findBestMatch(decision, optionKeys)
		if decision != "" {
			chosenChain = step.Options[decision]
			logger.Info("decision matched to valid option", "step", step.Name, "original", rawDecision, "matched", decision)
		} else {
			// No match found - check if there's a default option
			if defaultChain, hasDefault := step.Options["DEFAULT"]; hasDefault {
				decision = "DEFAULT"
				chosenChain = defaultChain
				logger.Info("decision unparseable, using DEFAULT", "step", step.Name, "raw_response", rawDecision)
			} else {
				return fmt.Errorf("invalid decision '%s' for step %s. Valid options are: %v", rawDecision, step.Name, optionKeys)
			}
		}
	}
	
	logger.Info("decision made", "step", step.Name, "choice", decision, "raw_was", rawDecision)
	
	// Save decision if requested
	if step.SaveAs != "" {
		chainCtx.Data[step.SaveAs] = decision
	}
	
	logger.Info("executing chosen chain", "step", step.Name, "choice", decision, "chain", chosenChain.Name)
	
	// Execute the chosen chain with the current context
	return chosenChain.Run(ctx, gen, chainCtx, eventBus, debug)
}

// executeUserConfirmationStep handles the execution of a UserConfirmationStep
func (c *Chain) executeUserConfirmationStep(ctx context.Context, gen Gen, chainCtx *ChainContext, step UserConfirmationStep, stepCount, totalSteps int, logger logging.Logger, eventBus events.EventBus, debug bool) error {
	logger.Info("user confirmation step executing", "step", stepCount, "total", totalSteps, "name", step.Name)
	
	// Generate execution ID for this confirmation
	executionID := uuid.New().String()
	
	// Get session ID from context
	sessionID := "unknown"
	if ctx != nil {
		if id, ok := ctx.Value("sessionID").(string); ok && id != "" {
			sessionID = id
		}
	}
	
	// Build the confirmation message using template data from chain context
	confirmationMessage := step.Message
	for key, value := range chainCtx.Data {
		confirmationMessage = strings.ReplaceAll(confirmationMessage, "{{."+key+"}}", value)
	}
	
	// Create confirmation request
	request := events.UserConfirmationRequest{
		ExecutionID: executionID,
		SessionID:   sessionID,
		Title:       "Implementation Plan Confirmation",
		Content:     chainCtx.Data["implementation_plan"], // Assume plan is stored here
		ContentType: "plan",
		Message:     confirmationMessage,
		ConfirmText: "Proceed with implementation",
		CancelText:  "Revise plan",
	}
	
	// If no event bus is available, default to confirm (for testing)
	if eventBus == nil {
		logger.Info("no event bus available, defaulting to confirm", "step", step.Name)
		if step.SaveAs != "" {
			chainCtx.Data[step.SaveAs] = "true"
		}
		if step.ConfirmChain != nil {
			return step.ConfirmChain.Run(ctx, gen, chainCtx, eventBus, debug)
		}
		return nil
	}
	
	// Create a channel to receive the response
	responseChan := make(chan bool, 1)
	
	// Subscribe to confirmation responses for this execution ID
	eventBus.Subscribe("user.confirmation.response", func(event interface{}) {
		if response, ok := event.(events.UserConfirmationResponse); ok && response.ExecutionID == executionID {
			responseChan <- response.Confirmed
		}
	})
	
	// Send confirmation request
	eventBus.Publish(request.Topic(), request)
	
	// Wait for response with timeout
	select {
	case confirmed := <-responseChan:
		logger.Info("user confirmation received", "step", step.Name, "confirmed", confirmed)
		
		// Save confirmation result if requested
		if step.SaveAs != "" {
			chainCtx.Data[step.SaveAs] = fmt.Sprintf("%t", confirmed)
		}
		
		// Execute appropriate chain based on confirmation
		if confirmed && step.ConfirmChain != nil {
			return step.ConfirmChain.Run(ctx, gen, chainCtx, eventBus, debug)
		} else if !confirmed && step.CancelChain != nil {
			return step.CancelChain.Run(ctx, gen, chainCtx, eventBus, debug)
		}
		return nil
		
	case <-ctx.Done():
		return ctx.Err()
		
	case <-time.After(5 * time.Minute): // 5 minute timeout
		logger.Warn("user confirmation timeout", "step", step.Name)
		return fmt.Errorf("user confirmation timeout for step %s", step.Name)
	}
}

func (c *Chain) renderFile(fileName string, data map[string]string) (string, error) {
	engine := tplengine.NewEngine()
	return engine.RenderFile(fileName, data)
}

// Join takes one or more other chains and appends their steps to the current chain.
// It preserves the current chain's Name and DescribeAt.
func (c *Chain) Join(others ...*Chain) *Chain {
	for _, other := range others {
		c.Steps = append(c.Steps, other.Steps...)
	}
	return c
}

// AddDecision adds a decision step to the chain
func (c *Chain) AddDecision(name, context string, options map[string]*Chain) *Chain {
	decisionStep := DecisionStep{
		Name:    name,
		Context: context,
		Options: options,
		SaveAs:  name, // Save the decision result using the step name
	}
	c.Steps = append(c.Steps, decisionStep)
	return c
}

// AddUserConfirmation adds a user confirmation step to the chain
func (c *Chain) AddUserConfirmation(name, message string, confirmChain, cancelChain *Chain) *Chain {
	confirmationStep := UserConfirmationStep{
		Name:         name,
		Message:      message,
		ConfirmChain: confirmChain,
		CancelChain:  cancelChain,
	}
	c.Steps = append(c.Steps, confirmationStep)
	return c
}

// Describe returns a formatted string describing the chain.
func (c *Chain) Describe() string {
	funcMap := template.FuncMap{
		"inc": func(i int) int {
			return i + 1
		},
	}

	tmpl, err := template.New("chainDescribe").Funcs(funcMap).Parse(chainDescriptionTemplate)
	if err != nil {
		logging.Fatal("failed to parse chain description template", "error", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, c); err != nil {
		logging.Fatal("failed to execute chain description template", "error", err)
	}

	return buf.String()
}

const chainDescriptionTemplate = `
Chain Name: {{.Name}}
Number of Steps: {{len .Steps}}

{{range $index, $step := .Steps}}
Step {{inc $index}}) {{if $step.Name}}{{$step.Name}}{{else}}Unknown{{end}}
  Type         : {{printf "%T" $step}}
  {{- if $step.ForwardAs}}
  ForwardAs    : {{$step.ForwardAs}}
  {{- end}}
  {{- if $step.SaveAs}}
  SaveAs       : {{$step.SaveAs}}
  {{- end}}
  {{- if $step.Cache}}
  Cache        : {{$step.Cache}}
  {{- end}}
  {{- if $step.Requires}}
  Requires     : {{range $req := $step.Requires}}{{$req}}, {{end}}
  {{- end}}
  {{- if $step.Prompt}}
  Prompt       : 
    Name       : {{$step.Prompt.Name}}
  {{- end}}
  {{- if $step.TemplateFile}}
  TemplateFile : {{$step.TemplateFile}}
  {{- end}}
  {{- if $step.LocalContext}}
  LocalContext :
    {{- range $k, $v := $step.LocalContext}}
    {{$k}} = {{$v}}
    {{- end}}
  {{- end}}
  {{- if $step.Context}}
  Context      : {{$step.Context}}
  {{- end}}
  {{- if $step.Options}}
  Options      : {{range $k, $v := $step.Options}}{{$k}} ({{$v.Name}}), {{end}}
  {{- end}}
{{end}}
`

// findBestMatch tries to find a partial match for the decision in the valid options
func findBestMatch(decision string, validOptions []string) string {
	decision = strings.TrimSpace(decision)
	
	// Handle empty decision
	if decision == "" {
		return ""
	}
	
	// Handle empty options
	if len(validOptions) == 0 {
		return ""
	}
	
	// Clean and uppercase for comparison
	decisionUpper := strings.ToUpper(decision)
	
	// First try exact match (case-insensitive)
	for _, option := range validOptions {
		if strings.EqualFold(decision, option) {
			return option
		}
	}
	
	// Then try prefix match
	for _, option := range validOptions {
		if decisionUpper != "" && strings.HasPrefix(strings.ToUpper(option), decisionUpper) {
			return option
		}
	}
	
	// Then try contains match
	for _, option := range validOptions {
		optionUpper := strings.ToUpper(option)
		if strings.Contains(optionUpper, decisionUpper) || 
		   strings.Contains(decisionUpper, optionUpper) {
			return option
		}
	}
	
	return ""
}
