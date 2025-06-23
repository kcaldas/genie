# Plan: Leverage Existing Interaction Capture Middleware for Chain Debug Analysis

## Analysis of Existing Infrastructure

You're absolutely right! Genie already has sophisticated interaction capture middleware that we should leverage:

### ✅ **Existing Capture System:**
- **CaptureMiddleware** wraps LLM calls and captures full interactions
- **Environment Configuration:** `GENIE_CAPTURE_LLM=true` and `GENIE_DEBUG=true`
- **JSON Output:** Saves interactions to timestamped files like `genie-capture-vertex-ai-TIMESTAMP.json`
- **Complete Data:** Captures prompts, responses, tools, context, duration, errors
- **Already Integrated:** Wire DI automatically enables capture based on env vars

### 🔍 **Current Capture Capabilities:**
- Individual LLM interactions (prompt → response)
- Tool function calls and responses
- Session context and debugging info
- Error conditions and timing data
- Provider information and metadata

## Enhanced Plan: Chain-Level Debug Analysis

Instead of building new debug infrastructure, enhance the existing capture system:

### 1. **Chain-Level Capture Enhancement**
- Extend capture middleware to understand chain context
- Add chain name, step name, and step type to captured interactions
- Group related interactions by chain execution ID
- Capture decision step choices and user confirmations

### 2. **Chain Debug Analysis Tools**
- Build analysis tools that parse captured JSON files
- Create side-by-side visualization: chain flow vs individual LLM calls
- Generate chain execution reports from captured data
- Enable Claude/LLM analysis of chain effectiveness using captured data

### 3. **Environment-Based Activation**
- Leverage existing `GENIE_DEBUG=true` for comprehensive chain debugging
- Add `GENIE_CHAIN_DEBUG=true` for chain-specific debugging
- Enhance capture output to include chain execution metadata
- Create debug folder structure organized by chain execution

### 4. **Chain Context Integration**
- Pass chain execution context through existing capture system
- Add chain step metadata to interaction context
- Capture decision paths and user confirmation flows
- Include chain timing and performance data

## Benefits of This Approach
- **Reuses proven infrastructure** instead of building from scratch
- **Already environment-configurable** with `GENIE_DEBUG=true`
- **Rich data capture** includes everything needed for analysis
- **JSON format** perfect for automated analysis and visualization
- **Minimal code changes** required to enhance existing system

## Implementation Steps
1. **Enhance capture context** - Add chain metadata to captured interactions
2. **Chain execution grouping** - Group related interactions by chain run
3. **Analysis tools** - Build visualization and analysis from captured JSON
4. **Documentation** - Update usage to show chain debugging capabilities

This leverages your existing sophisticated capture middleware for the "Chain Debug Analysis System" idea!