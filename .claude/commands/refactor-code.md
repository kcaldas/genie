Please refactor the following code: $ARGUMENTS

## Instructions

1. **Code analysis**:
   - Read and understand the current code structure
   - Identify code smells, anti-patterns, and areas for improvement
   - Assess complexity, readability, and maintainability issues
   - Check for duplicate code and tight coupling

2. **Refactoring assessment**:
   - Determine the scope and goals of refactoring
   - Identify what tests exist to ensure behavior preservation
   - Plan refactoring steps to minimize risk
   - Consider performance implications of changes

3. **Safety preparations**:
   - Ensure comprehensive test coverage exists
   - Create additional tests if current coverage is insufficient
   - Document current behavior that must be preserved
   - Plan rollback strategy if issues arise

4. **Refactoring execution**:
   - Apply refactoring techniques systematically
   - Make small, incremental changes
   - Run tests after each significant change
   - Maintain functionality while improving structure

5. **Code improvements**:
   - **Extract methods/functions**: Break down large functions
   - **Remove duplication**: Consolidate repeated code
   - **Improve naming**: Use clear, descriptive names
   - **Simplify conditionals**: Reduce complexity and nesting
   - **Enhance error handling**: Make error handling more robust

6. **Quality verification**:
   - Run all tests to ensure no behavior changes
   - Check performance hasn't degraded
   - Verify code is more readable and maintainable
   - Ensure all edge cases still work

7. **Documentation updates**:
   - Update comments for any logic changes
   - Revise documentation if public interfaces changed
   - Add comments explaining complex refactored logic
   - Update examples if they reference changed code

## Common Refactoring Techniques

### Function/Method Level
- [ ] Extract method: Break large functions into smaller ones
- [ ] Rename method: Use more descriptive names
- [ ] Remove dead code: Delete unused functions
- [ ] Simplify conditional expressions

### Class/Structure Level  
- [ ] Extract class/struct: Create new abstractions
- [ ] Move method: Relocate methods to appropriate classes
- [ ] Remove middle man: Eliminate unnecessary indirection
- [ ] Encapsulate field: Improve data hiding

### Code Organization
- [ ] Move statements: Group related code together
- [ ] Extract variable: Clarify complex expressions
- [ ] Replace magic numbers: Use named constants
- [ ] Consolidate duplicate code

### Architecture Level
- [ ] Extract interface: Define clear contracts
- [ ] Replace inheritance with composition
- [ ] Eliminate feature envy: Move behavior to data
- [ ] Reduce coupling between components

## Refactoring Checklist

### Preparation
- [ ] Existing tests identified and running
- [ ] Additional tests written if needed
- [ ] Current behavior documented
- [ ] Refactoring plan created

### Execution
- [ ] Changes made incrementally
- [ ] Tests run after each change
- [ ] Functionality preserved throughout
- [ ] Code quality improved

### Verification
- [ ] All tests still pass
- [ ] Performance not degraded
- [ ] Code is more readable
- [ ] Maintainability improved

### Finalization
- [ ] Documentation updated
- [ ] Comments added where helpful
- [ ] Code committed with clear message
- [ ] Team notified of significant changes

## Success Confirmation

After refactoring, confirm:
- ✅ Code structure and readability significantly improved
- 🧪 All existing tests still pass
- 🎯 Functionality and behavior preserved
- 📈 Code maintainability and extensibility enhanced
- 📝 Changes documented and committed clearly