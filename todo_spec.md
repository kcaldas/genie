# Todo Tool Specification

## Overview

The todo tool system provides structured task management for AI assistants during complex conversations and multi-step operations. It enables tracking progress, organizing work, and maintaining context across extended interactions.

## Tool Specifications

### TodoRead Tool

**Purpose:** Retrieve the current todo list for the session

**Request Payload:**
```json
{}
```
- No parameters required
- Empty object or no input

**Response Payload:**
```json
[
  {
    "id": "string",
    "content": "string",
    "status": "pending | in_progress | completed",
    "priority": "high | medium | low"
  }
]
```

**Response Fields:**
- `id`: Unique identifier for the todo item
- `content`: Task description (non-empty string)
- `status`: Current state of the task
- `priority`: Importance level of the task

**Behavior:**
- Returns complete current todo list
- Empty array `[]` if no todos exist
- Items returned in creation order (unless specified otherwise)
- Read-only operation - does not modify state

---

### TodoWrite Tool

**Purpose:** Create and manage the structured task list

**Request Payload:**
```json
{
  "todos": [
    {
      "content": "string (required, minLength: 1)",
      "status": "pending | in_progress | completed (required)",
      "priority": "high | medium | low (required)",
      "id": "string (required)"
    }
  ]
}
```

**Request Fields:**
- `todos`: Complete array of todo items (replaces existing list)
- `content`: Task description (required, non-empty)
- `status`: Task state (required enum)
- `priority`: Task importance (required enum)
- `id`: Unique identifier (required)

**Response Payload:**
```json
{
  "success": true,
  "message": "string"
}
```

**Behavior:**
- Replaces entire todo list with provided array
- Validates all required fields
- Enforces enum constraints on status/priority
- Atomic operation - all items update or none do
- Maintains todo state across conversation

**Status Values:**
- `pending`: Task not yet started
- `in_progress`: Currently working on task
- `completed`: Task finished successfully

**Priority Values:**
- `high`: Critical or urgent tasks
- `medium`: Important but not urgent tasks  
- `low`: Nice-to-have or deferred tasks

## LLM Usage Instructions

### When to Use TodoWrite

**REQUIRED for:**
- Complex multi-step tasks (3+ distinct steps or actions)
- Non-trivial tasks requiring careful planning or multiple operations
- User explicitly requests todo list usage
- User provides multiple tasks (numbered list or comma-separated)
- Multi-file changes or complex refactoring operations

**RECOMMENDED for:**
- Tasks requiring coordination across different components
- Long-running operations with checkpoints
- Tasks where progress visibility is valuable
- Complex debugging or analysis workflows

**DO NOT USE for:**
- Single, straightforward tasks
- Trivial operations completable in <3 simple steps
- Purely conversational or informational requests
- Tasks providing no organizational benefit

### When to Use TodoRead

**Use PROACTIVELY and FREQUENTLY:**
- At beginning of conversations to check pending work
- Before starting new tasks to understand current state
- When user asks about previous tasks or progress
- When uncertain about what to do next
- After completing tasks to update understanding of remaining work
- After every few messages to ensure staying on track
- Before making major decisions about task prioritization

### Task Management Workflow

#### 1. Task Creation
```json
{
  "todos": [
    {
      "content": "Analyze codebase structure",
      "status": "pending", 
      "priority": "high",
      "id": "1"
    },
    {
      "content": "Implement feature X",
      "status": "pending",
      "priority": "medium", 
      "id": "2"
    }
  ]
}
```

#### 2. Starting Work
**CRITICAL:** Mark task as `in_progress` BEFORE beginning work
```json
{
  "todos": [
    {
      "content": "Analyze codebase structure",
      "status": "in_progress",
      "priority": "high", 
      "id": "1"
    },
    {
      "content": "Implement feature X", 
      "status": "pending",
      "priority": "medium",
      "id": "2"
    }
  ]
}
```

#### 3. Task Completion
**Mark completed IMMEDIATELY** after finishing:
```json
{
  "todos": [
    {
      "content": "Analyze codebase structure",
      "status": "completed",
      "priority": "high",
      "id": "1" 
    },
    {
      "content": "Implement feature X",
      "status": "in_progress", 
      "priority": "medium",
      "id": "2"
    }
  ]
}
```

### Critical Rules

