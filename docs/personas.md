# Genie Persona System Documentation

## Overview

The Genie Persona System allows you to create specialized AI assistants with different expertise, tools, and communication styles. Personas enable you to tailor Genie's behavior for specific roles, tasks, or domains, making your AI assistant more focused and effective.

## Quick Start

### Using a Persona

#### Command Line Flag

```bash
# Use a built-in persona
./build/genie --persona engineer
./build/genie ask --persona product_owner "analyze our roadmap"

# Use a custom persona
./build/genie --persona my_custom_persona
```

#### Environment Variable

Set the default persona for all sessions using the `GENIE_PERSONA` environment variable:

```bash
# Set default persona for current session
export GENIE_PERSONA=assistant
./build/genie ask "help me debug this code"

# Set persona for a single command
GENIE_PERSONA=genie-engineer ./build/genie ask "review this architecture"

# Persona precedence: command line flag > environment variable > default (engineer)
GENIE_PERSONA=assistant ./build/genie ask --persona engineer "help with Go code"
# This will use 'engineer' persona because --persona flag takes precedence
```

### Creating a Custom Persona

Create a file at `.genie/personas/my_persona/prompt.yaml`:

```yaml
name: "my-persona"
required_tools:
  - "readFile"
  - "writeFile"
text: |
  <%if .chat%>
    ## Conversation History
    <%.chat%>
  <%end%>
    ## User Message to be handled
  User: <%.message%>
instruction: |
  You are a specialized assistant focused on...
  
  ## Your expertise
  - Domain knowledge...
  - Specific skills...
  
  ## Your approach
  - How you work...
  - What you prioritize...
max_tokens: 8000
temperature: 0.7
```

## Persona Discovery Hierarchy

Genie searches for personas in the following order (highest to lowest priority):

1. **Project-level**: `$CWD/.genie/personas/{persona_name}/prompt.yaml`
   - Specific to the current project
   - Highest priority
   - Good for project-specific roles or temporary personas

2. **User-level**: `~/.genie/personas/{persona_name}/prompt.yaml`
   - Available across all projects for the user
   - Medium priority
   - Good for personal workflow preferences

3. **Internal**: Built into Genie binary
   - Always available
   - Lowest priority
   - Maintained by Genie team

## TUI Persona Management

When using Genie in interactive TUI mode (`./build/genie` with no arguments), you have access to powerful persona management features:

### Persona Commands

#### List Available Personas
```bash
:persona list     # or :p -l
```
Shows all available personas with their IDs, sources (internal/user/project), and display names.

#### Switch Personas
```bash
:persona swap engineer        # or :p -s engineer
:persona swap product_owner   # or :p -s product_owner
```
Immediately switches to the specified persona. The chat title will update to show the current persona name.

#### Persona Cycling

Create a list of your frequently used personas for quick cycling:

```bash
# Add personas to your cycle list
:persona cycle add engineer
:persona cycle add product_owner  
:persona cycle add reviewer

# Remove personas from cycle list
:persona cycle remove engineer
```

Once you have personas in your cycle list, you can quickly cycle through them using:

- **Manual cycling**: `:persona next` or `:p next`
- **Keyboard shortcuts**: 
  - **Ctrl+P** (always works)
  - **Shift+Tab** (works in most terminals)

The cycling feature:
- Wraps around (goes from last back to first persona)
- Handles missing personas gracefully (removes them from the cycle)
- Shows helpful messages when the cycle list is empty
- Updates the chat title to show the current persona name

### Visual Feedback

The TUI provides rich visual feedback for persona management:

- **Chat Title**: Shows the current persona name instead of generic "Messages"
- **Welcome Message**: Displays "Hello! I'm {persona_name}!" with the current persona
- **Status Messages**: Clear notifications for all persona operations
- **Real-time Updates**: UI immediately reflects persona changes

### Configuration Persistence

Your persona cycle list is automatically saved to your TUI configuration file (`~/.genie/settings.tui.json`) and persists between sessions.

## Built-in Personas

### engineer
The default persona with full development capabilities. Includes all tools and focuses on software engineering best practices.

### product_owner
Strategic and analytical persona focused on product management, documentation, and high-level planning. Excludes code modification tools.

### persona_creator
Specialized in designing custom personas. Expert in Genie's architecture, prompt engineering, and tool selection.

## Prompt Structure

### Required Fields

#### name
Unique identifier for the persona. Must match the directory name.

```yaml
name: "my-persona"
```

