# Detailed Reference Guide

This is a reference document that provides additional context and detailed information about using this skill.

## Advanced Topics

### Topic 1: Progressive Loading

Files are loaded on-demand rather than all at once. This conserves tokens and allows the AI to decide what information is needed.

### Topic 2: Security

All file paths are validated to ensure they're within the skill directory, preventing path traversal attacks.

### Topic 3: Use Cases

- Loading reference documentation when needed
- Inspecting scripts before execution
- Accessing example files and templates
- Building complex multi-file skill packages

## Best Practices

1. Keep SKILL.md focused and concise
2. Put detailed documentation in reference files
3. Use clear file naming conventions
4. Organize files in logical directories
