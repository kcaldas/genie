# TUI Architecture

This document describes the architecture of the Genie TUI (Terminal User Interface) client.

## Overview

The TUI follows a clean, layered architecture that separates concerns between the user interface layer and the core Genie business logic. The TUI acts as a client of the Genie core services, maintaining its own state and event management while delegating AI operations to the core.

## Architecture Diagrams

### Component Dependencies

```mermaid
graph TD
    subgraph "TUI Layer"
        subgraph "UI Components"
            MC[MessagesComponent]
            IC[InputComponent] 
            SC[StatusComponent]
            DC[DebugComponent]
        end
        
        subgraph "Controllers"
            ChatCtrl[ChatController]
            StatusCtrl[StatusController]
        end
        
        subgraph "Commands"
            ClearCmd[ClearCommand]
            HelpCmd[HelpCommand]
            ConfigCmd[ConfigCommand]
            ThemeCmd[ThemeCommand]
        end
        
        subgraph "State Management"
            SA[StateAccessor]
            CS[ChatState]
            US[UIState]
        end
        
        subgraph "Events"
            CE[CommandEventBus]
        end
    end
    
    subgraph "Genie Core"
        G[Genie Services]
        GE[Core EventBus]
    end

    %% Component dependencies
    MC --> SA
    IC --> SA
    SC --> SA
    DC --> SA
    
    %% Controller dependencies
    ChatCtrl --> SA
    ChatCtrl --> G
    ChatCtrl --> GE
    StatusCtrl --> SA
    
    %% Command dependencies
    ClearCmd --> ChatCtrl
    HelpCmd --> SA
    ConfigCmd --> SA
    ThemeCmd --> SA
    
    %% State dependencies
    SA --> CS
    SA --> US
    
    %% Event flow
    MC -.->|events| CE
    IC -.->|events| CE
    SC -.->|events| CE
    DC -.->|events| CE
    
    CE -.->|events| ChatCtrl
    CE -.->|events| StatusCtrl
    
    %% Styling
    classDef component fill:#e1f5fe
    classDef controller fill:#f3e5f5
    classDef command fill:#e8f5e8
    classDef state fill:#fff3e0
    classDef genie fill:#ffebee
    
    class MC,IC,SC,DC component
    class ChatCtrl,StatusCtrl controller
    class ClearCmd,HelpCmd,ConfigCmd,ThemeCmd command
    class SA,CS,US state
    class G,GE genie
```

### Interaction Flow

```mermaid
sequenceDiagram
    participant U as User
    participant C as Component
    participant Ctrl as Controller
    participant S as State
    participant G as Genie Core
    participant Cmd as Command

    Note over C, G: Only Controllers communicate with Genie Core

    %% User interaction flow
    U->>C: User input/interaction
    C->>S: Read current state
    S-->>C: Return state data
    C->>C: Render UI from state

    %% Component to Controller via events (async)
    C->>Ctrl: Emit event (async)
    Note right of C: Components send events<br/>to Controllers for<br/>business logic

    %% Controller executes command
    Ctrl->>Cmd: Execute command
    Cmd-->>Ctrl: Command result

    %% Controller calls Genie (exclusive access)
    Ctrl->>G: Call Genie services
    Note right of Ctrl: EXCLUSIVE access<br/>to Genie Core
    G-->>Ctrl: Response/Events

    %% Controller updates state
    Ctrl->>S: Update state
    Note right of Ctrl: Controllers are the only<br/>ones that update state<br/>based on Genie responses

    %% State change triggers re-render
    S->>C: State changed
    C->>C: Re-render UI

    %% Async Genie events
    G->>Ctrl: Core events (chat.response, tool.executed)
    Note right of G: Genie publishes events<br/>only Controllers subscribe
    Ctrl->>S: Update state from events
    S->>C: Trigger re-render
```

## Core Components

### 1. Commands
- **Location**: `cmd/tui/controllers/commands/`
- **Purpose**: Handle user input commands (e.g., `:help`, `:clear`, `:config`)
- **Pattern**: Each command implements the Command interface with `Execute()` method
- **Examples**: `ClearCommand`, `HelpCommand`, `ConfigCommand`, `ThemeCommand`

### 2. Controllers
- **Location**: `cmd/tui/controllers/`
- **Purpose**: Central orchestration layer that coordinates between Commands, State, and Components
- **Key Controllers**:
  - `ChatController`: Manages chat interactions and message flow
  - `StatusController`: Updates status bar information
- **Responsibilities**:
  - Execute commands via the `OPTIONER` interface
  - Update state based on command results
  - Trigger component re-renders
  - **EXCLUSIVE** communication with Genie core services
  - Subscribe to core events and handle async responses