#### required_tools
List of tools this persona can access. Only include tools the persona needs.

```yaml
required_tools:
  - "readFile"
  - "writeFile"
  - "listFiles"
```

#### text
The conversation template using Go template syntax. This structures how the conversation history and user message are presented.

```yaml
text: |
  <%if .chat%>
    ## Conversation History
    <%.chat%>
  <%end%>
    ## User Message to be handled
  User: <%.message%>
```

#### instruction
The system prompt that defines the persona's behavior, expertise, and approach.

```yaml
instruction: |
  You are a specialized assistant...
```

### Optional Fields

#### max_tokens
Maximum response length (default: 8000)

```yaml
max_tokens: 10000
```

#### temperature
Controls response creativity (0.0-1.0, default: 0.7)
- Lower (0.3-0.5): More focused and deterministic
- Higher (0.7-0.9): More creative and varied

```yaml
temperature: 0.5
```

## Available Tools

### File System Tools
- `listFiles` - List directory contents with optional depth limit
- `readFile` - Read file contents
- `writeFile` - Create or modify files
- `findFiles` - Search for files by pattern (e.g., "*.go")

### Search Tools
- `searchInFiles` - Search for text patterns within files
- `bash` - Execute shell commands

## Template Variables

Personas can access these context variables in their prompts:

- `<%.chat%>` - Conversation history
- `<%.message%>` - Current user message
- `<%.project%>` - Project description (when available)
- `<%.files%>` - Known project files (when available)

## Template Escape Syntax

When writing personas that need to show Go template syntax in examples or documentation, use the `<%...%>` escape syntax instead of `{{...}}`. This prevents the template engine from interpreting the syntax during rendering.

### Why This Is Needed

When creating personas that generate other prompts or documentation (like the `persona_creator`), you need to show Go template syntax without it being executed. The `<%...%>` syntax is automatically converted to `{{...}}` after template rendering.

### Example

Instead of writing:
```yaml
# This would be interpreted as a template and fail
instruction: |
  Show users this template:
  {{if .chat}}
    Chat history: {{.chat}}
  {{end}}
```

Write:
```yaml
# This will display correctly as template syntax
instruction: |
  Show users this template:
  <%if .chat%>
    Chat history: <%.chat%>
  <%end%>
```

The output will show the correct Go template syntax to users.

## Creating Effective Personas

### 1. Define Clear Boundaries

Be specific about what the persona does and doesn't do:

```yaml
instruction: |
  You are a Security Auditor focused on identifying vulnerabilities.
  
  ## What you DO:
  - Analyze code for security issues
  - Suggest security improvements
  - Explain security concepts
  
  ## What you DON'T do:
  - Write implementation code
  - Modify system configurations
  - Make architectural decisions
```

### 2. Select Minimal Tools

Only include tools the persona actually needs:

```yaml
# For a code reviewer - read-only tools
required_tools:
  - "readFile"
  - "findFiles"
  - "searchInFiles"

# For a developer - full access
required_tools:
  - "readFile"
  - "writeFile"
  - "listFiles"
  - "findFiles"
  - "searchInFiles"
  - "bash"
```

### 3. Structure Instructions Logically

Organize instructions for clarity:

```yaml
instruction: |
  ## Role
  You are a...
  
  ## Expertise
  - Domain knowledge...
  - Technical skills...
  
  ## Approach
  1. First, analyze...
  2. Then, identify...
  3. Finally, recommend...
  
  ## Communication Style
  - Be concise and specific
  - Use examples
  - Explain rationale
```

### 4. Set Appropriate Parameters

Match parameters to the persona's purpose:

```yaml
# For analytical tasks
max_tokens: 6000
temperature: 0.4

# For creative tasks
max_tokens: 10000
temperature: 0.8
```

## Example Personas

### Code Reviewer

```yaml
name: "code-reviewer"
required_tools:
  - "readFile"
  - "findFiles"
  - "searchInFiles"
text: |
  <%if .chat%>
    ## Conversation History
    <%.chat%>
  <%end%>
    ## User Message to be handled
  User: <%.message%>
instruction: |
  You are an experienced Code Reviewer focused on improving code quality.
  
  ## Your Expertise
  - Code quality and maintainability
  - Security vulnerability identification
  - Performance optimization
  - Best practices enforcement
  
  ## Your Approach
  1. Read and understand the code context
  2. Identify issues and improvement opportunities
  3. Provide specific, actionable feedback
  4. Explain the rationale behind suggestions
  
  ## What You DON'T Do
  - Write or modify code directly
  - Make unsubstantiated claims
  - Focus on style over substance
  
  ## Communication Style
  - Constructive and educational
  - Specific with examples
  - Focused on learning and improvement
max_tokens: 6000
temperature: 0.4
```

