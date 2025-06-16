Please perform a comprehensive code review of: $ARGUMENTS

## Instructions

1. **Scope analysis**:
   - If $ARGUMENTS is a file path, review that specific file
   - If $ARGUMENTS is a PR number, use `gh pr view $ARGUMENTS` to get details
   - If $ARGUMENTS is a commit hash, use `git show $ARGUMENTS`
   - If no argument, review staged changes with `git diff --staged`

2. **Code quality assessment**:
   - **Readability**: Check if code is clear and well-documented
   - **Maintainability**: Assess how easy it would be to modify/extend
   - **Performance**: Look for potential bottlenecks or inefficiencies
   - **Error handling**: Verify proper error handling and edge cases
   - **Security**: Check for potential security vulnerabilities

3. **Architecture review**:
   - Evaluate if the code follows project patterns and conventions
   - Check for proper separation of concerns
   - Assess if abstractions are appropriate
   - Verify interfaces and dependencies are well-designed

4. **Testing coverage**:
   - Identify what tests exist for the code
   - Suggest additional test cases if needed
   - Check if edge cases are covered
   - Verify test quality and effectiveness

5. **Best practices compliance**:
   - Check adherence to language-specific best practices
   - Verify consistent coding style and formatting
   - Look for code smells and anti-patterns
   - Assess documentation completeness

6. **Suggestions and improvements**:
   - Provide specific, actionable feedback
   - Suggest refactoring opportunities
   - Recommend performance optimizations
   - Propose additional features or error handling

## Review Format

Organize feedback into these categories:

### ✅ Strengths
- List what the code does well
- Highlight good practices observed

### ⚠️ Issues Found
- **Critical**: Security vulnerabilities, bugs, breaking changes
- **Major**: Performance issues, poor architecture, missing error handling
- **Minor**: Style issues, minor optimizations, suggestions

### 🔧 Specific Recommendations
- Provide code examples where helpful
- Suggest specific changes with reasoning
- Prioritize recommendations by impact

### 📝 Summary
- Overall assessment of code quality
- Key takeaways and action items
- Approval status (if reviewing a PR)

## Success Confirmation

After completing the review, confirm:
- ✅ Code has been thoroughly analyzed
- 📋 Issues are categorized by severity
- 💡 Specific, actionable recommendations provided
- 📊 Overall quality assessment given