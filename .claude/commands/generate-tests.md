Please generate comprehensive tests for: $ARGUMENTS

## Instructions

1. **Analyze the target**:
   - If $ARGUMENTS is a file path, analyze all functions/methods in that file
   - If $ARGUMENTS is a function name, find and analyze that specific function
   - If $ARGUMENTS is a module/package, analyze the public interface
   - Understand the functionality, inputs, outputs, and edge cases

2. **Test strategy planning**:
   - Identify what types of tests are needed (unit, integration, etc.)
   - Determine test framework and conventions used in the project
   - Plan test data and mock requirements
   - Consider performance and security test needs

3. **Test case design**:
   - **Happy path tests**: Normal, expected usage scenarios
   - **Edge cases**: Boundary conditions, empty inputs, maximum values
   - **Error conditions**: Invalid inputs, failure scenarios, exceptions
   - **Integration tests**: How the code interacts with dependencies

4. **Test implementation**:
   - Write clear, readable test functions with descriptive names
   - Use appropriate assertions and matchers
   - Set up proper test data and mocks
   - Follow project testing conventions and patterns

5. **Coverage verification**:
   - Ensure all public functions/methods are tested
   - Test all significant code paths and branches
   - Cover error handling and exception cases
   - Verify edge cases and boundary conditions

6. **Test quality assurance**:
   - Make tests independent and repeatable
   - Ensure tests are fast and reliable
   - Add clear documentation for complex test scenarios
   - Verify tests actually catch real bugs

## Test Categories to Include

### Unit Tests
- [ ] Function/method behavior with valid inputs
- [ ] Return value validation
- [ ] Parameter validation and error handling
- [ ] State changes and side effects

### Edge Case Tests
- [ ] Empty inputs (null, empty string, empty array)
- [ ] Boundary values (min/max, zero, negative)
- [ ] Large inputs and stress conditions
- [ ] Invalid or malformed inputs

### Error Condition Tests
- [ ] Exception handling and error messages
- [ ] Failure recovery and graceful degradation
- [ ] Invalid state transitions
- [ ] Resource exhaustion scenarios

### Integration Tests
- [ ] Interaction with external dependencies
- [ ] Database operations (if applicable)
- [ ] File system operations (if applicable)
- [ ] Network calls and API interactions

## Test Structure Template

```go
func TestFunctionName_Scenario(t *testing.T) {
    // Arrange
    input := setupTestData()
    expected := expectedResult()
    
    // Act
    result, err := FunctionName(input)
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

## Best Practices

### Test Naming
- Use descriptive names: `TestCalculateTotal_WithTaxAndDiscount_ReturnsCorrectAmount`
- Include the scenario being tested
- Make it clear what success/failure means

### Test Organization
- Group related tests in the same file
- Use table-driven tests for multiple scenarios
- Separate unit tests from integration tests
- Use setup/teardown functions appropriately

### Test Data
- Use realistic test data that represents actual usage
- Create helper functions for common test setup
- Use factories or builders for complex objects
- Keep test data focused and minimal

## Success Confirmation

After generating tests, confirm:
- ✅ Comprehensive test coverage implemented
- 🎯 All major functionality and edge cases covered
- 🧪 Tests follow project conventions and best practices
- ✅ All tests pass and are reliable
- 📝 Test documentation is clear and helpful