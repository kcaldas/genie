# Chain Assembly Examples

This directory contains reference examples for building complex AI chains in Genie. These examples are **not part of the active codebase** but serve as educational references for developers who want to understand how to create sophisticated chain structures.

## Purpose

The examples here demonstrate:

- **Complex decision-based chains** with multiple routing options
- **Sub-chain composition** and nested chain structures  
- **Prompt management** for multi-stage conversations
- **Error handling** and fallback strategies
- **Best practices** for chain architecture

## Examples

### `complex_decision_chain.go`

Demonstrates how to build a sophisticated decision-based chain that:

1. **Classifies user requests** using an `ai.DecisionStep`
2. **Routes to specialized sub-chains** based on request type
3. **Handles multiple prompts** for different conversation stages
4. **Provides fallback handling** for unmatched requests

Key concepts covered:
- Decision steps with multiple options
- Sub-chain creation and composition
- Prompt loading and error handling
- Response handlers for different chain types

## Usage

These examples are **reference-only** and do not implement any active interfaces. They serve as:

- **Learning materials** for understanding complex chain patterns
- **Copy-paste starting points** for building custom chains
- **Architecture reference** for chain design decisions

## Integration

To use patterns from these examples in your own chains:

1. **Adapt the structure** to your specific use case
2. **Create appropriate prompts** for your decision steps
3. **Implement required interfaces** (e.g., `PersonaAwareChainFactory`)
4. **Add proper error handling** and logging
5. **Test thoroughly** with your specific scenarios

## Notes

- Examples may reference prompts that don't exist in the active system
- Code is provided for educational purposes and may need adaptation
- Always test custom chains thoroughly before production use
- Consider performance implications of complex nested chains