#### Task State Management
- **Only ONE task `in_progress` at any time**
- **Complete current tasks before starting new ones**
- **Mark completed IMMEDIATELY after finishing** (don't batch completions)
- **Remove irrelevant tasks** entirely from the list

#### Completion Criteria
**ONLY mark task as `completed` when:**
- Task is FULLY accomplished
- No unresolved errors or blockers remain
- Tests are passing (if applicable)
- Implementation is complete and working
- All stated requirements are met

**NEVER mark `completed` if:**
- Tests are failing
- Implementation is partial or incomplete
- Unresolved errors or exceptions occurred
- Couldn't find necessary files or dependencies
- Blocked on external factors

#### Error Handling
- If encountering errors/blockers: keep task as `in_progress`
- Create new tasks to resolve blockers when appropriate
- Add specific error resolution tasks to the list
- Provide clear status updates in task content

## Solution Flow Integration

### 1. Initial Planning Phase
```
User Request ’ Assess Complexity ’ TodoWrite (if complex) ’ TodoRead ’ Begin Work
```

### 2. Execution Phase  
```
TodoRead ’ Select Next Task ’ Mark in_progress ’ Execute ’ Mark completed ’ TodoRead ’ Repeat
```

### 3. Progress Tracking
```
Every 2-3 Actions ’ TodoRead ’ Status Update ’ Continue
```

### 4. Completion Phase
```
TodoRead ’ Verify All Complete ’ Final Status Update
```

## Frequency Guidelines

### TodoRead Usage
- **Start of conversation:** Always check for existing todos
- **Every 2-3 messages:** Check progress and remaining work
- **Before major decisions:** Understand current state
- **After user questions:** Show current progress
- **When feeling uncertain:** Get context and direction

### TodoWrite Usage  
- **Immediately upon complex request:** Capture all requirements
- **When starting tasks:** Mark as in_progress
- **Upon task completion:** Mark as completed immediately
- **When discovering new requirements:** Add to list
- **When priorities change:** Update accordingly

## Best Practices

### Task Breakdown
- Make tasks specific and actionable
- Break large tasks into smaller, manageable pieces
- Use clear, descriptive task names
- Include context when helpful

### Priority Management
- `high`: Blocking other work or user-critical
- `medium`: Important but can be scheduled
- `low`: Nice-to-have or future improvements

### Content Guidelines
- Start with action verbs ("Implement", "Fix", "Analyze")
- Include relevant context ("Fix bug in user authentication")
- Be specific enough to track progress
- Avoid vague descriptions

### ID Management
- Use simple incrementing numbers ("1", "2", "3")
- Or descriptive names ("auth-fix", "ui-update")
- Keep IDs stable across updates
- Don't reuse IDs within a session

## Example Workflows

### Software Development Task
```
User: "Add dark mode support to the application"

1. TodoWrite: Create tasks
   - "Analyze current theme system" (high, pending)
   - "Design dark theme color palette" (high, pending) 
   - "Implement theme switching logic" (medium, pending)
   - "Update UI components for dark mode" (medium, pending)
   - "Test theme switching functionality" (low, pending)

2. TodoRead: Check current state
3. Mark "Analyze current theme system" as in_progress
4. Execute analysis
5. Mark as completed, start next task
6. Continue until all completed
```

### Debugging Workflow
```
User: "Fix the authentication bug"

1. TodoWrite: Create investigation tasks
   - "Reproduce authentication issue" (high, pending)
   - "Analyze authentication flow" (high, pending)
   - "Identify root cause" (high, pending)
   - "Implement fix" (medium, pending)
   - "Test fix thoroughly" (medium, pending)

2. Work through systematically
3. Add new tasks if additional issues discovered
4. Mark completed only when fully resolved
```

## Anti-Patterns to Avoid

### Don't Do This:
- Batch completing multiple tasks at once
- Mark tasks completed with unresolved issues
- Create todos for trivial single-step operations
- Forget to mark tasks as in_progress when starting
- Let tasks stay in_progress when actually completed
- Create vague or unmeasurable tasks
- Have multiple tasks in_progress simultaneously

### Do This Instead:
- Complete tasks immediately upon finishing
- Only mark completed when fully done
- Use todos for complex multi-step work
- Always mark in_progress before starting
- Update status accurately and promptly
- Create specific, actionable tasks
- Maintain single-threaded task execution

This specification ensures consistent, effective use of the todo system for managing complex AI assistant workflows.