# Generalist Chain Factory

The Generalist Chain Factory provides a simpler, single-prompt approach to AI interactions that may improve performance and reduce complexity.

## Overview

**Default Chain Factory** (Multi-step):
```
Conversation → Clarification → Planning → Execution → Verification
```

**Generalist Chain Factory** (Single-step):
```
Generalist Engineer (with all tools and capabilities)
```

## Key Differences

### Default Chain Factory
- **Multiple prompts**: Specialized prompts for different phases
- **Multi-step workflow**: Clarification → Planning → Execution
- **Deliberate process**: Careful planning and confirmation steps
- **Higher latency**: Multiple LLM calls for complex requests

### Generalist Chain Factory  
- **Single prompt**: One comprehensive "generalist engineer" prompt
- **Integrated workflow**: Understanding, analysis, and implementation in one step
- **All tools available**: Complete toolset accessible immediately
- **Lower latency**: Single LLM call with full capabilities

## Usage

### Enable Generalist Chain Factory
```bash
export GENIE_CHAIN_FACTORY=generalist
./build/genie
```

### Use Default Chain Factory (default)
```bash
export GENIE_CHAIN_FACTORY=default
# or simply omit the variable
./build/genie
```

## Generalist Capabilities

The generalist prompt includes:

### File Operations
- **Exploration**: listFiles, findFiles, readFile, searchInFiles
- **Creation**: writeFile tool or FILE block format
- **Analysis**: Code review, pattern detection

### System Operations  
- **Git**: gitStatus, bash commands for git operations
- **Build**: bash commands for compilation, testing
- **Package management**: bash commands for dependencies

### Integrated Workflows
- **Code analysis**: Explore → understand → explain
- **Feature development**: Research → design → implement → test
- **Debugging**: Investigate → identify → fix → verify
- **Project setup**: Design → create files → configure → initialize

## Response Handlers

The generalist chain uses the `file_generator` response handler, which processes:

```yaml
FILE: path/to/file.ext
CONTENT:
[file contents]
END_FILE
```

This allows the LLM to create multiple files in a single response when using the FILE block format.

## Use Cases

### Best for Generalist Chain Factory
- Quick prototyping and experimentation
- Simple to medium complexity tasks
- Performance-sensitive applications
- When you prefer direct, action-oriented responses

### Best for Default Chain Factory
- Complex, multi-phase projects
- When you want explicit planning and confirmation
- Learning and educational scenarios
- Mission-critical implementations requiring deliberation

## Testing Performance

Compare both approaches:

```bash
# Test default chain factory
time echo "create a simple go web server" | ./build/genie ask

# Test generalist chain factory  
time GENIE_CHAIN_FACTORY=generalist echo "create a simple go web server" | ./build/genie ask
```

## Example Generalist Response

Input: "Create a simple Go web server with health check"

The generalist chain will:
1. Understand the requirement
2. Design a simple web server structure
3. Create the necessary files (main.go, go.mod, etc.)
4. Provide build/run instructions
5. All in a single, comprehensive response

The response includes both tool usage (writeFile) and direct file creation (FILE blocks) as appropriate.