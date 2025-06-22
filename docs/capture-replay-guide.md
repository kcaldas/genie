# 📖 **Genie LLM Interaction Capture & Replay System - Complete User Guide**

## 🎯 **Overview**

The Genie Capture & Replay System is a comprehensive testing and debugging infrastructure that allows you to:
- **Capture real LLM interactions** from production or development
- **Replay captured scenarios** in controlled test environments  
- **Debug complex issues** systematically without manual testing
- **Test TUI interactions** programmatically

## 🏗️ **Architecture**

```
Application Layer
    ↓
AI Capture Middleware (optional, configurable)
    ↓
LLM Implementation (Vertex AI, OpenAI, etc.)
```

The system uses a middleware pattern that wraps your existing LLM client without requiring code changes.

---

## 🚀 **Quick Start**

### **1. Enable Capture (Development/Production)**

```bash
# Enable capture with debug output
GENIE_DEBUG=true ./genie

# Enable capture without debug noise  
GENIE_CAPTURE_LLM=true ./genie

# Capture to specific file
GENIE_CAPTURE_LLM=true GENIE_CAPTURE_FILE=my-issue.json ./genie
```

### **2. Reproduce Issue in Tests**

```go
func TestMyIssue(t *testing.T) {
    framework := NewTUITestFramework(t)
    
    // Load captured scenario
    err := framework.GetMockLLM().ReplayScenarioFromFile("my-issue.json")
    require.NoError(t, err)
    
    // Reproduce user interaction
    framework.TypeText("problematic command")
    framework.SendKeyString("enter")
    framework.StartChat("problematic command")
    
    // Analyze results
    response := framework.GetLastMessage()
    // Add your assertions here
}
```

---

## 🔧 **Environment Configuration**

### **Environment Variables**

| Variable | Description | Example |
|----------|-------------|---------|
| `GENIE_CAPTURE_LLM` | Enable basic capture | `true` |
| `GENIE_DEBUG` | Enable capture + debug logging | `true` |
| `GENIE_CAPTURE_FILE` | Custom output file | `issue-123.json` |

### **Automatic File Naming**

When no custom file is specified, captures are saved as:
```
genie-capture-{provider}-{timestamp}.json
# Example: genie-capture-vertex-ai-20241222-145400.json
```

---

## 📊 **Capture System**

### **What Gets Captured**

```json
{
  "id": "interaction_1234567890",
  "timestamp": "2024-12-22T14:54:00Z",
  "prompt": {
    "name": "conversation",
    "text": "Respond to: {{.message}}",
    "functions": [
      {"name": "listFiles", "description": "List files"}
    ]
  },
  "args": ["message", "find my files"],
  "response": "Here are your files...",
  "duration": "250ms",
  "llm_provider": "vertex-ai",
  "tools": ["listFiles", "findFiles"],
  "context": {
    "sessionID": "user-session-123"
  }
}
```

### **Capture Middleware Usage**

The capture middleware is automatically injected via Wire dependency injection. No code changes needed!

```go
// This happens automatically in Wire:
baseGen := vertex.NewClient()
if captureEnabled {
    return ai.NewCaptureMiddleware(baseGen, config)
}
return baseGen
```

---

## 🧪 **Testing Framework**

### **TUI Test Framework**

#### **Basic Setup**

```go
func TestMyFeature(t *testing.T) {
    // Create framework with real components + mock LLM
    framework := NewTUITestFramework(t)
    
    // Configure mock responses
    framework.GetMockLLM().SetResponses("Expected response")
    
    // Test interaction
    framework.TypeText("user input")
    framework.SendKeyString("enter")
    framework.StartChat("user input")
    
    // Verify results
    assert.True(t, framework.WaitForAIResponse(2 * time.Second))
    assert.Equal(t, "Expected response", framework.GetLastMessage())
}
```

#### **Available Methods**

