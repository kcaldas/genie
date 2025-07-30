package commands

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/events"
)

type DemoCommand struct {
	BaseCommand
	eventBus     events.EventBus
	notification types.Notification
}

func NewDemoCommand(eventBus events.EventBus, notification types.Notification) *DemoCommand {
	return &DemoCommand{
		BaseCommand: BaseCommand{
			Name:        "demo",
			Description: "Demo command for showcasing themes and views (internal)",
			Usage:       ":demo <type>",
			Examples: []string{
				":demo diff",
				":demo markdown",
				":demo chat",
			},
			Aliases:  []string{},
			Category: "Development",
			Hidden:   true, // Hide from help - internal command
		},
		eventBus:     eventBus,
		notification: notification,
	}
}

func (c *DemoCommand) Execute(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: :demo <type>\nAvailable types: diff, markdown, chat")
	}

	demoType := strings.ToLower(args[0])

	switch demoType {
	case "diff":
		return c.showDemosDiff()
	case "markdown":
		return c.showDemoMarkdown()
	case "chat":
		return c.showDemoChat()
	default:
		return fmt.Errorf("unknown demo type: %s\nAvailable types: diff, markdown, chat", demoType)
	}
}

func (c *DemoCommand) showDemosDiff() error {
	demoContent := `--- a/src/main.go
+++ b/src/main.go
@@ -1,10 +1,15 @@
 package main
 
 import (
 	"fmt"
+	"log"
+	"os"
 )
 
+const AppName = "Genie Demo"
+
 func main() {
-	fmt.Println("Hello World")
+	log.SetFlags(log.LstdFlags | log.Lshortfile)
+	fmt.Printf("Welcome to %s!\n", AppName) 
+	fmt.Println("This is a demo of our diff viewer capabilities")
 }`

	confirmationRequest := events.UserConfirmationRequest{
		ExecutionID: "demo-diff-showcase",
		Title:       "🎨 Demo: Diff Viewer Showcase",
		Content:     demoContent,
		ContentType: "diff",
		Message:     "This demo showcases how diffs are displayed with syntax highlighting and themes. Try different themes to see the variations!",
		ConfirmText: "Continue",
		CancelText:  "Close",
	}

	c.eventBus.Publish("user.confirmation.request", confirmationRequest)
	return nil
}

func (c *DemoCommand) showDemoMarkdown() error {
	demoContent := `# 🧞 Genie Theme Showcase

Welcome to the **Genie theme demonstration**! This markdown preview shows how different elements are rendered.

## Code Blocks

Here's some Go code with syntax highlighting:

` + "```go" + `
func main() {
    fmt.Println("Hello, Genie!")
    
    // This demonstrates syntax highlighting
    for i := 0; i < 10; i++ {
        log.Printf("Iteration %d", i)
    }
}
` + "```" + `

## Lists and Formatting

### Features Showcase:
- **Bold text** for emphasis  
- *Italic text* for subtle emphasis
- ` + "`inline code`" + ` for technical terms
- [Links](https://github.com/kcaldas/genie) to external resources

### Numbered Steps:
1. First, notice the **color scheme** 
2. Then, observe the **typography**
3. Finally, check the **spacing** and **alignment** 

## Tables

| Theme Element | Description | Status |
|---------------|-------------|--------|
| Headers | Large, prominent text | ✅ Working |
| Code blocks | Syntax highlighted | ✅ Working |
| Lists | Bullet and numbered | ✅ Working |
| Tables | Structured data | ✅ Working |

## Quotes and Emphasis

> This is a blockquote showing how quoted text appears.
> It can span multiple lines and maintains proper formatting.

---

*Try switching themes while viewing this to see the different color schemes and styling options available in Genie!*`

	confirmationRequest := events.UserConfirmationRequest{
		ExecutionID: "demo-markdown-showcase",
		Title:       "📝 Demo: Markdown Viewer Showcase", 
		Content:     demoContent,
		ContentType: "markdown",
		Message:     "This demo showcases markdown rendering with various elements and formatting. Perfect for testing themes!",
		ConfirmText: "Continue",
		CancelText:  "Close",
	}

	c.eventBus.Publish("user.confirmation.request", confirmationRequest)
	return nil
}

