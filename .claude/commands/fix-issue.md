Please analyze and fix the GitHub issue: $ARGUMENTS

## Instructions

1. **Retrieve issue details**:
   - Use `gh issue view $ARGUMENTS` to get the full issue description
   - If GitHub CLI isn't available, ask for the issue details to be provided

2. **Understand the problem**:
   - Analyze the issue description, steps to reproduce, and expected behavior
   - Identify the root cause and scope of the problem
   - Note any specific requirements or constraints mentioned

3. **Search the codebase**:
   - Find relevant files related to the issue
   - Use grep/find to locate code that might be causing the problem
   - Review related test files and documentation

4. **Implement the fix**:
   - Make the necessary code changes to resolve the issue
   - Ensure the fix addresses the root cause, not just symptoms
   - Follow existing code patterns and conventions
   - Add error handling where appropriate

5. **Test the fix**:
   - Write or update tests to cover the fixed functionality
   - Run existing tests to ensure no regressions
   - Test the specific scenario described in the issue
   - Verify edge cases are handled

6. **Quality assurance**:
   - Run linting and formatting tools
   - Ensure code passes type checking (if applicable)
   - Check for any potential side effects
   - Review code for security implications

7. **Document the fix**:
   - Create a clear, descriptive commit message
   - Update documentation if the fix changes behavior
   - Add comments to code where the logic isn't obvious

8. **Prepare for review**:
   - Stage the changes with `git add`
   - Create a commit with a message referencing the issue
   - Optionally create a pull request if requested

## Success Confirmation

After completing the fix, confirm:
- ✅ Issue has been analyzed and understood
- 🔧 Code changes implement a proper fix
- ✅ Tests pass and cover the fixed functionality  
- 📝 Changes are committed with descriptive message
- 🔗 Issue number is referenced in commit/PR