| Method | Description | Example |
|--------|-------------|---------|
| `TypeText(text)` | Simulate typing | `framework.TypeText("hello")` |
| `SendKeyString(key)` | Press special keys | `framework.SendKeyString("enter")` |
| `StartChat(message)` | Start chat via Genie core | `framework.StartChat("hello")` |
| `WaitForAIResponse(timeout)` | Wait for response | `framework.WaitForAIResponse(2*time.Second)` |
| `GetLastMessage()` | Get most recent message | `msg := framework.GetLastMessage()` |
| `GetMessages()` | Get all messages | `all := framework.GetMessages()` |
| `HasMessage(text)` | Check for specific message | `framework.HasMessage("error")` |

### **MockLLM Configuration**

#### **Basic Response Configuration**

```go
mock := framework.GetMockLLM()

// Set sequential responses
mock.SetResponses("First response", "Second response")

// Set error responses  
mock.SetErrors(fmt.Errorf("API error"))

// Set tool-specific responses
mock.SetToolResponse("listFiles", "Files: file1.txt, file2.txt")

// Enable debug logging
mock.EnableDebugMode()
```

#### **Advanced Configuration**

```go
// Simulate processing delay
mock.SetDelay(500 * time.Millisecond)

// Custom response processing
mock.SetResponseProcessor(func(prompt ai.Prompt, rawResponse string) string {
    // Transform response for testing
    return processedResponse
})

// Simulate timeout
mock.SimulateTimeout()
```

---

## 🎬 **Scenario Replay**

### **Loading Scenarios**

#### **Load Entire Scenario File**

```go
func TestReplayScenario(t *testing.T) {
    framework := NewTUITestFramework(t)
    
    // Load all interactions from file
    err := framework.GetMockLLM().ReplayScenarioFromFile("captured-issue.json")
    require.NoError(t, err)
    
    // Interactions will be replayed in sequence
    framework.StartChat("first command")  // Uses first captured response
    framework.StartChat("second command") // Uses second captured response
}
```

#### **Load Specific Interaction**

```go
func TestSpecificInteraction(t *testing.T) {
    framework := NewTUITestFramework(t)
    
    // Load only specific interaction by ID
    err := framework.GetMockLLM().ReplaySpecificInteraction(
        "captured-issue.json", 
        "problematic_interaction_id")
    require.NoError(t, err)
    
    // Test that specific scenario
    framework.StartChat("problematic command")
}
```

#### **Manual Interaction Replay**

```go
func TestManualReplay(t *testing.T) {
    // Load interaction from file
    interactions, err := ai.LoadInteractionsFromFile("issue.json")
    require.NoError(t, err)
    
    framework := NewTUITestFramework(t)
    
    // Replay specific interaction
    framework.GetMockLLM().ReplayInteraction(interactions[0])
    
    // Test the replayed scenario
    framework.StartChat("test input")
}
```

### **Replay Metadata**

```go
// Check if currently replaying a scenario
metadata := framework.GetMockLLM().GetReplayMetadata()
if metadata["is_replay"] == true {
    t.Logf("Replaying from: %v", metadata["replay_source"])
    t.Logf("Original provider: %v", metadata["context"].(map[string]interface{})["original_provider"])
}
```

---

## 🔍 **Debugging & Analysis**

### **Interaction Inspection**

#### **Real-time Debug Output**

```go
framework.GetMockLLM().EnableDebugMode()
// Output:
// [MockLLM] Interaction #1:
//   Tools: [listFiles, findFiles]
//   Raw Response: "Here are your files"
//   Processed: "Here are your files"  
//   Duration: 250ms
```

#### **Detailed Analysis**

```go
func TestDetailedAnalysis(t *testing.T) {
    framework := NewTUITestFramework(t)
    framework.GetMockLLM().EnableDebugMode()
    
    // Perform interaction
    framework.StartChat("analyze this")
    framework.WaitForAIResponse(2 * time.Second)
    
    // Get detailed interaction data
    interaction := framework.GetMockLLM().GetLastInteraction()
    
    t.Logf("=== Interaction Analysis ===")
    t.Logf("Tools in prompt: %v", interaction.ToolsInPrompt)
    t.Logf("Raw LLM response: %q", interaction.RawResponse)
    t.Logf("Processed response: %q", interaction.ProcessedResponse)
    t.Logf("Final TUI display: %q", framework.GetLastMessage())
    t.Logf("Processing context: %v", interaction.Context)
    t.Logf("Duration: %v", interaction.Duration)
    
    // Check for specific issues
    if strings.Contains(framework.GetLastMessage(), "{") {
        t.Logf("⚠️  JSON detected in final message")
    }
}
```

