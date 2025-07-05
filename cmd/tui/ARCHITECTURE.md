# TUI2 Architecture Overview

This document describes the architecture of the refactored TUI2 module, which follows architectural patterns inspired by lazygit's GUI structure.

## Directory Structure

```
cmd/tui2/
├── types/           # Core interfaces and data structures
├── context/         # View state management and lifecycle
├── controllers/     # Business logic and user interaction handlers
├── presentation/    # Rendering and formatting logic
├── state/          # State management structures
├── helpers/        # Shared utilities
├── layout/         # Advanced layout management system
└── *.go            # Main app components
```

## Architecture Components

### 1. **types/** - Core interfaces and data structures
- `common.go` - Basic types like Message, UserInput, Theme, Config
- `interfaces.go` - Key interfaces that define contracts:
  - `Context` - View context interface
  - `Controller` - Business logic handler interface
  - `State` - State management interface
  - `IGuiCommon` - Common GUI operations interface
  - `IStateAccessor` - State access interface

### 2. **context/** - View state management
Each context manages its own:
- View state and rendering
- Keybindings
- Focus/blur lifecycle
- Specific behaviors

Files:
- `base.go` - Base context with common functionality
- `messages.go` - MessagesContext for chat display with scrolling, copying
- `input.go` - InputContext for user input with history navigation
- `debug.go` - DebugContext for debug panel with visibility toggle

### 3. **controllers/** - Business logic handlers
Controllers handle business logic without UI concerns:
- Process user input
- Coordinate with services
- Update state through accessors

Files:
- `base.go` - Base controller with common functionality
- `chat.go` - ChatController for message handling and Genie integration
- `command.go` - SlashCommandHandler for command processing

### 4. **presentation/** - Rendering and formatting
Separates "what to show" from "how to show it":
- `message_formatter.go` - Message formatting with markdown support, timestamps, wrapping
- `themes.go` - Theme definitions and management

### 5. **state/** - State management
Thread-safe state management with clear ownership:
- `chat_state.go` - Thread-safe chat state (messages, loading status)
- `ui_state.go` - Thread-safe UI state (debug messages, focused panel, config)
- `state_accessor.go` - Unified state access interface for controllers/contexts

### 6. **helpers/** - Shared utilities
Reusable utilities aggregated in a Helpers struct:
- `clipboard.go` - Cross-platform clipboard operations
- `config.go` - Configuration file management
- `notification.go` - System notifications
- `helpers.go` - Aggregates all helpers

### 7. **layout/** - Advanced layout management system
Sophisticated layout system inspired by lazygit:
- `box.go` - Box layout engine with hierarchical arrangement
- `window.go` - Window management separate from views
- `screen.go` - Screen mode management (normal, half, full)
- `manager.go` - Layout manager with conditional layout functions
- `responsive.go` - Responsive design with breakpoints

### 8. **Main App Components**
- `app.go` - Thin coordinator that:
  - Wires together all components
  - Manages initialization
  - Handles lifecycle
  - Delegates work to specialized components
- `gui_common.go` - IGuiCommon implementation
- `commands.go` - Command implementations (/help, /clear, etc.)
- `tui.go` - Simple entry point

## Key Design Principles

### 1. **Single Responsibility**
Each component has one clear purpose:
- Contexts manage views
- Controllers handle business logic
- State manages data
- Helpers provide utilities

### 2. **Interface-Based Design**
Heavy use of interfaces for:
- Flexibility in implementation
- Easy testing with mocks
- Clear contracts between components

### 3. **Separation of Concerns**
Clear boundaries between:
- UI rendering (contexts)
- Business logic (controllers)
- Data management (state)
- Cross-cutting concerns (helpers)

### 4. **Thread Safety**
All state mutations go through thread-safe state objects with proper locking.

### 5. **Event-Driven Updates**
UI updates are posted through PostUIUpdate to ensure thread safety with gocui.

## Benefits

1. **Testability**: Controllers and business logic can be tested without UI dependencies
2. **Extensibility**: Easy to add new views, controllers, or features
3. **Maintainability**: Clear structure makes code navigation and understanding easier
4. **Reusability**: Contexts, controllers, and helpers can be reused or composed
5. **Scalability**: Architecture supports growth without becoming unwieldy

## Adding New Features

To add a new feature:

1. **New View**: Create a new context in `context/`
2. **New Command**: Add to `SlashCommandHandler` in `controllers/command.go`
3. **New Business Logic**: Create a new controller in `controllers/`
4. **New State**: Extend state objects in `state/`
5. **New Utility**: Add to `helpers/`

## Migration from Old Architecture

The old monolithic TUI struct has been decomposed into:
- State extracted to `state/` package
- Rendering logic moved to `presentation/`
- Business logic moved to `controllers/`
- View management moved to `context/`
- Utilities moved to `helpers/`

This makes the codebase more modular, testable, and maintainable while preserving all functionality.

## New Layout System Features

### **Box Layout Engine**
- Hierarchical layout with ROW/COLUMN directions
- Weight-based proportional sizing
- Static size constraints
- Conditional layout functions based on screen size

### **Window Management**
- Separation of logical windows from physical views
- Context-window relationships
- Dynamic window properties and positioning

### **Screen Modes**
- **Normal**: Balanced layout with all panels visible
- **Half**: Focused panel gets more space, side panels may hide
- **Full**: Focused panel takes maximum space

### **Responsive Design**
- Automatic breakpoints: xs, sm, md, lg, xl
- Portrait mode detection and layout switching
- Adaptive panel sizing based on terminal dimensions
- Feature toggles for different screen sizes

### **New Commands**
- `/focus <panel>` - Switch focus to specific panel
- `/toggle` - Cycle through screen modes
- `/layout <mode>` - Set specific layout mode (normal/half/full)

### **Configuration-Driven Layout**
- User-customizable panel proportions
- Border style selection
- Portrait mode preferences
- Sidebar and compact mode toggles