func (c *DemoCommand) showDemoChat() error {
	// Add user message first
	c.notification.AddSystemMessage("📝 Demo: Markdown Chat Showcase - Simulating conversation...")

	// Add first user message as system message (since we can't add user through notification)
	c.notification.AddSystemMessage("👤 User: Can you show me some markdown examples?")

	// Add assistant response with rich markdown
	c.notification.AddAssistantMessage(`# 🧞 Genie Markdown Showcase

Welcome! Here are some **markdown examples** to demonstrate our chat rendering:

## Code Examples

Here's a Go function with syntax highlighting:

` + "```go" + `
func main() {
    fmt.Println("Hello, Genie!")
    
    // Loop with markdown rendering
    for i := 0; i < 3; i++ {
        log.Printf("Iteration %d", i)
    }
}
` + "```" + `

And some JavaScript:

` + "```javascript" + `
const greet = (name) => {
    console.log(` + "`Hello, ${name}!`" + `);
};
greet("World");
` + "```" + `

## Lists and Formatting

### Features:
- **Bold text** for emphasis
- *Italic text* for subtle emphasis  
- ` + "`inline code`" + ` for technical terms
- [Links](https://github.com/kcaldas/genie) work too!

### Steps:
1. Notice the **syntax highlighting** in code blocks
2. See how **tables** are formatted below
3. Check out the **theme integration**

## Tables

| Feature | Status | Notes |
|---------|--------|-------|
| Syntax highlighting | ✅ Working | Multiple languages |
| Tables | ✅ Working | Clean formatting |
| Lists | ✅ Working | Bullets and numbers |
| Links | ✅ Working | Clickable references |

## Quotes and Emphasis

> This is a blockquote showing how quoted text appears in chat.
> Perfect for highlighting important information!

---

*Try switching themes while viewing this to see different color schemes!*`)

	// Add second user message
	c.notification.AddSystemMessage("👤 User: That looks great! Can you show me more complex examples?")

	// Add second assistant response
	c.notification.AddAssistantMessage(`## 🚀 Advanced Markdown Features

### Code Diffs in Chat
You can also show diffs inline:

` + "```diff" + `
  func calculateTotal(items []Item) float64 {
-     var total float64
+     total := 0.0
      for _, item := range items {
-         total += item.Price
+         total += item.Price * item.Quantity
      }
      return total
  }
` + "```" + `

### Math and Technical Content

Complex algorithms work well:

` + "```python" + `
def fibonacci(n):
    """Calculate Fibonacci number using dynamic programming"""
    if n <= 1:
        return n
    
    dp = [0] * (n + 1)
    dp[1] = 1
    
    for i in range(2, n + 1):
        dp[i] = dp[i-1] + dp[i-2]
    
    return dp[n]
` + "```" + `

### Nested Lists

1. **Project Setup**
   - Install dependencies
   - Configure environment
   - Set up database
   
2. **Development**
   - Write tests
   - Implement features
   - Review code
   
3. **Deployment**
   - Build application
   - Run tests
   - Deploy to production

### Complex Table

| Language | Syntax | Themes | Performance |
|----------|--------|--------|-------------|
| Go | ⭐⭐⭐⭐⭐ | 🎨 All themes | 🚀 Excellent |
| Python | ⭐⭐⭐⭐ | 🎨 All themes | ⚡ Good |
| JavaScript | ⭐⭐⭐⭐ | 🎨 All themes | ⚡ Good |
| Rust | ⭐⭐⭐⭐⭐ | 🎨 All themes | 🚀 Excellent |

> **Pro Tip:** All of this renders beautifully with theme-aware colors and proper syntax highlighting!`)

	c.notification.AddSystemMessage("🎨 Try switching themes with :theme <name> to see different color schemes!")

	return nil
}