#### **Complete Interaction Log**

```go
// Print summary of all interactions
framework.GetMockLLM().PrintInteractionSummary()

// Get programmatic access to all interactions
interactions := framework.GetMockLLM().GetInteractionLog()
for i, interaction := range interactions {
    t.Logf("Interaction %d: %s -> %s", i+1, interaction.RawResponse[:50], interaction.ProcessedResponse[:50])
}
```

### **Pipeline Analysis**

#### **Response Processing Testing**

```go
func TestResponseProcessing(t *testing.T) {
    framework := NewTUITestFramework(t)
    
    // Set up custom response processor to test transformations
    framework.GetMockLLM().SetResponseProcessor(func(prompt ai.Prompt, rawResponse string) string {
        // Simulate response processing that might cause issues
        if strings.Contains(rawResponse, "files") {
            return fmt.Sprintf(`{"processed": true} %s`, rawResponse)
        }
        return rawResponse
    })
    
    framework.GetMockLLM().SetResponses("Here are your files")
    framework.StartChat("show files")
    framework.WaitForAIResponse(2 * time.Second)
    
    interaction := framework.GetMockLLM().GetLastInteraction()
    
    // Verify processing occurred
    assert.NotEqual(t, interaction.RawResponse, interaction.ProcessedResponse)
    assert.Contains(t, interaction.ProcessedResponse, `{"processed": true}`)
    
    t.Logf("Raw: %s", interaction.RawResponse)
    t.Logf("Processed: %s", interaction.ProcessedResponse)
    t.Logf("Final: %s", framework.GetLastMessage())
}
```

---

## 🎯 **Common Use Cases**

### **1. Reproducing User-Reported Issues**

