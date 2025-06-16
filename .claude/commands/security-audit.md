Please perform a security audit of: $ARGUMENTS

## Instructions

1. **Scope definition**:
   - If $ARGUMENTS is a file, audit that specific file
   - If $ARGUMENTS is a directory, audit all files in that directory
   - If $ARGUMENTS is a feature/component, audit related code
   - Define the security boundaries and attack surface

2. **Input validation audit**:
   - Check all user inputs are properly validated
   - Verify input sanitization and encoding
   - Look for injection vulnerabilities (SQL, command, code)
   - Test boundary conditions and malformed inputs

3. **Authentication and authorization**:
   - Review authentication mechanisms
   - Check authorization controls and permissions
   - Verify session management and token handling
   - Look for privilege escalation vulnerabilities

4. **Data protection audit**:
   - Check for hardcoded secrets or credentials
   - Verify sensitive data is encrypted at rest and in transit
   - Review data exposure in logs and error messages
   - Check for information leakage

5. **Configuration security**:
   - Review security-related configuration settings
   - Check for insecure defaults
   - Verify proper error handling without information disclosure
   - Review dependency and library security

6. **Code-level security issues**:
   - Look for buffer overflows and memory safety issues
   - Check for race conditions and concurrency issues
   - Review cryptographic implementations
   - Identify potential denial of service vulnerabilities

7. **Dependencies and third-party code**:
   - Audit third-party dependencies for known vulnerabilities
   - Check for outdated libraries with security issues
   - Review license compatibility and legal issues
   - Verify integrity of external dependencies

## Security Vulnerability Categories

### Input Validation
- [ ] SQL injection vulnerabilities
- [ ] Cross-site scripting (XSS)
- [ ] Command injection
- [ ] Path traversal attacks
- [ ] Buffer overflow conditions

### Authentication & Authorization
- [ ] Weak authentication mechanisms
- [ ] Session management flaws
- [ ] Privilege escalation opportunities
- [ ] Missing access controls
- [ ] Token handling issues

### Data Protection
- [ ] Hardcoded credentials or secrets
- [ ] Unencrypted sensitive data
- [ ] Information disclosure in errors
- [ ] Inadequate data sanitization
- [ ] Logging sensitive information

### Configuration Issues
- [ ] Insecure default configurations
- [ ] Missing security headers
- [ ] Overly permissive file permissions
- [ ] Debug mode enabled in production
- [ ] Unnecessary services or features enabled

### Code Quality Security
- [ ] Race conditions and TOCTOU issues
- [ ] Memory safety violations
- [ ] Integer overflow/underflow
- [ ] Weak cryptographic implementations
- [ ] Improper error handling

## Security Assessment Framework

### Critical Severity
- Remote code execution vulnerabilities
- Authentication bypass
- Privilege escalation to admin/root
- Data corruption or loss
- Complete system compromise

### High Severity
- Information disclosure of sensitive data
- Local privilege escalation
- Denial of service (resource exhaustion)
- Significant authorization bypass
- Cryptographic weaknesses

### Medium Severity
- Limited information disclosure
- Input validation bypass
- Weak authentication mechanisms
- Missing security controls
- Configuration issues

### Low Severity
- Information disclosure (non-sensitive)
- Missing security headers
- Weak password policies
- Logging security issues
- Minor configuration problems

## Security Testing Checklist

### Static Analysis
- [ ] Code reviewed for common vulnerability patterns
- [ ] Dependencies scanned for known vulnerabilities
- [ ] Configuration files reviewed for security issues
- [ ] Secrets and credentials scan completed

### Dynamic Testing
- [ ] Input validation tested with malicious inputs
- [ ] Authentication and authorization tested
- [ ] Session management tested
- [ ] Error handling tested for information disclosure

### Infrastructure Security
- [ ] File permissions and access controls verified
- [ ] Network security configurations reviewed
- [ ] Logging and monitoring capabilities assessed
- [ ] Backup and recovery procedures evaluated

## Remediation Recommendations

For each vulnerability found, provide:

### Issue Description
- Clear description of the vulnerability
- Potential impact and exploitability
- Affected components and code locations

### Remediation Steps
- Specific code changes needed
- Configuration updates required
- Additional security controls to implement
- Testing recommendations

### Prevention Measures
- Code review guidelines
- Secure coding practices
- Security testing procedures
- Monitoring and detection recommendations

## Success Confirmation

After completing the security audit, confirm:
- 🔍 Comprehensive security assessment completed
- 🚨 All vulnerabilities identified and categorized
- 🛡️ Specific remediation recommendations provided
- 📋 Security checklist completed and documented
- 🎯 Priority order for fixes established