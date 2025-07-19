# CLI Usage - Command Line Interface

The CLI provides quick, scriptable access to Genie's AI capabilities with full Unix pipe support.

## Basic Usage

```bash
# Ask a question
genie ask "your question here"

# Use with pipes (new!)
echo "some content" | genie ask "analyze this"
git diff | genie ask "suggest a commit message"
find . -name "*.go" | genie ask "what patterns do you see?"

# Get help
genie --help
genie ask --help
```

## 🔗 Unix Pipe Integration

Genie seamlessly integrates with Unix pipes, making it a natural part of your command-line workflow:

```bash
# Analyze command output
git diff | genie ask "suggest a commit message"
find . -name "*.go" | genie ask "what patterns do you see?"
ps aux | genie ask "which processes are using too much memory?"

# Combine with traditional tools
curl -s https://api.github.com/repos/user/repo | jq '.description' | genie ask "improve this description"
cat error.log | grep ERROR | genie ask "categorize these errors"

# Data processing pipelines
cat data.csv | head -10 | genie ask "what columns are most important?"
docker logs container_name | tail -100 | genie ask "any errors in these logs?"
```

**Benefits:**
- **Composable**: Works with any command that produces output
- **Natural**: Follows Unix philosophy of small, focused tools
- **Efficient**: No need to save intermediate files
- **Scriptable**: Perfect for automation and CI/CD

## Examples

### Development
```bash
# Code review (using pipes or redirection)
cat myfile.go | genie ask "review this function for bugs"
genie ask "review this function for bugs" < myfile.go

# Generate code
genie ask "write a REST API handler for user login"

# Debug help (using pipes)
tail -f debug.log | genie ask "why is this giving a segmentation fault?"
genie ask "analyze these stack traces" < debug.log

# Git workflow integration
git diff | genie ask "suggest a commit message"
git log --oneline -10 | genie ask "summarize recent changes"

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

### Unix Pipes and Redirection
```bash
# Process input from files (redirection)
genie ask "optimize this SQL query" < slow_query.sql

# Process input from commands (pipes)
cat slow_query.sql | genie ask "optimize this SQL query"

# Save output to files
genie ask "generate a Dockerfile for this app" > Dockerfile

# Chain multiple commands
cat logs/*.log | genie ask "find the root cause of errors"

# Combine with other Unix tools
find . -name "*.py" -exec cat {} \; | genie ask "what Python patterns do you see?"
ls -la | genie ask "how many files are here and what types?"

# Process git output
git status | genie ask "explain what needs to be committed"
git diff HEAD~1 | genie ask "what changed in the last commit?"

# System administration pipes
ps aux | genie ask "which processes are using the most memory?"
df -h | genie ask "is disk space getting low anywhere?"
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