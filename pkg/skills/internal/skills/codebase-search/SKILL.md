---
name: codebase-search
description: Search and analyze code repositories to find specific implementations, understand architecture, and answer questions about how code works. Use this when the user asks "where is...", "how does... work", "find...", or similar exploratory questions about the codebase.
---

# Codebase Search Skill

You are an expert at navigating and understanding codebases. Use this skill when the user needs to:
- Find specific code implementations
- Understand how a feature works
- Locate where functionality is defined
- Answer questions about code architecture
- Discover patterns and conventions in the code

## Process

When activated, follow this systematic approach:

### 1. Understand the Query
- Parse what the user is looking for
- Identify key terms, function names, or concepts
- Consider alternative names or patterns

### 2. Plan Your Search Strategy
Choose the most appropriate tools based on the query:
- **findFiles**: When looking for files by name or path pattern
- **searchInFiles**: When looking for specific code patterns, function names, or text
- **readFile**: To examine specific files in detail
- **bash**: For complex searches (e.g., git commands, ag, rg)

### 3. Execute Progressive Search
Start broad, then narrow:
1. **Initial Discovery**: Cast a wide net to find relevant files
2. **Refinement**: Examine promising results
3. **Deep Dive**: Read and analyze the actual implementation
4. **Verification**: Confirm findings and gather related context

### 4. Synthesize Findings
Present results that include:
- **Location**: File paths and line numbers
- **Context**: How the code fits into the larger system
- **Related Code**: Connected implementations or dependencies
- **Insights**: Patterns, conventions, or architectural notes

## Search Patterns

### Finding Functions/Methods
```
searchInFiles pattern:"function_name\\(" type:"go"
searchInFiles pattern:"def function_name" type:"py"
searchInFiles pattern:"function function_name" type:"js"
```

### Finding Type Definitions
```
searchInFiles pattern:"type TypeName (struct|interface)" type:"go"
searchInFiles pattern:"class ClassName" type:"py"
searchInFiles pattern:"interface InterfaceName" type:"ts"
```

### Finding Configuration
```
findFiles pattern:"**/*config*.{yaml,json,toml}"
searchInFiles pattern:"ConfigKey" glob:"**/*.{yaml,json}"
```

### Finding Tests
```
findFiles pattern:"**/*_test.go"
findFiles pattern:"**/*.test.{js,ts}"
searchInFiles pattern:"Test.*\\(" type:"go"
```

## Best Practices

1. **Start Simple**: Use basic searches before complex ones
2. **Use Context**: Look at imports, file structure, and related code
3. **Verify Understanding**: Read the actual code, don't just grep
4. **Follow References**: Track how code is used across the codebase
5. **Note Patterns**: Identify naming conventions and architecture patterns

## Example Workflow

User asks: "Where is user authentication handled?"

1. **Search for auth keywords**:
   ```
   searchInFiles pattern:"authenticate|auth" glob:"**/*.go"
   ```

2. **Check common locations**:
   ```
   findFiles pattern:"**/auth*.go"
   findFiles pattern:"**/middleware*.go"
   ```

3. **Read promising files**:
   ```
   readFile path:"pkg/auth/handler.go"
   ```

4. **Find usages**:
   ```
   searchInFiles pattern:"AuthMiddleware|RequireAuth"
   ```

5. **Synthesize**: Explain the auth flow, entry points, and integration

## Tips

- **File Organization**: Modern projects often organize by feature or layer
- **Naming Conventions**: Look for consistent patterns (handlers, services, repositories)
- **Entry Points**: Check main.go, routes, or server setup files
- **Tests**: Test files often show how components are used
- **Comments**: Look for package documentation and inline comments

When you complete your search, invoke the Skill tool with an empty skill name to clear this context.
