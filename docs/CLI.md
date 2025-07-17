# CLI Usage - Command Line Interface

The CLI provides quick, scriptable access to Genie's AI capabilities.

## Basic Usage

```bash
# Ask a question
genie ask "your question here"

# Get help
genie --help
genie ask --help
```

## Examples

### Development
```bash
# Code review
genie ask "review this function for bugs" < myfile.go

# Generate code
genie ask "write a REST API handler for user login"

# Debug help
genie ask "why is this giving a segmentation fault?" < debug.log

# Architecture advice
genie ask "design a microservices architecture for e-commerce"
```

### Writing & Research
```bash
# Documentation
genie ask "write API documentation for this endpoint" < openapi.yaml

# Content creation
genie ask "write a blog post about Docker security best practices"

# Research
genie ask "summarize the latest trends in machine learning"

# Editing
genie ask "improve this README file" < README.md
```

### System Administration
```bash
# Log analysis
genie ask "analyze these nginx logs for errors" < access.log

# Script generation
genie ask "create a backup script for PostgreSQL database"

# Monitoring
genie ask "create alerting rules for this Prometheus config" < prometheus.yml

# Troubleshooting
genie ask "diagnose this server performance issue" < system_info.txt
```

### Project Management
```bash
# Task breakdown
genie ask "break down 'implement user authentication' into subtasks"

# Planning
genie ask "create a project timeline for migrating to microservices"

# Status reports
genie ask "summarize this week's git commits into a status report"

# Documentation
genie ask "create user stories from these requirements" < requirements.txt
```

## Advanced Usage

### Piping and Redirection
```bash
# Process input from files
genie ask "optimize this SQL query" < slow_query.sql

# Save output to files
genie ask "generate a Dockerfile for this app" > Dockerfile

# Chain commands
cat logs/*.log | genie ask "find the root cause of errors"

# Process multiple files
find . -name "*.py" | xargs -I {} genie ask "add docstrings to this file: {}"
```

### Automation Scripts
```bash
#!/bin/bash
# Daily report generator

# Analyze logs
ERRORS=$(genie ask "summarize errors from today" < /var/log/app.log)

# Generate report  
genie ask "create a daily report with this error analysis: $ERRORS" > daily_report.md

# Send notification
echo "Daily report generated" | mail -s "Report Ready" team@company.com
```

### CI/CD Integration
```bash
# In your CI pipeline
genie ask "review this pull request for potential issues" < changes.diff

# Code quality check
genie ask "analyze code quality and suggest improvements" < src/

# Documentation update
genie ask "update API docs based on these changes" < api_changes.txt
```

## Personas

Use different AI personalities for specialized tasks:

```bash
# Engineering focus
genie --persona engineer ask "review this architecture"

# Product management
genie --persona product-owner ask "prioritize these features"

# Technical writing
genie --persona technical-writer ask "improve this documentation"
```

## Configuration

### Environment Variables
```bash
# Model selection
export GENIE_MODEL_NAME="gemini-2.5-flash"

# Adjust creativity
export GENIE_MODEL_TEMPERATURE="0.3"  # More focused
export GENIE_MODEL_TEMPERATURE="0.9"  # More creative

# Token limits
export GENIE_MAX_TOKENS="32000"
```

### Output Formatting
```bash
# Raw output (no formatting)
genie ask "generate JSON config" --raw

# Verbose output
genie ask "explain this code" --verbose
```

## Tips

### Best Practices
- Be specific in your questions
- Provide context with file input
- Use appropriate personas for different tasks
- Combine with standard Unix tools

### Performance
- Shorter prompts = faster responses
- Use environment variables for consistent config
- Pipe large inputs rather than inline

### Scripting
- Check exit codes for error handling
- Use `--quiet` flag for script output
- Combine with `jq` for JSON processing