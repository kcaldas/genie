package ai

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"text/template"

	"github.com/kcaldas/genie/pkg/fileops"
	"github.com/kcaldas/genie/pkg/logging"
	tplengine "github.com/kcaldas/genie/pkg/template"
)

// Chain represents a sequence of chain steps.
// - Name: a name/identifier for this chain.
// - Steps: the ordered list of steps to be executed.
type Chain struct {
	Name       string
	Steps      []ChainStep
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
	Name         string
	LocalContext map[string]string
	ForwardAs    string
	Cache        bool
	SaveAs       string
	Requires     []string
	Prompt       *Prompt
	Fn           func(data map[string]string, debug bool) (string, error)
	TemplateFile string
}

// ChainContext holds information that gets passed through the steps in the chain.
//   - Data: a map used to store any data that steps may produce or consume.
//     For instance, a step might store the entire output text under a particular key.
type ChainContext struct {
	Data map[string]string
}

// NewChainContext returns a new context with optional initial data.
func NewChainContext(initial map[string]string) *ChainContext {
	if initial == nil {
		initial = make(map[string]string)
	}
	return &ChainContext{Data: initial}
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

func (c *Chain) Run(ctx context.Context, gen Gen, chainCtx *ChainContext, debug bool) error {
	logger := logging.NewChainLogger(c.Name)
	logger.Info("chain execution started", "steps", len(c.Steps))
	totalSteps := len(c.Steps)
	stepCount := 0
	for _, step := range c.Steps {
		stepCount++
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
				output, err = gen.GenerateContentAttr(ctx, *step.Prompt, debug, MapToAttr(allData))
				if err != nil {
					return err
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

			if step.SaveAs != "" {
				logger.Info("saving step output", "step", step.Name, "file", step.SaveAs)
				// Save the output to the saveAs file using file manager
				fileManager := fileops.NewFileOpsManager()
				err := fileManager.WriteFile(step.SaveAs, []byte(output))
				if err != nil {
					return err
				}
			}
		}

		if step.ForwardAs != "" {
			// Save the output to the chain context
			chainCtx.Data[step.ForwardAs] = output
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
Step {{inc $index}}) {{.Name}}
  ForwardAs    : {{$step.ForwardAs}}
  SaveAs       : {{$step.SaveAs}}
  Cache        : {{$step.Cache}}
  Requires     : {{if $step.Requires}}{{range $req := $step.Requires}}{{$req}}, {{end}}{{else}}None{{end}}
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
{{end}}
`