```go
// User reports JSON appearing in responses
func TestJSONLeakageIssue(t *testing.T) {
    // Step 1: Capture the issue (done in production with GENIE_CAPTURE_LLM=true)
    
    // Step 2: Replay in test
    framework := NewTUITestFramework(t)
    err := framework.GetMockLLM().ReplayScenarioFromFile("user-reported-json-leak.json")
    require.NoError(t, err)
    
    // Step 3: Reproduce exact user interaction
    framework.TypeText("find my todo list")
    framework.SendKeyString("enter")
    framework.StartChat("find my todo list")
    
    // Step 4: Analyze the issue
    finalMessage := framework.GetLastMessage()
    if strings.Contains(finalMessage, `{"`) {
        t.Logf("✅ JSON leakage reproduced: %s", finalMessage[:100])
        
        // Step 5: Investigate root cause
        interaction := framework.GetMockLLM().GetLastInteraction()
        t.Logf("Root cause analysis:")
        t.Logf("  LLM returning JSON: %v", strings.Contains(interaction.RawResponse, `{"`))
        t.Logf("  Processing changes response: %v", interaction.RawResponse != interaction.ProcessedResponse)
        t.Logf("  TUI changes display: %v", interaction.ProcessedResponse != finalMessage)
    }
}
```

### **2. Testing Tool Integration**

```go
func TestToolIntegration(t *testing.T) {
    framework := NewTUITestFramework(t)
    
    // Test normal tool response
    framework.GetMockLLM().SetToolResponse("listFiles", "Files: file1.txt, file2.txt")
    framework.StartChat("list my files")
    framework.WaitForAIResponse(2 * time.Second)
    
    interaction := framework.GetMockLLM().GetLastInteraction()
    assert.Contains(t, interaction.ToolsInPrompt, "listFiles")
    assert.Equal(t, "Files: file1.txt, file2.txt", framework.GetLastMessage())
}
```

### **3. Testing Confirmation System**

```go
func TestConfirmationFlow(t *testing.T) {
    framework := NewTUITestFramework(t)
    
    // Configure response that would trigger confirmation
    framework.GetMockLLM().SetToolResponse("runBashCommand", "Do you want to run: ls -la?")
    
    framework.TypeText("run ls -la")
    framework.SendKeyString("enter")
    framework.StartChat("run ls -la")
    
    // Wait for confirmation request
    framework.WaitForAIResponse(2 * time.Second)
    
    // Simulate user confirmation
    framework.TypeText("y")
    framework.SendKeyString("enter")
    
    // Verify confirmation flow
    assert.True(t, framework.HasMessage("Do you want to run: ls -la?"))
}
```

### **4. Performance Testing**

```go
func TestResponseTiming(t *testing.T) {
    framework := NewTUITestFramework(t)
    
    // Simulate slow LLM response
    framework.GetMockLLM().SetDelay(2 * time.Second)
    framework.GetMockLLM().SetResponses("Slow response")
    
    start := time.Now()
    framework.StartChat("test")
    gotResponse := framework.WaitForAIResponse(5 * time.Second)
    duration := time.Since(start)
    
    assert.True(t, gotResponse)
    assert.True(t, duration >= 2*time.Second, "Should respect configured delay")
    
    // Check actual interaction timing
    interaction := framework.GetMockLLM().GetLastInteraction()
    t.Logf("Simulated duration: %v", interaction.Duration)
}
```

### **5. Multi-step Conversation Testing**

```go
func TestConversationFlow(t *testing.T) {
    framework := NewTUITestFramework(t)
    
    // Set up multi-step conversation
    framework.GetMockLLM().SetResponses(
        "Hello! How can I help?",
        "I'll list your files",
        "Here are your files: file1.txt, file2.txt",
    )
    
    // Step 1
    framework.StartChat("hello")
    framework.WaitForAIResponse(2 * time.Second)
    assert.Equal(t, "Hello! How can I help?", framework.GetLastMessage())
    
    // Step 2  
    framework.StartChat("list files")
    framework.WaitForAIResponse(2 * time.Second)
    assert.Equal(t, "I'll list your files", framework.GetLastMessage())
    
    // Step 3
    framework.StartChat("show them")
    framework.WaitForAIResponse(2 * time.Second)
    assert.Contains(t, framework.GetLastMessage(), "file1.txt")
    
    // Verify complete conversation
    allMessages := framework.GetMessages()
    assert.Len(t, allMessages, 6) // 3 user + 3 assistant messages
}
```

---

## 📁 **File Management**

### **Capture File Structure**

Capture files are JSON arrays of interactions:

```json
[
  {
    "id": "interaction_1",
    "timestamp": "2024-12-22T14:54:00Z",
    "prompt": {...},
    "args": [...],
    "response": "...",
    "duration": "250ms",
    "llm_provider": "vertex-ai",
    "tools": [...],
    "context": {...}
  },
  {
    "id": "interaction_2",
    ...
  }
]
```

### **File Operations**

```go
// Load interactions from file
interactions, err := ai.LoadInteractionsFromFile("capture.json")

// Save interactions to file  
err := ai.SaveInteractionsToFile(interactions, "output.json")

// Capture middleware auto-save
capture := ai.NewInteractionCapture()
capture.SetOutputFile("auto-save.json") // Saves automatically
```

### **Organizing Capture Files**

Recommended file organization:

```
captures/
├── production/
│   ├── json-leakage-issue-20241222.json
│   ├── timeout-issue-20241223.json
│   └── user-reported-bug-456.json
├── development/
│   ├── feature-testing-20241222.json
│   └── integration-testing.json
└── test-scenarios/
    ├── normal-conversation.json
    ├── error-conditions.json
    └── edge-cases.json
```

---

## ⚡ **Advanced Features**

### **Custom Capture Configuration**

```go
// Create custom capture configuration
config := ai.CaptureConfig{
    Enabled:      true,
    DebugMode:    true,
    OutputFile:   "custom-capture.json",
    ProviderName: "my-llm-provider",
}