### API Designer

```yaml
name: "api-designer"
required_tools:
  - "readFile"
  - "writeFile"
  - "findFiles"
  - "searchInFiles"
text: |
  <%if .chat%>
    ## Conversation History
    <%.chat%>
  <%end%>
    ## User Message to be handled
  User: <%.message%>
instruction: |
  You are an API Design Expert specializing in RESTful and GraphQL APIs.
  
  ## Your Expertise
  - RESTful API design principles
  - GraphQL schema design
  - API documentation (OpenAPI/Swagger)
  - Versioning strategies
  - Security best practices
  
  ## Your Approach
  1. Understand the domain and use cases
  2. Design clear, consistent API contracts
  3. Document thoroughly with examples
  4. Consider scalability and evolution
  
  ## Tools You Use
  - Read existing API definitions and code
  - Write OpenAPI specifications
  - Create API documentation
  - Search for API patterns in the codebase
  
  ## Communication Style
  - Clear and precise
  - Use concrete examples
  - Consider developer experience
max_tokens: 8000
temperature: 0.6
```

## Promoting Personas

After creating a project-level persona, you can promote it for wider use:

### To User Level

Make the persona available across all your projects:

```bash
cp -r .genie/personas/my_persona ~/.genie/personas/
```

### To Team Level

Share with your team by committing to the project:

```bash
git add .genie/personas/my_persona/
git commit -m "Add custom persona for our workflow"
```

## Advanced Features

### Dynamic Context

Personas can adapt based on available context:

```yaml
instruction: |
  <%if .project%>
    ## Project Context
    <%.project%>
  <%end%>
  
  <%if .files%>
    ## Known Files
    <%.files%>
  <%end%>
```

### Tool-Specific Guidance

Provide specific instructions for tool usage:

```yaml
instruction: |
  ## Tool Usage Guidelines
  
  ### When using readFile:
  - Start with entry points (main.go, index.js)
  - Follow imports to understand structure
  
  ### When using searchInFiles:
  - Search for function definitions first
  - Look for usage patterns
  - Check test files for examples
```

## Troubleshooting

### Persona Not Found

If Genie reports "persona not found":

1. Check the directory structure: `.genie/personas/{name}/prompt.yaml`
2. Ensure the persona name matches the directory name
3. Verify the prompt.yaml file exists and is valid YAML

### Template Syntax Errors

If you see template parsing errors:

1. Check for unescaped template syntax - use `<%...%>` for examples
2. Verify all template variables are properly closed
3. Ensure YAML indentation is correct

### Tools Not Working

If tools aren't available:

1. Verify tools are listed in `required_tools`
2. Check tool names match exactly (case-sensitive)
3. Ensure the tool exists in Genie's tool registry

## Quick Reference

### TUI Persona Commands
```bash
# List personas
:persona list                    # or :p -l

# Switch personas  
:persona swap <persona_id>       # or :p -s <persona_id>

# Manage cycle list
:persona cycle add <persona_id>    # Add to cycle list
:persona cycle remove <persona_id> # Remove from cycle list

# Cycle through personas
:persona next                    # or :p next
Ctrl+P                          # Keyboard shortcut (always works)
Shift+Tab                       # Keyboard shortcut (most terminals)
```

### TUI Visual Features
- **Chat title** shows current persona name
- **Welcome message** shows "Hello! I'm {persona_name}!"
- **Real-time updates** for all persona changes
- **Persistent configuration** saves cycle list between sessions

## Best Practices

1. **Start Simple**: Begin with minimal tools and expand as needed
2. **Test Iteratively**: Try the persona and refine based on results
3. **Document Purpose**: Include clear descriptions of the persona's role
4. **Version Control**: Commit project personas to share with your team
5. **Regular Updates**: Refine personas as your needs evolve
6. **Use Cycling**: Set up a cycle list with your most-used personas for quick switching

## Future Enhancements

The persona system is actively evolving. Planned features include:

- Persona inheritance/composition
- Dynamic tool loading based on context
- Persona marketplace for sharing
- Performance metrics per persona
- Custom model selection per persona

For the latest updates, check the Genie repository and documentation.
