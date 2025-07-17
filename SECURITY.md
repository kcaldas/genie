# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability in Genie, please report it responsibly.

### How to Report

**DO NOT** create a public GitHub issue for security vulnerabilities.

Instead, please:

1. **Use GitHub Security Advisories**: [Report privately](https://github.com/kcaldas/genie/security/advisories/new)

### What to Include

Please provide as much information as possible:

- Description of the vulnerability
- Steps to reproduce the issue
- Potential impact
- Suggested fixes (if any)
- Your contact information

### Response Timeline

- **Initial Response**: Within 48 hours
- **Status Update**: Within 1 week
- **Fix Timeline**: Depends on severity and complexity

## Security Considerations

### API Keys
- Genie handles AI API keys - keep them secure
- Never commit API keys to version control
- Use environment variables or `.env` files
- Regularly rotate your API keys

### Docker Security
- Genie runs as non-root user (UID 1001)
- Mount directories read-only when possible
- Don't mount sensitive directories (`/`, `/home`, etc.)
- Use official images from GitHub Container Registry

### Tool Execution
- Bash tool can execute system commands - use with caution
- File operations are limited to mounted directories
- Consider running in sandboxed environments

### Network Security
- Genie makes HTTPS requests to AI APIs
- No data is stored persistently by default
- Chat history is kept in memory during session

## Best Practices

### For Users
```bash
# Use read-only mounts in Docker
docker run -v "$(pwd):/workspace:ro" genie

# Don't expose API keys in commands
export GEMINI_API_KEY="your-key"  # Good
genie ask "GEMINI_API_KEY=key ..."  # Bad - visible in process list

# Use .env files for local development
echo "GEMINI_API_KEY=your-key" > .env

# Regularly update to latest version
```

### For Developers
- Validate all user inputs
- Sanitize file paths to prevent traversal
- Use secure defaults
- Follow principle of least privilege
- Regular dependency updates

## Known Security Considerations

### AI Model Access
- Genie sends your prompts to external AI services
- Be mindful of sensitive information in prompts
- Consider using local models for sensitive data

### File System Access
- Tools can read/write files in current directory
- Use appropriate file permissions
- Be cautious with automated file operations

### Command Execution
- Bash tool executes system commands
- Validate commands before execution
- Consider disabling bash tool in sensitive environments

## Security Updates

Security updates are released as soon as possible:
- Critical: Within 24-48 hours
- High: Within 1 week
- Medium/Low: Next regular release

## Contact

For security-related questions or concerns:
- GitHub Security Advisories (preferred)

## Acknowledgments

We appreciate responsible disclosure and will acknowledge security researchers who help improve Genie's security.