// Apply to any LLM implementation
capturedLLM := ai.NewCaptureMiddleware(baseLLM, config)
```

### **Capture Integration Testing**

```go
func TestCaptureIntegration(t *testing.T) {
    framework := NewTUITestFramework(t)
    
    // Enable capture on the mock (simulates real capture)
    capturedMock := ai.EnableCaptureForTesting(framework.GetMockLLM())
    
    // Configure test
    capturedMock.GetCapture().SetOutputFile("test-capture.json")
    framework.GetMockLLM().SetResponses("Test response")
    
    // Perform interaction
    framework.StartChat("test")
    framework.WaitForAIResponse(2 * time.Second)
    
    // Verify capture occurred
    interactions := capturedMock.GetCapturedInteractions()
    assert.Len(t, interactions, 1)
    assert.Equal(t, "Test response", interactions[0].Response)
    
    // Save for later use
    err := capturedMock.SaveCapture("saved-test.json")
    assert.NoError(t, err)
}
```

### **Response Transformation Testing**

```go
func TestResponseTransformation(t *testing.T) {
    framework := NewTUITestFramework(t)
    
    // Test different transformation scenarios
    testCases := []struct {
        name     string
        input    string
        expected string
        processor func(ai.Prompt, string) string
    }{
        {
            name:  "json_removal",
            input: `{"data": "test"} Regular response`,
            processor: func(p ai.Prompt, r string) string {
                // Remove JSON prefix
                if idx := strings.Index(r, "}"); idx != -1 && idx < 50 {
                    return strings.TrimSpace(r[idx+1:])
                }
                return r
            },
            expected: "Regular response",
        },
        {
            name:  "markdown_formatting", 
            input: "Here are **bold** items",
            processor: func(p ai.Prompt, r string) string {
                // Convert markdown to plain text
                return strings.ReplaceAll(strings.ReplaceAll(r, "**", ""), "*", "")
            },
            expected: "Here are bold items",
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            framework.GetMockLLM().Reset()
            framework.GetMockLLM().SetResponseProcessor(tc.processor)
            framework.GetMockLLM().SetResponses(tc.input)
            
            framework.StartChat("test")
            framework.WaitForAIResponse(2 * time.Second)
            
            interaction := framework.GetMockLLM().GetLastInteraction()
            assert.Equal(t, tc.input, interaction.RawResponse)
            assert.Equal(t, tc.expected, interaction.ProcessedResponse)
            assert.Equal(t, tc.expected, framework.GetLastMessage())
        })
    }
}
```

---

## 🛠️ **Troubleshooting**

### **Common Issues**

#### **1. Capture Not Working**

```bash
# Check environment variables
echo $GENIE_CAPTURE_LLM
echo $GENIE_DEBUG

# Verify file permissions
ls -la genie-capture-*.json

# Check for errors in logs
GENIE_DEBUG=true ./genie 2>&1 | grep -i capture
```

#### **2. Replay Not Working**

```go
// Verify file exists and is valid JSON
_, err := ai.LoadInteractionsFromFile("scenario.json")
if err != nil {
    t.Fatalf("Invalid scenario file: %v", err)
}

// Check replay metadata
metadata := framework.GetMockLLM().GetReplayMetadata()
t.Logf("Replay active: %v", metadata["is_replay"])
```

#### **3. Test Timeouts**

```go
// Increase timeout for slow interactions
gotResponse := framework.WaitForAIResponse(10 * time.Second)

// Check if response was received
if !gotResponse {
    interaction := framework.GetMockLLM().GetLastInteraction()
    if interaction != nil {
        t.Logf("Last interaction: %+v", interaction)
    }
    t.Fatal("No response received within timeout")
}
```

### **Debug Helpers**

```go
// Enable maximum debugging
framework.GetMockLLM().EnableDebugMode()
framework.GetMockLLM().PrintInteractionSummary()

// Check TUI state
t.Logf("Messages: %v", framework.GetMessages())
t.Logf("Loading: %v", framework.IsLoading())
t.Logf("Input: %v", framework.GetInput())