### 3. State
- **Location**: `cmd/tui/state/`
- **Purpose**: Manages TUI-specific application state
- **Key Components**:
  - `ChatState`: Messages, conversation history, loading state
  - `UIState`: Current view, theme, configuration
  - `StateAccessor`: Unified interface for state operations
- **Pattern**: Thread-safe state management with mutex protection

### 4. Components
- **Location**: `cmd/tui/component/`
- **Purpose**: UI rendering and view management
- **Key Components**:
  - `MessagesComponent`: Chat message display
  - `InputComponent`: User input handling
  - `StatusComponent`: Status bar display
  - `DebugComponent`: Debug information panel
- **Pattern**: Each component implements the `Component` interface with `Render()` method
- **Communication**: Send events to Controllers for business logic

### 5. Genie Core
- **Location**: `pkg/genie/`
- **Purpose**: Core AI functionality, session management, tool execution
- **Interface**: **ONLY Controllers** communicate with Genie through well-defined service interfaces
- **Responsibilities**:
  - AI chain execution
  - Session management
  - Tool calling and execution
  - Event publishing for async operations

## Data Flow

### ① Component Renders from State
1. Components read current state via StateAccessor
2. Components format and display state data
3. Components handle user interactions (input, scrolling)
4. Components emit events to Controllers for business logic

### ② Controller Updates State
1. Controllers receive events from Components (async)
2. Controllers execute commands and business logic
3. Controllers call Genie core services when needed
4. Controllers update TUI state based on results
5. State changes trigger component re-renders

### ③ State Reads from Controller Updates
1. Controllers subscribe to Genie core events (e.g., `chat.response`, `tool.executed`)
2. Controllers handle async responses from Genie
3. Controllers update TUI state
4. State changes trigger UI updates

## Event-Driven Architecture

The TUI uses two levels of event systems:

### Core Events (`pkg/events`)
- **Purpose**: Communication between Genie core and Controllers
- **Examples**: `chat.response`, `chat.started`, `tool.executed`
- **Pattern**: Async event publishing for long-running operations
- **Access**: **ONLY Controllers** subscribe to these events

### Command Events (`cmd/events`)
- **Purpose**: TUI-internal communication between Components and Controllers
- **Examples**: `theme.changed`, `status.update`
- **Pattern**: Async event handling for UI coordination
- **Flow**: Components → Controllers via events

## Key Design Principles

### 1. Separation of Concerns
- **TUI Layer**: User interface, input handling, display formatting
- **Core Layer**: AI logic, session management, tool execution
- **Clear Boundaries**: Only Controllers access Genie core services

### 2. Event-Driven Updates
- Components communicate with Controllers via events (async)
- Controllers react to both UI events and core events
- Avoids tight coupling between UI and business logic

### 3. State Management
- Centralized state with controlled access via StateAccessor
- Thread-safe operations with proper synchronization
- Clear state ownership and mutation patterns

### 4. Controller Pattern
- Controllers are the **exclusive gateway** to Genie core
- Controllers orchestrate complex workflows
- Commands focus on single responsibilities
- Clear separation between command execution and state management

## Testing Architecture

### TUI Testing (`cmd/tui/testing/`)
- **TUIDriver**: High-level test interface for UI interactions
- **MockChainRunner**: Mocks AI responses for predictable testing
- **Component Testing**: Individual component render and behavior tests
- **Integration Testing**: Full chat flow with mock expectations

### Test Pattern
```go
driver := NewTUIDriver(t)
driver.ExpectMessage("hello").RespondWith("Hi there!")
driver.Input().Type("hello").PressEnter()
// Verify expected behavior
```

**Note**: Special characters like `?` and `!` cause input system hangs - avoid in test messages.

## Directory Structure

```
cmd/tui/
├── ARCHITECTURE.md          # This document
├── app.go                   # Main TUI application
├── component/               # UI components
├── controllers/             # Business logic controllers
│   └── commands/           # Command implementations
├── helpers/                # Utility helpers
├── layout/                 # Layout management
├── presentation/           # Formatting and display logic
├── state/                  # State management
├── testing/                # Test infrastructure
└── types/                  # Type definitions
```

## Configuration

- **Settings**: `~/.genie/settings.tui.json`
- **History**: `.genie/history`
- **Themes**: Built-in theme system with user customization
- **Keybindings**: Configurable key mappings

## Future Considerations

1. **Remote Deployment**: Architecture supports future remote Genie core deployment
2. **Plugin System**: Controller pattern enables easy command extensibility
3. **Multiple Clients**: Clear separation allows multiple client implementations
4. **Real-time Collaboration**: Event system supports multi-user scenarios

---

*This architecture documentation reflects the current TUI implementation and design patterns established in the Genie project.*