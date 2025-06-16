Please help debug the following error: $ARGUMENTS

## Instructions

1. **Error analysis**:
   - Parse the error message to understand the type of error
   - Identify the file, line number, and function where error occurs
   - Determine if it's a compile-time, runtime, or logic error
   - Note any stack trace or additional context provided

2. **Reproduce the error**:
   - Try to reproduce the error with minimal steps
   - Identify the exact conditions that trigger the error
   - Test with different inputs to understand the scope
   - Check if the error is consistent or intermittent

3. **Investigate root cause**:
   - Examine the code at the error location
   - Check variable states and data flow leading to the error
   - Look for recent changes that might have introduced the issue
   - Review related code and dependencies

4. **Research common causes**:
   - Check for typical causes of this error type
   - Look for similar issues in documentation or forums
   - Consider environment or configuration issues
   - Check for version compatibility problems

5. **Diagnostic investigation**:
   - Add logging or debugging statements if needed
   - Use debugger to step through problematic code
   - Check memory usage, file permissions, network connectivity
   - Verify input validation and boundary conditions

6. **Solution development**:
   - Develop one or more potential fixes
   - Test each solution to ensure it resolves the issue
   - Verify the fix doesn't introduce new problems
   - Choose the most robust and maintainable solution

7. **Implementation and testing**:
   - Implement the chosen fix
   - Create tests to prevent regression
   - Test edge cases and error conditions
   - Verify the original functionality still works

8. **Documentation**:
   - Document the root cause and solution
   - Add comments explaining the fix if complex
   - Update error handling or documentation if needed

## Debugging Checklist

### Error Understanding
- [ ] Error message and type identified
- [ ] Location and context determined
- [ ] Stack trace analyzed (if available)
- [ ] Error reproduction confirmed

### Investigation
- [ ] Code path to error traced
- [ ] Recent changes reviewed
- [ ] Dependencies and environment checked
- [ ] Variable states examined

### Solution
- [ ] Root cause identified
- [ ] Fix implemented and tested
- [ ] Regression tests added
- [ ] Solution documented

### Verification
- [ ] Original issue resolved
- [ ] No new issues introduced
- [ ] Edge cases tested
- [ ] Performance impact assessed

## Common Error Patterns

Based on the error type, consider these common causes:

### Runtime Errors
- Null pointer/undefined variable access
- Array/slice bounds violations
- Type conversion failures
- Resource exhaustion (memory, file handles)

### Logic Errors
- Incorrect conditional logic
- Off-by-one errors in loops
- Race conditions in concurrent code
- Incorrect algorithm implementation

### Environment Errors
- Missing dependencies or files
- Permission issues
- Network connectivity problems
- Configuration mismatches

## Success Confirmation

After debugging, confirm:
- ✅ Error root cause identified and understood
- 🔧 Fix implemented and tested thoroughly
- 🧪 Regression tests added to prevent recurrence
- 📝 Solution documented for future reference