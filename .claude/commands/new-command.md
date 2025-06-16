Please create a new Claude Code command file with the following specifications:

**Command Name:** $ARGUMENTS

## Instructions

1. **Ask for command details** if not provided:
   - What should this command do?
   - What files should it interact with?
   - What arguments/parameters should it accept?
   - What's the expected workflow?

2. **Create the command file** at `.claude/commands/[command-name].md`:
   - Use the command name provided in $ARGUMENTS as the filename
   - Follow the proper Claude Code command format
   - Include clear instructions for the LLM to follow
   - Use `$ARGUMENTS` parameter for user input where appropriate

3. **Structure the command properly**:
   - Start with a clear description of what the command does
   - Include step-by-step instructions
   - Specify target files and their locations
   - Include proper formatting requirements
   - Add context about the project structure if needed
   - Include success confirmation details

4. **Follow Claude Code best practices**:
   - Make commands reusable and structured
   - Include error handling (duplicate checking, file existence, etc.)
   - Preserve existing file structures
   - Provide clear feedback on what was done

## Template Structure

Use this general template for new commands:

```markdown
Please [action description] using the following input:

**[Parameter Name]:** $ARGUMENTS

## Instructions

1. **[Step 1]** - [detailed description]
2. **[Step 2]** - [detailed description]
3. **[Step 3]** - [detailed description]

## Context

[Relevant project context, file locations, structure information]

## Success Confirmation

After completing the task, please confirm:
- ✅ [Success indicator 1]
- 📍 [Location/target information]
- 📝 [Specific details about what was done]
```

## Examples of Command Types

### File Management Commands
- `add-feature.md` - Add new feature to feature list
- `update-phase.md` - Update implementation phase status
- `create-doc.md` - Create new documentation file

### Development Commands
- `add-tool.md` - Add new tool to tools inventory
- `update-timeline.md` - Update project timeline
- `create-test.md` - Create test specification

### Project Management Commands
- `add-milestone.md` - Add project milestone
- `update-status.md` - Update project status
- `create-report.md` - Generate status report

## Command Naming Conventions

- Use kebab-case for command names (e.g., `new-feature`, `update-docs`)
- Use descriptive, action-oriented names
- Keep names concise but clear
- Avoid abbreviations unless commonly understood

## Success Confirmation

After creating the command, please confirm:
- ✅ New command file created at `.claude/commands/[command-name].md`
- 📍 Command can be invoked as `/project:[command-name]`
- 📝 Command follows proper Claude Code format with `$ARGUMENTS` parameter
- 🔧 Command includes clear instructions and success confirmation