// Dump capture content
if capture := framework.GetMockLLM().GetCapture(); capture != nil {
    t.Logf("Capture summary: %s", capture.GetSummary())
}
```

---

## 📚 **Best Practices**

### **1. Organizing Tests**

```go
// Group related tests
func TestJSONLeakageIssues(t *testing.T) {
    tests := []struct {
        name         string
        scenarioFile string
        userInput    string
        expectJSON   bool
    }{
        {"list_files_leak", "scenarios/list-files-json.json", "list files", true},
        {"find_files_leak", "scenarios/find-files-json.json", "find todos", true},
        {"normal_response", "scenarios/normal-chat.json", "hello", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            framework := NewTUITestFramework(t)
            err := framework.GetMockLLM().ReplayScenarioFromFile(tt.scenarioFile)
            require.NoError(t, err)
            
            framework.StartChat(tt.userInput)
            framework.WaitForAIResponse(2 * time.Second)
            
            hasJSON := strings.Contains(framework.GetLastMessage(), `{"`)
            assert.Equal(t, tt.expectJSON, hasJSON)
        })
    }
}
```

### **2. Naming Conventions**

- **Capture files**: `{issue-type}-{date}-{description}.json`
  - `json-leakage-20241222-user-report.json`
  - `timeout-20241223-large-response.json`
  
- **Test functions**: `Test{Component}_{Issue}_{Scenario}`
  - `TestTUI_JSONLeakage_ListFiles`
  - `TestMock_Replay_MultipleInteractions`

### **3. Documentation**

```go
func TestComplexScenario(t *testing.T) {
    // Document the issue being tested
    // Issue: JSON responses leak into user interface when using listFiles tool
    // Captured from: Production user session on 2024-12-22
    // User command: "find my todo list"
    // Expected: Clean response without JSON
    // Actual: JSON object visible to user
    
    framework := NewTUITestFramework(t)
    err := framework.GetMockLLM().ReplayScenarioFromFile("json-leakage-production.json")
    require.NoError(t, err, "Failed to load production scenario")
    
    // Reproduce exact user interaction
    framework.StartChat("find my todo list")
    
    // Test passes if we can reproduce the issue
    // This allows us to verify our fix later
}
```

### **4. Continuous Integration**

```yaml
# .github/workflows/test.yml
- name: Run capture tests
  run: |
    # Test with captured scenarios
    go test ./cmd/tui/... -run="TestReplay"
    
    # Test capture functionality
    GENIE_CAPTURE_LLM=true go test ./pkg/ai/... -run="TestCapture"
```

---

## 🎯 **Next Steps**

### **For JSON Leakage Investigation**

1. **Capture the issue**:
   ```bash
   GENIE_DEBUG=true ./genie
   # Reproduce the issue and save the capture file
   ```

2. **Create focused test**:
   ```go
   func TestJSONLeakageRootCause(t *testing.T) {
       // Load your captured scenario
       // Analyze each step of the pipeline
       // Identify where JSON gets introduced
   }
   ```

3. **Test potential fixes**:
   ```go
   // Test response processing changes
   // Test prompt modifications
   // Test tool integration fixes
   ```

### **For System Enhancement**

1. **Add more debugging tools**
2. **Create scenario libraries for common issues**
3. **Build automated issue detection**
4. **Integrate with CI/CD for regression testing**

---

## 📖 **API Reference**

### **Core Types**

```go
type Interaction struct {
    ID           string
    Timestamp    time.Time
    Prompt       CapturedPrompt
    Args         []string
    Response     string
    Duration     time.Duration
    LLMProvider  string
    Tools        []string
    Context      map[string]interface{}
}

type CaptureConfig struct {
    Enabled      bool
    DebugMode    bool
    OutputFile   string
    ProviderName string
}
```

### **Key Functions**

```go
// Capture
ai.NewCaptureMiddleware(underlying Gen, config CaptureConfig) Gen
ai.GetCaptureConfigFromEnv(providerName string) CaptureConfig
ai.LoadInteractionsFromFile(filename string) ([]Interaction, error)
ai.SaveInteractionsToFile(interactions []Interaction, filename string) error

// Testing
NewTUITestFramework(t *testing.T) *TUITestFramework
framework.GetMockLLM() *MockLLMClient
mock.ReplayScenarioFromFile(filename string) error
mock.ReplayInteraction(interaction Interaction)

// Analysis
mock.GetLastInteraction() *LLMInteraction
mock.PrintInteractionSummary()
mock.EnableDebugMode()
```

---

This comprehensive guide provides everything needed to use the Genie Capture & Replay System effectively. The system transforms debugging complex LLM interactions from manual, error-prone processes into systematic, reproducible testing workflows.