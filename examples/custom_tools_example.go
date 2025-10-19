package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/tools"
)

// Example 1: Simple custom tool
type GreetingTool struct{}

func NewGreetingTool() tools.Tool {
	return &GreetingTool{}
}

func (t *GreetingTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "greet",
		Description: "Greets a person by name",
		Parameters: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"name": {
					Type:        ai.TypeString,
					Description: "The name of the person to greet",
				},
			},
			Required: []string{"name"},
		},
	}
}

func (t *GreetingTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		name := args["name"].(string)
		greeting := fmt.Sprintf("Hello, %s! Nice to meet you!", name)

		return map[string]interface{}{
			"greeting": greeting,
		}, nil
	}
}

func (t *GreetingTool) FormatOutput(result map[string]interface{}) string {
	return fmt.Sprintf("Greeting: %v", result["greeting"])
}

// Example 2: Custom tool that does calculations
type CalculatorTool struct{}

func NewCalculatorTool() tools.Tool {
	return &CalculatorTool{}
}

func (t *CalculatorTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "calculate",
		Description: "Performs basic arithmetic operations",
		Parameters: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"operation": {
					Type:        ai.TypeString,
					Description: "The operation to perform: add, subtract, multiply, divide",
					Enum:        []string{"add", "subtract", "multiply", "divide"},
				},
				"a": {
					Type:        ai.TypeNumber,
					Description: "First number",
				},
				"b": {
					Type:        ai.TypeNumber,
					Description: "Second number",
				},
			},
			Required: []string{"operation", "a", "b"},
		},
	}
}

func (t *CalculatorTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		operation := args["operation"].(string)
		a := args["a"].(float64)
		b := args["b"].(float64)

		var result float64
		switch operation {
		case "add":
			result = a + b
		case "subtract":
			result = a - b
		case "multiply":
			result = a * b
		case "divide":
			if b == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			result = a / b
		default:
			return nil, fmt.Errorf("unknown operation: %s", operation)
		}

		return map[string]interface{}{
			"result": result,
		}, nil
	}
}

func (t *CalculatorTool) FormatOutput(result map[string]interface{}) string {
	return fmt.Sprintf("Result: %v", result["result"])
}

// Uncomment the main function below to run these examples
// Note: Only one main() can exist at a time in the examples directory
/*
func main() {
	fmt.Println("=== Custom Tools Examples ===\n")

	// Example 1: Add custom tools to default registry
	fmt.Println("Example 1: Adding custom tools to defaults")
	exampleAddCustomTools()

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// Example 2: Full control over tool registry
	fmt.Println("Example 2: Full control over tool registry")
	exampleFullControl()

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// Example 3: Advanced - using custom registry factory
	fmt.Println("Example 3: Custom registry factory")
	exampleCustomFactory()
}
*/

func exampleAddCustomTools() {
	// Create custom tools
	greetingTool := NewGreetingTool()
	calculatorTool := NewCalculatorTool()

	// Create Genie with custom tools added to defaults
	g, err := genie.NewGenie(
		genie.WithCustomTools(greetingTool, calculatorTool),
	)
	if err != nil {
		log.Fatalf("Failed to create Genie: %v", err)
	}

	// Start Genie
	session, err := g.Start(nil, nil)
	if err != nil {
		log.Fatalf("Failed to start Genie: %v", err)
	}

	fmt.Printf("✓ Genie started with session ID: %s\n", session.GetID())
	fmt.Printf("✓ Custom tools 'greet' and 'calculate' added to default tools\n")
	fmt.Printf("✓ Genie now has both default tools (readFile, bash, etc.) and custom tools\n")
}

func exampleFullControl() {
	// Create a custom registry with ONLY your tools
	registry := tools.NewRegistry()

	// Add only the tools you want
	err := registry.Register(NewGreetingTool())
	if err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}

	err = registry.Register(NewCalculatorTool())
	if err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}

	// Create Genie with custom registry (no default tools)
	g, err := genie.NewGenie(
		genie.WithToolRegistry(registry),
	)
	if err != nil {
		log.Fatalf("Failed to create Genie: %v", err)
	}

	// Start Genie
	session, err := g.Start(nil, nil)
	if err != nil {
		log.Fatalf("Failed to start Genie: %v", err)
	}

	fmt.Printf("✓ Genie started with session ID: %s\n", session.GetID())
	fmt.Printf("✓ Using custom registry with ONLY 'greet' and 'calculate' tools\n")
	fmt.Printf("✓ Default tools are NOT included\n")
}

func exampleCustomFactory() {
	// Use a factory function for advanced customization
	g, err := genie.NewGenie(
		genie.WithCustomRegistryFactory(func(eventBus events.EventBus, todoMgr tools.TodoManager) tools.Registry {
			// Start with default tools
			registry := tools.NewDefaultRegistry(eventBus, todoMgr)

			// Add your custom tools
			_ = registry.Register(NewGreetingTool())
			_ = registry.Register(NewCalculatorTool())

			// You could also create custom tool sets
			customTools := []tools.Tool{
				NewGreetingTool(),
				NewCalculatorTool(),
			}
			_ = registry.RegisterToolSet("my_tools", customTools)

			return registry
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create Genie: %v", err)
	}

	// Start Genie
	session, err := g.Start(nil, nil)
	if err != nil {
		log.Fatalf("Failed to start Genie: %v", err)
	}

	fmt.Printf("✓ Genie started with session ID: %s\n", session.GetID())
	fmt.Printf("✓ Using custom factory to build registry with defaults + custom tools\n")
	fmt.Printf("✓ Custom tool set 'my_tools' registered\n")
}

// Example: Using Genie with custom tools in your application
func exampleRealUsage() {
	// Create your custom tools
	myTools := []tools.Tool{
		NewGreetingTool(),
		NewCalculatorTool(),
	}

	// Initialize Genie with your tools
	g, err := genie.NewGenie(
		genie.WithCustomTools(myTools...),
	)
	if err != nil {
		log.Fatalf("Failed to initialize Genie: %v", err)
	}

	// Start a session
	workingDir := "/path/to/project"
	persona := "engineer"
	_, err = g.Start(&workingDir, &persona)
	if err != nil {
		log.Fatalf("Failed to start Genie: %v", err)
	}

	// Subscribe to chat responses
	g.GetEventBus().Subscribe("chat.response", func(event interface{}) {
		// Handle response
		fmt.Printf("Received response: %v\n", event)
	})

	// Send a chat message
	ctx := context.Background()
	err = g.Chat(ctx, "Can you greet Alice and calculate 5 + 3?")
	if err != nil {
		log.Fatalf("Chat failed: %v", err)
	}

	// The AI can now use both default tools and your custom tools
}
